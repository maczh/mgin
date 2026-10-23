# 可观测性 · 部署 · 测试（v1 jh 分支）

> 依据：`docs/development/06-observability/*`、`07-deployment/*`、`03-development/05-testing.md`、
> `10-contributing/03-testing-strategy.md`，并与 `app.go` / `health/` / `metrics/` / `logsink/` 源码核对。

---

## 1. 健康探针（`health` 包）

| 项 | 说明 |
|---|---|
| 启用方式 | ① `go.health.enabled: true`（在 `baseRouter` **最前面**挂载，早于 trace/postlog/cors/ratelimit，因此不受 jwt/casbin 拦截）；② `app.EnableHealth()`（`NewApp` 之后调用，会带上已注册的默认中间件链） |
| 端点 | `/healthz`（根别名）、`/health/live`、`/health/ready`、`/health/startup` |
| startup | 默认 503；调用 `health.MarkStarted()` 或配 `go.health.autoStarted: true`（`Run()` 监听后自动标记）后转 200 |
| ready | **实时**检查 MySQL / MongoDB / Redis；SQLite 不参与；`db.Redis.Check()` 恒返回 nil，Redis 故障**不会**体现 |
| 手动检查 | `health.CheckDependencies() (bool, map[string]string)`；`SetVersion/ResetStarted/IsStarted/StartedAt/Router` |

```go
app := mgin.NewApp("conf/application.yml", "订单服务", Version, false)
app.EnableHealth()          // 或只写 go.health.enabled: true
// ... 业务初始化（DB、缓存预热）...
health.MarkStarted()        // K8s startup 探针放行
```

> 探针永远不要挂到需要鉴权的路由组下；需要「绝对干净」的调用链时只用配置开关。

---

## 2. 指标（`metrics` 包）

```yaml
go:
  metrics:
    enabled: true
```

```go
app.EnableMetrics()                  // 等价写法
app.Router.Use(metrics.Middleware()) // ★ HTTP 指标必须手动挂，否则只有 build/plugin/dependency 指标
metrics.SetBuildInfo(Version, GitHash, runtime.Version())
metrics.SetPluginHealth("kafka", kafka.Kafka.Check() == nil)
metrics.SetDependencyUp("mysql", ok)
```

端点固定 `GET /metrics`，在默认中间件链之前注册（Prometheus 抓取不会被 401）。

| 指标 | 类型 | 标签 |
|---|---|---|
| `mgin_http_requests_total` | counter | `method,path,status`（未匹配路由 `path="<nomatch>"`） |
| `mgin_http_request_duration_seconds` | histogram | `method,path` |
| `mgin_http_requests_in_flight` | gauge | — |
| `mgin_build_info` | gauge | `version,commit,go_version` |
| `mgin_plugin_health` | gauge | `name` |
| `mgin_dependency_up` | gauge | `name` |

业务指标请加自己的前缀，避免与 `mgin_*` 冲突。本分支**无 OpenTelemetry**。

---

## 3. 追踪（`middleware/trace`）

**不是分布式追踪**，只做请求标识透传，header 契约：

| Header | 来源 |
|---|---|
| `X-Request-ID` | 优先沿用上游，否则生成 16 位 hex |
| `X-Real-IP` | `X-Forwarded-For` 首个 → `X-Real-IP` → `ClientIP` |
| `X-User-Agent` | `User-Agent` / `X-User-Agent` |

存储方式：gin context + **以 goroutine ID 为 key 的内存缓存（5 分钟）**，不是 `context.Context`，也没有 W3C `traceparent`。

```go
// 异步协程必须恢复上下文，参数是「父 goroutine 的 ID」，不是 *gin.Context
go func(preID uint64) {
    trace.CopyPreHeaderToCurRoutine(preID)
    logs.Info("异步任务")            // 带上同一个 request_id
}(trace.GetGoroutineID())

trace.GetRequestId()   // 无参
trace.GetClientIp() / GetUserAgent()
trace.GetHeader(k) / SetHeader(k, v) / GetHeaders()
```

`client.Call` 会自动带这三个头；只往 gin context 里塞自定义头不会出网，要用 `client.Options.Header`。

---

## 4. 日志

三类相互独立：

| 类型 | 产生方 | 落点 | 开关 |
|---|---|---|---|
| 运行日志 | `logs.Debug/Info/Warn/Error` | console / file / VictoriaLogs / 自定义 handler | `go.logger.level`、`go.logger.outputs` |
| 接口访问日志 | `postlog.RequestLogger()` | MongoDB（**唯一内置落库**）或自定义 `logsink` handler | `go.log.req` + `used` 含 `mongodb` |
| 调用日志 | `client.Call` | VictoriaLogs | `go.log.victorialogs.client: true` |

```yaml
go:
  logger:
    level: info
    outputs:
      - { target: console, format: text, logtext: "[{time}] [{level}] [{request_id}] {message}" }
      - { target: file, file: ./logs/order, format: json }
      - { target: victorialogs, format: json, fields: [time, level, app, request_id, message] }
  log:
    get: line                       # full|line|off
    req: orderRequestLog            # 非空 + used 含 mongodb 才落库
    sink: { queue: 100, workers: 1, shutdown: 3000 }
    victorialogs: { url: "https://host:37624", api_token: "", request: true, client: true, timeout: 5000 }
```

要点：

