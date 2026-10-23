# 框架简介

MGin 是一个**进程内**的 Go 微服务运行时，建立在 Gin 之上。它不是独立的网关或 Sidecar：业务进程 `import "github.com/maczh/mgin"`，调用 `mgin.NewApp(...)` 后得到一个已经装配好配置、数据源、中间件与生命周期的 `*App`。

## 1. 它解决什么问题

写一个内部微服务，通常要重复处理这些事情：

- 从本地文件或配置中心读配置，还要按环境拆分；
- 连接 MySQL/Redis/MongoDB，处理多库、连接池、断线重连；
- 注册到 Nacos/Consul/etcd/Polaris，并在调用其他服务时做服务发现；
- 统一响应结构、错误码、国际化文案；
- 采集访问日志、运行日志，落到文件或日志平台；
- 暴露 K8s 探针与 Prometheus 指标；
- 收到 SIGTERM 时排空日志、注销实例、优雅关闭。

MGin 把这些横切能力收敛为可配置的组件，业务只写路由与领域逻辑。一个最小服务只需要一个 `main.go` 和一个 YAML。

## 2. 适合与不适合

| 适合 | 不适合 |
|------|--------|
| 内部 RESTful 微服务、管理后台 API | 需要强一致分布式事务的系统 |
| 需要快速接入 MySQL/Redis/MongoDB 的业务服务 | 需要多实例协调的分布式调度（`job` 是单机调度） |
| 多团队共享统一日志、错误码、探针规范 | 需要开箱即用的完整管理后台（本分支只有 `models/sys` 模型） |
| 单二进制 + 配置文件的部署形态 | 以 gRPC 为主的体系（本分支无 gRPC 服务端） |

## 3. 与「只用 Gin」的区别

只用 Gin 时，配置、数据库、日志、探针、优雅关闭都由业务自行拼装；MGin 额外提供：

1. **配置驱动的组件装配**：`go.config.used` 决定加载哪些组件，未启用时零开销。
2. **统一协议**：`models.Result{status,msg,data,page}` + `errcode` + i18n 字典。
3. **服务治理基础件**：注册发现（含本地缓存与降级）、四种调用协议、限流、JWT/Casbin。
4. **可观测契约**：探针路径、指标名、访问日志结构固定，接入监控无需二次约定。
5. **可生成骨架**：`cmd/mgin` 生成可编译工程，减少从零搭建。

## 4. 能力一览（当前分支）

| 能力 | 入口 | 默认 |
|------|------|------|
| 应用生命周期 | `mgin.NewApp` / `App.Run` | 启用 |
| 配置（本地/HTTP/Nacos/Consul/etcd/Polaris） | `config.Config` | 启用 |
| MySQL / SQLite / MongoDB / Redis | `db.Mysql` / `db.Sqlite` / `db.Mongo` / `db.Redis` | 按 `used` |
| 泛型 DAO + 查询缓存 | `dao.MySQLDao` / `dao.MgoDao` | 代码使用 |
| 内存 / bitcask 缓存 | `cache.OnMemCache` / `OnDiskCache` | 代码使用 |
| 服务调用（x-form/json/restful/file） | `client.Call*` | 代码使用 |
| 注册发现（Nacos/Consul/etcd/Polaris） | `registry.Registry` | 按配置 |
| 中间件（trace/postlog/cors/recovery） | `App.Router` | 启用 |
| JWT / Casbin / XSS / IP 限制 / 并发限制 / Session | `middleware/*` | 按需挂载 |
| 限流（令牌桶/滑动窗口/并发） | `go.ratelimit` | 关闭 |
| 国际化 + 错误码 | `i18n` / `errcode` | 关闭 |
| 日志 + 日志管道 | `logs` / `logsink` | 启用 |
| 定时任务 | `job` | 关闭 |
| 健康探针 / Prometheus 指标 | `health` / `metrics` | 关闭 |
| 脚手架 | `cmd/mgin` | — |

## 5. 关键事实（避免踩坑）

- **一个进程只能有一个 App**：配置、数据源、注册中心都是全局单例。
- **探针与 `/metrics` 在 `NewApp` 内最先注册**，不会被后续业务中间件拦截。
- **业务错误默认 HTTP 200**，用响应体的 `status` 表达；限流默认也是 200 + `status=1011`。
- **Redis 的 `Check()` 恒成功**，`/health/ready` 不会反映 Redis 故障。
- **配置项 `used` 是子串匹配**，组件名不要互相包含。
- 本分支**没有** PostgreSQL/ClickHouse/ElasticSearch/Kafka/S3 实现，详见 [README §2.4](../../../README.md#24-当前分支未实现的能力)。

## 下一步

- 想立刻运行：[快速上手](03-quickstart.md)
- 想理解结构：[架构说明](../02-concepts/01-architecture.md)
- 想按角色选阅读路径：[阅读路径](04-learning-paths.md)
