# mgin v2 架构设计文档（v2-design）

- **基线分支**：`v2-arch`（`a285b65`，已含健康探针 / client.Call 韧性 / HTTPS 优雅关闭）
- **作者**：架构师 高见远
- **配套**：`docs/v2-prd.md`（许清楚）、`docs/mgin-architecture-options.md`
- **范围边界**：本文件只做**设计 + 任务分解**，不含实现代码。所有 API / 包路径 / 配置项名均来自对 `v2-arch` 源码的实测，未编造。
- **已拍板的决策**（来自上游）：
  1. 单 module `github.com/maczh/mgin/v2`，内部按 `pkg/`（对外可导入）与 `internal/`（框架私有）严格分层。
  2. 允许引入 `go.opentelemetry.io/otel` 与 `prometheus/client_golang` 作为 direct 依赖；其余仍坚持最少依赖。
  3. `client.Call` 零破坏：`Call(ctx...)` 升级为新增 `CallCtx(ctx, service, uri, op)`，老 `Call` 内部委托 `CallCtx(context.Background(), ...)`。
  4. 本轮 P1 精选 **4 个**：客户端负载均衡、Prometheus 指标 + `/metrics`、统一错误码 + 国际化增强、OpenTelemetry 接入（详见 §11 结论）。

---

## 1. 现状盘点（实测摘要，依据 `a285b65`）

| 部位 | 实测事实（函数 / 签名与源码一致） | 改造点 |
|------|------|------|
| 生命周期 | `mgin.go` 的 `Init()/SafeExit()/checkAll()` 用一长串 `if strings.Contains(config.Config.Config.Used, "mysql")` 硬编码初始化 / 关闭 / 检查 | → §3 统一 Plugin |
| 插件接口 | `mgin.go:19` `MginPlugin{Init(configData []byte); Close(); Check() error}`，仅 `s3` 走 `UsePlugin`（`mgin.go:38,140`） | → §3 适配层 |
| 服务调用 | `client/client.go:41` `Call(service, uri string, op *Options)` **无 ctx**；`registry.Registry.GetServiceURL(service, group...) (string, string)` 返回**单 host**（`registry.go:15`） | → §6 ctx、§7 LB |
| 韧性 | `client/resilience.go` `CallResilient` + `client/breaker.go` 三态熔断（全局注册表，`GetBreaker(name)`）；超时用 goroutine+timer 兜底，注释自承"底层 HTTP 无法真正取消" | → §6 原生取消 |
| 健康 | `health/health.go` `Router(g)`、`MarkStarted()`、`CheckDependencies() (bool, map[string]string)`；`/health/live|ready|startup`，挂到主引擎早于一切中间件（`app.go:126`） | → §5/§8 不变 |
| 配置 | `config/configure.go` 全扁平：`go.application.*` `go.config.*` `go.discovery.*` `go.jwt.*` `go.sys.*` `go.casbin.*`，**无 `go.framework.*`**；`GetShutdownTimeout()` 读 `go.application.shutdownTimeout` 回退 5s | → §4 分层 |
| DAO | `db/dao/define.go:3` `Dao[E] interface { Insert(entity *E) error }` 仅一个方法；`MySQLDao` 实际有 `Where/Create/...` | → §3 扩展 |
| 错误码 | `errcode/constant.go` 仅有数字常量（如 `URI_NOT_FOUND=1000`），**无 HTTP 状态映射**；`i18n.Error(code, messageId)` 第二参是硬编码中文串（`app.go:153,313` `i18n.Error(errcode.URI_NOT_FOUND, errcode.UrlNotFound)`），且 `NoRoute`/`recoveryHandler` 均返回 `http.StatusOK` | → §9 三向映射 |
| 可观测 | 仅 `middleware/trace` 注入 TraceId；无 OTel、无 `/metrics` | → §10 OTel、§8 Metrics |
| 脚手架 | `cmd/scaffold.go:66-86` 把 `router/controller/service/dao/model` 平铺在项目顶层，配置放 `conf/` | → §2 收纳 internal |

