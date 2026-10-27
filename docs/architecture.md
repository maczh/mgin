# MGin v2 系统架构设计说明

| 项 | 内容 |
|----|------|
| 文档版本 | v2.0 |
| 对应代码 | `github.com/maczh/mgin/v2`（Go 1.25.7） |
| 读者 | 框架维护者、服务负责人、需要排查框架行为的开发者 |
| 配套 | [需求文档](requirements.md) · [模块与接口](modules-and-api.md) · [数据设计](data-design.md) |

---

## 1. 架构概览

### 1.1 系统上下文

```
        ┌────────────┐        HTTP/JSON         ┌──────────────────────┐
        │  调用方     │ ───────────────────────► │  mgin 应用进程        │
        │ (前端/网关) │ ◄─────────────────────── │  gin.Engine :port     │
        └────────────┘                          │  gin.Engine :port_ssl │
                                                └───────┬──────────────┘
        ┌────────────┐   注册/发现/配置                  │
        │ Nacos      │ ◄────────────────────────────────┤
        │ Consul     │                                   │
        │ etcd       │                                   │
        │ Polaris    │                                   │
        └────────────┘                                   │
        ┌────────────┐                                   │
        │ MySQL      │ ◄────────────────────────────────┤
        │ SQLite     │                                   │
        │ MongoDB    │                                   │
        │ Redis      │                                   │
        └────────────┘                                   │
        ┌────────────┐                                   │
        │ Prometheus │ ◄── GET /metrics ─────────────────┤
        │ K8s Probe  │ ◄── GET /health/* ────────────────┤
        │ 日志采集    │ ◄── VictoriaLogs / Kafka / Mongo ─┘
        └────────────┘
```

### 1.2 架构风格

- **单体进程 + 组件化**：不是每个能力一个 sidecar，而是进程内库。
- **端口-适配器（局部）**：`Plugin`、`LoadBalancer`、`RegistryClient`、`ICache` 是端口；
  各中间件/数据库/注册中心是适配器。
- **约定优于配置**：脚手架按约定生成目录与配置；框架按约定读取 `go.*` 键。
- **显式优先**：生命周期、中间件顺序、错误码状态都由代码显式声明，不依赖隐式发现。

### 1.3 质量属性与实现手段

| 质量属性 | 手段 | 代价/限制 |
|----------|------|-----------|
| 可扩展 | Plugin/LB/Registry/Cache 四组接口 + 注册表 | `Dao[E]` 接口未对齐，扩展点不完整 |
| 向后兼容 | 保留 v1 签名；`logs/` 别名壳 | 顶层包与 `pkg/` 并存一段时间 |
| 可运维 | 三探针、`/metrics`、结构化日志、优雅关闭 | 指标 Gauge 需业务手动写入 |
| 容错 | 熔断、重试、超时、依赖检查 | 超时不可取消底层 HTTP |
| 低依赖 | 熔断器纯标准库；OTel 只依赖抽象包 | 无自动插桩 |
| 启动健壮 | 插件失败不中断、panic 隔离 | 部分组件"假健康"（Redis/SQLite Check 恒 nil） |

---

## 2. 逻辑分层

```
┌──────────────────────────────────────────────────────────────────┐
│ 接入层  gin.Engine                                               │
│  /health/*  /healthz  /metrics  /i18n/*  /job/*  业务路由         │
│  中间件链：trace → postlog → cors → xlang → ratelimit → recovery  │
└───────────────────────────────┬──────────────────────────────────┘
                                │ *gin.Context
┌───────────────────────────────▼──────────────────────────────────┐
│ 业务层  controller / service（业务代码，框架不约束目录名）          │
└───────────────────────────────┬──────────────────────────────────┘
                                │
┌───────────────────────────────▼──────────────────────────────────┐
│ 能力层（pkg/）                                                    │
│  client  registry  loadbalancer  errcode  i18n  job  health       │
│  metrics  otel  casbin  cache  logs  logsink  middleware  utils   │
└───────────────────────────────┬──────────────────────────────────┘
                                │
┌───────────────────────────────▼──────────────────────────────────┐
│ 资源层  db/{mysql,sqlite,mongo,redis} + dao  + config            │
└──────────────────────────────────────────────────────────────────┘
```

