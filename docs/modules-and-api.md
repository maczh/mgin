# MGin v2 模块与接口说明

| 项 | 内容 |
|----|------|
| 文档版本 | v2.0 |
| 对应代码 | `github.com/maczh/mgin/v2` |
| 说明 | 本文只列**对外可用**的导出符号；未导出函数以文字说明。签名均与当前代码一致 |

包索引：

| 组 | 包 |
|----|----|
| 入口 | `mgin`（根） |
| 生命周期 | `pkg/plugin` |
| 配置/数据 | `pkg/config`、`pkg/db`、`pkg/db/dao`、`pkg/cache` |
| 服务治理 | `pkg/registry`、`pkg/client`、`pkg/loadbalancer` |
| 可观测 | `pkg/health`、`pkg/metrics`、`pkg/otel`、`pkg/logs`、`pkg/logsink` |
| 安全与流量 | `pkg/middleware/*`、`pkg/casbin` |
| 响应与模型 | `pkg/models`、`pkg/models/sys`、`pkg/vo`、`pkg/errcode`、`pkg/i18n` |
| 任务 | `pkg/job` |
| 工具 | `pkg/utils` |

---

## 1. 框架入口（根包 `mgin`）

### 1.1 `mgin.go`

```go
// 兼容接口：v1 的组件适配器
type MginPlugin interface {
    Init(configData []byte)
    Close()
    Check() error
}

func Init(configFile string)

func (m *mgin) Use(dbConfigName string, dbInit dbInitFunc, dbClose dbCloseFunc, dbCheck dbCheckFunc)
func (m *mgin) UsePlugin(dbConfigName string, mginPlugin MginPlugin)
func (m *mgin) SafeExit()
```

- `Init` 顺序：配置 → VictoriaLogs → 注册中心实例 → Runtime → `plugin.RegisterBuiltins` →
  `plugin.InitAll` → 旧 `initPlugins`。
- `Use` 仅当 `go.config.used` 精确包含 `dbConfigName` 时执行 Init（使用 `configuredComponent`，
  是**精确匹配**，与插件层的子串匹配不同）。
- `SafeExit`：`postlog.Close` → `i18n.Close` → `plugin.CloseAll` → 旧插件 Close。

### 1.2 `app.go`

```go
type App struct {
    Router *gin.Engine   // 业务可直接注册路由/中间件
    MGin   *mgin
}

func NewApp(configFile, appName, version string, xlang bool) *App

func (app *App) EnableHealth()          // 立即挂载探针并置内部开关；建议紧跟 NewApp 调用
func (app *App) EnableMetrics()         // 立即挂载 /metrics 且置内部开关（不读配置）
func (app *App) MarkHealthStarted()     // 等价 health.MarkStarted()
func (app *App) GetVersion() string
func (app *App) Run()                   // 启动 HTTP/HTTPS，阻塞至信号
```

坑位：

| 行为 | 说明 |
|------|------|
| `-f` 默认值 | 可执行文件同名 `.yml`；相对路径拼可执行文件目录 |
| `-v` | 打印 `appName, 版本号: version` 后 `os.Exit(0)` |
| `go.metrics.enabled` | 单独配置**不挂载**，必须调用 `EnableMetrics()` |
| `go.health.enabled` | 单独配置**可以**挂载（`baseRouter` 调用 `mountHealth()` 时检查该键） |
| `EnableHealth()` | 无视配置立即挂载；若在业务路由之后调用，探针也会注册在路由链末端 |
| 重复挂载 | `healthMounted`/`metricsMounted` 去重，不会 panic |

---

## 2. `pkg/plugin` — 统一插件契约

```go
type Plugin interface {
    Name() string
    Order() int
    Init(ctx context.Context) error
    Close(ctx context.Context) error
    Health() error
    Enabled() bool
}

func Register(p Plugin)               // 同名覆盖，order 仅首次记录
func Unregister(name string)
func GetByName(name string) Plugin
func All() []Plugin
func InitAll(ctx context.Context) error
func CloseAll(ctx context.Context) error
func HealthAll() map[string]error

func RegisterBuiltins()
```

内置构造器（均在 `builtins.go`）：`Mysql()`、`Sqlite()`、`Mongodb()`、`Redis()`、`Registry()`、`Job()`。

语义：

