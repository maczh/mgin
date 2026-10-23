# 模块与接口说明（jh 分支）

本文按包列出**对外可用的导出符号、签名与约束**。示例中的模块路径为 `github.com/maczh/mgin`；业务工程替换为实际 module 即可。

约定：

- 所有 `Init` 系方法失败只打日志，不返回 error（`config.Init` 与 `i18n.Init` 除外）。
- 返回 `*gorm.DB` 的方法可能因连接失败返回 `nil`，调用前判空。
- 「门面」指 `db.Mysql` 等包级单例；一个进程只应初始化一次。

---

## 1. 入口与运行时（根包 `mgin`）

### `mgin.NewApp`

```go
func NewApp(configFile, appName, version string, xlang bool) *App
```

- `configFile` 为空时用 `-f`（默认 `<可执行文件名>.yml`）；不含路径分隔符时拼到可执行文件目录。
- `xlang=true` 或 `go.i18n.enabled=true` 时初始化 i18n；失败 `panic`。
- 副作用：注册 `-v` 标志；启动 5 分钟一次的 `checkAll` 协程；完成 `baseRouter()`。

### `App` 方法

| 方法 | 说明 |
|------|------|
| `Run()` | 启动 HTTP（`port>0`）与 HTTPS（`cert` 非空），阻塞等待信号；关闭超时 5s |
| `EnableHealth()` | 挂载 `/health/*` 与 `/healthz`；重复调用安全 |
| `EnableMetrics()` | 挂载 `/metrics`；重复调用安全 |
| `MarkHealthStarted()` | 等价 `health.MarkStarted()` |
| `GetVersion() string` | 返回构造时传入的版本号 |
| `Router *gin.Engine` | 业务注册路由与中间件 |

### 包级函数与全局

| 符号 | 说明 |
|------|------|
| `Init(configFile string)` | 配置 + VictoriaLogs + 资源初始化（`NewApp` 内部调用，也可单独使用） |
| `MGin.Use(name string, init dbInitFunc, close dbCloseFunc, check dbCheckFunc)` | 按 `used` 注册并初始化外部组件 |
| `MGin.UsePlugin(name string, p MginPlugin)` | `Use` 的对象化版本（`Init([]byte)/Close()/Check() error`） |
| `MGin.Reload(configFile string) error` | 配置热重载（串行）；供 `/cfg/reload` 使用 |

---

## 2. 配置（`config`）

### 单例与结构

```go
var Config *config
```

主要分组（均为导出字段）：

| 字段 | 内容 |
|------|------|
| `Cnf` | koanf 实例，可直接读任意键 |
| `WorkDir` | 配置文件所在目录（相对路径基准） |
| `App` | `Name/Project/Port/PortSSL/Cert/Key/Debug/IpAddr` |
| `Config` | `Server/Type/Token/ReloadToken/Path/Env/Used/Prefix{...}` |
| `Logger` / `Log` | 日志目标、等级、VictoriaLogs、sink、访问日志表名 |
| `Discovery` | `Registry/CallType/CacheTTL/CacheRefresh` |
| `Jwt` / `Sys` / `Casbin` | 对应配置段 |

### API

```go
func (c *config) Init(configFile string)
func (c *config) GetConfigString(key string) string
func (c *config) GetConfigStringArray(key string) []string
func (c *config) GetConfigInt(key string) int
func (c *config) GetConfigBool(key string) bool
func (c *config) Exists(key string) bool
func (c *config) GetConfigData(prefix string) []byte   // 取整份子配置
func (c *config) GetConfigUrl(prefix string) string
```

- 读取方法均加读锁；`Cnf == nil`（加载失败）时返回零值。
- `GetConfigData` 的分支：`etcd` / `polaris` / `file` / 默认 HTTP。
- `ReloadToken` 非空才会注册 `/cfg/reload`。

---

## 3. 数据访问（`db`、`db/dao`）

### 3.1 门面

```go
var (
    Mysql  *mysql.MysqlClient
    Mongo  *mongo.Mongodb
    Redis  redis.Redis         // db/redis 包内单例
    Sqlite *sqlite.Sqlite
)
```

> 本分支**没有** `db.Pg`、`db.Clickhouse`、`db.ElasticSearch`、`db.Kafka`。

### 3.2 MySQL（`db.Mysql`）

