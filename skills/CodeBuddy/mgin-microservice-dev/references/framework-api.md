# MGin 框架 API 参考（v1 jh / v2）

> 以 v1 `jh` 分支源码为准（module `github.com/maczh/mgin`，go 1.25.7）；v2 差异处单列。包路径在 v2 下加 `/pkg`：`mgin/config` → `mgin/v2/pkg/config`。

---

## 1. 应用生命周期

### 1.1 main.go

```go
package main

import (
	"github.com/maczh/mgin"
	"github.com/maczh/mgin/health"
	"jihaihotpot.com/jihai/myservice/mysql"
	"jihaihotpot.com/jihai/myservice/router"
)

// @title myservice API 文档
// @version 1.0.0
// @BasePath /api/v1
func main() {
	// 参数: 配置文件路径, 应用名, 版本号(Makefile -ldflags 注入), 是否启用国际化
	app := mgin.NewApp("conf/application.yml", "某某服务", Version, false)

	mysql.MySQLInit()                              // 业务侧 AutoMigrate + 缓存
	app.MGin.UsePlugin("kafka", mgkafka.Kafka)     // 插件：name 必须在 go.config.used
	app.Router.Use(mw.XTokenAuth())                // 业务中间件（排在框架默认链之后）
	router.RegisterRoutes(app)                     // v2；v1 是 router.Register(app.Router)

	app.Router.Use(metrics.Middleware())           // 需要 HTTP 指标时手动挂
	health.MarkStarted()                           // 业务初始化完成后放行 startup 探针
	app.Router.GET("/api/myservice/v1/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	app.Run()                                      // 阻塞；内部接管信号 + 优雅关闭
}
```

- `-v` 参数打印版本后 **`os.Exit(0)`**，`NewApp` 不会再返回 nil；保留 `if app == nil { return }` 无害但已无意义。
- 探针/指标推荐用配置开关（`go.health.enabled` / `go.metrics.enabled`），这样框架在 `baseRouter` 最前面注册，**不会**被后面挂的 jwt/casbin 拦截；`app.EnableHealth()` / `app.EnableMetrics()` 也能用，但路由会带上当时已注册的默认中间件。

### 1.2 `NewApp` 内部顺序

1. 解析 `-v`（命中即退出）、`-f`（仅当 `configFile==""` 时注册，默认 `<exe名>.yml`）
2. 纯文件名 → 拼到**可执行文件目录**
3. `Init(configFile)` = `config.Init` → `logs.InitVictoriaLogs()`（**配置错误直接 panic**）→ `initResources`：
   ```
   registry.NewRegistry()（统一套 CachedRegistry）
   mysql / sqlite / mongodb / redis 初始化
   nacos / etcd / consul → registry.Registry.Register
   job → job.Start()
   ```
   注意：`Init` 阶段插件**不初始化**（传入的 `mgin` 为 nil），插件必须由业务在 `NewApp` 之后 `UsePlugin`。
4. 起 5 分钟 ticker 跑 `checkAll()`（mysql/mongodb/redis，**不含 SQLite**）
5. `gin.SetMode`（`go.application.debug`）
6. i18n：`xlang || go.i18n.enabled` → `i18n.Init()`，**失败 panic**
7. 记录 `go.metrics.enabled` → `baseRouter()`

### 1.3 `baseRouter` 固定顺序（不可重排）

```
gin.Default()
mountHealth()                 // go.health.enabled：/healthz /health/live /health/ready /health/startup
/metrics                      // go.metrics.enabled
trace.TraceId()
postlog.RequestLogger()
cors.Cors(go.cors.headers)
xlang.RequestLanguage()       // 开启 i18n 时
ratelimit.RateLimit()         // go.ratelimit.enabled
nice.Recovery(recoveryHandler) // panic → 200 + status=1001
NoRoute → 200 + status=1000
/i18n/*                       // go.i18n.api.enabled
/job/*                        // go.job.enabled（无内置鉴权）
PUT /cfg/reload               // go.config.reload.token 非空
```

Gin 在注册路由时就快照中间件链，所以**先注册**的探针/指标不会被后挂的鉴权中间件影响。业务中间件只能 `Use` 在它们之后。

### 1.4 优雅退出