存量 17 个测试文件（回归底线）：`client/breaker_test.go`、`client/resilience_test.go`、`config/configure_test.go`、`health/health_test.go`、`job/job_test.go`、`middleware/ratelimit/ratelimit_test.go`、`registry/{consul,etcd,polaris}_test.go`、`storage/s3/client_test.go`、`utils/util_test.go`。

---

## 2. 新目录结构（pkg / internal 分层）

### 2.1 目录树

```text
mgin/                                  # module github.com/maczh/mgin/v2 (根)
├── go.mod                             # 单 module，不拆子 module
├── mgin.go                            # 框架入口（保留）：原 Init/SafeExit 下沉到 internal/bootstrap
├── app.go                             # App 结构、NewApp/Run 保留（Run 内部调用 bootstrap 生命周期）
├── cmd/                               # mgin CLI（脚手架等），保留顶层
│   ├── scaffold.go                    # 生成工程改收进 internal/（见 2.2）
│   └── templates.go
├── docs/                              # 文档，保留
├── pkg/                               # 【对外可导入】公开契约
│   ├── client/                        # ← client/  (Call/CallCtx/CallT/CallResilient/breaker/resilience/Options)
│   ├── config/                        # ← config/  (Configure, GetConfig*, GetShutdownTimeout, 新增 Framework/Runtime)
│   ├── errcode/                       # ← errcode/ (常量 + 新增 ErrorCode 注册表 + HTTP 状态映射)
│   ├── i18n/                          # ← i18n/    (Error/String/Format/Success，键驱动)
│   ├── health/                        # ← health/  (Router/MarkStarted/CheckDependencies；内部依赖检查下沉 internal/healthcheck)
│   ├── models/                        # ← models/  (Result 等；models/sys 一并迁入 pkg/models/sys)
│   ├── registry/                      # ← registry/ (RegistryClient/NewRegistry + nacos/etcd/consul/polaris)
│   ├── middleware/                    # ← middleware/ (casbin cors iplimit jwt limit postlog ratelimit session trace xlang xss)
│   │   └── oauth2/                    # 新增（P2，本轮不实现，预留路径）
│   ├── casbin/                        # ← casbin/  (RBAC enforcer 辅助)
│   ├── utils/                         # ← utils/   (通用工具，对外保留)
│   ├── logger/                        # ← logs/     (原 logs 包改名 logger，对外日志 API)
│   ├── job/                           # ← job/     (定时任务注册/调度 API，对外)
│   ├── db/                            # ← db/      (db.Mysql 等全局门面 + 各驱动，对外；驱动内部实现可 private)
│   ├── storage/                       # ← storage/ (S3 等存储门面，对外；s3 内部实现可 private)
│   ├── metrics/                       # 新增（P1）：Prometheus 指标 + /metrics 挂载
│   └── oteltrace/                     # 新增（P1）：OTel TracerProvider + gin/gorm/grequests 插桩辅助
├── internal/                          # 【框架私有】仅本 module 内可 import
│   ├── plugin/                        # 新增：Plugin 接口、Register、Run 生命周期驱动
│   ├── bootstrap/                      # 新增：由插件注册表驱动的 Init/SafeExit/checkAll（取代 mgin.go 硬编码链）
│   ├── healthcheck/                   # 新增：依赖检查装配（抽自 health.dependencies() 与 mgin.checkAll()）
│   ├── configloader/                  # 新增：koanf 加载/GetConfigData 内部实现（pkg/config 暴露公开 API）
│   └── cache/                         # ← cache/   (框架内部缓存，被 i18n/registry 复用；不对外)
└── (脚手架生成工程) internal/router|controller|service|dao|model   # 旧平铺工程仍可运行
```

### 2.2 迁移映射表（逐包）