| 方法 | 签名 | 说明 |
|------|------|------|
| `Init` | `Init(cnf []byte)` | koanf 解析；支持单库/多库与连接池 |
| `Close` | `Close()` | 关闭全部连接 |
| `Check` | `Check() error` | 失败自动 Close + 重新 Init 再 ping 一次 |
| `GetConnection` | `GetConnection(dbName ...string) (*gorm.DB, error)` | 多库必须传且只传一个库名 |
| `IsMultiDB` | `IsMultiDB() bool` | — |
| `ListConnNames` | `ListConnNames() []string` | — |

配置键：`go.data.mysql`、`go.data.mysql.multidb`、`go.data.mysql.dbNames`、`go.data.mysql.<db>`、`go.data.mysql_debug`、`go.data.mysql_pool.{max,total,timeout,life}`。
连接池仅当 `pool.max > 1` 生效；`max<10` 抬到 10，空闲数缺省 `5*max`。

### 3.3 SQLite（`db.Sqlite`）

```go
Init(dbFileName string)          // 空名 → <可执行目录>/<go.application.name>.db
Close()
Check() error
GetConnection() (*gorm.DB, error) // 未初始化返回 "SQLite not opened"
```

纯 Go 驱动（`glebarez/sqlite`），无需 CGO。`go.config.prefix.sqlite` 的值就是文件名。

### 3.4 MongoDB（`db.Mongo`）

```go
Init(cnf []byte); Close(); Check() error
GetConnection(dbName ...string) (*mgo.Database, error)
ReturnConnection(conn *mgo.Database)   // 必须与 GetConnection 成对
IsMultiDB() bool; ListConnNames() []string
```

多库模式 `dbName` 为空时取第一个库。`Check()` 失败会关闭 session 并按 uri 重连。

### 3.5 Redis（`db.Redis`）

```go
Init(cnf []byte); Close(); Check() error   // Check 当前恒返回 nil
GetConnection(dbName ...string) (redis.UniversalClient, error)
IsMultiDB() bool; ListConnNames() []string
PSubscribe(dbName string, handler func(*redis.Message, string), channels ...string)
```

`PSubscribe` 内含无限重连循环，`{db}` 占位会替换为库号。

### 3.6 事务（`db`）

```go
func MySQLTx[T any](ctx context.Context, fn func(tx *gorm.DB) (T, error), dbName ...string) (T, error)
func TransactionOn[T any](ctx context.Context, conn *gorm.DB, fn func(tx *gorm.DB) (T, error)) (T, error)
```

回调内必须使用 `tx`；回调返回 error 即回滚，错误被包装为 `transaction callback: ...` / `transaction failed: ...`。

### 3.7 泛型 DAO（`db/dao`）

```go
type Dao[E any] interface {
    Insert(entity *E) error
}
```

> 接口只有 `Insert`，而 `MySQLDao` 的方法叫 `Create`，因此**只有 `MgoDao` 满足** `Dao[E]`；业务直接用具体类型。

#### `MySQLDao[E schema.Tabler]`

```go
type MySQLDao[E schema.Tabler] struct {
    debug bool
    ctx   *context.Context
    Tag   func() string
    cache *CacheConfig
    tx    *gorm.DB
}
```

| 分类 | 方法 |
|------|------|
| 链式 | `Debug()`, `WithTx(*gorm.DB)`, `WithContext(*context.Context)`, `WithCache(CacheConfig)` |
| 写 | `Create(*E)`, `MultiCreate([]*E)`, `Updates(*E)`, `Save(*E)`, `Delete(E)` |
| 读 | `All(E, ...QueryOption)`, `One(E)`, `Exists(E) bool`, `Count(E)`, `Pager(*gorm.DB, page, size)` |
| 条件 | `Where(query, args...) *gorm.DB`, `WhereAll`, `WhereOne`, `WhereCount`, `WhereExists` |
| 原生 | `Alias(string) *gorm.DB`, `DB() *gorm.DB`, `JOIN(alias, query, args...) *gorm.DB` |

辅助类型：

```go
type QueryOption struct { Preloads []string; OrderBy []string }
type CacheConfig struct { Enabled bool; Store cacheStore.ICache; TTL time.Duration; Prefix string }
```

注意事项（均已在代码确认）：

- `WithContext` **修改接收者自身**并返回自身；并发场景先复制。
- `Where/DB/Alias` 连接失败返回 `nil`。
- `One` 对 `ErrRecordNotFound` 返回 `(nil, nil)`，且不写缓存。
- `Updates(*E)` 不带 Where，生产更新请用 `DB()/Where(...)`。
- `Pager` 不参与缓存；`size==0` 时取 20。