- `InitAll` 按 `Order` 升序，跳过 `Enabled()==false`；失败记录首个 error 后继续。
- `CloseAll` 逆序，单插件 panic 被 `recover` 隔离。
- `HealthAll` 当前唯一消费者是 `mgin.checkAll` 的 5 分钟巡检，**不参与** `/health/ready`。
- 注册表是**包级全局**，无实例概念。

---

## 3. `pkg/config` — 配置加载

```go
var Config = &config{}

func (c *config) Init(cf string)
func (c *config) GetShutdownTimeout() time.Duration
func (c *config) GetConfigString(name string) string
func (c *config) GetConfigStringArray(name string) []string
func (c *config) GetConfigInt(name string) int
func (c *config) GetConfigBool(name string) bool
func (c *config) Exists(name string) bool
func (c *config) GetConfigUrl(prefix string) string
func (c *config) GetConfigData(prefix string) []byte
```

导出结构（字段均导出，可直接读）：

| 结构 | 关键字段 |
|------|----------|
| `Config.App` | `Name, Project, Port, PortSSL, Cert, Key, Debug, IpAddr, ShutdownTimeout` |
| `Config.Config` | `Server, Type, Token, Path, Env, Used, Prefix{...}` |
| `Config.Log` | `RequestTableName, CallTableName, LogDb, DbName, Get, Req, Api, Kafka{Use,Topic}, Sink{Queue,Workers,Shutdown}, VictoriaLogs{...}` |
| `Config.Logger` | `Outputs []LogOutput, Level, Out, File` |
| `Config.Discovery` | `Registry, CallType, CacheTTL, CacheRefresh` |
| `Config.Jwt` | `Secret` |
| `Config.Sys` | `Enabled, Initdb, BaseUri, Swagger{Enabled,Uri}, Casbin` |
| `Config.Casbin` | `Enabled, ModelFile` |
| `Config.Framework` | `HealthEnabled, I18nEnabled, RatelimitEnabled, RecoverEnabled, LogRequests` |
| `Config.Runtime` | `CommitHash, BuildTime, GoVersion, StartedAt, Pid` |
| `Config.WorkDir` | 配置文件所在目录（`Casbin.ModelFile` 相对路径拼接基准） |
| `Config.Cnf` | 底层 `*koanf.Koanf`（限流等包直接 Unmarshal） |

数据源与 URL 规则：

| `go.config.server_type` | 取数方式 |
|-------------------------|----------|
| `nacos` | `{server}nacos/v1/cs/configs?group={Project}&dataId={prefix}-{env}.yml`（server 需以 `/` 结尾） |
| `consul` | `{server}/v1/kv/{Project}/{prefix}-{env}.yml?dc=dc1&raw=true` |
| `etcd` | `clientv3`，key `/config/{Project}/{prefix}-{env}.yml` |
| `polaris` | `GET /config/v1/configfiles/release?...`，Header `X-Polaris-Token`，`code==200000` 取 `content` |
| `springconfig` | `{server}/{Project}/{prefix}-{env}.yml` |
| `file` | `{WorkDir 或 go.config.path}/{prefix}-{env}.yml` |
| 其他 | `{server}{prefix}-{env}.yml` |

---

## 4. 数据访问

### 4.1 `pkg/db` 门面

```go
var Mysql  = &mysql.MysqlClient{}
var Mongo  = &mongo.Mongodb{}
var Redis  = redis.Redis
var Sqlite = &sqlite.Sqlite{}
```

事务：

```go
func MySQLTx[T any](ctx context.Context, fn func(tx *gorm.DB) (T, error), dbName ...string) (T, error)
func TransactionOn[T any](ctx context.Context, conn *gorm.DB, fn func(tx *gorm.DB) (T, error)) (T, error)
```

### 4.2 各客户端公共方法

| 客户端 | 方法 |
|--------|------|
| `MysqlClient` | `Init([]byte)`、`Close()`、`Check() error`、`GetConnection(dbName ...string) (*gorm.DB, error)`、`IsMultiDB() bool`、`ListConnNames() []string` |
| `Sqlite` | `Init(dbFileName string)`、`Close()`、`Check() error`（恒 nil）、`GetConnection() (*gorm.DB, error)` |
| `Mongodb` | `Init([]byte)`、`Close()`、`Check() error`、`GetConnection(dbName ...string) (*mgo.Database, error)`、`ReturnConnection(*mgo.Database)`、`IsMultiDB()`、`ListConnNames()` |
| `RedisClient` | `Init([]byte)`、`Close()`、`Check() error`（恒 nil）、`GetConnection(dbName ...string) (redis.UniversalClient, error)`、`PSubscribe(dbName string, handler func(*redis.Message, string), channels ...string)`、`IsMultiDB()`、`ListConnNames()` |

