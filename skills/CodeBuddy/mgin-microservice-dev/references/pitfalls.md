# 踩坑清单

> 原则：**文档与代码冲突时以代码为准**。本文件每一条都尽量给出「为什么」和「怎么绕」。

## 1. 文档与代码不一致（已核实）

| 文档/旧经验说的 | 实际情况（jh 分支） |
|---|---|
| `NewApp` 在 `-v` 时返回 nil，必须判空 | `app.go` 已改为打印版本后 `os.Exit(0)`，正常路径**总是**返回非 nil；`if app == nil` 已无意义（保留无害） |
| `mgin.MGin.UsePlugin(...)`（包级变量） | `mgin.go` 里 `// var MGin = &mgin{}` **已注释**，只能用 `app.MGin.UsePlugin(...)`；`docs/development/02-concepts/01-architecture.md` §6 与 `09-advanced/01-extension.md` §1 的写法**按字面无法编译** |
| `mgin.Default()` | **不存在**（hbase README 里写了但编译不过） |
| i18n 依赖外部 `x-lang` 微服务 | **已改自包含**：文案存 MySQL/SQLite，`i18n.Init()` 失败直接 panic，`X-Lang` 只是语言选择头 |
| `client` 无超时、无熔断 | 现有 `Options.Timeout`（默认 10s，`go.client.timeout`）、熔断（3 次 / 30s）、`Fallback` 兜底；`05-governance/02-traffic-and-resilience.md` §4/§5 的说法已过期 |
| `db.Pg` / `db.Clickhouse` / `db.ElasticSearch` / `db.Kafka` / `storage/s3` | jh `db/db.go` **只有** `Mysql / Mongo / Redis / Sqlite` |
| `db.MySql.UseCache()` | jh 已移除（v2 仍在），改用 DAO 的 `WithCache` |
| 有 `GetServices` / 独立负载均衡 / 独立熔断层 | 无；`GetServiceURL` 返回单实例，选择策略在 `CachedRegistry` 内 |
| 注册中心自动注册 polaris | `initResources` 只对 nacos/etcd/consul 调 Register，`SafeExit` 也不注销 polaris |
| 服务发现默认缓存 3s / 刷新 1s | `config.Init` 实际补 **60s/60s**；3s/1s 只是 `CachedRegistry` 在拿不到配置时的兜底、以及 `application.example.yml` 的示例值 |
| `sdk_document.md` 覆盖全量 API | 未覆盖 job / ratelimit / logsink / casbin / iplimit / session / xss / cmd 脚手架 / health / metrics |
| job 存储支持 PostgreSQL | 本树 Driver 只有 `DriverMysql` / `DriverSqlite` |
| `logsink` 队列上限 100 | 代码 `DefaultQueueSize = 1024`（`<=0` 或 `>1024` 回落 1024），`WithQueueSize` 注释里的「最大 100」是过期注释 |
| `trace.GetRequestId(c)` / `CopyPreHeaderToCurRoutine(c)` | 实际 `GetRequestId()` 无参、`CopyPreHeaderToCurRoutine(preRoutineId uint64)`（传 goroutine ID） |
| 热重载 ≈ 重启 | 只重载配置与数据源/注册/job；**路由、中间件、端口、TLS、app 名不重建** |

## 2. 插件 README 与代码不一致

| 插件 | README 写的 | 实际（以源码为准） |
|---|---|---|
| hbase | `hbase.HBASE` | **`hbase.Hbase`** |
| hive | `hive.HIVE` | **`hive.Hive`** |
| mssql | `mssql_multidb` / `mssql_dbNames` / `go.data.mssql: "<dsn>"` | `go.data.mssql.multidb` / `.dbNames` / `.dsn` / `.pool.*` |
| influxdb | `influxdb_multidb` 平铺键 | `go.data.influxdb.multidb` / `.dbNames` |
| mssql | `UseCache()` | 源码整段被注释，只有 `UseCacheWithRedis(*caches.Caches)` |
| nats | 多连接示例写成 `go.data.mqtt:` | 应为 `go.data.nats.conns` / `go.data.nats.<tag>.*` |
| mqtt | `subscribe` / `handlerFuncName` 配置项 | 代码中**无实现**（`SubTopics` 定义了但 Init 未读） |
| mgrabbit | `go.data.rabbitmq.*` | **`go.rabbitmq.*`（没有 data 层）** |

## 3. 编译 / 运行期坑