**依赖规则**：

1. `pkg/*` 之间允许依赖，但**禁止**反向依赖根包（`mgin.go`/`app.go`）。
   例外：`pkg/health` 被根包调用，但 health 不 import 根包。
2. `pkg/*` 不得 import `internal/*`（当前为空）。
3. `cmd/` 是独立 module，只能依赖已发布版本，不 import 本地框架包。
4. 业务代码只依赖 `pkg/*` 与根包，不直接依赖 `db/mysql` 等具体实现（推荐）。

---

## 3. 运行时视图

### 3.1 启动时序（精确到函数）

```
main()
 └─ mgin.NewApp(cfgFile, appName, version, xlang)
     ├─ flag: -f / -v
     ├─ health.SetVersion(version)
     ├─ mgin.Init(cfgFile)
     │   ├─ config.Config.Init(cfgFile)            // koanf 解析 application.yml
     │   ├─ logs.InitVictoriaLogs()                // 校验 URL/证书，失败 panic
     │   ├─ registry.Registry = registry.NewRegistry()
     │   ├─ Runtime{StartedAt, Pid, GoVersion}
     │   ├─ plugin.RegisterBuiltins()
     │   │   └─ Register(mysql, sqlite, mongodb, redis, registry, job)
     │   └─ plugin.InitAll(ctx)
     │       ├─ sortedByOrder(): 10,10,10,15,30,90
     │       ├─ 跳过 Enabled()==false
     │       ├─ 逐个 Init，失败记录首个 error 并继续
     │       └─ 日志：即使 Init 失败也会打印"初始化成功"（已知缺陷）
     ├─ 启动 5 分钟巡检 goroutine (checkAll)
     ├─ gin.SetMode(debug/release)
     ├─ i18n.Init()（xlang 或 go.i18n.enabled）
     └─ baseRouter()
         ├─ mountHealth()      // 条件：EnableHealth() 或 go.health.enabled
         ├─ /metrics           // 条件：metricsEnabled && go.metrics.enabled
         ├─ Use(trace, postlog, cors, [xlang], [ratelimit], recovery)
         ├─ NoRoute（404 + errcode + i18n）
         ├─ [i18n.RouterGroup("/i18n")]
         └─ [job.RouterGroup("/job")]
 └─ app.Run()
     ├─ 构造 server :port 与 serverSsl :port_ssl
     ├─ 按需启动监听 goroutine
     ├─ go.health.autoStarted → MarkStarted()
     ├─ 等待信号
     └─ SafeExit + 逐个 Shutdown
```

### 3.2 关闭时序

```
SIGTERM
 ├─ 停 checkTicker，close(checkDone)
 ├─ SafeExit()
 │   ├─ postlog.Close()       // 排空接口日志队列
 │   ├─ i18n.Close()
 │   ├─ plugin.CloseAll(ctx)  // 逆序：90 → 30 → 15 → 10；单插件 panic 被 recover
 │   └─ 旧 m.plugins 逐个 Close
 └─ for s in servers: s.Shutdown(ctx with shutdownTimeout)
     └─ 每个 server 独立 ctx，互不挤占
```

**设计要点**：

- `signalChan` 有 1 个缓冲，避免信号在阻塞前被丢弃。
- HTTPS server 也被收集，修复了 v1 "HTTPS 永不关闭"的问题。
- `SafeExit` 内部固定 5s 上限（与 `shutdownTimeout` 独立），避免插件关闭无限阻塞。

### 3.3 插件生命周期

```
Register(p)                 // 同名覆盖；order 只在首次记录
   │
InitAll(ctx)  ── Order 升序 ──► Init(ctx)
   │                              ├─ 成功 → 继续
   │                              └─ 失败 → 记录首个 error，继续
HealthAll()                    // map[name]error，供 5 分钟巡检
CloseAll(ctx) ── Order 降序 ──► Close(ctx)（defer recover）
```

`Plugin` 接口（`pkg/plugin/plugin.go`）：

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

内置插件与 `Enabled()` 判定：