连接选择规则：

- MySQL/Redis 多库时**必须**恰好传一个库名，否则报错；单库时参数被忽略。
- MongoDB 多库时可不传，默认第一个连接；返回的连接需 `ReturnConnection`。
- SQLite 无多库。

### 4.3 `pkg/db/dao`

```go
type MySQLDao[E schema.Tabler] struct {
    debug bool
    ctx   *context.Context
    Tag   func() string
    cache *CacheConfig
    tx    *gorm.DB
}

type CacheConfig struct {
    Enabled bool
    Store   cacheStore.ICache   // 通常用 dao.NewRedisCache(client)
    TTL     time.Duration
    Prefix  string              // 默认 "mgin:mysql:"
}
```

方法（MySQL）：

| 方法 | 签名 | 缓存 |
|------|------|------|
| `Debug` | `Debug() *MySQLDao[E]` | — |
| `WithTx` | `WithTx(tx *gorm.DB) *MySQLDao[E]` | 事务内禁用缓存 |
| `WithContext` | `WithContext(ctx *context.Context) *MySQLDao[E]` | — |
| `WithCache` | `WithCache(config CacheConfig) *MySQLDao[E]` | 启用 |
| `Where` | `Where(query interface{}, args ...interface{}) *gorm.DB` | 否 |
| `WhereAll` | `WhereAll(query, args...) ([]E, error)` | ✅ |
| `WhereOne` | `WhereOne(query, args...) (*E, error)` | ✅ |
| `WhereCount` | `WhereCount(query, args...) (int64, error)` | ✅ |
| `WhereExists` | `WhereExists(query, args...) bool` | ✅ |
| `Create` | `Create(entity *E) error` | 成功后清前缀 |
| `MultiCreate` | `MultiCreate(entities []*E) error` | 同上 |
| `Delete` | `Delete(entity E) error` | 同上 |
| `Updates` | `Updates(entity *E) error` | 同上 |
| `Save` | `Save(entity *E) error` | 同上 |
| `All` | `All(entity E, opts ...QueryOption) ([]E, error)` | ✅ |
| `One` | `One(entity E) (*E, error)` | ✅ |
| `Exists` | `Exists(entity E) bool` | ✅ |
| `Count` | `Count(entity E) (int64, error)` | ✅ |
| `Pager` | `Pager(conn *gorm.DB, page, size int) ([]E, *models.ResultPage, error)` | 否 |
| `Alias` / `DB` / `JOIN` | 返回 `*gorm.DB` | 否 |

`MgoDao[E any]`（字段 `CollectionName string`、`Tag func() string`）：
`Insert(*E)`、`Delete(bson.M)`、`Updates(primitive.ObjectID, E)`、`Update(bson.M, bson.M)`、
`All(bson.M)`、`One(bson.M)`、`Pager(bson.M, []string, page, size)`。

`Dao[E any]` 接口（`define.go`）目前**没有实现方**，禁止作为业务抽象依赖。

缓存键：`{prefix}{tag}:{TableName()}:{operation}:md5(排序后的 k=v&...)`，
空值字段被剔除。`RedisCache.Clear()` 等价 `FLUSHDB`，生产慎用。

---

## 5. `pkg/registry` — 注册与发现

```go
type RegistryClient interface {
    Register(registryConfigData []byte)
    GetServiceURL(servicename string, groupName ...string) (url string, group string)
    GetServices(servicename string, groupName ...string) ([]string, error)
    DeRegister()
}

var Registry RegistryClient          // 全局实例
func NewRegistry() RegistryClient    // 依 go.discovery.registry 创建 + 缓存装饰
func NewCachedRegistry(inner RegistryClient) *CachedRegistry
```

| 后端 | 类型 | 子配置键 |
|------|------|----------|
| Nacos | `nacos.NacosClient` | `go.nacos.{lan,lanNet,server,port,apiversion,group,clusterName}` |
| etcd | `etcd.EtcdClient` | `go.etcd.{lan,lanNet,server,port,prefix,group,clusterName}` |
| Consul | `consul.ConsulClient` | `go.consul.{lan,lanNet,server,port,group,clusterName}` |
| Polaris | `polaris.PolarisClient` | `go.polaris.{token,lan,lanNet,server,port,namespace}` |

差异：