`app.Run()` 自己接管 `SIGINT / SIGHUP / SIGTERM / SIGQUIT`（**SIGHUP 也是退出，不是 reload**）。顺序：`postlog.Close()`（排空接口日志）→ `i18n.Close()` → 各数据源 Close → nacos/etcd/consul `DeRegister()`（**无 polaris**）→ `job.Stop()` → 插件 `Close()` → `server.Shutdown(5s)`。

---

## 2. 配置系统

### 2.1 application.yml 主干

```yaml
go:
  application:
    name: myservice          # 注册中心服务名
    port: 8080
    port_ssl: 8443           # 非 0 时优先用 SSL 端口注册，并打 ssl=true
    cert: ""                 # 配了才起 HTTPS（路径基于可执行目录）
    key: ""
    project: myproj          # 配置中心 group，缺省 DEFAULT
    debug: true              # gin DebugMode + 日志强制 debug
    ip: ""                   # 注册 IP，空则按 lan/lanNet 自动取
  logger:
    level: info              # debug|info|warn|error，缺省 error
    out: console,file        # 旧式输出（与 outputs 可并存）
    file: logs/myservice
    outputs:                 # 新式多目标，非空即替代 out/file
      - { target: console, format: text }
      - { target: file, file: ./logs/myservice, format: json }
      - { target: victorialogs, format: json, fields: [time, level, app, request_id, message] }
  config:
    server: http://127.0.0.1:2379
    server_type: file        # file|nacos|consul|springconfig|etcd|polaris
    token: ""                # polaris
    path: conf               # file 模式的子配置目录（相对 WorkDir）
    env: test                # <prefix>-<env>.yml
    used: "mysql,redis,etcd,kafka"    # ★ 组件总开关（逗号分隔，子串匹配）
    prefix:                           # ★ 每个组件的子配置前缀
      mysql: mysql
      redis: redis
      etcd: etcd
      kafka: kafka
    reload:
      token: ""              # 非空才注册 PUT /cfg/reload
  log:
    req: MyServiceReqLog     # 接口日志集合名（需 used 含 mongodb）
    dbName: Partner-Id       # 多库时从该 header 取库名
    get: full                # full|line(截断120)|off
    sink: { queue: 100, workers: 1, shutdown: 3000 }   # workers 实际强制 1
    victorialogs: { url: "", api_token: "", request: false, client: false, timeout: 5000 }
  discovery:
    registry: etcd           # nacos|etcd|consul|polaris，空=不注册不发现
    callType: json           # x-form|json|restful，缺省 x-form
    cacheTTL: 60             # 发现缓存秒数（未配/<=0 → 60）
    cacheRefresh: 60         # 热刷新秒数（未配/<=0 → 60）
    strategy: random         # random|round_robin|weighted（直读，未进结构体）
    circuitFailures: 3       # client 熔断阈值
    circuitOpenWindow: 30s   # 支持 30s 或纯数字秒
  client:
    timeout: 10s             # 单次调用超时，支持 10s/500ms 或数字秒
    fallback:
      enabled: false         # 失败回放上次成功响应
  health:
    enabled: false
    autoStarted: false       # Run() 监听后自动 MarkStarted
  metrics:
    enabled: false
  ratelimit:
    enabled: false           # 规则见 go.ratelimit.rules[]
  job:
    enabled: false
    dbName: ""
    initdb: false
    tablePrefix: mgin_
  i18n:
    enabled: false
    default: zh-cn
    dbName: ""               # 多库 MySQL 时指定库
    sqlite: ""               # 独立库文件名
    api:
      enabled: false         # /i18n CRUD
  jwt:
    secret: xxxxxxxx
  cors:
    headers: X-Token,X-User-Id,X-Tenant-Id
  framework:                 # ★ 仅 v2
    health: true
    metrics: true
    otel: false
    shutdownTimeout: 15
    loadBalancer: round
```

### 2.2 配置读取 API

```go
config.Config.GetConfigString("go.application.name")
config.Config.GetConfigStringArray("go.config.used")
config.Config.GetConfigInt("go.application.port")
config.Config.GetConfigBool("go.application.debug")
config.Config.GetConfigDuration("go.client.timeout", 10*time.Second) // "30s"/"500ms" 或数字(秒)
config.Config.Exists("go.sys.enabled")
config.Config.GetConfigUrl(prefix)    // 拼配置中心 URL
config.Config.GetConfigData(prefix)   // 取回 []byte（插件 Init 收到的就是这个）
```

底层 koanf，点号路径。键是否生效要用 `GetConfigXxx` 打印验证，不要只看 YAML 写了没有。