| 现有位置 | 新位置 | 公开/私有 | 是否破坏 import 路径 | 兼容性处理 |
|----------|--------|-----------|----------------------|------------|
| `client/` | `pkg/client/` | 公开 | 是 | v2 主版本 + 迁移脚本（`cmd/migrate` 或文档 sed 映射）；`Call` 签名不变 |
| `config/` | `pkg/config/` | 公开 | 是 | 同上；旧配置键全部保留可读 |
| `errcode/` | `pkg/errcode/` | 公开 | 是 | 常量名不变，新增注册表 |
| `i18n/` | `pkg/i18n/` | 公开 | 是 | `Error/String/Format/Success` 签名不变 |
| `health/` | `pkg/health/` | 公开 | 是 | `Router/MarkStarted` 不变；内部 `dependencies()` 下沉 `internal/healthcheck` |
| `models/` + `models/sys/` | `pkg/models/` + `pkg/models/sys/` | 公开 | 是 | `Result` 类型不变 |
| `registry/` + 子包 | `pkg/registry/` + 子包 | 公开 | 是 | `RegistryClient` 接口新增 `GetServices`（见 §7） |
| `middleware/*` | `pkg/middleware/*` | 公开 | 是 | 各中间件导出函数名不变 |
| `casbin/` | `pkg/casbin/` | 公开 | 是 | 仅 framework 内部 + 业务引用，一并迁移 |
| `utils/` | `pkg/utils/` | 公开 | 是 | 通用工具保留公开 |
| `logs/` | `pkg/logger/` | 公开 | 是（改名） | 提供 `pkg/logger` 并保留旧 `logs` 别名包一个 release（类型别名） |
| `job/` | `pkg/job/` | 公开 | 是 | `Start/Stop/GetManager` 不变 |
| `db/` | `pkg/db/` | 公开 | 是 | `db.Mysql` 等全局门面不变；驱动实现细节留 `pkg/db/*` 内 |
| `storage/` | `pkg/storage/` | 公开 | 是 | `storage.NewS3()`/`GetS3()` 不变 |
| `cache/` | `internal/cache/` | 私有 | 是（且外部不可再 import） | 若存量直接 import `cache`，本轮一并迁移或保留顶层 `cache` 兼容壳 |
| `mgin.go` `Init/SafeExit/checkAll` | `internal/bootstrap/` | 私有 | 否（入口函数保留） | `mgin.Init(configFile)` 仍导出，内部转调 bootstrap |
| `health/health.go` 的 `dependencies()` / `checkAll` 逻辑 | `internal/healthcheck/` | 私有 | 否 | `pkg/health.Router` 调用 `healthcheck.CheckDependencies` |
| `config/configure.go` 的 koanf 加载 | `internal/configloader/` | 私有 | 否 | `pkg/config` 暴露 `Init/GetConfig*` |
| （无） | `internal/plugin/` | 私有 | — | 新增 Plugin 注册表 |

> **公开 API 列表（不破坏）**：`client.Call`/`CallT`/`CallResilient`/`GetBreaker`；`config.Config.*`/`GetConfig*`/`GetShutdownTimeout`；`errcode.*` 常量；`i18n.Error/ErrorT/String/Format/Success`；`health.Router/MarkStarted/IsStarted`；`models.Result`；`registry.Registry`/`NewRegistry`/`GetServiceURL`；各 `middleware.*` 导出函数；`app.NewApp/Run/EnableHealth`；`mgin.Init/Use/UsePlugin`。
> **内部 API 列表（可调整）**：`bootstrap.Init/SafeExit`；`plugin.Register/Run`；`healthcheck.CheckDependencies`；`configloader.Load/GetConfigData`；`cache.*`。

### 2.3 向后兼容策略（与 PRD §5 对齐）

- **import 路径变化**：v2 视为一次主版本演进。提供 `docs/migration-v2.md` + 一个 `cmd/migrate` 工具（或 `gofmt -r`/`sed` 映射表）把旧路径批量改写为 `pkg/...`。`mgin new` 生成的新工程直接使用 `pkg/` 路径。
- **运行时不破坏**：旧式平铺工程（顶层 `router/controller/...`）仍可 `go build`——框架不强依赖 `internal/` 业务目录；只有 `mgin new` 生成的结构采用新布局。
- **API / 配置键不变**：`client.Call` 签名、配置项 `go.application.*`/`go.config.*` 等全部保留；新增 `go.framework.*`/`go.runtime.*` 不替代旧键。

---

## 3. 统一 Plugin 接口规范

### 3.1 接口定义

