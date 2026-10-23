# MGin 需求文档（jh 分支）

| 项 | 内容 |
|----|------|
| 文档版本 | v1.0（对应代码分支 `jh`，tag 基线 `v1.25.27-jh`） |
| 适用读者 | 新加入的后端开发者、架构师、测试与运维 |
| 关联文档 | [架构说明](01-architecture.md) · [模块与接口](../08-reference/api/README.md) · [数据结构](../03-development/04-data-access.md) · [发布规范](../10-contributing/04-release-process.md) |

> 本文只描述**当前代码已经实现**的能力。凡标为「非目标」的条目，仓库内没有实现，请勿按其它分支的文档排期。

---

## 1. 背景与问题

团队内多个 Go 服务重复实现同一套基础设施：配置文件加载、MySQL/Redis/MongoDB 连接、注册发现、服务间调用、日志落库、错误码与国际化、K8s 探针。重复实现导致：

1. 配置键与日志格式各自为政，排障成本高；
2. 数据源连接缺少重连与池化，故障恢复依赖重启；
3. 错误码与 HTTP 状态码混用，前端难以统一处理；
4. 新服务从零搭建，缺少可编译的骨架与规范。

MGin 的目标是把这些横切能力收敛为一个可复用模块，并提供脚手架生成标准工程。

## 2. 目标与成功标准

| 编号 | 目标 | 可验证标准 |
|------|------|-----------|
| G1 | 一个最小服务可在 10 分钟内跑起来 | 按 README §6.1 执行，`/ping` 与 `/health/live` 返回 200 |
| G2 | 组件按配置启用，关闭时不产生副作用 | `used: ""` 时可启动，且不连接任何外部服务 |
| G3 | 数据源故障可自愈 | MySQL/MongoDB `Check()` 发现断连后自动重连并重试一次 |
| G4 | 统一响应与错误码 | 所有示例接口返回 `models.Result`；限流/404/panic 的 `status` 可被客户端识别 |
| G5 | 可观测 | 开启后 `/health/*`、`/metrics` 可用，访问日志可落 MongoDB |
| G6 | 可平滑退出 | 收到 SIGTERM 后先排空日志队列、注销注册中心，再关连接 |
| G7 | 新成员可按文档独立开发 | 本文档集 + README 覆盖环境搭建、调试、测试、发布 |

## 3. 角色与场景

| 角色 | 关注点 | 主要场景 |
|------|--------|----------|
| 业务开发者 | 快速接入、少写样板 | 建工程 → 配数据源 → 写路由/DAO → 本地调试 |
| 服务负责人 | 稳定性与可观测 | 探针、限流、日志、发布与回滚 |
| 架构师 | 一致性与演进 | 新增组件、统一协议、跨服务调用规范 |
| 运维/SRE | 部署与排障 | 配置中心、注册中心、优雅关闭、指标告警 |
| 测试 | 可测性 | 单元测试、依赖 stub、集成环境 |

## 4. 功能需求

优先级：P0 必须、P1 应该、P2 可选。验收项均可通过代码或命令复核。

### FR-01 应用生命周期（P0）

- `mgin.NewApp(configFile, appName, version, xlang)` 完成配置加载、资源初始化、Gin 引擎与默认中间件装配。
- `App.Run()` 依据 `go.application.port` / `port_ssl` 启动 HTTP/HTTPS；捕获 `SIGINT/SIGHUP/SIGTERM/SIGQUIT` 后执行 `SafeExit()`，HTTP 关闭超时 5s。
- 支持 `-v` 打印版本后退出、`-f` 指定配置文件。
- 验收：未配置任何组件时进程可启动；`curl /health/live` 为 200；`kill -TERM` 后日志出现 `Shutdown Server` 与 `Server exiting`。

### FR-02 配置加载与热重载（P0）