### 2.3 配置文件定位

| 写法 | 解析结果 |
|---|---|
| 绝对路径 | 直接使用 |
| 含 `/` 或 `\` 的相对路径 | 相对**当前工作目录** |
| 纯文件名 | `<可执行文件目录>/<file>` |
| 空 | `-f` 参数，默认 `<exe名>.yml` |

配置文件所在目录记为 `WorkDir`，同时是子配置、`casbin.model_file`、SQLite 文件、i18n 独立库的相对基准；`go.config.path` 只覆盖子配置目录。

### 2.4 配置中心路径

| server_type | 位置 |
|---|---|
| file | `<WorkDir>/<prefix>-<env>.yml` |
| nacos | `nacos/v1/cs/configs?group=<project>&dataId=<prefix>-<env>.yml` |
| consul | `/v1/kv/<project>/<prefix>-<env>.yml?dc=dc1&raw=true` |
| springconfig | `/<project>/<prefix>-<env>.yml` |
| etcd | `/config/<project>/<prefix>-<env>.yml` |
| polaris | `/config/v1/configfiles/release?namespace=default&group=<project>&name=<prefix>-<env>.yml` |

### 2.5 热重载

```bash
curl -X PUT http://127.0.0.1:8080/cfg/reload -H "X-MGin-Reload-Token: <token>"
```

仅 `go.config.reload.token` 非空时注册；只接受 loopback/私网/链路本地来源（看 `RemoteAddr`，不信 XFF）。重连数据源、重新注册、重启 job；**不重建**路由、中间件、端口、TLS、app 名。详见 `observability-deploy-testing.md` §5。

---

## 3. 数据库与泛型 DAO

### 3.1 全局客户端（jh 只有 4 个）

```go
db.Mysql.GetConnection(dbName ...string) (*gorm.DB, error)
db.Redis.GetConnection(dbName ...string) (redis.UniversalClient, error)
db.Mongo.GetConnection(dbName ...string) (*mgo.Database, error)
db.Mongo.ReturnConnection(conn *mgo.Database)   // ★ 必须归还
db.Sqlite.GetConnection() (*gorm.DB, error)
```

`db.Pg` / `db.Clickhouse` / `db.ElasticSearch` / `db.Kafka` **在本分支不存在**；`db.Mysql.UseCache()` 已移除（改用 DAO 的 `WithCache`）。

MySQL（`conf/mysql-test.yml`）：

```yaml
go:
  data:
    mysql: root:123456@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local
    mysql_debug: true
    mysql_pool: { max: 200, total: 1000, timeout: 30, life: 5 }
```

多库：

```yaml
go:
  data:
    mysql:
      multidb: true
      dbNames: test1,test2
      test1: user1:pwd1@tcp(...)/db1?...
      test2: user2:pwd2@tcp(...)/db2?...
```

多库路由：`&dao.MySQLDao[User]{Tag: func() string { return "test2" }}`。

Redis：`go.data.redis.{uri | host,port,password,database,master}` + `go.data.redis_pool.{min,max,idleTimeout,timeout}`（`database=-1` 强制 DB0）。
Mongo：`go.data.mongodb.{uri,db}` + `go.data.mongo_pool.max`（最小 10）。

### 3.2 泛型 DAO

```go
// 实例化（v1 值类型，v2 指针）
var OrderDao = dao.MySQLDao[model.Order]{}          // v1
var OrderDao = &dao.MySQLDao[model.Order]{}         // v2
```

链式/配置：

```go
OrderDao.Debug() *MySQLDao[E]
OrderDao.WithTx(tx *gorm.DB) *MySQLDao[E]           // 事务内使用，返回副本
OrderDao.WithContext(ctx *context.Context) *MySQLDao[E]  // ⚠ 修改接收者，见 pitfalls
OrderDao.WithCache(dao.CacheConfig{...}) *MySQLDao[E]
// Tag 是结构体字段（func() string），多库路由写法：&dao.MySQLDao[E]{Tag: func() string { return "test2" }}
OrderDao.Where(query, args...) *gorm.DB              // gorm 逃生口，不缓存
OrderDao.Alias(alias) / DB() / JOIN(alias, query, args...)
```

读取：

```go
All(entity E, opts ...QueryOption) ([]E, error)   // 结构体非零字段等值
One(entity E) (*E, error)                         // 未命中 (nil, nil)
Exists(entity E) bool
Count(entity E) (int64, error)
WhereAll(query, args...) ([]E, error)
WhereOne(query, args...) (*E, error)
WhereCount(query, args...) (int64, error)
WhereExists(query, args...) bool
Pager(conn *gorm.DB, page, size int) ([]E, *models.ResultPage, error)  // size=0 → 20；page<1 报错；不缓存