1. **`UsePlugin` 静默失败**：name 不在 `go.config.used`，或 `go.config.prefix.<name>` 不存在 → 只打一行日志，插件不初始化 → 调用时 nil panic。排查见 §6。
2. **`WithContext(&ctx)` 修改接收者**：`m.WithContext()` 会把 ctx 写到同一个 DAO 上，并发复用会串上下文。并发场景先复制：`d := *userDao; d.WithContext(&ctx)`，或每次新建 DAO。
3. **`Updates(entity)` 不带 WHERE**：可能全表更新或直接报错。要条件更新就用 `dao.DB().Model(&X{}).Where(...).Updates(...)`。
4. **`WithCache` 的 `TTL=0` = 永不过期**：必须显式给 TTL，否则改库后永远读到旧值。
5. **`RedisCache.Clear()` 是 FLUSHDB**：与业务共用一个 Redis DB 会清空业务键；`Range` 同理（全库 SCAN）。
6. **模型缺 `TableName()`**：DAO 泛型约束 `schema.Tabler`，编译期报错。
7. **Mongo 连接泄漏**：`GetConnection()` 后必须 `defer db.Mongo.ReturnConnection(db)`。
8. **跨协程丢 trace**：headers 以 goroutine ID 为 key 存在内存缓存（5 分钟），新协程必须 `trace.CopyPreHeaderToCurRoutine(preID)`。
9. **日志 `{}` 写成 `%s`**：不报错但不替换（框架只认 `{}`，顺序替换）。
10. **i18n 启动 panic**：`i18n.Init()` 失败会 panic（表名/库名/连接不通），不是降级。
11. **VictoriaLogs 启动 panic**：`go.log.victorialogs.url` 写错会在 `Init` 阶段 panic；TLS 严格校验不可跳过。
12. **探针被鉴权拦截**：用 `app.EnableHealth()` 挂的探针会带上当时已注册的中间件；要绝对干净就用 `go.health.enabled: true`（框架在 `baseRouter` 最前挂）。
13. **`/metrics` 没有 HTTP 指标**：只开 `go.metrics.enabled` 不够，还要 `app.Router.Use(metrics.Middleware())`。
14. **重载后组件不可用**：新配置解析成功但数据源连不上时，旧连接已被关闭，只能修复后重启。
15. **多实例重复跑定时任务**：`job` 无分布式锁；要么单副本，要么业务侧加锁。
16. **`logsink` 的 handler 里再打 `logs.*`**：会递归/死锁，`Write` 只做转发。
17. **`session` / `xss` 顺序**：xss 只清洗 query/body（不含头），session 需要自己 `Use`，两者都不在默认链里。
18. **`go vet ./...` 有历史告警**：`middleware/xss`、`utils/rsautil.go` unreachable、`i18n/store_test.go` copylocks；不是本次引入，但会让 CI 红。

## 4. utils 已知缺陷（历史修复记录）

> 背景：v2 曾整体落后 jh 一个 commit（`6cd633b` 修复框架的指针异常与崩溃）。以下多数已在 jh 修复，列出来是为了遇到旧版本时能对照。

- `LinkList.Get` 越界 panic → 已修为返回零值 + `TryGet(index)(T,bool)`；`Remove` 越界空操作。
- `ExpireCache.checkExpire` 误用 value 作 key → 已修（全程持 `c.mu`）。
- `sqlvalidate.CheckSqlValidate` 仅关键字黑名单 → jh 已修首字符漏判，但**仍是黑名单**，不能当注入防护；含 `table` 之类的正常英文也可能误判。
- `SftpConnect` 的 `HostKeyCallback` 恒返回 nil（MITM 风险）→ 保留旧函数，新增 `SftpConnectWithHostKey` / `SftpConnectWithKnownHosts`。
- `IsPortUse` 语义与命名相反 → 保留旧函数，新增 `IsPortAvailable(port) bool`（true=空闲）。

## 5. 设计层面的坑

- **`GetServiceURL` 返回单个实例**：TTL 窗口内同一服务固定落同一实例；负载分散靠 `cacheRefresh` 轮换。需要多实例请用 `GetServiceInstances`（但 `CachedRegistry` 会再裁成 1 条）。
- **HTTP 状态码恒 200**：404 / panic / 限流 / 业务失败都看 body 的 `status`（框架）或 `code`（jihai 业务），`==1` 才是成功。
- **插件 `Init` 不返回 error**（v1 契约）：配置错只在日志里，启动不会失败；上线前必须看启动日志。
- **job 的 cron 在数据库里**：`job.Register` 只注册执行器，改 cron 要改 DB 或调 `/job` 接口。
- **DAO 缓存不覆盖 `Where` / `Pager`**：只有 `All/One/Exists/Count/Where*` 走缓存。
- **`Dao[E]` 接口只声明了 `Insert`**：只有 `MgoDao` 满足；`MySQLDao` 的写入叫 `Create`，别对着接口写代码。
- **限流计数在进程内**：多副本总配额 = 单实例 × 副本数，要全局配额得走网关/Redis。
- **`/job` 与 `/i18n` 管理接口无内置鉴权**：直接暴露公网等于开放写权限，必须在网关或路由组上加鉴权。

## 6. 排查顺序（插件没生效 / 配置读不到）

1. `grep` 启动日志里有没有 `加载XXX失败，配置文件中未使用` 或 `XXX配置错误，无法获取配置地址`
2. 确认 `go.config.used` 逗号分隔字符串里**包含**该名字（`strings.Contains` 匹配，注意大小写）
3. 确认 `go.config.prefix.<name>` 存在
4. 确认配置文件名是 `<prefix>-<env>.yml`，且 `go.config.env` 对得上
5. `server_type: file` 时确认文件在 `<WorkDir>/` 下（WorkDir = 配置文件所在目录）
6. `server_type: etcd/nacos` 时确认配置中心里 dataId / key 拼对了（见 `framework-api.md` §2.4）
7. 仍然读不到，就用 `config.Config.GetConfigString("键")` 打印实际值——**不要只看 YAML 写了没有**
