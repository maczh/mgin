# 术语表

文档、代码注释、提交信息统一使用以下术语。括号内为英文/标识，左列为**推荐用法**，右列为**不要混用**的说法。

| 术语 | 英文/标识 | 定义 | 不要混用 |
|------|-----------|------|----------|
| 应用 | App | `mgin.NewApp` 返回的运行时对象，持有 Gin 引擎与生命周期 | 服务、进程（可指部署实例，但不是 App） |
| 组件 | component | `go.config.used` 中声明的可加载资源：mysql/redis/mongodb/sqlite/nacos/etcd/consul/job | 插件、模块 |
| 插件 | plugin | 业务通过 `MGin.Use/UsePlugin` 注册的外部组件 | 组件 |
| 前缀 | prefix | `go.config.prefix.<组件>`，子配置文件名的前缀；SQLite 例外，值为文件名 | 路径、目录 |
| 子配置 | child config | `<prefix>-<env>.yml`，由组件自行解析 | 从配置、分配置 |
| 环境后缀 | env | `go.config.env`，拼在子配置名后 | profile |
| 工作目录 | WorkDir | 主配置文件所在目录，是子配置/casbin/SQLite/磁盘缓存的相对基准 | 运行目录、CWD |
| 数据源 | datasource | MySQL / SQLite / MongoDB / Redis 四类存储客户端 | 缓存、注册中心 |
| 门面 | facade | `db.Mysql`、`db.Mongo`、`db.Redis`、`db.Sqlite` 包级单例 | 客户端、连接池 |
| DAO | Data Access Object | `db/dao` 的泛型数据访问对象（`MySQLDao`/`MgoDao`） | 数据源、Repository |
| 事务绑定 | WithTx | 返回绑定 `*gorm.DB` 的 DAO 副本 | 事务本身 |
| 执行器 | job handler | `job.Register` 注册的函数，`HandlerFunc` 类型 | 任务 |
| 任务 | job | 数据库中的调度记录（`JobInfo`），通过 `handler_name` 关联执行器 | 执行器、定时器 |
| 调度策略 | block/misfire strategy | `serial/discard/concurrent/cover`、`do_nothing/fire_now` | 重试策略 |
| 探针 | probe | `/health/live`、`/health/ready`、`/health/startup` | 监控、指标 |
| 指标 | metric | Prometheus 指标，经 `/metrics` 暴露 | 日志、追踪 |
| 接口日志 | PostLog | `models.PostLog`，一次 HTTP 请求/响应记录 | 运行日志、访问日志（口语可用） |
| 运行日志 | Record | `logsink.Record`，`logs.*` 产生的结构化日志 | 接口日志 |
| 调用日志 | call log | `client` 上报到 VictoriaLogs 的下游调用记录 | 接口日志 |
| 日志管道 | logsink | 异步队列 + handler，负责投递与排空 | 日志库 |
| 处理器 | handler | `logsink.Handler`/`RecordHandler`，日志落地目标 | 中间件、HTTP handler |
| 中间件 | middleware | `gin.HandlerFunc`，请求链上的处理单元 | 拦截器、过滤器 |
| 响应信封 | envelope | `models.Result`（业务）/`vo.Response`（调用） | 响应体、返回结构 |
| 业务码 | status / code | 信封中的业务状态字段 | HTTP 状态码 |
| 消息 ID | messageId | i18n 字典键，也是 `errcode` 的文案常量 | 错误信息、错误码 |
| 错误码 | errcode | `errcode` 包中的 1000–1014 常量 | 业务码（值是业务码的一部分） |
| 注册中心 | registry | Nacos/Consul/etcd/Polaris，提供服务注册与发现 | 配置中心 |
| 配置中心 | config center | 存放 `application.yml`/子配置的服务 | 注册中心 |
| 服务发现缓存 | discovery cache | `CachedRegistry` 的进程内 TTL 缓存 | 本地缓存（易混，需限定） |
| 调用协议 | protocol | `x-form`/`json`/`restful`/`file` | 传输协议（HTTP/TCP） |
| 追踪 ID | request id | `X-Request-ID`，用于串联日志 | trace id（OTel，语义不同） |
| 优雅关闭 | graceful shutdown | 信号触发后排空日志、注销、关连接、等待在途请求 | 停止、退出 |
| 热重载 | reload | 重新加载配置与资源，不重建路由/端口 | 热部署、热更新（口语可用） |
| 能力边界 | scope | 本分支明确不实现的功能集合 | 限制（过于笼统） |

## 使用示例

- 正确：「在 `used` 中启用 `mysql` 组件，并在子配置中配置 DSN。」
- 错误：「打开 mysql 插件，把连接串放到从配置里。」
- 正确：「接口日志需要 MongoDB；运行日志走 `logsink`。」
- 错误：「访问日志和系统日志都写 Mongo。」