#### `MgoDao[E any]`

```go
type MgoDao[E any] struct {
    CollectionName string   // 必须手动赋值，否则所有方法返回错误
    Tag            func() string
}
```

`Insert(*E)`、`Delete(bson.M)`、`Updates(id primitive.ObjectID, doc E)`、`Update(selector, update bson.M)`、`All(bson.M)`、`One(bson.M)`、`Pager(query, sort []string, page, size)`。
不支持 `Debug/WithContext/WithTx/缓存/Count/Exists`。

#### `RedisCache`

```go
func NewRedisCache(client redis.UniversalClient) *RedisCache
```

实现 `cacheStore.ICache`：`Add/Value/IsExist/Clear/Get/Set/Range/Delete/Close`。
⚠️ `Clear()` 执行 `FLUSHDB`，`Range` 全库 `SCAN`，键不加前缀。

---

## 4. 缓存（`cache`）

```go
type ICache interface {
    Add(key, value any, lifeSpan time.Duration)
    Value(key any) (any, bool)
    IsExist(key any) bool
    Clear() bool
    Get(key any) (any, bool)
    Set(key any, value any, duration time.Duration)
    Range(f func(key, value any) bool)
    Delete(key any)
    Close()
}
```

| 工厂 | 后端 | 说明 |
|------|------|------|
| `OnMemCache(name string) ICache` | 内存 | 同名复用；后台每分钟清理过期；无容量上限 |
| `OnDiskCache(path string) ICache` | bitcask | 相对路径基于可执行目录；空名 `cache.db`；`Get` 返回 string |
| `OnGetCache(name string, persistent ...bool)` | 二者之一 | `persistent[0]==true` 走磁盘 |
| `CloseCache()` | — | 关闭全部实例 |

内存实现用 `clone.Clone` 做深拷贝；磁盘实现 `lifeSpan<=0` 表示永久。

---

## 5. 服务调用（`client`）

```go
const (
    CONTENT_TYPE_FORM = "x-form"
    JSON              = "json"
    RESTFUL           = "restful"
    FILE              = "file"
)

type Options struct {
    Method   string
    Protocol string
    Group    string
    Header   map[string]string
    Query    map[string]string
    Data     map[string]string
    Json     any
    Path     map[string]string
    Files    []grequests.FileUpload
    Retry    bool
}

func Call(service, uri string, op *Options) (string, error)
func CallT[T any](service, uri string, op *Options) vo.Response[T]
func CallPageT[T any](service, uri string, op *Options) vo.ResponsePage[T]
func CallResultT[T any](service, uri string, op *Options) models.Result[T]
```

行为：

- `Protocol` 为空取 `go.discovery.callType`；`Files` 非空强制 `file`。
- 地址来自 `registry.Registry.GetServiceURL(service, op.Group)`；未初始化直接返回错误。
- 自动带上 trace 头（`X-Request-ID`/`X-Real-IP`/`X-User-Agent`）。
- HTTP 非 2xx 视为错误；`Retry=true` 只重试一次；**无超时**。
- `go.log.victorialogs.client=true` 时异步记录调用日志。

---

## 6. 注册与发现（`registry`）

```go
type RegistryClient interface {
    Register(registryConfigData []byte)
    GetServiceURL(servicename string, groupName ...string) (string, string)
    DeRegister()
}
var Registry RegistryClient
func NewRegistry() RegistryClient
```

| 实现 | 选择键 | 关键配置 |
|------|--------|----------|
| Nacos | `go.discovery.registry: nacos` | `go.nacos.{server,port,group,clusterName,lan,lanNet,apiversion}` |
| Consul | `consul` | `go.consul.{server,port,group,clusterName,lan,lanNet}` |
| etcd | `etcd` | `go.etcd.{server,port,prefix,group,clusterName,lan,lanNet}` |
| Polaris | `polaris` | `go.polaris.{server,port,token,namespace}`（**不会自动注册**，需手动 `Register`） |
| 空/未知 | — | `noopRegistryClient`，不报错 |

`NewCachedRegistry` 提供：TTL 缓存（`cacheTTL`）、并发 singleflight、后台热刷新（`cacheRefresh`）、注册中心无实例时降级复用旧缓存。

---

## 7. 中间件（`middleware`）