- `GetServiceURL` 返回单实例 + 命中分组；`GetServices` 返回该分组全部实例。
- Nacos 的 `GetServices` 过滤不健康实例，`GetServiceURL` 不过滤。
- etcd/Consul 通过健康检查过滤（Consul passingOnly）；Polaris 不过滤。
- 缓存 TTL 来自 `go.discovery.cacheTTL/cacheRefresh`。

---

## 6. `pkg/client` + `pkg/loadbalancer`

### 6.1 调用 API

```go
type Options struct {
    Method       string
    Protocol     string                 // x-form | json | restful | file
    Group        string
    Header       any
    Query        any
    Data         any
    Json         any
    Path         map[string]string
    Files        []grequests.FileUpload
    Retry        bool
    LoadBalancer string                 // 注册名
}

type ResilienceOptions struct {
    Timeout        time.Duration
    MaxRetries     int
    BaseDelay      time.Duration
    MaxDelay       time.Duration
    RetryOnStatus  func(code int) bool
    Breaker        *CircuitBreaker
}

func Call(service, uri string, op *Options) (string, error)
func CallCtx(ctx context.Context, service, uri string, op *Options) (string, error)
func CallT[T any](service, uri string, op *Options) vo.Response[T]
func CallResultT[T any](service, uri string, op *Options) models.Result[T]
func CallCtxT[T any](ctx context.Context, service, uri string, op *Options) models.Result[T]
func CallPageT[T any](service, uri string, op *Options) vo.ResponsePage[T]

func CallResilient(service, uri string, op *Options, ro *ResilienceOptions) (string, error)
func CallResilientCtx(ctx context.Context, service, uri string, op *Options, ro *ResilienceOptions) (string, error)

func GetBreaker(name string) *CircuitBreaker
func ResetBreakers()
```

常量：`CONTENT_TYPE_FORM`、`CONTENT_TYPE_JSON`、`CONTENT_TYPE_RESTFUL`、`CONTENT_TYPE_FILE`。

错误：

| 变量 | 值/含义 |
|------|---------|
| `ErrAllInstancesCircuitOpen` | 所有实例被熔断，快速失败（**不回退**单实例） |
| `ErrCircuitOpen` | 韧性层熔断打开 |
| `ErrCallTimeout` | 韧性层超时 |

### 6.2 熔断器

```go
type State int
const (StateClosed State = 0; StateHalfOpen State = 1; StateOpen State = 2)

func NewCircuitBreaker(name string, maxFailures int, interval, timeout time.Duration) *CircuitBreaker
func (b *CircuitBreaker) Name() string
func (b *CircuitBreaker) Allow() bool
func (b *CircuitBreaker) Success()
func (b *CircuitBreaker) Failure()
func (b *CircuitBreaker) State() State
func (b *CircuitBreaker) Counts() (failures, successes int)
func (b *CircuitBreaker) Reset()
```

默认值：`maxFailures=5`、`interval=60s`、`timeout=30s`、`maxHalfOpen=1`。

### 6.3 负载均衡

```go
type LoadBalancer interface {
    Pick(instances []string, key string) (string, error)
}

func Register(name string, lb LoadBalancer)
func Get(name string) LoadBalancer     // 未命中返回 nil
func SetDefault(lb LoadBalancer)
func Default() LoadBalancer

var RoundRobinLB, RandomLB, LeastConnectionsLB, ConsistentHashLB
var ErrEmptyInstances
```

注册名（**只有这四个**）：`round_robin`、`random`、`least_connections`、`consistent_hash`。

- `least_connections` 需业务调用 `LeastConnectionsLB.Inc(host)`，返回 `dec` 函数；否则恒选第一个。
- `consistent_hash` 用 FNV-1a 取模，`replicas` 字段未使用（无虚拟节点）。
- `key` 为空时一致性哈希退化为轮询。

---

## 7. 可观测性

### 7.1 `pkg/health`

```go
func Router(g *gin.RouterGroup)                 // /live /ready /startup
func HealthzHandler(c *gin.Context)             // /healthz，等价 /health/live
func SetVersion(v string)
func MarkStarted()
func ResetStarted()
func IsStarted() bool
func StartedAt() time.Time
func CheckDependencies() (bool, map[string]string)
```

响应：

```json
// /health/live 与 /healthz
{"code":1,"msg":"success","srvTime":"...","data":{"status":"UP","version":"1.0.0","uptime":12}}

// /health/ready
{"status":"ok","dependencies":{"mysql":"ok","redis":"fail"}}

// /health/startup
{"status":"ok"}
```