| 插件 | Order | 启用条件 | Init 动作 |
|------|-------|----------|-----------|
| mysql | 10 | `used` 含 `mysql` | `db.Mysql.Init(prefix.mysql 子配置)` |
| sqlite | 10 | `used` 含 `sqlite` | `db.Sqlite.Init(prefix.sqlite 值=文件路径)` |
| mongodb | 10 | `used` 含 `mongodb` | `db.Mongo.Init(子配置)` |
| redis | 15 | `used` 含 `redis` | `db.Redis.Init(子配置)` |
| registry | 30 | `used` 含 nacos/etcd/consul | 对应后端 `Register` |
| job | 90 | `used` 含 `job` | `job.Start()` |

> 注意：`Enabled()` 使用 `strings.Contains`，不是精确匹配；`polaris` 未被 registry 插件识别。

### 3.4 请求生命周期

```
Client ──► trace.TraceId
             ├─ 透传/生成 traceparent
             ├─ X-Trace-Id / X-Request-ID / X-Real-IP
             └─ 写入协程局部 Header 缓存（TTL 5min）
          postlog.RequestLogger
             ├─ 缓存请求/响应体（受 go.log.get 控制）
             └─ 投递 logsink（非阻塞，队列满淘汰最旧）
          cors.Cors
          xlang.RequestLanguage（i18n 开启时）
          ratelimit.RateLimit（go.ratelimit.enabled）
          nice.Recovery
          业务 handler
             └─ service → client.CallCtx → registry.GetServices → LB Pick → grequests
          metrics.Middleware（业务挂载时统计）
```

---

## 4. 配置子系统

### 4.1 配置来源与优先级

```
application.yml（本地，必需）
   │ 提供 go.config.server/server_type/prefix/env
   ▼
远程配置中心（可选）  nacos | consul | etcd | polaris | springconfig | file
   │ 文件命名：{prefix}-{env}.yml
   ▼
GetConfigData(prefix) → []byte → 各组件 Init
```

- 本地文件缺失：`Cnf = nil`，`GetConfig*` 返回零值，进程继续（随后组件启用但无配置）。
- 远程文件缺失：`GetConfigData` 返回 `nil`，插件 Init 可能报错但不中断。

### 4.2 命名空间划分

| 命名空间 | 归属 | 典型键 |
|----------|------|--------|
| `go.application.*` | 业务/进程 | name, port, debug, shutdownTimeout |
| `go.config.*` | 数据源编排 | used, env, server_type, prefix.* |
| `go.logger.*` / `go.log.*` | 日志 | level, outputs, sink, victorialogs |
| `go.discovery.*` | 服务发现 | registry, callType, cacheTTL |
| `go.jwt` / `go.casbin` / `go.sys` | 安全 | secret, model_file, enabled |
| `go.health` / `go.metrics` / `go.ratelimit` / `go.job` / `go.i18n` | 能力开关 | enabled 等 |
| `go.framework.*` | 框架元配置 | 仅 i18nEnabled 实际生效 |
| `go.runtime.*` | 运行期元数据 | 只读，不来自 YAML |

### 4.3 设计权衡

- **保持 v1 键名**：避免存量项目改配置；代价是 `go.health` 与 `go.framework.healthEnabled`
  语义重叠，且后者未接线。
- **远程配置按文件粒度**：一个组件一个 `-env.yml`，便于按组件授权；代价是配置项多时文件数增长。
- **koanf + `.` 分隔**：路径直观；代价是键名大小写敏感，`shutdownTimeout` 不能写成 `shutdown-timeout`。

---

## 5. 服务发现与调用链

命名多注册中心由 `go.discovery.registries` 配置。每个条目包含唯一 `name`、
后端 `type`、基础配置 `prefix`，并可覆盖 `namespace/group/clusterName`。
相同 `prefix` 可复用同一个物理集群连接参数；`client.Options.Registry` 同时控制
`GetServices` 负载均衡主路径与 `GetServiceURL` 兼容路径。各命名实例的发现缓存
和生命周期相互独立。

Nacos 配置中心与注册中心支持 v1/v2/v3 Client OpenAPI。v3 使用
`/nacos/v3/client/...`，实例续约通过注册接口的 `heartBeat=true` 完成。