| 包 | 导出 | 触发/配置 | 失败响应 |
|----|------|-----------|----------|
| `trace` | `TraceId()`, `Headers()`, `GetRequestId/GetClientIp/GetUserAgent/GetHeader/SetHeader/GetHeaders/CopyPreHeaderToCurRoutine` | 恒启用 | 不拦截 |
| `postlog` | `RequestLogger()`, `RegisterHandler`, `Close` | 恒启用；落库需 `go.log.req` + mongodb | 不拦截 |
| `cors` | `Cors(headers ...string)` | `go.cors.headers` 追加允许头 | 预检 204 |
| `ratelimit` | `RateLimit()`, `RateLimitWith(...)`, `GetManager()`, `LoadConfig/LoadConfigFrom` | `go.ratelimit.enabled` | 默认 200 + `status=1011` |
| `jwt` | `JwtAuthorize()` | 业务挂载；`go.jwt.secret` | 401，无 body |
| `casbin` | `CasbinHandler()` | `go.casbin.enabled`；需在 jwt 之后 | 401/403/500（`models.Error`） |
| `xss` | `(*XssMw).RemoveXss()` | 业务挂载；`StrictPolicy`/`UGCPolicy` | 策略错误 500 |
| `iplimit` | `CIDR(CIDRs string)` | 业务挂载 | 403 |
| `limit` | `MaxAllowed(n int)` | 业务挂载；`n<=0` 不限 | 499（客户端断开） |
| `session` | `New(...)`, `NewWithConfig`, `FromContext`, `Destroy`, `Refresh` | 业务挂载 | 500 |
| `xlang` | `RequestLanguage()`, `GetCurrentLanguage()` | i18n 开启时自动挂载 | 不拦截 |

CORS 细节：允许方法固定 `POST/GET/OPTIONS/PUT/DELETE/PATCH/HEAD/UPDATE`；有 `Origin` 时回显并允许凭证，无 `Origin` 时返回 `*` 且不带凭证。

---

## 8. 返回结构（`models`、`models/vo`）

```go
// models.Result —— 业务接口统一返回
type Result[T any] struct {
    Status int         `json:"status"`
    Msg    string      `json:"msg"`
    Data   T           `json:"data,omitempty"`
    Page   *ResultPage `json:"page,omitempty"`
}
type ResultPage struct{ Count, Index, Size, Total int }

// models/vo.Response —— 跨服务调用返回
type Response[T any] struct {
    Code    int       `json:"code"`
    Data    T         `json:"data"`
    Msg     string    `json:"msg"`
    SrvTime time.Time `json:"srvTime"`
}
type ResponsePage[T any] struct {
    Code int; Msg string; SrvTime time.Time
    Data *PageData[T]
}
type PageData[T any] struct{ List []T; Total int64; Page, PageSize int }

const ( ERROR = 500; SUCCESS = 1; UnLogin = 999 )
```

构造函数：`models.Success/SuccessWithMsg/SuccessPage/Error/ErrorT/ToAny`；`vo.Success/SuccessT/SuccessPage/Error/ErrorT/ResponseToT/ResponsePageToT`；类型转换 `ToResponse/ToResponsePage/ToAny`。

`models.PostLog` 为接口日志结构（`time/requestId/responseTime/ttl/appName/apiName/method/contentType/uri/clientIP/requestHeader/requestParam/requestBody/responseStr/responseMap`）。

---

## 9. 国际化与错误码（`i18n`、`errcode`）

### i18n

```go
func Init() error
func GetStore() (*Store, error)
func String(messageId string) string                 // 按当前协程语言
func GetString(messageId, language string) string
func Format(messageId string, args ...any) string    // 顺序替换 {}
func Error(code int, messageId string) models.Result[any]
func ErrorT[T any](code int, messageId string) models.Result[T]
func ErrorWithMsg(code int, messageId, msg string)
func Success[T any](data T) models.Result[T]
func SuccessWithPage[T any](data T, count, index, size, total int) models.Result[T]
func ParamLostError(param string) models.Result[any]
func ParamError(param string) models.Result[any]
func RouterGroup(g *gin.RouterGroup)                 // GET /list /get；POST /save /update /delete
```

- 存储：优先 `used` 中的 MySQL（`go.i18n.dbName` 或第一个库），否则 SQLite（`go.i18n.sqlite`，缺省 `{app}_i18n.db`）。
- 表：`{appName}_i18n(message_id, language, text, created_at, updated_at)`，主键 `(message_id, language)`。
- 语言：`X-Lang`，`_` 归一化为 `-`，缺省 `zh-cn`；未命中回退默认语言，再未命中返回 messageId。
- 未初始化：`GetString` 返回空串，`String` 返回 messageId，不 panic。