`/ready` 失败返 503，`/startup` 未标记返 503。依赖仅 `mysql`/`mongodb`/`redis`。

### 7.2 `pkg/metrics`

```go
func SetBuildInfo(version, commit, goVersion string)
func SetPluginHealth(name string, healthy bool)
func SetDependencyUp(name string, up bool)
func Middleware() gin.HandlerFunc
func Handler() http.Handler
```

6 个指标：`mgin_http_requests_total`、`mgin_http_request_duration_seconds`、
`mgin_http_requests_in_flight`、`mgin_build_info`、`mgin_plugin_health`、`mgin_dependency_up`。

### 7.3 `pkg/otel`

```go
func SetTracerProvider(tp trace.TracerProvider)
func Disable()
func Tracer() trace.Tracer
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
func IsEnabled() bool
```

不提供 `Init`/`Shutdown`，不依赖 SDK/Exporter。

### 7.4 `pkg/logs` 与 `pkg/logsink`

```go
func Debug(format string, v ...any)
func Info(format string, v ...any)
func Warn(format string, v ...any)
func Error(format string, v ...any)
func OutPrint(format string, v []any) string

func InitVictoriaLogs() error
func RegisterHandler(h logsink.RecordHandler, opts ...logsink.Option) error
func FormatRecord(r *logsink.Record, output config.LogOutput) ([]byte, error)
```

- 占位符是 `{}`，按参数顺序替换首个 `{}`；不是 `%s`。
- 级别：`go.logger.level`，每次输出实时读取；`go.application.debug=true` 强制 debug。
- 多目标：`go.logger.outputs[]`（`target/format/fields/logtext/file`）。

`logsink`：

```go
type Handler interface { Name() string; Write(*models.PostLog) error }
type RecordHandler interface { Name() string; Write(*Record) error }

func Register(h Handler, opts ...Option)
func Emit(e *models.PostLog)
func EmitRecord(r *Record)
func EmitRecordTo(target string, r *Record)
func Stats(name string) Stats
func Close()
```

参数：`WithQueueSize(n)`（默认 1024）、`WithWorkers(n)`（恒归 1，保证 FIFO）。

---

## 8. 中间件

| 包 | 构造函数 | 说明 | 配置/头 |
|----|----------|------|---------|
| `trace` | `TraceId() gin.HandlerFunc`、`Headers()` | 生成/透传 trace | `traceparent`、`X-Trace-Id`、`X-Request-ID`、`X-Real-IP` |
| `postlog` | `RequestLogger()`、`RegisterHandler(h, opts...)`、`Close()` | 访问日志 | `go.log.*` |
| `cors` | `Cors(headers ...string)` | CORS，`*` 且不允许凭证 | `go.cors.headers` |
| `xlang` | `RequestLanguage()`、`GetCurrentLanguage() string` | 语言上下文 | `X-Lang`、`go.i18n.default` |
| `jwt` | `JwtAuthorize()` | Bearer 校验 | `go.jwt.secret` |
| `casbin` | `CasbinHandler()` | 用户/角色双校验，失败 401/403 | `go.casbin.enabled` |
| `session` | `New(...)`、`NewWithConfig(...)`、`FromContext`、`Refresh`、`Destroy` | 会话 | — |
| `limit` | `MaxAllowed(n int)` | 并发信号量（阻塞） | — |
| `iplimit` | `CIDR(CIDRs string)` | CIDR 白名单，失败 403 | — |
| `ratelimit` | `RateLimit()`、`RateLimitWith(rules ...Rule)`、`GetManager()`、`Allow(key, rule)` | 三算法限流 | `go.ratelimit.*` |
| `xss` | `(&XssMw{}).RemoveXss()` | bluemonday 净化 | — |

### 8.1 限流规则

```go
type Rule struct {
    Name, Path string
    Methods    []string
    Algorithm  string  // token_bucket | sliding_window | concurrency
    Dimension  string  // global | ip | path | ip_path | header
    HeaderKey  string
    Rate       float64
    Burst      int
    Window     int
    WindowMs   int
    MaxConcurrent int
    Wait       bool
    WaitTimeoutMs int
    Code       int
    Message    string
    HttpStatus int
}
```

默认：`Algorithm=token_bucket`、`Dimension=global`、`Window=1`、`Code=1011`、`HttpStatus=200`。
响应头：`X-RateLimit-Limit/Remaining/Policy`、`Retry-After`（`headers=true` 时）。