```go
// internal/plugin/plugin.go
type Plugin interface {
    Name() string            // 唯一名，如 "mysql" "s3" "job" "nacos"
    Order() int              // 启动顺序，越小越先 Init（如 db=10, cache=5, registry=20, job=90）
    Init(ctx context.Context) error
    Close(ctx context.Context) error
    Health() error           // 给 /health/ready 自报健康；无依赖可返回 nil
    Enabled() bool           // 是否由配置启用（读 go.config.used）
}
```

### 3.2 注册表与生命周期驱动

```go
// internal/plugin/registry.go
func Register(p Plugin)                 // 幂等，按 Name 去重；可被 init() 或 main 调用
func Unregister(name string)
func Run(ctx context.Context) error     // 按 Order 升序 Init；失败时按逆序 Close 已启动项
func Shutdown(ctx context.Context) error// 按 Order 降序 Close
func Health() map[string]error          // 汇总各 Plugin.Health()，供 /health/ready
```

`internal/bootstrap` 的 `Init(configFile)` 改为：
1. `config.Config.Init(configFile)`（兼容旧逻辑）；
2. `registry.Registry = registry.NewRegistry()`；
3. `plugin.Run(ctx)` —— 内部对每个 `Enabled()==true` 的插件按 `Order` 调 `Init`；
4. 删除全部 `if strings.Contains(Used, ...)` 硬编码链。

`SafeExit` 改为 `plugin.Shutdown(ctx)`。

### 3.3 适配层（把现有组件接入 Plugin）

现有三类来源统一包装为 `Plugin`：

**(a) `db.Mysql` 等数据源**（`db/mysql` 有 `Init([]byte)/Close()/Check() error`）
```go
// internal/plugin/adapters/db.go
type dbPlugin struct {
    name      string
    initFn    func([]byte)
    closeFn   func()
    checkFn   func() error
    prefixKey string // 如 "go.config.prefix.mysql"
}
func (p *dbPlugin) Init(ctx context.Context) error {
    data := config.Config.GetConfigData(config.Config.GetConfigString(p.prefixKey))
    if data == nil { return fmt.Errorf("%s 配置缺失", p.name) }
    p.initFn(data); return nil
}
func (p *dbPlugin) Close(ctx context.Context) error { p.closeFn(); return nil }
func (p *dbPlugin) Health() error                   { return p.checkFn() }
func (p *dbPlugin) Enabled() bool                   { return strings.Contains(config.Config.Config.Used, p.name) }
```
注册：`plugin.Register(&dbPlugin{name:"mysql", initFn: db.Mysql.Init, closeFn: db.Mysql.Close, checkFn: db.Mysql.Check, prefixKey:"go.config.prefix.mysql"})`（其余 mysql/postgres/mongodb/redis/clickhouse/elasticsearch/kafka/sqlite 同理）。

**(b) `storage.S3`（已实现 `MginPlugin{Init([]byte);Close();Check() error}`，见 `storage/s3/client.go:62-185`）**
```go
type s3Plugin struct{ *s3.S3 }
func (s s3Plugin) Init(ctx context.Context) error {
    data := config.Config.GetConfigData("go.s3"); if data==nil { return errors.New("s3 配置缺失") }
    s.S3.Init(data); return nil
}
func (s s3Plugin) Name() string { return "s3" }
func (s s3Plugin) Order() int  { return 40 }
func (s s3Plugin) Enabled() bool { return strings.Contains(config.Config.Config.Used, "s3") }
// Close/Health 直接复用 *s3.S3 的方法
```

**(c) `job` 与 `registry`**
- `jobPlugin`：`Init`→`job.Start()`，`Close`→`job.Stop()`，`Health`→`job.GetManager().Check()`，`Enabled`→`Used` 含 "job"。
- `registryPlugin`：`Init`→`registry.Registry.Register(data)`，`Close`→`registry.Registry.DeRegister()`，`Enabled`→`Used` 含对应注册中心。

### 3.4 兼容性

- `Use(dbConfigName, initFn, closeFn, checkFn)` 与 `UsePlugin(name, MginPlugin)` **保留**，内部统一转调 `plugin.Register(...)` 适配后的插件。存量 `mgin.UsePlugin("s3", s3.NewS3())` 行为不变。

---

## 4. 配置分层

### 4.1 三个命名空间