type QueryOption struct {
	Preloads []string
	OrderBy  []string   // ["id desc"]
}
```

写入：`Create(*E)`、`MultiCreate([]*E)`、`Updates(*E)`（⚠ 不带 WHERE）、`Save(*E)`、`Delete(E)`。

### 3.3 缓存

```go
rds, _ := db.Redis.GetConnection()

// Tag 是 MySQLDao 的字段（func() string），用于多库/多租户隔离，不是 CacheConfig 的字段
var OrderDao = (&dao.MySQLDao[model.Order]{
	Tag: func() string { return "order" },
}).WithCache(dao.CacheConfig{
	Enabled: true,
	Store:   dao.NewRedisCache(rds),   // 也可用 cache.OnMemCache("orders") 等实现
	TTL:     5 * time.Minute,
	Prefix:  "mgin:mysql:",            // 缺省值
})
```

- 缓存键：`<Prefix><Tag>:<TableName>:<op>:<md5(json(query))>`（`Tag` 为空时该段为空）。
- 命中范围：`All/One/Exists/Count/WhereAll/WhereOne/WhereExists/WhereCount`；`Where/Pager/Alias/DB/JOIN` 不缓存。
- 写操作成功后按键前缀删缓存（先写库再删缓存，无分布式锁）。
- **`TTL<=0` 表示永不过期**，务必显式给 TTL。
- `RedisCache.Clear()` = **FLUSHDB**、`Range` = 全库 SCAN；不要与业务共用 DB。

### 3.4 事务

```go
n, err := db.MySQLTx(ctx, func(tx *gorm.DB) (int64, error) {
	return tx.Model(&User{}).Where("id = ?", 1).Update("name", "李四").RowsAffected, nil
})

err := db.TransactionOn(ctx, conn, func(tx *gorm.DB) (struct{}, error) { ... })
```

只在 MySQL/SQLite 内生效；回调里**必须全程用 `tx`**（再 `GetConnection()` 就跑到事务外）。`Dao.WithTx(tx)` 可以绑定；事务内不清缓存，提交后自行失效。Mongo/Redis 没有跨库事务。

### 3.5 Mongo DAO

```go
var LogDao = dao.MgoDao[model.ApiPostLog]{CollectionName: model.ApiPostLog{}.CollectionName()}
// Insert / Delete(bson.M) / Updates(id, doc) / Update(selector, update)
// All(bson.M) / One(bson.M) / Pager(query, sort, page, size)
```

---

## 4. 统一响应

### 4.1 框架自带 `models.Result[T]`

```go
type Result[T any] struct {
	Status int         `json:"status"`
	Msg    string      `json:"msg"`
	Data   T           `json:"data,omitempty"`
	Page   *ResultPage `json:"page,omitempty"`
}
type ResultPage struct{ Count, Index, Size, Total int }

models.Success(data) / SuccessWithMsg(msg, data)
models.SuccessPage(data, page) / SuccessWithPage(data, count, index, size, total)
models.Error(status, msg) / models.ErrorT[T](status, msg)
```

### 4.2 i18n 版（`NewApp(..., true)` 或 `go.i18n.enabled: true`）

```go
i18n.Error(errcode.DATA_NOT_FOUND, errcode.DataNotFound)
i18n.ErrorT[T](code, messageID)
i18n.Success(user) / SuccessWithPage(...)
i18n.ParamLostError("id") / ParamError("id")
i18n.CheckParametersLost(params, "a", "b")
i18n.String(messageID) / Format(messageID, args...) / GetString(messageID, lang)
```

**已不再依赖外部 `x-lang` 服务**：文案存库，回退链「`X-Lang` → `go.i18n.default` → messageID」。库选择：`used` 含 mysql → 复用（多库取 `go.i18n.dbName`）；否则配了 sqlite → 复用；否则自动开独立 SQLite `<app.name>_i18n.db`（无需在 `used` 里加 sqlite）。**`i18n.Init()` 失败会 panic**。`GetXLangString` 只是 `GetString` 的别名。

表名 `{go.application.name}_i18n`，主键 `(message_id, language)`；`app` 名需匹配 `^[A-Za-z0-9_.-]{1,59}$`。

CRUD（`go.i18n.api.enabled: true` 时挂 `/i18n`，**无内置鉴权**）：`POST /i18n`、`GET /i18n?messageId=&language=`、`PUT /i18n`、`DELETE /i18n?messageId=&language=`、`GET /i18n/list?page=&pageSize=`。

错误码（`errcode/constant.go`）：`URI_NOT_FOUND=1000`、`SYSTEM_ERROR=1001`、`REQUEST_PARAMETER_LOST=1003`、`DATA_NOT_FOUND=1004`、`TOKEN_ERROR=1008`、`TOO_MANY_REQUESTS=1011`、`JOB_ERROR=1012`、`PARAM_ERROR=1014`。业务错误码从 **2000** 起。

### 4.3 jihai 业务实际用法

业务项目一般不 import `models.Result`，而是每个项目复制一份 `vo.Response[T]`（字段 `code/data/msg/srvTime`），详见 `project-conventions.md` §4 与 `code-templates.md` §1。**不要混用 `status` 与 `code`**。

---

## 5. 跨服务调用（`client`）

```go
import "github.com/maczh/mgin/client"