### 8.2 框架挂载顺序（`app.baseRouter`）

```
gin.Default()
→ /health/*（条件）
→ /metrics（条件）
→ trace.TraceId()
→ postlog.RequestLogger()
→ cors.Cors(...)
→ xlang.RequestLanguage()（i18n）
→ ratelimit.RateLimit()（go.ratelimit.enabled）
→ nice.Recovery
→ NoRoute
→ /i18n、/job（条件）
```

`jwt`、`casbin`、`session`、`xss`、`iplimit`、`limit` 由业务自行 `Use`。

---

## 9. 响应模型、错误码与 i18n

### 9.1 `pkg/models`

```go
type Result[T any] struct {
    Status int
    Msg    string
    Data   T
    Page   *ResultPage
}
type ResultPage struct { Count, Index, Size, Total int }

func Success[T any](data T) Result[T]                     // Status=1, Msg="Success"
func SuccessWithMsg[T any](msg string, data T) Result[T]
func SuccessPage[T any](data T, page *ResultPage) Result[T]
func SuccessWithPage[T any](data T, count, index, size, total int) Result[T]
func Error(s int, m string) Result[any]
func ErrorT[T any](s int, m string) Result[T]
func (r Result[T]) ToAny() Result[any]
func ToAny[T any](result Result[any]) Result[T]           // 类型不一致会 panic
```

### 9.2 `pkg/vo`

```go
type Response[T any] struct {
    Code    int
    Data    T
    Msg     string
    SrvTime time.Time
}
type ResponsePage[T any] struct { Code int; Msg string; SrvTime time.Time; Data *PageData[T] }
type PageData[T any] struct { List []T; Total int64; Page int; PageSize int }

const (ERROR = 500; SUCCESS = 1; UnLogin = 999)

func Success(data any) Response[any]
func SuccessT[T any](data T) Response[T]
func SuccessPage(data any, page *models.ResultPage) ResponsePage[any]
func Error(status int, msg string) Response[any]
func ErrorT[T any](status int, msg string) Response[T]
func ToResponse(r models.Result[any]) Response[any]
func ToResponsePage(r models.Result[any]) ResponsePage[any]
func ResponseToT[T any](result Response[any]) Response[T]
func ResponsePageToT[T any](result ResponsePage[any]) ResponsePage[T]
```

### 9.3 `pkg/errcode`

```go
type Definition struct {
    Code       int
    HTTPStatus int
    MessageKey string
    Module     string
}

func New(module string, code, httpStatus int, messageKey string) Definition
func RegisterDef(d Definition)
func LookupDef(code int) Definition
func AllDefs() map[int]Definition
```

内置错误码：

| 常量 | Code | HTTP | MessageKey |
|------|------|------|------------|
| `URI_NOT_FOUND` | 1000 | 404 | errcode.uri_not_found |
| `SYSTEM_ERROR` | 1001 | 500 | errcode.system_error |
| `DB_CONNECT_ERROR` | 1002 | 503 | errcode.db_connect_error |
| `REQUEST_PARAMETER_LOST` | 1003 | 400 | errcode.request_parameter_lost |
| `DATA_NOT_FOUND` | 1004 | 404 | errcode.data_not_found |
| `USER_NOT_FOUND` | 1005 | 401 | errcode.user_not_found |
| `PASSWORD_ERROR` | 1006 | 401 | errcode.password_error |
| `VERIFY_CODE_ERROR` | 1007 | 400 | errcode.verify_code_error |
| `TOKEN_ERROR` | 1008 | 401 | errcode.token_error |
| `AUTHENTICATION_FAILURE` | 1009 | 401 | errcode.authentication_failure |
| `SERVICE_UNAVAILABLE` | 1010 | 503 | errcode.service_unavailable |
| `PARAM_ERROR` | 1014 | 400 | errcode.param_error |
| `TOO_MANY_REQUESTS` | 1011 | 429 | errcode.too_many_requests |
| `JOB_ERROR` | 1012 | 500 | errcode.job_error |
| `STORAGE_ERROR` | 1013 | 500 | errcode.storage_error |

模块常量：`common/auth/user/database/job/storage/client/plugin`。

### 9.4 `pkg/i18n`