| 命名空间 | 含义 | 承载字段（新增于 `config` 结构） | 旧键映射 |
|----------|------|------|----------|
| `go.framework.*` | 框架自身元配置 | `Framework{ShutdownTimeout int; Health{Enabled,AutoStarted bool}; Plugin map[string]bool; Trace{Enabled,Exporter,Endpoint}; Metrics{Enabled,Path}}` | 无（新增） |
| `go.application.*` | 存量业务配置 | 现有 `App`/`Config`/`Log`/`Logger`/`Discovery`/`Jwt`/`Sys`/`Casbin` 全部保留 | 100% 兼容 |
| `go.runtime.*` | 运行时元数据 | `Runtime{StartTime, CommitHash, BuildTime, Version}`（由 `version.go` 经 `-ldflags` 注入；`GetConfigString("go.runtime.commit")` 可读） | 无（新增） |

### 4.2 `Configure` 字段映射

在 `config/configure.go` 的 `config` 结构新增：
```go
Framework framework `json:"framework" bson:"framework"`
Runtime   runtime   `json:"runtime"   bson:"runtime"`
```
- `GetShutdownTimeout()` 升级为：优先 `go.framework.shutdownTimeout` → 回退 `go.application.shutdownTimeout` → 默认 5s（与升级前硬编码一致）。
- `go.config.env` 继续作为 dev/test/prod 单一维度；`go.framework` 不引入新环境维度，环境隔离沿用现有 `<prefix>-<env>.yml` 机制（`GetConfigUrl` 已实现）。

### 4.3 `application.yml` v2 典型样例

```yaml
go:
  framework:
    shutdownTimeout: 10          # 框架级，覆盖旧 go.application.shutdownTimeout
    health:
      enabled: true              # /health/live|ready|startup 开关
      autoStarted: true
    trace:
      enabled: true
      exporter: otlp             # otlp | stdout | none
      endpoint: http://localhost:4318
    metrics:
      enabled: true
      path: /metrics
  application:                    # 旧键原样保留，向后兼容
    name: demo
    port: 8080
    port_ssl: 8443
    cert: server.crt
    key: server.key
    debug: false
  config:
    used: "mysql,redis,nacos,job"
    env: dev
  discovery:
    registry: nacos
    callType: x-form
  jwt:
    secret: xxxxx
```

---

## 5. 健康探针（与 v2 的关系，不改动）

- `health` 包保持 `GET /health/live|ready|startup`，挂到主引擎且早于一切中间件（`app.go:126` 的 `mountHealth()` 位置不变）。
- v2 中 `/health/ready` 的数据源检查逻辑下沉到 `internal/healthcheck`（复用各 `Plugin.Health()`），不再依赖 `mgin.checkAll()` 的硬编码链；语义与现有 `CheckDependencies()` 一致（自愈重连）。
- `/metrics`（§8）**单独挂载**，不被探针逻辑耦合。

---

## 6. client.Call 零破坏升级

### 6.1 签名

```go
// pkg/client/client.go
func Call(service, uri string, op *Options) (string, error) {
    return CallCtx(context.Background(), service, uri, op)   // 老签名不变，零破坏
}

// 新增：首参 ctx
func CallCtx(ctx context.Context, service, uri string, op *Options) (string, error)
func CallTCtx[T any](ctx context.Context, service, uri string, op *Options) models.Result[T]
```

### 6.2 ctx 透传到底层 HTTP

`grequests` 的 `RequestOptions` 支持 `HTTPClient *http.Client`（resilience.go:148 已确认）。v2 做法：
- 在 `CallCtx` 内构造 `&http.Client{}`，并尽量把 `ctx` 注入到底层 `*http.Request`：若所用 `grequests` 版本支持 `WithContext(ctx)`（或 `RequestOptions` 接受 context），则原生取消；**否则保留 `resilience.go` 的 goroutine+timer 兜底作为超时路径**，并在 `select` 中叠加 `ctx.Done()` 实现取消信号外溢（超时/取消后不再阻塞调用方，但底层连接仍由对端关闭——与现有局限一致，列为已知项并评估升级 grequests 以达成原生取消）。
- 该改造**不影响**现有 `Call` 调用方：`Call` 用 `context.Background()` 调用 `CallCtx`，行为与现在完全一致。

### 6.3 `Options.Retry` 去留