### 5.1 注册

```
plugin.Registry.Init
 ├─ used 含 nacos  → registry.Registry.Register(prefix.nacos 子配置)
 ├─ used 含 etcd   → ...
 └─ used 含 consul → ...
```

注册中心实现差异：

| 后端 | 注册机制 | 健康判定 | 多实例发现 |
|------|----------|----------|-----------|
| Nacos | v1 HTTP OpenAPI，心跳失败 3 次重注册 | `/list` 的 Healthy/Enabled | ✅ 过滤不健康 |
| etcd | 租约（10000s）+ keepAlive | 无 | ✅ prefix 扫描 |
| Consul | Agent 注册 + TTL 续活（30s） | `Health().Service(passingOnly)` | ✅ |
| Polaris | 北向 REST（建服务 + 注册实例） | 无过滤 | ✅ 但**不自动注册** |

### 5.2 发现缓存

```
GetServices(service, group)
 ├─ 查内存 entries[key]，未过期 → 直接返回
 ├─ 过期 → 回源（inFlight 抑制并发回源）
 ├─ 回源失败 → 返回旧值（降级）
 └─ 后台 refresh goroutine 预热
```

key = `service + "\x00" + join(groups, ",")`；TTL/Refresh 来自 `go.discovery.cacheTTL/cacheRefresh`
（配置层默认 60/30 秒）。

### 5.3 调用链

```
client.CallCtx(ctx, service, uri, op)
 ├─ 复制 op（防并发复用竞态）
 ├─ otel span（启用时）
 ├─ headers = trace.GetHeaders() + op.Header
 ├─ selectHostByLB
 │   ├─ registry.GetServices → 空则回落 GetServiceURL
 │   ├─ lb = Default() 或 Get(op.LoadBalancer)
 │   ├─ breaker = GetBreaker(service+"@"+host)
 │   └─ 熔断实例剔除重选；全熔断 → ErrAllInstancesCircuitOpen（不回退）
 ├─ grequests.DoRegularRequest / Post
 └─ 可选 op.Retry（递归一次）
```

`CallResilientCtx` 在外层再叠加：超时（goroutine+timer）→ 重试（指数退避 + jitter，
仅幂等方法）→ 熔断结果记账（每次整体调用记一次）。

---

## 6. 可观测性架构

### 6.1 三条链路

| 链路 | 数据 | 采集点 | 出口 |
|------|------|--------|------|
| 探针 | 依赖是否可用 | `/health/ready` 实时 Check | K8s |
| 指标 | 请求量/耗时/并发/构建信息 | `metrics.Middleware` | Prometheus |
| 日志 | 访问日志 + 运行日志 | `postlog` + `logs` | console/file/Mongo/Kafka/VictoriaLogs |

### 6.2 指标定义

| 指标 | 类型 | label | 写入方 |
|------|------|-------|--------|
| `mgin_http_requests_total` | CounterVec | method,path,status | Middleware |
| `mgin_http_request_duration_seconds` | HistogramVec | method,path | Middleware |
| `mgin_http_requests_in_flight` | Gauge | — | Middleware |
| `mgin_build_info` | GaugeVec | version,commit,go_version | `SetBuildInfo` |
| `mgin_plugin_health` | GaugeVec | name | `SetPluginHealth`（需业务调用） |
| `mgin_dependency_up` | GaugeVec | name | `SetDependencyUp`（需业务调用） |

`path` 使用 `c.FullPath()`（路由模板），未匹配路由为 `<nomatch>`，避免高基数。

### 6.3 日志架构

```
logs.Info("...{}", v)
 ├─ 级别判定：go.logger.level（实时读），debug 强制 debug
 ├─ 抓取调用栈（剔除 pkg/logs 帧）
 ├─ 输出目标：outputs[]（target/format/fields/logtext）
 │   ├─ console → color.Output
 │   ├─ file    → <file>.YYYY-MM-DD.log
 │   └─ 其他    → logsink.EmitRecordTo(target)
 └─ 队列：每个 handler 独立 channel，满则淘汰最旧（计数）
```

`postlog` 独立走 `logsink.Handler`（默认 Mongo），退出时先 Close 日志再关数据库。