```go
func GetString(messageID, language string) string
func String(messageId string) string                 // 用当前协程语言；缺失返回 messageID
func Format(messageId string, args ...any) string    // {} 顺序替换
func Error(code int, messageId string) models.Result[any]
func ErrorT[T any](code int, messageId string) models.Result[T]
func ErrorWithMsg(code int, messageId, msg string) models.Result[any]
func ErrorWithMsgT[T any](code int, messageId, msg string) models.Result[T]
func ErrorDef(def errcode.Definition, args ...any) models.Result[any]
func ErrorDefT[T any](def errcode.Definition, args ...any) models.Result[T]
func Success[T any](data T) models.Result[T]
func SuccessWithPage[T any](data T, count, index, size, total int) models.Result[T]
func ParamLostError(param string) models.Result[any]
func ParamError(param string) models.Result[any]
func CheckParametersLost(params map[string]string, paramNames ...string) models.Result[any]

func Init() error
func GetStore() (*Store, error)
func Close()
func RouterGroup(group *gin.RouterGroup)   // /i18n CRUD
```

`Store`：`NewStore(conn *gorm.DB, appName string)`、`Get/Create/Update/Delete/List/Translate`。
表名 `<appName>_i18n`，主键 `(message_id, language)`。

---

## 10. `pkg/job` — 定时任务

```go
type HandlerFunc func(ctx *Context) error

func Register(name string, h HandlerFunc)
func Unregister(name string)
func GetHandler(name string) HandlerFunc
func ListHandlers() []string

func Start() error
func Stop()
func GetManager() *Manager
func (m *Manager) Start() error
func (m *Manager) Stop()
func (m *Manager) Check() error
func (m *Manager) IsRunning() bool
func (m *Manager) Trigger(id int64, param map[string]interface{}) (int64, error)
func RouterGroup(g *gin.RouterGroup)
```

`Context` 字段：`JobId`、`JobName`、`JobGroup`、`HandlerName`、`Param`、`LogId`、
`TriggerType`、`RetryNum`；方法：`Done()`、`Err()`、`Deadline()`、`Ctx()`、`Log(format, args...)`。

管理接口（挂到 `/job`）：`GET /list`、`GET /:id`、`POST ""`、`PUT ""`、`DELETE /:id`、
`POST /:id/start`、`POST /:id/stop`、`POST /:id/trigger`、`GET /handlers`、`GET /log`。

---

## 11. `pkg/cache` 与 `pkg/casbin`

### 11.1 cache

```go
type ICache interface {
    Add(key string, value interface{}, lifeSpan time.Duration) error
    Value(key string) (interface{}, bool)
    IsExist(key string) bool
    Clear() bool
    Get(key string) interface{}
    Set(key string, value interface{}, duration time.Duration)
    Range(f func(key, value interface{}) bool)
    Delete(key string)
    Close()
}

func OnDiskCache(cachePath string) *DiskCache
func OnGetCache(name string, persistent ...bool) ICache
func OnMemCache(name string) ICache
func CloseCache()
```

- memcache：`sync.Map` + 定期清理 ticker，`Add` 使用深拷贝。
- diskcache：bitcask，默认文件 `<可执行文件目录>/cache.db`。
- Redis 版 `ICache` 在 `pkg/db/dao/redis_cache.go`（`NewRedisCache(client)`）。

### 11.2 casbin

```go
type CasbinInfo struct { Path, Method string }

var Casbin = &casbinService{...}

func (c *casbinService) GetEnforcer() *casbin.SyncedCachedEnforcer
func (c *casbinService) UpdateCasbin(RoleID uint, casbinInfos []CasbinInfo) error
func (c *casbinService) UpdateCasbinApi(oldPath, oldMethod, path, method string) error
func (c *casbinService) GetPolicyPathByRoleId(RoleId uint) ([]CasbinInfo, error)
func (c *casbinService) ClearCasbin(v int, p ...string) bool
func (c *casbinService) FreshCasbin() error
```

模型（`casbin.conf`）：`r = sub,obj,act`，`p = sub,obj,act`，matcher 为
`r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act`。

---

## 12. `pkg/utils` 工具分组