- `Retry bool` **保留**（向后兼容），在 `CallCtx` 内仍按现有朴素逻辑处理一次重试；
- 富重试 / 退避 / 熔断统一走 `CallResilient(service, uri, op, *ResilienceOptions)`，不再依赖 `Options.Retry`。PRD/文档标注 `Retry` 为"遗留字段，新代码优先用 `CallResilient`"。

---

## 7. 客户端负载均衡（P1，本轮实现）

### 7.1 扩展 Registry 接口

```go
// pkg/registry/registry.go
type RegistryClient interface {
    Register(registryConfigData []byte)
    GetServiceURL(servicename string, groupName ...string) (string, string) // 保留，兼容
    GetServices(servicename string, groupName ...string) ([]string, error)   // 新增：多实例 URL 列表
    DeRegister()
}
```
四个实现（nacos/etcd/consul/polaris）均补充 `GetServices`：其 `GetServiceURL` 内部本就拉取实例列表（`nacos.go:138-210` 的 `instance.Hosts`），重构为把"拉列表 + 按策略选一个"拆开，`GetServices` 直接返回全部 host URL（含 `http/https` 协议前缀，沿用 metadata 的 `ssl` 判断）。接口新增方法需同步更新 4 个实现（受控，不破坏外部）。

### 7.2 LoadBalancer 接口与策略

```go
// pkg/client/loadbalancer.go
type LoadBalancer interface {
    Name() string
    Pick(service string, candidates []string) (string, error)
}
// 4 种策略实现 + 按名注册
var strategies = map[string]LoadBalancer{
    "roundrobin":       &RoundRobin{},
    "random":           &Random{},
    "leastconnections": &LeastConnections{},
    "consistenthash":   &ConsistentHash{},
}
```
- `LeastConnections` 需要一个连接计数后端：复用 `cache`（或 `internal/cache`）记录每实例活跃连接；
- `ConsistentHash` 以 `service + op.Group + 请求特征` 为 key 做哈希，保证同 key 落到同实例（有状态服务友好）。

### 7.3 与 `CallCtx` / 熔断协同

```text
CallCtx(ctx, service, uri, op)
   │
   ├─ registry.GetServices(service, op.Group) → []host
   ├─ 剔除：对每 host 查 per-instance 熔断状态（key = service@host），跳过 Open 的
   ├─ LoadBalancer.Pick(service, 健康hosts) → 选定 host
   ├─ 构造 &http.Client，发起请求（ctx 透传）
   └─ 调用结果回报 per-instance 熔断：GetBreaker(service+"@"+host)
```

- **per-instance 熔断**：`client.GetBreaker(name)` 的 `name` 由单服务粒度升级为 `service@host` 粒度，使一盘棋的熔断细到实例（与 `CallResilient` 的 `Breaker` 字段兼容：调用方显式传 `ResilienceOptions.Breaker` 时仍以传入为准）。

---

## 8. Prometheus 指标（P1，本轮实现，[需要新依赖] `prometheus/client_golang`）

### 8.1 包与端点

- 包：`pkg/metrics/`，导出 `Init(opts)`、`Middleware() gin.HandlerFunc`、`Handler() http.Handler`（promhttp）。
- 挂载：`/metrics` 在 `app.baseRouter()` 中最早期注册（与 `mountHealth()` 同位置思路），**早于业务中间件**，避免被 jwt/casbin/ratelimit 拦截；默认 `go.framework.metrics.path`（缺省 `/metrics`），`go.framework.metrics.enabled=false` 时不挂载（不破坏未启用项目）。

### 8.2 指标命名（遵循 Prometheus 约定，前缀 `mgin_`）

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `mgin_http_requests_total` | Counter | `method`,`path`,`status` | HTTP 请求计数 |
| `mgin_http_request_duration_seconds` | Histogram | `method`,`path` | 请求延迟（桶：.005,.01,.025,.05,.1,.25,.5,1,2.5,5,10） |
| `mgin_http_requests_in_flight` | Gauge | — | 在途请求数 |
| `mgin_go_goroutines` | Gauge | — | runtime.NumGoroutine |
| `mgin_go_memstats_heap_inuse_bytes` | Gauge | — | go_memstats_* 系列 |
| `mgin_go_gc_duration_seconds` | Summary | — | GC 耗时 |
| `mgin_client_calls_total` | Counter | `service`,`status` | 出站微服务调用（与 §7 协同） |