- 支持本地文件、HTTP、Nacos、Consul、etcd、Polaris、Spring Config 作为配置来源（`go.config.server_type`）。
- 子配置按 `go.config.prefix.<组件>` 与 `go.config.env` 定位：`<prefix>-<env>.yml`。
- `go.config.reload.token` 非空时注册 `PUT /cfg/reload`，仅允许内网/本机且 token 校验通过；重载后重连数据源、重新注册注册中心、重启 job。
- 约束：重载不重建 Gin 路由、中间件与监听端口。
- 验收：`go.config.used` 变化后调用 `/cfg/reload`，日志显示资源关闭与重建；业务路由行为不变。

### FR-03 数据源管理（P0）

- MySQL：单库/多库、连接池、调试 SQL、断线重连；`GetConnection(dbName...)` 返回 `*gorm.DB`。
- SQLite：纯 Go 驱动，文件路径相对可执行文件目录；未初始化返回 `SQLite not opened`。
- MongoDB：单库/多库、会话归还（`ReturnConnection` 必须成对）、断线重连。
- Redis：单库/多库、`PSubscribe` 订阅。
- 验收：断开 MySQL 后调用 `Check()`，恢复网络后再次 `GetConnection()` 成功。

### FR-04 泛型 DAO 与缓存（P0）

- `MySQLDao[E schema.Tabler]` 提供 `Create/MultiCreate/Updates/Save/Delete/All/One/Exists/Count/Pager/Where*`、`WithTx/WithContext/WithCache`。
- `Where*` 与 `All/Exists/Count` 支持缓存；写操作后清缓存。
- `MgoDao[E]` 提供 Mongo 版本的基础 CRUD 与分页。
- 验收：同一查询两次调用，第二次命中缓存（可通过 Redis/Mem 观察）；写入后旧结果不再命中。

### FR-05 缓存（P1）

- `cache.ICache` 统一接口；`OnMemCache` 返回带过期清理的内存缓存；`OnDiskCache` 返回 bitcask 持久化缓存。
- 验收：写入带 TTL 的键，过期后 `Get` 返回 false；磁盘缓存在进程重启后仍可读。

### FR-06 服务调用（P0）

- `client.Call` 支持 `x-form`/`json`/`restful`/`file`；`CallT/CallPageT/CallResultT` 解析两种返回协议。
- 目标地址由 `registry.GetServiceURL(service, group)` 解析；调用自动携带 trace 头；可选单次重试。
- 约束：未内置超时；未初始化注册中心时直接报错。
- 验收：本地起两个服务，A 通过服务名调用 B 成功，B 的日志中可看到 `X-Request-ID`。

### FR-07 注册与发现（P0）

- `registry.Registry` 统一 `Register/GetServiceURL/DeRegister`；内置 Nacos、Consul、etcd、Polaris。
- `NewCachedRegistry` 装饰：TTL 缓存、singleflight、后台热刷新、无实例时降级复用旧缓存。
- 验收：注册中心重启后，缓存过期前调用仍可用；实例下线后缓存刷新不再返回该实例。

### FR-08 HTTP 中间件（P0）

- 默认启用：trace、postlog、cors、recovery、404 处理。
- 按需启用：jwt、casbin、xss、iplimit、limit、session、ratelimit（配置驱动）。
- 验收：CORS 预检返回 204 且带允许头；未登录访问受保护路由返回 401；超限流返回业务码 1011。

### FR-09 错误码与国际化（P0）

- `errcode` 提供 1000–1014 业务码；`i18n.Error/Success` 产出 `models.Result`。
- i18n 字典存 MySQL 或 SQLite，表名 `{appName}_i18n`，主键 `(message_id, language)`；按 `X-Lang` 协商语言，缺省 `zh-cn`，取不到时返回 messageId。
- 验收：同一请求分别带 `X-Lang: zh-cn` 与 `en-us`，返回文案不同；未初始化时接口不 panic。

### FR-10 日志（P0）

- `logs.Debug/Info/Warn/Error` 使用 `{}` 占位；输出目标支持 console、file（按天）、VictoriaLogs 与自定义 handler。
- `logsink` 每 handler 单协程 FIFO，队列满淘汰最旧并计数；`SafeExit` 时排空。
- 验收：配置 `outputs` 后同一条日志同时出现在控制台与文件；VictoriaLogs 可达时可在平台检索。

