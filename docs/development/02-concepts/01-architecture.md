# 架构总览

> 本章回答“框架由什么组成、如何协作”。具体怎么用见 03；配置键见 08。
> 深度版本（含 ADR 全文与债务清单）见 [架构设计说明](../../architecture.md)。

---

## 1. 系统上下文

```
调用方（浏览器/网关/其他服务）
        │ HTTP/JSON
        ▼
┌──────────────────────────────────────────────┐
│ MGin 进程                                     │
│  gin.Engine :port  ──┐                       │
│  gin.Engine :port_ssl ─┤ 同一份路由与中间件    │
│                      │                       │
│  插件：MySQL SQLite Mongo Redis Registry Job │
└───────┬──────────────┬───────────────┬───────┘
        │              │               │
   注册/配置        数据存储         观测后端
 Nacos/Consul     MySQL/Redis     Prometheus
 etcd/Polaris     Mongo/SQLite    VL/Kafka/Mongo
```

关键事实：

- **一个进程、一个 `gin.Engine`**（HTTPS 是第二个 `http.Server`，共用同一个 handler）。
- 不内置网关、Sidecar、消息总线；所有能力是**进程内库**。
- 组件以插件形式管理生命周期，顺序固定（见 §3）。

---

## 2. 逻辑分层

```
接入层   gin 路由 + 全局中间件（trace → postlog → cors → xlang → ratelimit → recovery）
         探针 /health/*、/metrics 最先注册，绕过业务中间件
业务层   controller → service → dao（目录名不强制，分层由你决定）
能力层   client / registry / loadbalancer / errcode / i18n / job / health
         metrics / otel / casbin / cache / logs / logsink / middleware / utils
资源层   db/{mysql,sqlite,mongo,redis} + dao + config
```

依赖规则：

| 规则 | 说明 |
|------|------|
| `pkg/*` 不反向依赖根包 | 根包只做装配；`pkg/health` 被根包调用但不 import 根包 |
| `pkg/*` 不依赖 `internal/*` | 当前 `internal/` 为空 |
| `cmd/` 是独立 module | 只依赖已发布版本，不 import 本地框架包 |
| 业务只依赖 `pkg/*` 与根包 | 不直接 import 具体数据库实现，便于替换与测试 |

---

## 3. 启动与关闭时序

### 3.1 启动

```
main()
 └─ mgin.NewApp(configFile, name, version, xlang)
     ├─ 解析 -f / -v
     ├─ 把非绝对 configFile 拼到 filepath.Dir(os.Args[0])
     ├─ health.SetVersion(version)
     ├─ mgin.Init(cfgFile)
     │   ├─ config.Init            ← koanf 读取 application.yml
     │   ├─ logs.InitVictoriaLogs  ← 校验 URL/证书，配置错误直接 panic
     │   ├─ registry.NewRegistry   ← 按 go.discovery.registry 创建
     │   ├─ plugin.RegisterBuiltins
     │   └─ plugin.InitAll         ← Order 升序，失败记录后继续
     ├─ 启动 5 分钟健康巡检 goroutine
     ├─ gin.SetMode(debug/release)
     ├─ i18n.Init（可选）
     └─ baseRouter()
         ├─ 挂载 /health/*（如启用）
         ├─ 挂载 /metrics（如 EnableMetrics）
         ├─ Use(trace, postlog, cors, [xlang], [ratelimit], recovery)
         ├─ NoRoute（404 + 错误码 + i18n）
         └─ 挂载 /i18n、/job（如启用）
 └─ app.Run()
     ├─ 启动 HTTP(:port) 与 HTTPS(:port_ssl)
     ├─ go.health.autoStarted=true 时 MarkStarted
     ├─ 等待 SIGINT/SIGHUP/SIGTERM/SIGQUIT
     └─ 优雅关闭
```

**探针必须在所有业务 `Use()` 之前注册**。Gin 在路由注册时快照中间件链，
后注册的 `Use` 不会回溯已注册路由；反过来，探针若注册在鉴权之后就会被 401 拦截。

### 3.2 关闭

```
SIGTERM
 ├─ 停止巡检 ticker
 ├─ postlog.Close()        ← 排空接口日志
 ├─ i18n.Close()
 ├─ plugin.CloseAll()      ← Order 降序：Job(90) → Registry(30) → Redis(15) → DB(10)
 │                          单插件 panic 被 recover，不影响其他插件
 └─ 每个 http.Server.Shutdown(独立超时 ctx)
```

