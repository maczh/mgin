# 核心类型与关系

> 完整字段表见 [api/](api/)。本文只画关系、说明零值与使用约束。

---

## 1. 响应模型

```
models.Result[T]                    vo.Response[T]
├─ Status int                       ├─ Code int
├─ Msg string                       ├─ Msg string
├─ Data T                           ├─ Data T
└─ Page *ResultPage                 └─ SrvTime time.Time
     ├─ Count int
     ├─ Index int                   vo.ResponsePage[T]
     ├─ Size int                    ├─ Code/Msg/SrvTime
     └─ Total int                   └─ Data *PageData[T]
                                        ├─ List []T
                                        ├─ Total int64
                                        ├─ Page int
                                        └─ PageSize int
```

| 类型 | 成功判定 | 用途 | 转换 |
|------|----------|------|------|
| `models.Result[T]` | `Status == 1` | v1 兼容、i18n、client 解析 | `ToAny`/`ToAny[T]` |
| `vo.Response[T]` | `Code == vo.SUCCESS`(1) | 对外 API | `ToResponse`/`ResponseToT` |
| `vo.ResponsePage[T]` | 同上 | 分页 API | `ToResponsePage`/`ResponsePageToT` |

约束：

- `models.ToAny[T]` 直接类型断言 `Data.(T)`，**类型不一致会 panic**；
- `vo.ToResponsePage` 断言 `Data.([]any)`，传入 `[]Order` 会 panic；
- 两套模型不要在同一服务混用。

---

## 2. 错误定义

```
errcode.Definition
├─ Code int         业务码（1000+）
├─ HTTPStatus int   HTTP 状态
├─ MessageKey string i18n 键
└─ Module string    分组
```

关系：

- `errcode.New(module, code, httpStatus, key)` 构造并校正非法状态；
- `RegisterDef` 放入全局 map；`LookupDef` 查询，未注册返回 500 + `errcode.unknown`；
- `i18n.ErrorDef(def)` → `models.Result[any]`（`Status=Code`，`Msg=翻译`）。

零值：`Definition{}` 不是合法定义（HTTPStatus=0），必须用 `New`。

---

## 3. 插件与生命周期

```
plugin.Plugin (interface)
├─ MysqlPlugin / SqlitePlugin / MongoPlugin / RedisPlugin
├─ RegistryPlugin
└─ JobPlugin
```

| 方法 | 语义 |
|------|------|
| `Order() int` | 初始化升序、关闭降序 |
| `Enabled() bool` | 每次 InitAll 时求值 |
| `Init(ctx) error` | 失败记录后继续 |
| `Close(ctx) error` | panic 被 recover |
| `Health() error` | 仅巡检使用 |

实现自定义插件时，`Order` 必须晚于其依赖（如用到 DB 的插件 Order > 10）。

---

## 4. 注册与负载均衡

```
registry.RegistryClient (interface)
├─ NacosClient
├─ EtcdClient
├─ ConsulClient
├─ PolarisClient
└─ CachedRegistry（装饰器，持有 inner）

loadbalancer.LoadBalancer (interface)
├─ RoundRobin
├─ Random
├─ LeastConnections
└─ ConsistentHash
```

关系：

- `NewRegistry()` 返回 `CachedRegistry`，不是具体后端；
- `client` 先调 `GetServices`，失败再调 `GetServiceURL`；
- LB 只接收 `[]string` 地址，不感知注册中心。

---

## 5. 数据访问

```
MySQLDao[E schema.Tabler]
├─ debug bool
├─ Tag func() string
├─ ctx *context.Context
├─ tx *gorm.DB
└─ cache *CacheConfig
     ├─ Enabled bool
     ├─ Store cacheStore.ICache
     ├─ TTL time.Duration
     └─ Prefix string

MgoDao[E any]
├─ CollectionName string
└─ Tag func() string
```

约束：

- `E` 必须实现 `TableName() string`（值接收者）；
- `WithTx` 返回副本并绑定事务；事务内不清缓存（重要）；
- `WithContext` 修改的是**原对象**（返回自身），并发复用需先复制；
- `WithCache` 也修改原对象；
- `Dao[E]` 接口当前无实现方，勿依赖。

---

## 6. 配置聚合