resp, err := client.Call("user-service", "/api/v1/user/get", &client.Options{
	Method:   "POST",                    // 缺省 POST
	Protocol: client.CONTENT_TYPE_JSON,  // x-form|json|restful|file
	Json:     map[string]any{"id": 1},
	Header:   map[string]string{"x-token": tk},
	Timeout:  2 * time.Second,           // 0 → go.client.timeout（默认 10s）
	Retry:    true,                      // 失败重试一次
})

result := client.CallT[User]("user-service", "/api/v1/user/get", &client.Options{...}) // vo.Response[T]
if result.Code != vo.SUCCESS { /* 失败被置为 -1 */ }

legacy := client.CallResultT[User](...)   // models.Result[T]（status/msg/data）
page   := client.CallPageT[User](...)     // vo.ResponsePage[T]
```

`Options` 全字段：

```go
type Options struct {
	Method, Protocol, Group string
	Header, Query, Data, Json any
	Path  map[string]string
	Files []grequests.FileUpload
	Retry bool
	Timeout time.Duration                 // 整次调用超时
	Fallback func(error) (string, bool)   // 兜底，返回 (响应体, 是否命中)
	CircuitKey string                     // 默认 service + "\x00" + uri
	CircuitFailures int                   // 0 → go.discovery.circuitFailures → 3
	CircuitOpenWindow time.Duration       // 0 → go.discovery.circuitOpenWindow → 30s
}
func CachedFallback(service, uri string, params ...map[string]any) func(error) (string, bool)
```

行为要点：

- `Protocol` 缺省取 `go.discovery.callType`；`Files` 非空强制 file 协议；非法协议直接返回 `unsupported client protocol "xxx"`。
- `Call` 先浅拷贝 `Options`、深拷贝 `Header`，同一份 `Options` 可并发复用。
- 自动注入 trace 三头（`X-Request-ID`/`X-Real-IP`/`X-User-Agent`）。
- 服务发现走 `registry.Registry.GetServiceURL(service, group)`（单实例 + TTL 缓存）。
- 熔断：key 维度连续失败达阈值即打开，窗口后放行一个半开探测；打开时返回 `circuit breaker is open`（优先走 `Fallback`）。
- 兜底：`Fallback` 或 `go.client.fallback.enabled: true`（等价自动 `CachedFallback`）；响应缓存进程内、无 TTL/容量上限，只适合幂等查询。
- 非 2xx、非 JSON 对象、空响应都会转成 error；`CallT` 失败返回 `Code=-1`。
- 调用日志：`go.log.victorialogs.client: true` 时异步上报，独立于 `logger.level`。

直接拿地址：

```go
host, group := registry.Registry.GetServiceURL("baseServ")
```

---

## 6. 日志与日志总线

```go
logs.Debug("订单{}创建,耗时{}ms", orderId, ttl)   // ★ {} 不是 %s
logs.Info(...) / logs.Warn(...) / logs.Error(...)
```

级别由 `go.logger.level` 控制（`debug=true` 强制 debug）；消息自动带 `[requestId]` 前缀；`logs.InitVictoriaLogs()` 配置错误会 panic；`logs.RegisterHandler` 可挂自定义输出。

`logsink` 有两条独立通道：

```go
// 接口访问日志（Entry = models.PostLog）
logsink.MustRegister(logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
	return mgkafka.Kafka.Send("myapp-log", utils.ToJSON(e))
}).WithClose(mgkafka.Kafka.Close), logsink.WithQueueSize(512))