### errcode

```go
const (
    URI_NOT_FOUND = 1000; SYSTEM_ERROR = 1001; DB_CONNECT_ERROR = 1002
    REQUEST_PARAMETER_LOST = 1003; DATA_NOT_FOUND = 1004; USER_NOT_FOUND = 1005
    PASSWORD_ERROR = 1006; VERIFY_CODE_ERROR = 1007; TOKEN_ERROR = 1008
    AUTHENTICATION_FAILURE = 1009; SERVICE_UNAVAILABLE = 1010
    TOO_MANY_REQUESTS = 1011; JOB_ERROR = 1012; STORAGE_ERROR = 1013
    PARAM_ERROR = 1014
)
```

`errcode` 还定义了 messageId 常量（如 `UrlNotFound="404"`、`SystemError`、`DataNotFound`、`Success="success"`），供 `i18n.Error(code, messageId)` 使用。

---

## 10. 日志（`logs`、`logsink`）

### logs

```go
func Debug(format string, v ...any)
func Info(format string, v ...any)
func Warn(format string, v ...any)
func Error(format string, v ...any)
func RegisterHandler(h logsink.RecordHandler, opts ...logsink.Option) error
func InitVictoriaLogs() error
```

- 占位符为 `{}`，按顺序替换；多余参数不报错。
- 输出目标：`console`、`file`（`<file>.YYYY-MM-DD.log`）、`victorialogs`、自定义 `RecordHandler`。
- `go.logger.outputs[]` 支持 `target/format(text|json)/fields/logtext/file`；字段白名单：`time,level,message,app,request_id,goroutine_id,package,file,function,line`。
- 消息自动带 `[requestId] ` 前缀；`goroutine_id` 由 `runtime.Stack` 解析。

### logsink

```go
type Entry = models.PostLog
func Register(h Handler, opts ...Option) error
func MustRegister(h Handler, opts ...Option)
func Unregister(name string) bool
func Emit(entry Entry)
func Close(timeout ...time.Duration)
func Stats() []Stat

func RegisterRecord(h RecordHandler, opts ...Option) error
func EmitRecordTo(name string, r Record)
```

每个 handler 独立队列 + **1 个消费协程**（保证 FIFO）；队列满淘汰最旧并计数；`Close` 先停收再排空。默认队列 1024、关闭超时 3s；配置文件上限 100。

---

## 11. 定时任务（`job`）

```go
func Register(name string, h HandlerFunc)
func Unregister(name string) bool
func GetHandler(name string) HandlerFunc
func ListHandlers() []string
func GetManager() *Manager
func Start() error
func Stop()
func RouterGroup(g *gin.RouterGroup)

type HandlerFunc func(ctx *Context) error
type Context struct {
    JobId       int64
    JobName     string
    JobGroup    string
    HandlerName string
    Param       map[string]interface{}
    LogId       int64
    TriggerType string
    RetryNum    int
}
```

- 调度类型：`cron`（5/6 段与 `@every/@daily` 等）、`fixed_rate`、`fixed_delay`、`once`。
- 阻塞策略：`serial`、`discard`、`concurrent`、`cover`；misfire：`do_nothing`、`fire_now`。
- 管理接口（挂在 `/job`）：`GET /list`、`GET /:id`、`POST ""`、`PUT ""`、`DELETE /:id`、`POST /:id/start|stop|trigger`、`GET /handlers`、`GET /log`。
- 表：`mgin_job_info`、`mgin_job_log`（前缀可配）；`initdb: true` 时 `AutoMigrate`。
- **单机调度**，多实例会重复执行。

---

## 12. 健康与指标（`health`、`metrics`）

### health

| 端点 | 响应 |
|------|------|
| `/healthz`、`/health/live` | `{"code":1,"msg":"success","srvTime":...,"data":{"status":"UP","version":...,"uptime":s}}`，恒 200 |
| `/health/ready` | 200 `{"status":"ok"}`；依赖失败 503 `{"status":"fail","dependencies":{...}}` |
| `/health/startup` | 未 `MarkStarted` 时 503 |

```go
func MarkStarted(); func ResetStarted(); func IsStarted() bool; func StartedAt() time.Time
func SetVersion(v string)
func CheckDependencies() (bool, map[string]string)
func Router(g *gin.RouterGroup)
```