```
config.Config
├─ App{Name,Project,Port,PortSSL,Cert,Key,Debug,IpAddr,ShutdownTimeout}
├─ Config{Server,Type,Token,Path,Env,Used,Prefix{...}}
├─ Log{RequestTableName,CallTableName,LogDb,DbName,Get,Req,Api,Kafka,Sink,VictoriaLogs}
├─ Logger{Outputs []LogOutput,Level,Out,File}
├─ Discovery{Registry,CallType,CacheTTL,CacheRefresh}
├─ Jwt{Secret}
├─ Sys{Enabled,Initdb,BaseUri,Swagger,Casbin}
├─ Casbin{Enabled,ModelFile}
├─ Framework{HealthEnabled,I18nEnabled,RatelimitEnabled,RecoverEnabled,LogRequests}
├─ Runtime{CommitHash,BuildTime,GoVersion,StartedAt,Pid}
├─ WorkDir string
└─ Cnf *koanf.Koanf
```

注意：`Cnf` 只含 application.yml；子配置在组件内部另建 koanf。

---

## 7. 任务

```
JobInfo (表 <prefix>job_info)
JobLog  (表 <prefix>job_log)

job.Context
├─ JobId/JobName/JobGroup/HandlerName
├─ Param map[string]interface{}
├─ LogId int64
├─ TriggerType string
└─ Done/Err/Deadline/Ctx/Log

job.HandlerFunc func(ctx *Context) error
```

关系：

- `Register(name, handler)` 注册处理器；`JobInfo.HandlerName` 引用它；
- 每次执行生成一条 `JobLog`；
- `Context.Ctx()` 受任务 `timeout` 控制（若有）。

---

## 8. 日志

```
logsink.Record
├─ Time/Level/Message/App/RequestID/GoroutineID
├─ trace 字段（可选）
└─ Fields map[string]any

models.PostLog（访问日志，Mongo 文档）
├─ Time/RequestId/ResponseTime/Ttl/AppName/ApiName
├─ Method/ContentType/Uri/ClientIP
├─ RequestHeader/RequestParam/RequestBody
└─ ResponseStr/ResponseMap
```

Handler 两类：

| 接口 | 输入 |
|------|------|
| `logsink.Handler` | `*models.PostLog` |
| `logsink.RecordHandler` | `*logsink.Record` |

---

## 9. 限流

```
ratelimit.Config
├─ Enabled/Whitelist/WhiteIps/IdleTimeout/Headers
├─ Rules []Rule
└─ global Rule（同层配置反序列化）

ratelimit.Rule
├─ Name/Path/Methods/Algorithm/Dimension/HeaderKey
├─ Rate/Burst/Window/WindowMs/MaxConcurrent
└─ Wait/WaitTimeoutMs/Code/Message/HttpStatus
```

零值经 `normalize()` 填充：算法 token_bucket、维度 global、window 1、burst=ceil(rate)、code 1011、httpStatus 200。

---

## 10. 其它核心类型

| 类型 | 要点 |
|------|------|
| `client.Options` | 调用参数；`Method` 空 = POST；`Files` 非空强制 file 协议 |
| `client.ResilienceOptions` | 零值 = 不超时/不重试/不熔断 |
| `client.CircuitBreaker` | 三态；`GetBreaker` 复用；`String()` 输出状态 |
| `health` 响应 | 未导出结构；JSON 见 [协议](protocols.md#3-健康探针) |
| `job.Config` | 未导出；通过 `go.job.*` 读取 |
| `cache.ICache` | 9 个方法；`MemCache` 与 `DiskCache` 实现 |
| `dao.RedisCache` | 实现 `ICache`，`Clear()` = FLUSHDB |
| `session.Config` | `ErrorHandleFunc/StoreKey/ManageKey/Skipper` |
| `casbin.CasbinInfo` | `Path`/`Method`，用于策略与白名单 |

---

## 11. 泛型使用速查

| 场景 | 写法 |
|------|------|
| 统一响应 | `models.Result[Order]`；`models.Success(order)` |
| 分页 | `models.SuccessPage(list, page)` |
| 下游解析 | `client.CallT[Order](...)` |
| 事务 | `db.MySQLTx(ctx, func(tx *gorm.DB) (Order, error) {...})` |
| DAO | `dao.MySQLDao[Order]` |
| 深拷贝 | `utils.DeepCopy[Order](src)` |
| 任意转泛型 | `utils.StringToAny[Order](s)`（失败返回零值） |

约束：泛型类型断言失败会 panic（`models.ToAny`），跨服务反序列化请用 `json`。