// 运行日志（Record，VictoriaLogs 走这条）
logsink.MustRegisterRecord(logsink.NewRecordHandlerFunc("mylog", func(r *logsink.Record) error { ... }))
logsink.EmitRecordTo(logs.VictoriaLogsClientTarget, &logsink.Record{...})
```

常用 API：`Register/MustRegister/Unregister(name, timeout)/Emit(*Entry) bool/Close(timeout)/Stats() []HandlerStats`，Record 通道同名加 `Record` 后缀。队列每 handler 1 协程、硬上限 1024、满则丢最旧。

---

## 7. 定时任务

### 7.1 框架 job 模块（DB 持久化 + HTTP 管理）

```go
job.Register("syncUserJob", func(ctx *job.Context) error {
	ctx.Log("开始同步用户, 参数=%v", ctx.Param)
	return userService.Sync(ctx.Ctx())   // 返回 error 触发重试
})
```

**cron 表达式存在 DB**（`mgin_job_info`，`handler_name` 对应 `Register` 的名字），改 cron 要改库或调 `/job` 接口。建表由 `go.job.initdb: true` 完成。

```yaml
go:
  job:
    enabled: true
    dbName: ""              # 多库时指定
    initdb: true
    tablePrefix: "mgin_"
    scanInterval: 1
    refreshInterval: 30
    logRetainDays: 30
    maxConcurrent: 50
    maxSerialQueue: 10
    timezone: "Asia/Shanghai"
```

支持 5/6 段 cron、`@every 5m`、`@daily`；调度类型 `cron`/`fixed_rate`/`fixed_delay`/`once`；阻塞策略 `serial|discard|concurrent|cover`；错过策略 `do_nothing|fire_now`。**无分布式锁**，多实例会重复执行。

管理接口（`go.job.enabled=true` 自动挂 `/job`，无内置鉴权）：`GET /job/list`、`GET /job/:id`、`POST /job`、`PUT /job`、`DELETE /job/:id`、`POST /job/:id/{start,stop,trigger}`、`GET /job/handlers`、`GET /job/log`。

### 7.2 自管 cron（jihai 更常用）

```go
c := cron.New()                        // 需要秒级精度：cron.New(cron.WithSeconds())
c.AddFunc("@every 5m", expireScan)
c.Start()
```

---

## 8. 中间件

| 包 | 入口 | 挂载 | 行为要点 |
|---|---|---|---|
| `middleware/trace` | `trace.TraceId()` | 自动 | 注入三个 trace 头；头存在 goroutine 维度的内存缓存 |
| `middleware/postlog` | `postlog.RequestLogger()` | 自动 | 接口日志，落 MongoDB 或 logsink |
| `middleware/cors` | `cors.Cors(headers...)` | 自动 | 预检 204；方法集合固定 |
| `middleware/xlang` | `xlang.RequestLanguage()` | 自动（i18n 开） | `X-Lang` |
| `middleware/ratelimit` | `ratelimit.RateLimit()` | 自动（`go.ratelimit.enabled`） | 令牌桶/滑动窗口/并发；进程内计数 |
| `middleware/jwt` | `jwt.JwtAuthorize()` | 手动 | `go.jwt.secret` 为空则全 401；失败 **401 无 body**；成功 `c.Set("claims", ...)` |
| `middleware/casbin` | `casbin.CasbinHandler()` | 手动（必须在 jwt 之后） | 需 `go.casbin.enabled` + `go.casbin.model_file`（相对 WorkDir，仓库只提供 `casbin.conf`）；表 `casbin_rule`；多库模式不可用 |
| `middleware/iplimit` | `iplimit.CIDR("10.0.0.0/8")` | 手动 | 不在网段 403 |
| `middleware/limit` | `limit.MaxAllowed(n)` | 手动 | 并发信号量；客户端断开返回 499 |
| `middleware/session` | `session.New(opt...)` / `NewWithConfig(cfg, opt...)` | 手动 | 另有 `FromContext/Destroy/Refresh` |
| `middleware/xss` | `(&xss.XssMw{FieldsToSkip: []string{...}}).RemoveXss()` | 手动 | 只清洗 query/body，**不含请求头** |

trace 上下文 API：

```go
trace.GetRequestId() / GetClientIp() / GetUserAgent()
trace.GetHeader(k) / SetHeader(k, v) / GetHeaders()
trace.GetGoroutineID() uint64
trace.CopyPreHeaderToCurRoutine(preRoutineId uint64)   // ★ 参数是 goroutine ID，不是 *gin.Context
```

headers 以 **goroutine ID** 为 key 存内存缓存（5 分钟），新协程里取不到，必须 copy。

---

## 9. 注册中心

```go
type RegistryClient interface {
	Register(registryConfigData []byte)
	GetServiceURL(servicename string, groupName ...string) (string, string)
	DeRegister()
}
```

可选扩展接口（类型断言探测）：

```go
type InstanceProvider interface { GetServiceInstances(service string, group ...string) ([]ServiceInstance, string) }
type OfflineSubscriber interface { SubscribeOffline(func(service, group string)) error }
type ServiceWatcher interface { WatchService(service, group string, callback func()) (cancel func(), err error) }