超时来自 `go.application.shutdownTimeout`（秒，默认 5）。
`SafeExit` 内部另有固定 5s 上限，防止插件关闭无限阻塞。

---

## 4. 插件生命周期

```go
type Plugin interface {
    Name() string
    Order() int
    Init(ctx context.Context) error
    Close(ctx context.Context) error
    Health() error
    Enabled() bool
}
```

| 插件 | Order | 启用条件（`go.config.used`） | 初始化动作 |
|------|-------|------------------------------|-----------|
| mysql | 10 | 含 `mysql` | `db.Mysql.Init(prefix.mysql 子配置)` |
| sqlite | 10 | 含 `sqlite` | `db.Sqlite.Init(文件路径)` |
| mongodb | 10 | 含 `mongodb` | `db.Mongo.Init(子配置)` |
| redis | 15 | 含 `redis` | `db.Redis.Init(子配置)` |
| registry | 30 | 含 `nacos`/`etcd`/`consul` | 注册到对应注册中心 |
| job | 90 | 含 `job` | `job.Start()` |

语义细节：

- `Enabled()` 用 **子串匹配**（`strings.Contains`），`used` 必须是逗号分隔的精确名称；
- 某个插件 Init 失败**不会**阻断进程，只记录首个错误；
- `Health()` 只在 5 分钟巡检中调用，**不参与** `/health/ready`；
- Polaris 只参与发现，**不会自动注册**（registry 插件未列入 polaris）。

扩展方式见 [09 · 扩展点](../09-advanced/01-extension-points.md)。

---

## 5. 一次请求的完整流转

```
Client
  │
  ├─ trace.TraceId        透传/生成 traceparent、X-Trace-Id、X-Request-ID
  ├─ postlog              缓存请求/响应体，异步投递日志 Handler
  ├─ cors                 CORS 头
  ├─ xlang                X-Lang → 协程语言
  ├─ ratelimit            go.ratelimit.enabled 时按规则计数
  ├─ nice.Recovery        panic → HTTP 500 + 业务码 1001
  │
  ├─ 业务 handler
  │    └─ service
  │         ├─ dao（MySQL/Redis 缓存）
  │         └─ client.CallCtx
  │              ├─ registry.GetServices（本地缓存 + 回源）
  │              ├─ loadbalancer.Pick
  │              ├─ breaker.Allow/Failure/Success
  │              └─ grequests → 下游
  │
  └─ metrics.Middleware    统计计数/耗时（需业务显式挂载）
```

注意 `metrics.Middleware()` 不在框架默认链上，需要在 `main` 中
`app.Router.Use(metrics.Middleware())`，否则 `/metrics` 只有构建信息等静态指标。

---

## 6. 组件依赖关系

```
config ─────┬─> db ─> dao ─> cache
            ├─> registry ─> client ─> loadbalancer
            ├─> logs ─> logsink
            ├─> i18n ─> errcode
            ├─> health / metrics / otel
            ├─> job
            └─> middleware/*
```

- `config` 是所有组件的上游，任何包都不得在 `init()` 中读配置。
- `logs` 几乎被所有包依赖；因此日志初始化必须先于其他插件。
- `errcode` 只依赖 `models`，`i18n` 依赖 `errcode` + `models`，不反向。

---

## 7. 单进程模型的取舍

| 收益 | 代价 |
|------|------|
| 部署简单（一个二进制 + 一份配置） | 单实例容量上限受进程限制 |
| 调试直接（无跨进程上下文割裂） | 定时任务、限流是单机语义，多副本需额外设计 |
| 组件替换容易（接口 + 注册表） | 无自动故障转移的“进程外”能力 |
| 依赖少（不需要 Sidecar） | 无法做 Mesh 级 mTLS/流量镜像 |

如果团队需要跨语言、强契约、集群级治理，应把 MGin 限定在业务层，
把流量治理交给网关/Mesh，避免重复实现。

---

## 8. 延伸阅读

- 抽象与扩展点：[02 · 核心抽象](02-abstractions.md)
- 为什么这样设计：[03 · 设计决策](03-design-decisions.md)
- 源码主线：[09 · 源码走读](../09-advanced/02-source-walkthrough.md)