- 运行时指标（`go_*`）由 `pkg/metrics` 在 `Init` 时用 `prometheus.NewGoCollector()` / `process_collector` 自动注册，无需业务埋点。
- `/health/live` `/health/ready` 不变；`/metrics` 独立，互不影响。

---

## 9. 统一错误码 + 国际化增强（P1，本轮实现，零新增依赖）

### 9.1 现有痛点（实测）

1. **恒返回 200**：`app.go:151-153` `NoRoute` 用 `c.JSON(http.StatusOK, i18n.Error(errcode.URI_NOT_FOUND, errcode.UrlNotFound))`；`recoveryHandler`（`app.go:312-314`）同理。`errcode` 与 HTTP 状态无映射。
2. **i18n 不是真正的 i18n**：`i18n.Error(code, messageId)` 第二参传的是硬编码中文串（`errcode.UrlNotFound`/`errcode.SystemError`，见 `errcode/constant.go` 与 `app.go`），`i18n.String` 找不到键时原样返回该串，等价于写死中文。
3. **无错误链**：错误只能返回 `string`，无法 `errors.Is/As` 判断根因。

### 9.2 v2 设计

**(a) 错误码标准化（域 + 模块 + 序号）**
```go
// pkg/errcode/code.go
type ErrorCode struct {
    Code      int    // = Domain*10000 + Module*100 + Seq
    Domain    string // 业务域，如 "sys" "order"
    Module    string
    Seq       int
    HTTPStatus int   // 映射的 HTTP 状态码
    DefaultKey string // i18n 默认键（兜底文案）
}
func Register(ec ErrorCode)              // 注册到全局表
func HTTPStatusOf(code int) int          // code -> HTTP 状态，默认 200/400
```
把现有常量（如 `URI_NOT_FOUND=1000` → HTTP 404、`TOKEN_ERROR=1008`/`AUTHENTICATION_FAILURE=1009` → 401、`PARAM_ERROR=1014` → 400、`TOO_MANY_REQUESTS=1011` → 429、`SERVICE_UNAVAILABLE=1010` → 503、`SYSTEM_ERROR=1001` → 500）登记进映射表（文档化，见 `pkg/errcode/codes.go`）。

**(b) i18n 键驱动**
- `errcode/constant.go` 的硬编码中文串改为 **i18n 键**（如 `UrlNotFound` 改为键 `"err.url.not.found"`）；`i18n.Error(code, key)` 用 `key` 查多语言表，`String(key)` 找不到时回退到 `ErrorCode.DefaultKey` 的默认文案（默认语言 `zh-cn`）。
- 现有 `i18n.Error(code, messageId)` 签名保留（第二参语义由"中文串"变为"i18n 键"），并新增 `i18n.ErrorCode(ec ErrorCode, args ...any) models.Result` 推荐用法。

**(c) 错误链**
```go
// pkg/errcode/error.go
func New(code int, cause error, args ...any) *Error   // 包装根因，errors.Is/As 兼容
func (e *Error) Unwrap() error
```
- `recoveryHandler` 与 `NoRoute` 改为读取 `errcode.HTTPStatusOf(code)` 返回对应 HTTP 状态；新增 `app.ResultError(c *gin.Context, err error)` 统一写出 `code + msg + httpStatus`。

### 9.3 与 health 关系

无耦合。错误码映射表为纯内存结构，不触达探针。

---

## 10. OpenTelemetry 接入（P1，本轮实现，[需要新依赖] `go.opentelemetry.io/otel` + sdk + otlp exporter）

### 10.1 包与初始化

- 包：`pkg/oteltrace/`，导出 `Init(opts)`（读取 `go.framework.trace.{enabled,exporter,endpoint}`），构建 `TracerProvider`：
  - 默认导出 `otlp` HTTP（`http://localhost:4318/v1/traces`）；
  - `exporter=stdout` 时落控制台；`none` 时不启用（不破坏未配置项目）。