### FR-11 定时任务（P1）

- 支持 `cron/fixed_rate/fixed_delay/once`、阻塞策略 `serial/discard/concurrent/cover`、超时、失败重试、日志保留。
- 执行器通过 `job.Register(name, fn)` 注册；管理接口 `/job/*`（增删改查、启停、手动触发、执行日志）。
- 约束：单机调度，多实例会重复执行；需要 MySQL 或 SQLite。
- 验收：创建 cron 任务后按周期执行；`cover` 策略下重复触发不会并发。

### FR-12 健康与指标（P0）

- 探针：`/healthz`、`/health/live`（恒 200）、`/health/ready`（依赖检查）、`/health/startup`（`MarkStarted` 后 200）。
- `/metrics` 暴露 HTTP 请求数/时延/并发、构建信息、插件健康、依赖状态。
- 验收：MySQL 不可用时 `/health/ready` 返回 503 并列出依赖；`/metrics` 含 `mgin_http_requests_total`。

### FR-13 脚手架（P1）

- `mgin new` 生成 main/version/router/controller/service/dao/model/conf/Makefile/go.mod/README。
- 支持 `--db/--mq/--registry/--config-center/--i18n/--jwt/--casbin/--mgin-version/--force`。
- 验收：`--db mysql` 生成的工程 `go build` 通过（实测）；`--db postgres|clickhouse` 目前失败（见 §6）。

### FR-14 扩展点（P1）

- `mgin.MGin.Use(name, init, close, check)` / `UsePlugin`：仅当 `used` 含该名字时初始化，并纳入自检与关闭。
- 日志：`logs.RegisterHandler` / `logsink.Register` 接入外部存储。
- 验收：注册自定义组件后，`used` 增删可控制其初始化；`SafeExit` 时调用其 Close。

### FR-15 系统管理模型（P2）

- `models/sys` 提供用户、部门、岗位、角色、资源、API、字典、配置、社交、在线用户等表模型与 `TableName()`。
- 范围：**只提供模型**，不含 controller/service；`go.sys.initdb` 不会自动建表。
- 验收：业务可 `AutoMigrate` 这些模型；框架不因 `go.sys.enabled` 改变行为。

## 5. 非功能需求