type ServiceInstance struct { URL string; Weight int; Metadata map[string]string }
type SelectionStrategy string   // StrategyRandom | StrategyRoundRobin | StrategyWeighted
type RegistryOptions struct { TTL, Refresh time.Duration; Strategy SelectionStrategy }
func NewCachedRegistry(delegate RegistryClient) *CachedRegistry
```

- `NewRegistry()` 返回的实例统一被 `CachedRegistry` 包裹（TTL 缓存 + 热刷新 + 回退路径 singleflight + 空结果不缓存 + 旧缓存降级）。
- 选择策略 `go.discovery.strategy`：`random`（默认）/`round_robin`/`weighted`；权重目前都是 1，`weighted` 等价随机。
- 后端能力：`GetServiceInstances` 四家都有（Consul 仅单 group、Consul/etcd 不填 `Metadata`）；真实 watch 只有 etcd，Nacos/Consul/Polaris 是 5s 轮询边沿触发。
- 未配置 → `noopRegistryClient`，`GetServiceURL` 恒 `("","")`，`client.Call` 报「微服务获取 X 服务主机IP端口失败」。
- ⚠️ v1 `initResources` **只对 nacos/etcd/consul 自动 Register**，Polaris 需自己调：
  ```go
  registry.Registry.Register(config.Config.GetConfigData("polaris"))   // 前置：used 含 polaris
  ```
- ⚠️ `SafeExit` 也**不注销 polaris**。
- `CachedRegistry.GetServiceInstances` 会再套一次策略，通常只返回 1 条；要全量列表直接用具体后端。

| 注册中心 | 配置键 |
|---|---|
| nacos | `go.nacos.{server,port,apiversion,clusterName,group,username,password,lan,lanNet}` |
| etcd | `go.etcd.{server,port,prefix,keyTemplate,group,clusterName,username,password,lan,lanNet}` |
| consul | `go.consul.{server,port,token,group,clusterName,lan,lanNet}` |
| polaris | `go.polaris.{server,port,namespace,token,lan,lanNet}` |

注册的服务名 = `go.application.name`；端口 = `go.application.port`（`port_ssl` 非 0 时用 SSL 端口并标 `ssl=true`/`metadata.ssl`）；IP 按 `lan`+`lanNet` 取，或强制 `go.application.ip`。

---

## 10. 常用 utils

```go
utils.ToJSON(o) / utils.FromJSON(j, &o)
utils.ToDateTimeString(t) / utils.GetNowDateTime()
utils.MD5Encode(s) / utils.MapMD5(m)
utils.GenerateToken(claims) / utils.ValidateToken(s)   // 密钥 = go.jwt.secret
utils.AnyToMap(o) / utils.Struct2Map(o) / utils.Map2Struct(in, out)
utils.NewUUIDString() / utils.SimpleUUID()
utils.GetRandomString(n)
utils.IsChinaMobile / IsMail / IsIdCard
utils.GinParamMap(c) / utils.GinHeaders(c)
utils.CheckSqlValidate(s)          // ⚠ 只是关键字黑名单，不能防注入
utils.DownloadFile(url, path)
utils.AESEncrypt(data, key) / utils.Base64Encode(s) / utils.HmacSHA256Hex(key, data)
```

⚠️ utils 里**没有**跨服务调用函数，那是 `client` 包。