依赖检查仅覆盖 `used` 中的 `mysql`/`mongodb`/`redis`（不含 SQLite）；`Check()` 可能触发重连。

### metrics

```go
func Handler() http.Handler
func Middleware() gin.HandlerFunc
func SetBuildInfo(version, commit, goVersion string)
func SetPluginHealth(name string, healthy bool)
func SetDependencyUp(name string, up bool)
```

指标：`mgin_http_requests_total{method,path,status}`、`mgin_http_request_duration_seconds{method,path}`、`mgin_http_requests_in_flight`、`mgin_build_info{version,commit,go_version}`、`mgin_plugin_health{name}`、`mgin_dependency_up{name}`。未匹配路由的 `path` 记为 `<nomatch>`。

---

## 13. 工具集（`utils`）

| 类别 | 代表函数 |
|------|----------|
| 对称加密 | `AesEncrypt/AesDecrypt`、`AESBase64Encrypt`、`DesEncrypt/DesDecrypt`、`TripleDesEncrypt` |
| 非对称/签名 | `RSASecurity`、`PublicEncrypt`、`SignSha256WithRsa`、`VerifySignSha256WithRsa` |
| 摘要/HMAC/JWT | `MD5Encode`、`Sha256`、`HmacSHA256`、`GenerateToken/ValidateToken` |
| 编码 | `Base64Encode/Decode`、`ToJSON/FromJSON`、`LoadYaml/StoreYaml`、`Utf8ToGbk` |
| IP/网络 | `GetLocalIpAddress`、`IsIntranetIP`、`IsPortAvailable` |
| 文件 | `SelfDir`、`InsureDir`、`ReadFileToString`、`ZipFiles/Compress`、`SftpUploadFile` |
| 字符串 | `IsEmpty`、`SubChineseString`、`AnyToString`、`StringToAny[T]` |
| 时间 | `ToDateTimeString`、`Get0Hour`、`GroupByWeekDate` |
| 集合 | `SliceContains`、`SliceUnique`、`MapItoS`、`HashSet`、`Map[T]`（并发） |
| 并发/容器 | `NewSafeGo`、`GetGoroutineID`、`RingBuffer`、`LinkList[T]`、`ExpireCache` |
| 限流 | `LimitFreqSingle`（进程内滑动窗口，仅单机） |
| 校验 | `IsChinaMobile`、`IsMail`、`IsIdCard`、`CheckSqlValidate`（黑名单，不可当注入防护） |

---

## 14. Casbin（`casbin`）

```go
var Casbin = &casbinService{}
func (c *casbinService) GetEnforcer() *casbin.SyncedCachedEnforcer
func (c *casbinService) UpdateCasbin(roleID uint, infos []CasbinInfo) error
func (c *casbinService) UpdateCasbinApi(oldPath, oldMethod, path, method string) error
func (c *casbinService) GetPolicyPathByRoleId(roleId uint) ([]CasbinInfo, error)
func (c *casbinService) ClearCasbin(v int, p ...string) bool
func (c *casbinService) AddPolicy(rules [][]string) error
func (c *casbinService) FreshCasbin() error

type CasbinInfo struct { Path, Method string }
```

- Adapter 为 gorm-adapter，使用 `db.Mysql.GetConnection()`（无库名参数）→ 多库模式下不可用。
- 模型文件 `go.casbin.model_file`，默认 `rbac_model.conf`；**仓库只提供 `casbin.conf`**，必须显式配置。
- Policy 表为 gorm-adapter 的 `casbin_rule`。

---

## 15. 脚手架（`cmd`，独立 module）

```bash
cd cmd && go build -o bin/mgin .
./bin/mgin new <name> [--module ...] [--port ...] [--db ...] [--mq ...] \
    [--registry ...] [--config-center ...] [--i18n] [--jwt] [--casbin] \
    [--mgin-version ...] [--output ...] [--force]
```

- `--db` 默认 `mysql`；`sqlite/mongodb/redis/elasticsearch` 只生成子配置，业务层为内存示例；`postgres/clickhouse` 生成不存在的 DAO 类型（不可编译）。
- 数据层优先级：`mysql > postgres > clickhouse`，多选只取第一个。
- 生成工程的 `go.mod` 默认依赖最新发布版（离线回退 `v1.25.13-jh`），可用 `--mgin-version` 覆盖。