| 编号 | 类别 | 要求 | 现状/证据 |
|------|------|------|-----------|
| NFR-01 | 兼容性 | Go 1.25.7；Linux/amd64 可构建 | `go build ./...` 通过 |
| NFR-02 | 可维护性 | 新增配置键必须同时更新解析、示例与文档 | 见 [编码规范](../10-contributing/02-coding-standards.md#5-新增配置键清单) |
| NFR-03 | 性能 | 服务发现命中本地缓存，避免每次调用 RTT | `registry/cache.go` 默认 60s TTL + 热刷新 |
| NFR-04 | 稳定性 | 数据源断连自动重连；日志队列有界 | `Check()` 重连；`logsink` 队列上限淘汰最旧 |
| NFR-05 | 安全 | 重载接口仅内网 + 常量时间 token 比较；XSS 默认 StrictPolicy；JWT 仅 HMAC | `app.go`、`middleware/xss`、`middleware/jwt` |
| NFR-06 | 可观测 | 探针不被鉴权/限流拦截 | 探针在 `baseRouter()` 中最先注册 |
| NFR-07 | 可测试性 | 纯逻辑包可单测，外部依赖可用 stub | `health/metrics/logs/utils` 等已有测试 |
| NFR-08 | 可运维性 | 单二进制 + 配置文件即可部署 | 无外部运行时依赖 |

## 6. 约束、假设与非目标

**约束**

1. 框架是**库**而非独立服务：所有能力运行在业务进程内，全局单例（`config.Config`、`db.*`、`registry.Registry`）决定了**一个进程只能有一个 App**。
2. `go.config.used` 使用**子串匹配**，组件名不能互相包含（如同时启用 `mysql` 与自定义 `mysqlx` 会误判）。
3. `job` 为单机调度，无分布式锁/分片。
4. 仓库无 CI 与 Makefile，构建/测试由开发者本地执行。

**假设**

- 部署环境可访问注册中心与配置中心；不可访问时使用 `file` 模式。
- 业务对 HTTP 返回结构的约定为 `models.Result`；跨服务新协议使用 `vo.Response`。

**非目标（本分支不会做）**

- PostgreSQL、ClickHouse、ElasticSearch、Kafka、S3 的数据源/客户端实现；
- 内置系统管理后台的 HTTP 接口与自动建表；
- 多级缓存、分布式事务、gRPC 服务端；
- 把 `utils` 中的 ECB 加密、SQL 关键字黑名单当作安全边界。

## 7. 验收清单（发布前）

- [ ] `go build ./...` 通过；`cd cmd && go build ./...` 通过。
- [ ] `gofmt -l .` 无输出。
- [ ] `go vet ./...` 无新增告警（历史上 `iplimit` 的 format 告警已修复）。
- [ ] 可独立运行包测试通过：`health`、`metrics`、`logs`、`logsink`、`utils`、`middleware/cors`、`middleware/limit`。
- [ ] 依赖外部服务的包在目标环境验证：`db/*`、`i18n`、`job`、`registry/*`。
- [ ] 最小服务 + `/health/*` + `/metrics` 手工回归。
- [ ] 变更涉及配置键时，README §5、`application.example.yml`、`docs/` 同步更新。
- [ ] 变更涉及能力边界时，README §2.4 同步更新。

## 8. 术语表

统一术语，文档、代码注释与提交信息均以此为准。

| 术语 | 英文/标识 | 定义 | 不要混用 |
|------|-----------|------|----------|
| 应用 | App | `mgin.NewApp` 返回的运行时对象，持有 Gin 引擎与生命周期 | 不要与「服务」「进程」混用 |
| 组件 | component | `go.config.used` 中声明的可加载资源（mysql/redis/mongodb/sqlite/nacos/etcd/consul/job 等） | 不叫「插件」（插件专指 `Use` 注册的外部模块） |
| 插件 | plugin | 业务通过 `mgin.MGin.Use/UsePlugin` 注册的外部组件 | — |
| 前缀 | prefix | `go.config.prefix.<组件>`，子配置文件名的前缀；SQLite 例外，值为文件名 | 不叫「路径」 |
| 子配置 | child config | `<prefix>-<env>.yml`，由组件自行解析 | 不叫「从配置」 |
| 数据源 | datasource | MySQL / SQLite / MongoDB / Redis 四类存储客户端 | 不把缓存、注册中心叫数据源 |
| 门面 | facade | `db.Mysql`、`db.Redis`、`db.Mongo`、`db.Sqlite` 单例对象 | — |
| DAO | Data Access Object | `db/dao` 下的泛型数据访问对象（`MySQLDao`/`MgoDao`） | 不指 `db.Mysql` |
| 执行器 | handler | `job.Register` 注册的任务函数 | 不叫「任务」（任务是库中的记录） |
| 探针 | probe | `/health/live`、`/health/ready`、`/health/startup` | — |
| 记录 | Record | 运行日志的结构化条目（`logsink.Record`） | 不叫「日志对象」 |
| 接口日志 | PostLog | 一次 HTTP 请求/响应的记录（`models.PostLog`） | 与「运行日志」区分 |
| 消息 ID | messageId | i18n 字典键，也是 `errcode` 的文案常量 | 不叫「错误信息」 |
| 业务码 | status / code | 响应体中的业务状态（`models.Result.Status`、`vo.Response.Code`） | 不叫「HTTP 状态码」 |

## 9. 需求变更流程

1. 在 issue/需求单中写明：动机、影响模块、配置/协议是否变化、兼容性。
2. 涉及能力边界（新增或删除数据源、改变返回协议）需架构评审。
3. 合并前同步更新：本文 §4/§6、README §2/§5、对应设计文档。
4. 版本与变更记录遵循 [发布规范](../10-contributing/04-release-process.md)。