- 占位符只有 `{}`，顺序替换；级别由 `go.logger.level` 控制（`go.application.debug: true` 时强制 debug）。
- `outputs` 非空即替代旧式 `out`/`file`；`fields` 白名单：`time, level, message, app, request_id, goroutine_id, package, file, function, line`。
- VictoriaLogs 地址 = `go.log.victorialogs.url` + `/v1/logs`；**配置错误在启动时 panic**；TLS 严格校验、不可跳过；默认 5s 超时、503 重试 3 次。
- `logsink` 队列：每个 handler **1 个消费协程**（`workers` 被强制改回 1），`QueueSize` 硬上限 `DefaultQueueSize = 1024`（`<=0` 或 `>1024` 都回落 1024；`WithQueueSize` 上的注释「最大 100」是过期注释），满则**淘汰最旧**并计数。用 `logsink.Stats()` 看 `Dropped`。
- handler 的 `Write` 必须快，且**不能在里层再调 `logs.*`**（会递归）。

```go
// 接口日志通道（Entry = models.PostLog）
logsink.MustRegister(logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
    return mgkafka.Kafka.Send("app-log", utils.ToJSON(e))
}).WithClose(mgkafka.Kafka.Close))

// 运行日志通道（Record，VictoriaLogs 走这条）
logsink.MustRegisterRecord(logsink.NewRecordHandlerFunc("mylog", func(r *logsink.Record) error {
    return nil
}), logsink.WithQueueSize(256))

// 只给某个 handler 发
logsink.EmitRecordTo(logs.VictoriaLogsClientTarget, &logsink.Record{Time: ..., Level: "info", Message: ...})
```

---

## 5. 配置热重载

```bash
curl -X PUT http://127.0.0.1:8080/cfg/reload \
  -H "X-MGin-Reload-Token: <token>"      # 或 Authorization: Bearer <token>
```

- 端点只在 `go.config.reload.token` 非空时注册。
- 只接受 **loopback / 私网 / 链路本地** 来源（看 `RemoteAddr`，不信任 `X-Forwarded-For`），token 用等长常量时间比较；失败 401/403，不泄露内容。
- 生效：重读主/子配置、重连 MySQL/SQLite/Mongo/Redis、重新注册、重启 job。
- **不生效**：Gin 路由与中间件、监听端口、TLS 证书、`App.name/version/i18n`——这些只能重启。
- 新配置解析失败会保留旧连接；解析成功但组件连不上，则旧连接已关、组件不可用，需要修复后重启。

---

## 6. 部署

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X main.Version=v1.0.0 -X main.BuildTime=$(date -u +%FT%TZ) -X main.GitHash=$(git rev-parse --short HEAD)" \
  -o app .
```

- SQLite 用纯 Go 驱动（glebarez/sqlite），可静态编译；部署单元 = 二进制 + `conf/`。
- 容器基线：`gcr.io/distroless/static-debian12:nonroot`，`WORKDIR /app`，`ENTRYPOINT ["/app/app","-f","/app/conf/application.yml"]`（distroless 无 shell，配置必须绝对路径）。
- 探针：startup → `/health/startup`；readiness → `/health/ready`；liveness → `/health/live`。加 `preStop: sleep 5` 先摘流。
- 优雅关闭：`SIGTERM` → `SafeExit()`（排空 postlog → 关闭 i18n/数据源 → 注册中心反注册 → 停 job → 关插件）→ `server.Shutdown(5s)`。**HTTP 排空硬超时 5 秒**，`terminationGracePeriodSeconds` ≥ 20。
- 注意：`SIGHUP` 也会**退出进程**，不是 reload；reload 只能用上面的 HTTP 端点。

---

## 7. 测试

```bash
go test ./...
go test -race ./internal/...
go test -run TestGetUser -v ./controller/
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

免外部依赖、可直接跑的包：

```bash
go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ ./client/ \
        ./middleware/cors/ ./middleware/limit/ ./middleware/ratelimit/
```

需要依赖：`./i18n/ ./job/`（要 SQLite）、`./db/... ./config/`（要 MySQL/Redis/Mongo）、`./registry/...`（要可达注册中心）。

已知约束：

- 本仓库**无 CI、无框架级 Makefile**，`go test ./...` 不能作为唯一门禁。
- `go vet ./...` 有历史告警（`middleware/xss`、`utils/rsautil.go` unreachable、`i18n/store_test.go` copylocks）。
- 框架有全局单例（`config.Config`、`db.*`、`registry.Registry`）：测试要串行（`-p 1`）或先 reset；缓存用例必须给非 0 TTL，否则互相污染。
- 断言看响应体 `status`/`code`，不要看 HTTP 状态码（恒 200）。

---

## 8. 故障速查（R1–R12 摘要）

| 症状 | 处置 |
|---|---|
| 起不来/立即退出 | 路径改绝对路径；YAML 禁 Tab；i18n、VictoriaLogs 配置错会 panic；端口占用换端口 |
| `/health/ready` 503 | 看 `dependencies` 里的 `mysql`/`mongodb`；SQLite 不参与、Redis 恒成功 |
| 调用失败 | 「服务注册中心未初始化」→ 配 `used` + `go.discovery.registry`；「获取…主机IP端口失败」→ 服务名/group/实例 |
| 探针 401/403 | 探针被挂进受保护路由组 → 改用 `go.health.enabled` 让框架最前注册 |
| `/metrics` 404 或无 HTTP 指标 | 开关需重启；HTTP 指标要手动 `metrics.Middleware()` |
| 接口日志不落库 | `go.log.req` 非空、`used` 含 `mongodb`、多库要 `go.log.dbName` |
| 重载失败 | 401 token 不符；403 来源非内网；500 新配置重建失败（连接可能已断，需重启） |
| 定时任务不跑 | `go.job.enabled`、库是否可用、`/job/handlers` 是否注册、任务 status、cron 表达式 |
| 日志丢字段 | `sink.queue` 太小、异步没调 `trace.CopyPreHeaderToCurRoutine`、`outputs[].fields` 白名单漏字段 |