---

## 7. 安全架构

| 层 | 机制 | 默认 |
|----|------|------|
| 传输 | HTTPS（`cert`/`key` 非空时同启） | 关闭 |
| 认证 | JWT（`Authorization: Bearer`） | 业务挂载 |
| 授权 | Casbin `sub,obj,act` + `keyMatch2` | `go.casbin.enabled` |
| 会话 | go-session | 业务挂载 |
| 输入 | bluemonday XSS 净化 | 业务挂载 |
| 流量 | 限流（三算法）、并发信号量、IP 白名单 | 限流需 `go.ratelimit.enabled` |
| 跨域 | CORS（`*` + `Allow-Credentials:false`） | 恒挂载 |

**注意**：`casbin.conf` 的 matcher 是
`r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act`，
`g` 定义了但未参与匹配，因此是"角色 ID 直连路径"的平铺 ACL，不是 RBAC 继承。

---

## 8. 关键设计决策（ADR 摘要）

### ADR-01 用 Plugin 接口替代 if 链

- **背景**：v1 在 `mgin.Init/SafeExit/checkAll` 中硬编码十几次 `strings.Contains`。
- **决策**：定义 `Plugin{Name,Order,Init,Close,Health,Enabled}` + 包级注册表。
- **结果**：新增组件只需 `plugin.Register`；代价是注册表是全局单例，测试需注意隔离。

### ADR-02 探针/指标端点最先注册

- **背景**：Gin 在路由注册时快照中间件链，后注册的中间件不回溯。
- **决策**：`baseRouter` 中先挂 `/health/*`、`/metrics`，再 `Use`。
- **结果**：探针不会被 JWT/Casbin 拦截；代价是这两个端点无法使用业务中间件（如需限流需自行处理）。

### ADR-03 错误码三向映射

- **背景**：v1 所有错误返 HTTP 200，网关无法据状态码路由/重试。
- **决策**：`Definition{Code,HTTPStatus,MessageKey,Module}`；NoRoute 404、Recovery 500。
- **结果**：语义清晰；代价是业务 handler 必须显式使用 `def.HTTPStatus`，否则仍返 200。

### ADR-04 负载均衡放在 client 内部

- **背景**：v1 `GetServiceURL` 只返单实例。
- **决策**：新增 `GetServices` + `pkg/loadbalancer`；`client` 选点后再走 v1 回退。
- **结果**：零破坏；代价是策略名/回退逻辑分散，短名会静默回落。

### ADR-05 OTel 只做抽象

- **背景**：强制引入 SDK/Exporter 会给不使用方增加依赖与初始化负担。
- **决策**：`pkg/otel` 仅依赖 `otel` 与 `otel/trace`，`SetTracerProvider` 由业务调用。
- **结果**：依赖轻；代价是无自动插桩，需业务自行埋点。

### ADR-06 超时用 goroutine 兜底

- **背景**：`grequests` 当前版本不支持可取消的 context。
- **决策**：`CallResilientCtx` 用 goroutine + timer 返回，调用方不再阻塞。
- **结果**：调用方体验改善；代价是底层连接/协程可能残留（代码注释已声明）。

### ADR-07 兼容壳保留一个 release

- **背景**：v2 把对外包从仓库顶层收拢到 `pkg/`，使用顶层路径（`.../v2/client` 等）的代码 import 会断。
- **决策**：`logs/` 保留 re-export 别名壳（`.../v2/logs`）；其余路径通过迁移文档 + sed 脚本处理。
- **结果**：v2 早期代码升级成本可控；代价是两套路径并存，需在下一个 release 删除。
  从 v1（无 `/v2`）升级必须同时修改 module 路径，别名壳不能替代该步骤。

---

## 9. 部署视图

### 9.1 单实例

```
容器
 ├─ /app/<binary>
 ├─ /app/conf/application.yml
 ├─ /app/conf/mysql-test.yml ...
 ├─ /app/casbin.conf
 └─ /app/logs/
ENV: 无必需变量（配置全在 YAML）
CMD: ["./app"]
```

### 9.2 K8s 探针建议