| 组 | 文件 | 代表函数 |
|----|------|----------|
| 对称/非对称加密 | `aesutil.go`、`desutil.go`、`3desutil.go`、`rsautil.go` | `AESBase64Encrypt(origin, key, iv)`、`AESEncrypt`、`AesDecryptEcb`、`PublicEncrypt`、`PriKeyDecrypt` |
| 摘要 | `md5util.go`、`shautil.go`、`hmacutil.go`、`base64util.go` | `MD5Encode`、`FileMD5`、`Sha256`、`HmacSHA256Base64`、`Base64Encode` |
| JWT | `jwtutil.go` | `GenerateToken(claims jwt.MapClaims)`（HS256，exp=+24h）、`ValidateToken` |
| 传输 | `sftputil.go`、`downloadutil.go`、`ziputil.go` | `SftpConnect`、`SftpUploadFile`、`DownloadFile`、`ZipFiles` |
| 并发/缓存 | `limit.go`、`expire_cache.go`、`safemap.go`、`safego.go` | `LimitFreqSingle`、`ExpireCache.Store/Load`、`NewSafeGo` |
| 条件 | `conditionutil.go`、`checkutil.go`、`sqlvalidate.go` | `IfThen`、`DefaultIfNil`、`CheckSqlValidate` |
| ID/随机 | `uuidutil.go`、`randutil.go` | `NewUUIDString`、`GetRandomString`、`GetUUIDString` |
| Gin | `ginutil.go` | `GinParamMap`、`GinHeaders` |
| 字符串 | `stringutil.go`、`chineseutil.go`、`emojiutil.go`、`utf8gbkutil.go` | `IsBlank`、`AnyToString`、`StringToAny[T]`、`TrimEmoji` |
| 时间 | `timeutil.go` | `GetNowDateTime`、`DateFormat`、`Get0Hour`、`GroupByWeekDate` |
| 集合/容器 | `sliceutil.go`、`maputil.go`、`structutil.go`、`hashset.go`、`linklist.go`、`ringbuffer.go`、`values.go` | `Map2Struct`、`Struct2Map`、`DeepCopy[D]`、`NewRingBuffer[T]` |
| 文件/序列化 | `fileutil.go`、`jsonutil.go`、`xmlutil.go`、`ymlutil.go` | `ToJSON`、`FromJSON`、`LoadYaml` |
| 系统/IP | `iputil.go`、`sysutil.go`、`app.go`、`firewall.go`、`sizeutil.go` | `GetLocalIpAddress`、`IsIntranetIP`、`CmdExec`、`AppName`、`DisplaySize` |

---

## 13. `pkg/models/sys` — 内置系统模型

`BaseModel`（被多数表嵌入）：

```go
type BaseModel struct {
    DelFlag  uint      // del_flag tinyint, default 0, index
    CreateAt time.Time // create_at datetime
    UpdateAt time.Time // update_at datetime
    UpdateBy string    // update_by varchar(32)
    CreateBy string    // create_by varchar(32)
    TenantId int64     // tenant_id 声明为 string(32)（类型与 DDL 不一致，见数据设计）
}
```

13 个模型：`SysUser`、`SysUserExt`、`SysRole`、`SysApi`、`SysResource`、`SysRoleApi`、
`SysRoleResource`、`SysDept`、`SysPost`、`SysDict`、`SysConfig`、`SysSocial`、`SysUserOnline`。
字段清单见 [data-design.md](data-design.md)。

请求 DTO 在 `pkg/models/sys/request`（`LoginReq`、`ChangePasswordReq`、`ListSysUserReq` 等），
出参聚合在 `pkg/models/sys/vo`。

**框架不提供 CRUD 实现与建表迁移**。

---

## 14. 接口变更兼容矩阵

| API | v1 | v2 | 兼容性 |
|-----|----|----|--------|
| `mgin.NewApp/Run/Init/SafeExit` | ✓ | ✓ | 一致 |
| `mgin.Use/UsePlugin/MginPlugin` | ✓ | ✓ | 一致 |
| `client.Call` | ✓ | ✓ | 一致（内部委托 CallCtx） |
| `client.CallCtx` / `CallResilient*` | ✗ | ✓ | 新增 |
| `client.Options.LoadBalancer` | ✗ | ✓ | 新增字段 |
| `registry.GetServices` | ✗ | ✓ | 新增 |
| `errcode.New/RegisterDef/LookupDef` | ✗ | ✓ | 新增 |
| `health.Router/MarkStarted` | ✗ | ✓ | 新增 |
| `metrics.Handler/Middleware` | ✗ | ✓ | 新增 |
| `otel.*` | ✗ | ✓ | 新增 |
| import 路径 | 顶层包 | `pkg/` | **破坏**，见迁移文档 |
| NoRoute / Recovery HTTP 状态 | 200 | 404 / 500 | **行为变化** |