- `shutdown` 接入 `plugin.Shutdown` 或 `app.Run` 的优雅关闭链。

### 10.2 三个关键点位插桩

1. **gin（入站 span）**：升级 `pkg/middleware/trace`（`middleware/trace/mgtrace.go:7` 的 `TraceId()`），在生成内部 TraceId 的同时开启 OTel span，写入 W3C `traceparent`/`tracestate` 响应/透传头；保留 `trace.GetHeaders()` 既有传播，使旧调用方无感知。
2. **gorm（DB span）**：在 `db` 各驱动 `Init` 后，对 `*gorm.DB` 注入 `otel gorm` 插件（如 `gorm.io/plugin/opentelemetry/tracing`），记录 SQL 耗时 span。
3. **grequests（出站 span）**：在 `CallCtx`（§6/§7）发起请求前 `tracer.Start(ctx, "client "+service)`，把 `traceparent` 注入 `op.Header`，结束 span；与现有 trace 头共存。

### 10.3 兼容

- 现有"仅 TraceId 透传"行为保留：OTel `trace_id` 与内部 `X-Request-Id` 做映射，未接入 APM 时退化为原 TraceId 行为。

---

## 11. P1 精选结论（本轮 4 个 + 理由）

| 选中 | 对存量项目价值 | 实施复杂度 | 依赖前提 | 备注 |
|------|------|------|------|------|
| **客户端负载均衡** | 高：多实例治理刚需，解决 `GetServiceURL` 单 host 痛点 | 中 | P0-3 CallCtx、P0-2 注册表 | 与熔断 per-instance 协同，是 D/E 方案基础 |
| **Prometheus 指标 + /metrics** | 高：可观测性一等公民，运维刚需 | 中 | 用户已批准 `prometheus/client_golang` | 标准生态兼容 |
| **统一错误码 + 国际化增强** | 高：修复"响应恒为 200""i18n 写死中文"两大痛点 | 低—中 | 无新依赖 | 纯内部重构，回归风险低 |
| **OpenTelemetry 接入** | 高：跨进程链路串联，对标 Spring Cloud Sleuth | 中—高 | 用户已批准 `go.opentelemetry.io/otel` | trace/metrics/health 三位一体 |

**未选（本轮递延）及理由**：
- **API 网关**：价值高但体量最大，且强依赖负载均衡（已是本轮 P1），放在 v2.2 与 LB 协同落地更稳。
- **OAuth2 / OIDC 资源服务器**：有价值但 ROI 低于上述四项；可作 v2.2 独立增量（`pkg/middleware/oauth2`，路径已预留）。
- **分布式锁**：依赖已具备（go-redis/etcd），成本最低，但非存量项目燃眉之急，列入 v2.2（`pkg/lock`）。

---

## 12. 关键设计决策（需用户确认）

1. **import 路径破坏性迁移**：v2 将 `client`/`config`/`errcode`/`i18n`/`health`/`models`/`registry`/`middleware`/`casbin`/`utils`/`logs`/`job`/`db`/`storage` 全部迁入 `pkg/`，存量项目 import 路径会变更。我们按"主版本演进 + 迁移脚本"处理，**不保留顶层旧包壳**（仅 `logs` 提供一 release 别名）。→ 请确认是否接受"v2 允许 import 路径破坏性变更"，还是必须保留顶层包别名（增加兼容壳维护成本）。
2. **grequests 原生取消**：`CallCtx` 的 ctx 取消是否要求"底层 HTTP 真正中断"？若要，需评估升级 `levigross/grequests` 或替换 HTTP 库；若可接受"调用方不被阻塞、但连接残留"的当前兜底，则维持现状。→ 请确认 ctx 取消的严格程度。

---

## 13. 实施阶段切分（概览，详见 `docs/v2-tasks.md`）

- **v2.0（P0）**：目录分层 + 统一 Plugin + 配置分层 + client.Call ctx + Dao 扩展。
- **v2.1（P1，本轮）**：客户端负载均衡 + Prometheus 指标 + 统一错误码 + OpenTelemetry。
- **v2.2（P2/递延 P1）**：API 网关、OAuth2、分布式锁、gRPC、分布式 Session、多租户、API 文档、集群限流、分布式事务。