```yaml
livenessProbe:
  httpGet: { path: /health/live, port: 8080 }
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet: { path: /health/ready, port: 8080 }
  periodSeconds: 5
  timeoutSeconds: 3
startupProbe:
  httpGet: { path: /health/startup, port: 8080 }
  failureThreshold: 30
  periodSeconds: 2
```

`/health/ready` 会实时 `Check()` 数据库（可能触发重连），`timeoutSeconds` 需大于最慢一次检查。

### 9.3 多副本约束

| 能力 | 多副本行为 | 建议 |
|------|-----------|------|
| 服务注册 | 每个副本各自注册（需唯一实例 ID） | 确认注册中心实例 ID 含 IP/端口 |
| 定时任务 | 每个开启 `go.job.enabled` 的副本都会调度 | 仅一个副本开启 |
| 限流 | 每副本独立计数 | 集群限流需外部组件 |
| 服务发现缓存 | 各副本独立 | 可接受 |

---

## 10. 扩展点与扩展方式

### 10.1 新增一个数据源插件

```go
// pkg/plugin/builtins.go 同风格
func MyStore() Plugin {
    return &funcPlugin{
        name: "mystore", order: 20,
        enabledFn: func() bool { return enabled("mystore") },
        initFn: func(ctx context.Context) error { return mystore.Init(getCfg("...")) },
        closeFn: func(ctx context.Context) error { return mystore.Close() },
        healthFn: func() error { return mystore.Check() },
    }
}
```

注册后无需改 `mgin.Init`。注意 `Order` 要与依赖顺序一致。

### 10.2 新增负载均衡策略

实现 `Pick(instances []string, key string) (string, error)`，在 `init()` 中
`loadbalancer.Register("my_strategy", &MyLB{})`，调用时 `Options.LoadBalancer = "my_strategy"`。

### 10.3 新增中间件

1. 放在 `pkg/middleware/<name>/`，导出 `func Xxx() gin.HandlerFunc`。
2. 若必须全局生效，在 `app.baseRouter()` 中 `Use`，**位置必须在探针注册之后**；
   若会拦截请求（鉴权/限流），不要放在探针之前。
3. 读取配置用 `config.Config.GetConfigBool/String`，不要新增全局变量。

### 10.4 新增错误码

```go
const MY_ERROR = 2001
func init() {
    errcode.RegisterDef(errcode.New("mymodule", MY_ERROR, 422, "errcode.my_error"))
}
```

同时在 i18n 表插入 `errcode.my_error`。

---

## 11. 已知架构债务

| 编号 | 债务 | 影响 | 优先级 |
|------|------|------|--------|
| D-01 | `Dao[E]` 接口与实现不匹配、提及不存在的 postgres/clickhouse 实现 | 接口无法作为抽象使用 | 高 |
| D-02 | `go.framework.*` 多数字段未接线 | 配置无效且无提示 | 高 |
| D-03 | 负载均衡短名/配置键无消费方 | 策略与预期不符 | 高 |
| D-04 | `enabled()` 子串匹配 | 组件误启用 | 中 |
| D-05 | MySQL 多库分支提前 return | 池参数失效 | 中 |
| D-06 | Redis/SQLite `Check()` 恒 nil | 探针假健康 | 中 |
| D-07 | `client.CallCtx` 不可取消 | 连接残留 | 中 |
| D-08 | `logs/` 兼容壳未删除 | 双路径维护 | 低 |
| D-09 | 无 CI、无根 Makefile | 质量门禁靠人工 | 中 |
| D-10 | sys 模型无迁移 | 接入成本高 | 中 |
| D-11 | 定时任务/限流单机语义 | 多副本失效 | 中（需文档明确） |
| D-12 | Polaris 不自动注册 | 能力不完整 | 中 |
| D-13 | `go vet` 有 3 处 `unreachable code`、14 个文件未 `gofmt` | 无法把全仓库 vet/gofmt 作为门禁 | 低（建议单独清理 PR） |
| D-14 | `pkg/logsink` 队列上限用例断言 100，实现已改为 1024 | 核心测试无法全绿 | 中（改断言即可） |

债务修复建议与验收见 [需求文档 §8](requirements.md#8-风险与待决事项)。
