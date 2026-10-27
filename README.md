# MGin 微服务框架（jh 分支）

MGin 是一个基于 [Gin](https://github.com/gin-gonic/gin) 的 Go 微服务运行时与脚手架：把配置、数据源、缓存、注册发现、服务调用、日志、错误码/国际化、定时任务、限流、健康探针与指标统一收口，业务代码只保留路由与领域逻辑。

- 模块路径：`github.com/maczh/mgin`
- 当前分支：`jh`（寄海定制线，版本线 `v1.25.x-jh`）
- Go 版本：`1.25.7`
- 许可证：MIT（见 [LICENSE](LICENSE)）

> 说明：仓库另有 `master` 分支（模块路径 `github.com/maczh/mgin/v2`，tag `v2.0.x`）。两条线的目录结构、API 与能力不同，本文档**只描述 `jh` 分支**。`README_CN.md`/`README_en.md` 是历史长文手册，其中 PostgreSQL、ClickHouse、ElasticSearch、Kafka、S3、内置系统管理模块等章节在本分支**没有实现**，请以本文与 `docs/` 为准（见 [功能边界](#24-当前分支未实现的能力)）。

> 近期（2026-10）能力更新：Nacos 注册中心/配置中心支持 **v1/v2/v3**；`go.discovery.registries` **多注册中心**并存；服务发现本地缓存（TTL + singleflight + 热刷新）与实例选择策略；`client.Call` 的**超时/重试/熔断/降级回放**；etcd/Consul 的 namespace/group/cluster 作用域；Redis `Check()` 改为真实 `Ping()`。详见 [§2](#2-核心功能清单)、[§5.5](#55-多注册中心与-nacos-版本)、[§6.4](#64-微服务调用)、[§7](#7-常见问题与故障排查)。

---

## 目录

1. [项目概述与目标](#1-项目概述与目标)
2. [核心功能清单](#2-核心功能清单)
3. [技术架构与依赖](#3-技术架构与依赖)
4. [目录结构解析](#4-目录结构解析)
5. [配置项说明](#5-配置项说明)
6. [使用示例](#6-使用示例)
7. [常见问题与故障排查](#7-常见问题与故障排查)
8. [贡献指南](#8-贡献指南)
9. [许可证](#9-许可证)
10. [文档索引](#10-文档索引)

---

## 1. 项目概述与目标

### 1.1 定位

MGin 提供「一个进程内可完成的微服务底座」：`mgin.NewApp(...)` 返回 `*App`，框架完成配置加载、数据源初始化、中间件装配与优雅关闭；业务只需注册路由与执行器。框架同时提供 `cmd/mgin` 脚手架，一次性生成可编译运行的服务骨架。

### 1.2 设计目标

| 目标 | 落地方式 |
|------|----------|
| 约定优于配置 | `go.config.used` + `go.config.prefix` 决定加载哪些组件与子配置 |
| 可裁剪 | 未在 `used` 中声明的组件完全不初始化；中间件按需 `Use` |
| 多环境一致 | 同一份 `application.yml`，通过 `go.config.env` 切换 `<prefix>-<env>.yml` |
| 统一协议 | HTTP 统一返回 `models.Result`（`status/msg/data/page`），错误码集中在 `errcode` |
| 可观测 | 访问日志、调用日志、`/health/*` 探针、`/metrics`（Prometheus） |
| 平滑退出 | 信号捕获 → 排空日志队列 → 注销注册中心 → 关闭数据源 |

### 1.3 适用与不适用

**适用**：内部 RESTful 微服务、需要快速接入 MySQL/Redis/MongoDB 的业务服务、需要统一日志与错误码的多团队协作。

**不适用**：需要多实例协调的分布式调度（`job` 为单机调度）、需要强一致分布式事务、需要开箱即用的管理后台（`models/sys` 只有模型，无 HTTP 实现）。

---

## 2. 核心功能清单

| # | 能力 | 入口 | 启用方式 | 说明 |
|---|------|------|----------|------|
| 1 | 应用运行时 | `mgin.NewApp` / `App.Run` | 代码 | 配置加载、路由、HTTP/HTTPS、优雅关闭 |
| 2 | 配置体系 | `config.Config` | 自动 | 本地文件 / HTTP / Nacos / Consul / etcd / Polaris / Spring Config |
| 3 | 配置热重载 | `PUT /cfg/reload` | `go.config.reload.token` 非空 | 仅重载配置与资源，不重建路由与端口 |
| 4 | MySQL | `db.Mysql` | `used` 含 `mysql` | GORM v2，多库、连接池、断线重连 |
| 5 | SQLite | `db.Sqlite` | `used` 含 `sqlite` | 纯 Go 驱动（`glebarez/sqlite`），无需 CGO |
| 6 | MongoDB | `db.Mongo` | `used` 含 `mongodb` | 多库、连接自愈 |
| 7 | Redis | `db.Redis` | `used` 含 `redis` | 多库、订阅；`Check()` 真实 `Ping()`，参与 `/health/ready` |
| 8 | 泛型 DAO | `db/dao` | 代码 | `MySQLDao[E]` / `MgoDao[E]`，含 `Where*` 缓存、事务绑定 |
| 9 | 缓存 | `cache` | 代码 | 内存（带过期）/ bitcask 磁盘 |
| 10 | 服务调用 | `client.Call` / `CallT` / `CallPageT` / `CallResultT` | 代码 | `x-form` / `json` / `restful` / `file` 四种协议；超时、单次重试、熔断、降级回放 |
| 11 | 注册发现 | `registry.Registry` | `go.discovery.registry` | Nacos / Consul / etcd / Polaris 统一接口 |
| 12 | 多注册中心 | `go.discovery.registries` + `client.Options.Registry` | 配置 | 命名实例并存；同集群可共享 `prefix`，按实例覆盖 namespace/group/clusterName |
| 13 | 发现缓存与选路 | `registry.CachedRegistry` | `go.discovery.cacheTTL` 等 | TTL + singleflight + 热刷新 + 下线订阅；`random`/`round_robin`/`weighted` |
| 14 | 熔断与降级 | `client.Options` | `go.discovery.circuitFailures` / `go.client.fallback.enabled` | 连续失败打开熔断，半开探测；可回放上次成功响应 |
| 15 | 中间件 | `middleware/*` | `Use` / 配置 | trace、postlog、cors 恒启用；jwt、casbin、xss、iplimit、limit、session、ratelimit 按需 |
| 16 | 限流 | `middleware/ratelimit` | `go.ratelimit.enabled` | 令牌桶 / 滑动窗口 / 并发；维度 global/ip/path/ip_path/header |
| 17 | 国际化 | `i18n` | `NewApp(..., true)` 或 `go.i18n.enabled` | 字典落 MySQL/SQLite，按 `X-Lang` 取词 |
| 18 | 错误码 | `errcode` | 代码 | 1000–1014 业务码 + messageId |
| 19 | 日志 | `logs` | 自动 | 控制台 / 文件 / VictoriaLogs；`{}` 占位符 |
| 20 | 日志管道 | `logsink` | 代码 | 每 handler 单协程 FIFO 队列，满时淘汰最旧 |
| 21 | 定时任务 | `job` | `go.job.enabled` | cron/fixed_rate/fixed_delay/once，管理路由 `/job/*` |
| 22 | 健康探针 | `health` | `go.health.enabled` 或 `App.EnableHealth()` | `/healthz`、`/health/live`、`/health/ready`、`/health/startup` |
| 23 | 指标 | `metrics` | `go.metrics.enabled` 或 `App.EnableMetrics()` | `/metrics`，HTTP/构建/插件指标 |
| 24 | 工具集 | `utils` | 代码 | 加密、编码、IP、文件、切片、并发容器等 |
| 25 | 插件扩展 | `mgin.MGin.Use` / `UsePlugin` | 代码 | 按 `used` 自动初始化外部组件 |
| 26 | 脚手架 | `cmd/mgin` | 命令行 | 生成 main/router/controller/service/dao/model/conf/Makefile |

### 2.4 当前分支未实现的能力

以下内容出现在 `README_CN.md`、`README_en.md` 或 `application.example.yml` 中，但**本分支没有对应实现**，文档与代码以此表为准：

| 能力 | 现状 |
|------|------|
| PostgreSQL（`db.Pg`、`PostgresDao`） | 无 `db/postgres` 包；`go.config.prefix.postgres` 未被读取；脚手架 `--db postgres` 生成的 `dao.PostgresDao` 无法编译 |
| ClickHouse（`db.Clickhouse`、`ClickhouseDao`） | 同上；仅有配置字段与脚手架字符串 |
| ElasticSearch 数据源 | 无客户端实现；`go.config.prefix.elasticsearch` 仅被解析 |
| Kafka 数据源/日志投递 | 无实现（`go.log.kafka` 配置项无消费方；日志可通过 `logsink` 自定义 handler 接入） |
| S3 对象存储（`go.s3`、`storage/s3`） | 本分支无 `storage/` 包，配置段无消费方 |
| 内置系统管理模块（`go.sys.*` 的 HTTP 接口） | 只有 `models/sys` 表模型与 `go.sys.*` 配置解析，没有 controller/service |
| 多级缓存 | 无 L1+L2 组合；`MySQLDao` 只绑定单个 `ICache` |
| gRPC 服务端 | 未实现（仅历史计划） |

---

## 3. 技术架构与依赖

### 3.1 分层

```
业务代码 (router / controller / service / job handler)
        │
        ▼
App 运行时 (app.go)  ── 中间件链 / 路由 / 生命周期 / 重载
        │
        ├── config（配置聚合 + 配置中心）
        ├── i18n / errcode（文案与错误码）
        ├── logs / logsink（运行日志与访问日志管道）
        ├── client / registry（调用治理 + 多注册中心 + 发现缓存）
        ├── job（调度器 + 管理接口）
        └── db（mysql / sqlite / mongo / redis）── cache
                │
                ▼
          外部基础设施（MySQL / SQLite / MongoDB / Redis / 注册中心 / 配置中心 / VictoriaLogs）
```

依赖方向自上而下；`utils`、`models`、`errcode` 为叶子包，可被任意层引用。

### 3.2 请求处理链（顺序即代码顺序）

`App.baseRouter()` 按以下顺序装配，后续 `Use` 不会回溯影响已注册路由：

1. `mountHealth()`（可选）→ 2. `/metrics`（可选）→ 3. `trace.TraceId()` → 4. `postlog.RequestLogger()` → 5. `cors.Cors(...)` → 6. `xlang.RequestLanguage()`（i18n 开启时）→ 7. `ratelimit.RateLimit()`（可选）→ 8. `nice.Recovery`
→ 业务路由（业务自行追加 jwt / casbin / xss / iplimit / limit / session）
→ `NoRoute` 404 → i18n 管理路由（可选）→ `/job`（可选）→ `PUT /cfg/reload`（可选）

探针与指标必须最先注册，否则会被业务鉴权/限流中间件拦截。详见 [系统架构说明](docs/development/02-concepts/01-architecture.md#32-为什么探针必须最先注册)。

### 3.3 关键依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `github.com/gin-gonic/gin` | v1.11.0 | HTTP 框架 |
| `gorm.io/gorm` + `gorm.io/driver/mysql` | v1.31.0 / v1.6.0 | MySQL ORM |
| `github.com/glebarez/sqlite` | v1.7.0 | SQLite 纯 Go 驱动 |
| `go.mongodb.org/mongo-driver` | v1.17.4 | MongoDB |
| `github.com/go-redis/redis/v7` | v7.4.1 | Redis |
| `github.com/knadh/koanf` | v1.4.3 | 配置解析（YAML） |
| `github.com/casbin/casbin/v2` + `gorm-adapter/v3` | v2.105.0 / v3.32.0 | 接口鉴权 |
| `github.com/prometheus/client_golang` | v1.19.1 | 指标 |
| `github.com/robfig/cron/v3` | v3.0.1 | 定时任务 |
| `github.com/microcosm-cc/bluemonday` | v1.0.27 | XSS 过滤 |
| `github.com/levigross/grequests` | `v0.0.0-20250606...` | 服务调用 HTTP 客户端 |
| `git.mills.io/prologic/bitcask` | v1.0.2 | 磁盘缓存 |
| `github.com/dgrijalva/jwt-go` | v3.2.0+incompatible | JWT（已停止维护，见 §7） |

完整依赖见 [go.mod](go.mod)；脚手架是独立 module，见 [cmd/go.mod](cmd/go.mod)。

### 3.4 运行时事实

- `go build ./...` 通过（Go 1.25.7）。
- 仓库**没有** Makefile、CI 配置、Dockerfile；构建/测试命令见 [开发环境搭建](docs/development/01-getting-started/02-environment.md)。
- 单元测试非全绿或需外部服务，基线见 [测试策略](docs/development/10-contributing/03-testing-strategy.md)。

---

## 4. 目录结构解析

```
mgin/
├── mgin.go                 # 全局入口：Init / initResources / checkAll / SafeExit / Reload / Use
├── app.go                  # App：NewApp、baseRouter、Run、EnableHealth/EnableMetrics、/cfg/reload
├── application.example.yml # 配置示例（含未实现的 s3 段，见 §2.4）
├── casbin.conf             # Casbin RBAC 模型（默认文件名要显式配置）
├── LICENSE                 # MIT
├── README.md               # 本文
├── README_CN.md            # 历史中文长文（部分章节已过时）
├── README_en.md            # 历史英文长文（部分章节已过时）
├── sdk_document.md         # 早期 SDK 文档
├── cmd/                    # 脚手架（独立 module：github.com/maczh/mgin/cmd/mgin）
│   ├── mgin.go             # 命令行入口与 usage
│   ├── new.go              # 参数解析/交互、版本探测
│   ├── scaffold.go         # 工程生成、used/prefix/数据层推导
│   └── templates.go        # 所有生成模板（main/router/controller/service/dao/model/conf/Makefile/go.mod）
├── config/                 # 配置聚合、远程配置中心、ReloadToken
├── db/                     # 数据源门面（Mysql/Mongo/Redis/Sqlite）+ 事务
│   └── dao/                # MySQLDao / MgoDao / RedisCache / Dao 接口
├── cache/                  # ICache、MemCache、DiskCache(bitcask)
├── client/                 # 微服务调用（四种协议、超时/重试/熔断/降级 + 泛型返回）
├── registry/               # 注册发现统一接口、多注册中心集合、本地缓存装饰器
│   ├── consul/  etcd/  nacos/  polaris/
│   ├── model/              # 统一实例模型（URL/权重/元数据）
├── middleware/             # trace/postlog/cors/ratelimit/jwt/casbin/xss/iplimit/limit/session/xlang
├── models/                 # Result/ResultPage/PostLog/Timestamp
│   ├── vo/                 # Response/ResponsePage（code/msg/data/srvTime）
│   └── sys/                # 系统管理表模型（仅模型，无 HTTP 实现）
├── i18n/                   # 字典存储（MySQL/SQLite）、翻译 API、/i18n CRUD
├── errcode/                # 业务错误码与 messageId 常量
├── logs/                   # 日志门面、console/file/VictoriaLogs 输出、Record 格式化
├── logsink/                # 异步日志管道（接口日志 Entry、运行日志 Record）
├── job/                    # 定时任务调度器、执行器注册、管理路由、表模型
├── health/                 # 探针与依赖检查
├── metrics/                # Prometheus 指标与 /metrics
├── casbin/                 # Enforcer 单例与策略维护
├── utils/                  # 工具函数（46 个文件）
└── docs/                   # 设计/开发文档（见 §10）
```

---

## 5. 配置项说明

### 5.1 配置文件定位

- `mgin.NewApp(configFile, appName, version, xlang)` 第一个参数为空时，使用命令行 `-f`（默认 `<可执行文件名>.yml`）。
- 路径不含 `/` 或 `\` 时，会拼接到**可执行文件所在目录**，因此 `go run .` 与编译后运行的行为不同（见 §7 FAQ 1）。
- `go.config.path` 可覆盖本地配置目录；默认配置文件目录同时作为 casbin 模型、SQLite/i18n 数据库的相对基准（`config.WorkDir`）。

### 5.2 最小配置

```yaml
go:
  application:
    name: myapp
    port: 8080
    project: myproj
    debug: true
  logger:
    level: debug
    out: console,file
    file: ./logs/myapp
  config:
    used: ""          # 不启用任何外部组件即可启动
    env: test
```

### 5.3 主要配置键

| 分组 | 键 | 默认 | 说明 |
|------|----|------|------|
| 应用 | `go.application.name` | 空 | 服务名；注册中心实例名、SQLite 默认文件名、i18n 表名前缀 |
| | `go.application.port` / `port_ssl` | 0 | 监听端口；`port>0` 启动 HTTP，配 `cert`/`key` 启动 HTTPS |
| | `go.application.project` | `DEFAULT` | 配置中心路径中的项目名 |
| | `go.application.debug` | false | gin debug + 日志级别强制 debug |
| | `go.application.ip` | 自动探测 | 注册到注册中心的 IP |
| 配置 | `go.config.used` | 空 | 逗号分隔的启用组件，**子串匹配**（`mysql` 会命中 `mysql` 与任何包含它的值） |
| | `go.config.prefix.<组件>` | 空 | 子配置文件前缀；SQLite 例外，值为文件名 |
| | `go.config.server` / `server_type` | 空 / 空 | 配置中心地址与类型：`file`/`nacos`/`consul`/`etcd`/`polaris`/`springconfig` |
| | `go.config.env` | 空 | 子配置文件后缀：`<prefix>-<env>.yml` |
| | `go.config.path` | 空 | `file` 模式的目录覆盖 |
| | `go.config.token` | 空 | Polaris 配置中心 token |
| | `go.config.apiversion` | `v1` | Nacos 配置中心接口版本：`v1`/`v2`/`v3`（`1`/`2`/`3` 等价） |
| | `go.config.namespace` | 空 | Nacos 配置中心 namespaceId（v1 以 `tenant` 传参） |
| | `go.config.username` / `password` / `accessToken` | 空 | Nacos 鉴权；`accessToken` 优先，为空时用用户名密码登录换取 token |
| | `go.config.reload.token` | 空 | 非空才注册 `PUT /cfg/reload`；请求头 `X-MGin-Reload-Token` 或 `Bearer` |
| 日志 | `go.logger.level` | error | debug/info/warn/error |
| | `go.logger.out` / `file` | 空 | 旧式输出：`console`/`file`，文件按天切分 |
| | `go.logger.outputs[]` | 空 | 新式多目标：`target`(console/file/victorialogs/自定义)、`format`(text/json)、`fields`、`logtext` |
| 访问日志 | `go.log.req` | 空 | 非空**且** `used` 含 `mongodb` 时才落库 |
| | `go.log.get` | 空（按 `full` 处理） | GET 响应记录：`full`/`line`（截断 120 字符）/`off` |
| | `go.log.sink.queue` / `workers` / `shutdown` | 100 / 1 / 3000 | 队列上限、消费协程（强制 1）、关闭超时(ms) |
| | `go.log.victorialogs.*` | 空 | `url`/`api_token`/`api_crt`/`request`/`client`/`timeout` |
| 发现 | `go.discovery.registry` | 空 | `nacos`/`consul`/`etcd`/`polaris`；空则用空实现（不注册不发现） |
| | `go.discovery.registries[]` | 空 | 命名注册中心实例：`name`/`type`/`prefix`/`namespace`/`group`/`clusterName`；**非空时取代 `registry`**，见 §5.5 |
| | `go.discovery.defaultRegistry` | 列表首项 | `client.Options.Registry` 为空时使用的实例名 |
| | `go.discovery.callType` | `x-form` | 调用协议默认值 |
| | `go.discovery.cacheTTL` / `cacheRefresh` | 60 / 60（秒） | 发现结果缓存与热刷新 |
| | `go.discovery.strategy` | `random` | `random`/`round_robin`/`weighted` 实例选择策略 |
| | `go.discovery.circuitFailures` / `circuitOpenWindow` | 3 / `30s` | `client.Call` 熔断阈值与探测窗口 |
| 调用 | `go.client.timeout` | `10s` | 单次调用超时，支持 `5s` 或数字（秒） |
| | `go.client.fallback.enabled` | false | 失败时回放上次成功响应（缓存无 TTL） |
| 安全 | `go.jwt.secret` | 空 | HS 系列签名密钥；未配置时 jwt 中间件直接 401 |
| | `go.casbin.enabled` / `model_file` | false / `rbac_model.conf` | 模型相对路径基于配置目录；仓库只提供 `casbin.conf` |
| | `go.cors.headers` | 空 | 追加允许头（基础头已固定） |
| 探针 | `go.health.enabled` / `autoStarted` | false / false | 挂载探针 / 监听后自动标记启动完成 |
| 指标 | `go.metrics.enabled` | false | 挂载 `/metrics` |
| 限流 | `go.ratelimit.enabled` / `rules[]` | false | 规则见 `application.example.yml` |
| 任务 | `go.job.enabled` / `initdb` / `tablePrefix` / `scanInterval` / `refreshInterval` / `logRetainDays` / `maxConcurrent` / `maxSerialQueue` / `timezone` | false / true / `mgin_` / 1 / 30 / 30 / 50 / 10 / Local | 调度器与管理接口 |
| 国际化 | `go.i18n.enabled` / `default` / `dbName` / `sqlite` / `api.enabled` | false / `zh-cn` / 空 / 空 / false | 字典库与 CRUD 路由 |
| 系统模块 | `go.sys.*` | — | **仅解析，无实现** |

### 5.4 数据源子配置

`used` 中的组件会按 `go.config.prefix.<组件>` 找到 `<prefix>-<env>.yml`（或配置中心同名配置），内容由各客户端自行解析：

```yaml
# mysql-test.yml
go:
  data:
    mysql: user:pwd@tcp(127.0.0.1:3306)/demo?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true
    mysql_pool: { max: 200, total: 1000, timeout: 30, life: 5 }
```

```yaml
# redis-test.yml
go:
  data:
    redis: { host: 127.0.0.1, port: 6379, password: "", database: 0 }
    redis_pool: { min: 3, max: 100, idleTimeout: 10, timeout: 300 }
```

```yaml
# nacos-test.yml（注册中心与配置中心共用同一份子配置）
go:
  nacos:
    server: 127.0.0.1
    port: 8848
    apiversion: v3            # v1/v2/v3，默认 v1，需与服务端版本一致
    namespace: public         # 注册中心 namespace
    clusterName: DEFAULT
    group: DEFAULT_GROUP
    username: nacos           # 开启鉴权时填写
    password: ""
    accessToken: ""           # 已有 token 时直接使用，优先于用户名密码登录
    lan: true                 # 优先用内网 IP 注册
    lanNet: 192.168.          # 内网网段前缀，配合 lan
```

使用本地子配置文件时要显式配置 `go.config.server_type: file`（留空会按 HTTP 源拼接地址，子配置取不到）；除 `sqlite` 外，`go.config.prefix.<组件>` 都是文件名前缀，框架按 `<prefix>-<env>.yml` 查找。多库示例、MongoDB 与 SQLite 说明见 [配置与数据结构设计](docs/development/03-development/04-data-access.md#2-数据源与配置键)。

### 5.5 多注册中心与 Nacos 版本

**多注册中心**：当服务需要同时访问多个注册中心（或多活/单元化的不同 namespace/group）时，用命名列表替代单值 `registry`。同集群的多个条目可共享同一 `prefix`，服务器地址与凭据只维护一份，差异用 `namespace`/`group`/`clusterName` 覆盖。

```yaml
go:
  config:
    env: test
  discovery:
    defaultRegistry: jhris        # 不带 Registry 的调用走它
    registries:
      - name: jhris
        type: nacos
        prefix: nacos             # → nacos-test.yml
        group: jhris
      - name: iot
        type: nacos
        prefix: nacos             # 复用同一 Nacos 集群
        group: iot
      - name: finance-bank
        type: nacos
        prefix: nacos
        namespace: finance
        group: bank_account
        clusterName: DEFAULT
```

| 项 | 行为 |
|---|---|
| 启用条件 | `go.discovery.registries` 非空即生效，`go.discovery.registry` 与 `used` 中的 `nacos`/`etcd`/`consul` 只影响其他组件，不再决定注册动作 |
| 初始化 | `mgin.Init`/`NewApp` 调用 `registry.RegisterConfigured()`，逐实例读取各自 `prefix` 子配置并注册 |
| 作用域覆盖 | 覆盖后的 YAML 会合并进子配置：Nacos/Consul 用 `namespace`/`group`/`clusterName`；etcd 编码进 key（覆盖 `namespace`/`cluster` 时默认 `keyTemplate` 为 `{namespace}/{prefix}/{group}/{cluster}/{service}/{instance}`）；Consul 额外按 group + cluster 标签过滤实例 |
| 缓存与订阅 | 每个命名实例有独立缓存、热刷新与下线订阅，互不影响 |
| 注销 | `DeRegister` 一次性注销全部实例，重复调用安全 |
| 选择后端 | `client.Options.Registry`；未知名称**不回退**默认实例，直接返回发现失败 |
| 兼容 | 未配置 `registries` 时，原有 `registry` + `used` + `prefix.<type>` 行为不变 |

**Nacos 版本**：`apiversion` 在配置中心（`go.config.apiversion`）和注册中心（`go.nacos.apiversion`）分别配置，均支持 `v1`/`v2`/`v3`，缺省按 `v1` 处理。

| 用途 | v1 | v2 | v3 |
|---|---|---|---|
| 配置中心 | `/nacos/v1/cs/configs?tenant=` | `/nacos/v2/cs/config?namespaceId=` | `/nacos/v3/client/cs/config?groupName=&namespaceId=`，读取 `data.content` |
| 注册/发现 | `/nacos/v1/ns/instance`、`clusters` | `/nacos/v2/ns/instance`、`clusterName` | `/nacos/v3/client/ns/instance`，续约走注册接口 + `heartBeat=true` |
| 登录 | `/nacos/v1/auth/login` | 同 v1 | `/nacos/v3/auth/user/login` |

发现响应结构差异由框架内部解析（v2/v3 为 `{code,message,data}` 包装），业务代码无需区分。v3 的 HTTP OpenAPI 不提供服务推送，框架仍使用轮询 watcher。

---

## 6. 使用示例

以下示例均可在本仓库直接编译（模块路径按业务工程调整）。

### 6.1 最小可运行服务

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin"
    "github.com/maczh/mgin/i18n"
)

func main() {
    app := mgin.NewApp("conf/application.yml", "demo", "1.0.0", false)
    app.EnableHealth()       // /health/*
    app.EnableMetrics()      // /metrics
    app.Router.GET("/ping", func(c *gin.Context) {
        c.JSON(200, i18n.Success(map[string]any{"pong": true}))
    })
    app.MarkHealthStarted()  // /health/startup 转 200
    app.Run()
}
```

```bash
go build -o demo . && ./demo     # 配置从可执行文件目录下 conf/application.yml 读取
curl -s localhost:8080/ping
curl -s localhost:8080/health/ready
```

### 6.2 统一响应与错误码

```go
import (
    "github.com/maczh/mgin/errcode"
    "github.com/maczh/mgin/i18n"
)

func GetUser(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        c.JSON(200, i18n.ParamLostError("id"))       // status=1003
        return
    }
    // 业务失败：code 用 errcode 常量，文案走 i18n 字典
    c.JSON(200, i18n.Error(errcode.DATA_NOT_FOUND, errcode.DataNotFound))
}
```

返回结构固定为 `{"status":1,"msg":"...","data":...,"page":...}`（`models.Result`）；HTTP 状态通常为 200，业务语义由 `status` 表达。

### 6.3 DAO 与事务

```go
type User struct {
    Id   int64  `gorm:"column:id;primaryKey;autoIncrement"`
    Name string `gorm:"column:name"`
}
func (User) TableName() string { return "user" }

var userDao = &dao.MySQLDao[User]{}     // E 必须实现 schema.Tabler

users, err := userDao.All(User{})
one, err := userDao.WhereOne("name = ?", "张三")

// 事务：回调内必须使用 tx（或绑定 tx 的 DAO）
n, err := db.MySQLTx(ctx, func(tx *gorm.DB) (int64, error) {
    return tx.Model(&User{}).Where("id = ?", 1).Update("name", "李四").RowsAffected, nil
})
```

### 6.4 微服务调用

```go
resp := client.CallT[User]("user-service", "/api/v1/users/{id}", &client.Options{
    Method:   http.MethodGet,
    Protocol: "restful",
    Path:     map[string]string{"id": "1"},
    Timeout:  2 * time.Second,
})
if resp.Code != vo.SUCCESS {
    logs.Error("调用失败: {}", resp.Msg)
}
```

`client.CallT` 解析 `{code,msg,data,srvTime}`；旧协议用 `client.CallResultT`（`{status,msg,data}`），分页用 `client.CallPageT`。调用前必须先初始化注册中心（`used`/`registry` 配置），否则返回「服务注册中心未初始化」。

调用链内置三层保护：`Options.Timeout`/`go.client.timeout`（默认 10s）→ `Retry`（重试一次）→ 熔断（`go.discovery.circuitFailures`，默认 3 次失败后打开 30s），并可用 `Options.Fallback` 或 `go.client.fallback.enabled` 回放上次成功响应。详见 [服务调用](docs/development/03-development/03-service-call.md)。

多注册中心场景用 `Options.Registry` 选择目标实例（名称来自 `go.discovery.registries[].name`），留空走 `defaultRegistry`：

```go
result, err := client.Call("iot-service", "/api/v1/device/query", &client.Options{
    Method:   http.MethodPost,
    Protocol: "x-form",
    Registry: "iot",          // 留空 = go.discovery.defaultRegistry
    Group:    "iot",          // 仍覆盖该实例配置的 group
    Data:     map[string]any{"sn": "A001"},
})
```

熔断 key（默认 `registry + service + uri`）、降级缓存与响应缓存都按注册中心名隔离，同名服务在不同注册中心之间不会互相污染；名称拼错时不会回退默认实例，而是直接返回「微服务获取 X 服务主机IP端口失败」。

### 6.5 鉴权中间件挂载顺序

```go
// casbin 依赖 jwt 写入的 claims，必须先 jwt 后 casbin
app.Router.Use(jwt.JwtAuthorize())
app.Router.Use(casbin.CasbinHandler())
```

Casbin 必须显式配置 `go.casbin.model_file: casbin.conf`（仓库默认模型文件名不是 `rbac_model.conf`），策略存于 MySQL 的 `casbin_rule` 表。

### 6.6 定时任务

```go
job.Register("syncUserJob", func(ctx *job.Context) error {
    ctx.Log("参数=%v", ctx.Param["bizDate"])
    return nil
})
```

任务记录存于 `mgin_job_info` / `mgin_job_log`（`go.job.initdb: true` 时自动迁移），管理接口挂在 `/job`（需 `go.job.enabled`）。

### 6.7 脚手架

```bash
cd cmd && go build -o bin/mgin . && cd ..

./bin/mgin new demo --module example.com/demo --port 8080 --db mysql --output /tmp
cd /tmp/demo
go mod tidy && go build -o demo . && ./demo
```

选项：`--db`（mysql/postgres/sqlite/mongodb/redis/clickhouse/elasticsearch）、`--registry`、`--config-center`、`--i18n`、`--jwt`、`--casbin`、`--mgin-version`。**`--db postgres|clickhouse` 生成的 DAO 类型不存在，无法编译**（见 §2.4）。

---

## 7. 常见问题与故障排查

**Q1：启动报「配置文件加载失败」或读到错误的环境配置？**
`NewApp` 对不含路径分隔符的文件名会拼到**可执行文件目录**。`go run .` 的二进制在临时目录，务必传绝对路径：`mgin.NewApp("/abs/path/conf/application.yml", ...)`。

**Q2：`go build` 报 `cannot use grequests.FromRequestOptions(...) as *grequests.RequestOptions`？**
`grequests` 新版本 API 已变更；本仓库 `config/configure.go` 已改为直接传 `&grequests.RequestOptions{}`。若本地依赖被降级/升级，请保持该写法。

**Q3：`go test ./...` 在 `middleware/iplimit` 失败？**
旧版本存在 `log.Printf(变量)` 导致的 vet 报错（`non-constant format string`），已修复为 `log.Printf("%s", ...)`。若仍报错，确认代码未被回滚。

**Q4：MySQL 连接成功但 `GetConnection()` 返回 `multi get connection must specify a database name`？**
多库模式（`go.data.mysql.multidb: true`）下必须传库名：`db.Mysql.GetConnection("test1")`。同理，casbin 在多库模式下不可用（其 adapter 未指定库名）。

**Q5：`/health/ready` 返回 503，但 MySQL 实际可用？**
`Check()` 失败会自动重连一次，仍失败才返回 503；检查 `go.config.used` 是否包含 `mysql`/`mongodb`/`redis`。SQLite 不参与就绪检查。

**Q6：Redis 挂了，`/health/ready` 仍然 200？**
`db/redis` 的 `Check()` 已改为真实 `Ping()`（多库逐个校验，任一失败即返回错误），因此 Redis 不可用时 `/health/ready` 会返回 503。若仍为 200，确认 `used` 含 `redis`，且连接已初始化（未初始化时 `Check()` 会返回 `redis connection is not initialized`）。

**Q7：限流触发后客户端收到 200？**
`ratelimit` 默认用 HTTP 200 + 业务码 `1011`（`errcode.TOO_MANY_REQUESTS`）表达限流，可在规则中配置 `httpStatus` 改为 429。

**Q8：`jwt` 鉴权失败没有响应体？**
`JwtAuthorize` 使用 `AbortWithStatus(401)`，无 body；需要统一错误体时自行包一层中间件或改用业务鉴权。

**Q9：Casbin 报「权限服务不可用」/ Enforcer 为 nil？**
没有配置 `go.casbin.model_file` 时会找 `rbac_model.conf`，而仓库只有 `casbin.conf`；且 Enforcer 依赖 MySQL 连接。配置 `model_file: casbin.conf` 并确保 `used` 含 `mysql`。

**Q10：接口日志没有落库？**
需同时满足：`go.log.req` 非空、`go.config.used` 含 `mongodb`；多库模式还要 `go.log.dbName`。否则仅打印/丢弃。

**Q11：`i18n` 启动 panic 或查不到文案？**
`i18n.Init()` 失败会 panic。校验 `go.application.name` 是否匹配 `^[A-Za-z0-9_.-]{1,59}$`（表名 `{app}_i18n` 的前缀），以及 MySQL/SQLite 是否可用。未初始化时 `i18n.String` 原样返回 messageId。

**Q12：`job` 启动失败？**
`job` 至少需要一个数据库（MySQL 或 SQLite），且 `go.job.enabled: true`。它是**单机调度**，多实例部署会重复执行。

**Q13：脚手架生成的服务编译失败（`undefined: dao.PostgresDao`）？**
`--db postgres`/`--db clickhouse` 会生成不存在的 DAO 类型；改用 `mysql`，或自行实现数据访问。

**Q14：`go.sys.*` 配置打开了但没有管理接口？**
本分支只保留 `models/sys` 表模型，没有内置 controller/service；建表与接口需业务侧实现（`go.sys.initdb` 也不会执行迁移）。

**Q15：日志等级设了 debug 但看不到 debug 日志？**
`go.logger.level` 默认 `error`；`go.application.debug: true` 会强制 debug。使用 `outputs` 时各目标共享同一等级阈值。

**Q16：Nacos 配置中心升级到 v2/v3 后子配置读不到？**
先确认 `go.config.apiversion` 与服务端一致（v1/v2/v3 的路径、参数名与响应结构都不同，v3 读 `data.content`）；再确认 `go.config.namespace` 与 dataId 所在的 namespace 一致。开启鉴权时 v1 与 v3 的登录路径不同（`/nacos/v1/auth/login` 与 `/nacos/v3/auth/user/login`），可先用 `accessToken` 直接验证。鉴权或路径不匹配时子配置会返回 nil，只记录日志不返回错误，需留意启动日志中的「配置中心登录失败」「配置中心请求返回 HTTP」等提示。

**Q17：多注册中心调用返回「微服务获取 X 服务主机IP端口失败」？**
`Options.Registry` 必须与 `go.discovery.registries[].name` 完全一致，未知名称**不会**回退到 `defaultRegistry`；同时确认该实例的 `prefix` 指向的子配置能被读到（本地文件需 `go.config.server_type: file`）。每个实例有独立缓存，修改 group/namespace 后要等 `cacheTTL` 过期或重启生效。

**Q18：多实例部署后注册中心出现重复实例（同一 IP:端口登记多次）？**
Consul 的实例 ID 由 IP、端口、服务名、namespace、group、cluster 共同哈希（12 位），同机部署在不同 namespace/group 的实例会被区分开。若仍是重复，检查是否同时配置了 `go.discovery.registry` 与 `go.discovery.registries`（后者生效），或子配置中 `group`/`clusterName` 与注册中心控制台不一致。

---

## 8. 贡献指南

1. **分支**：`jh` 为当前定制线，功能开发从此拉分支（如 `feat/xxx`、`fix/xxx`）；面向上游通用能力的改动请同时评估 `master`。
2. **提交信息**：本仓库历史提交以「序号. 变更点」列举为主。新提交建议使用 `type(scope): subject`（`feat`/`fix`/`refactor`/`docs`/`test`/`chore`），正文说明动机与影响面，详见 [编码规范与协作流程](docs/development/10-contributing/02-coding-standards.md#7-git-工作流)。
3. **提交前自检**：
   ```bash
   gofmt -l . && go build ./... && go vet ./... && go test ./health/ ./metrics/ ./logs/ ./utils/ ./middleware/cors/
   ```
4. **新增配置键**：必须同时更新 `config/configure.go` 解析、`application.example.yml`、README §5 与对应文档，避免「文档有、代码无」。
5. **新增能力**：提供单元测试；涉及外部依赖的用例要能在无服务时跳过或使用本地 stub。
6. **文档**：术语使用 [需求文档 §8 术语表](docs/development/02-concepts/04-requirements.md#8-术语表)；行为变化必须更新 `docs/` 与本文「未实现能力」表。
7. **Pull Request**：描述变更动机、兼容性影响、验证命令与结果；不要提交本地配置、`logs/`、二进制产物。

---

## 9. 许可证

本项目采用 MIT 许可证，Copyright (c) 2022 maczh。完整条款见 [LICENSE](LICENSE)。使用、修改、分发请保留版权声明与许可声明。

---

## 10. 文档索引

完整文档树从 [docs/development/README.md](docs/development/README.md) 进入，按「快速上手 → 概念 → 开发 → 配置 → 治理 → 可观测 → 部署 → 参考 → 高级 → 贡献 → 附录」组织。

| 文档 | 内容 |
|------|------|
| [开发者文档总目录](docs/development/README.md) | 11 个分区、角色阅读路径、文档约定 |
| [需求文档](docs/development/02-concepts/04-requirements.md) | 背景、角色、功能/非功能需求、范围边界、验收标准、术语表 |
| [系统架构说明](docs/development/02-concepts/01-architecture.md) | 分层、启动时序、请求链、配置体系、生命周期、设计取舍、已知风险 |
| [模块与接口说明](docs/development/08-reference/api/README.md) | 各包职责、导出 API、签名与约束 |
| [变量与配置键](docs/development/08-reference/variables.md) | 全局变量、常量、全部配置键（类型/默认值/约束） |
| [类型定义](docs/development/08-reference/types.md) | 导出类型、字段、嵌套关系与示例 |
| [协议参考](docs/development/08-reference/protocols.md) | 响应信封、错误码、Header、调用协议、管理端点 |
| [命令参考](docs/development/08-reference/commands.md) | 脚手架、Makefile、运维命令 |
| [数据结构设计](docs/development/03-development/04-data-access.md) | MySQL/Redis/MongoDB/SQLite 表与键、DAO 缓存键、生命周期与迁移 |
| [开发环境搭建](docs/development/01-getting-started/02-environment.md) | 工具链、依赖、构建、运行、本地中间件、排障 |
| [编码规范与协作流程](docs/development/10-contributing/02-coding-standards.md) | 风格、错误与日志规范、配置/中间件扩展清单、Git 工作流、评审清单 |
| [测试策略](docs/development/10-contributing/03-testing-strategy.md) | 测试分层、现有用例清单、运行方式、实测基线、CI 建议 |
| [版本发布与变更记录规范](docs/development/10-contributing/04-release-process.md) | 版本号、分支模型、发布步骤、CHANGELOG 约定、回滚 |
| [演进路线图 v1.26→v3.0](docs/development/09-advanced/04-evolution-roadmap.md) | 行业能力基线、差距分析、分版本能力建设与优先级 |
| [i18n 使用说明](docs/i18n.md) | 字典表与 API 细节 |
| [VictoriaLogs 接入](docs/victorialogs.md) | 日志上报配置 |
| [服务发现与负载均衡](docs/development/05-governance/01-discovery-and-load-balancing.md) | 四种注册中心能力矩阵、可选接口、缓存/策略/熔断配置 |
| [服务调用](docs/development/03-development/03-service-call.md) | `client` 协议、超时、重试、熔断与兜底 |
| [注册中心优化说明](docs/registry_optimization_jh_overview.md) | 本地缓存与热刷新设计 |
| [SDK 文档](sdk_document.md) | 早期 SDK 使用说明（部分 API 已变更） |
