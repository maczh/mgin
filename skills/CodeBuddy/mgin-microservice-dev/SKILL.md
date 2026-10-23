---
name: mgin-microservice-dev
description: 基于 MGin 框架（github.com/maczh/mgin）快速开发 Go 微服务。涵盖 v1(jh 分支)/v2 两个版本的差异与选型、官方脚手架 `mgin new` 用法、配置系统（go.config.used 总开关、按 prefix-env 命名的子配置文件、HTTP 热重载）、泛型 DAO 与缓存/事务、统一响应封装、跨服务调用（超时/熔断/兜底）、健康探针与 Prometheus 指标、日志多目标与 logsink、定时任务、插件生态（mgkafka/mqtt/nats/mgrabbit/mssql/opensearch/hbase/hive/influxdb 的统一接入五步法）、寄海 jihai 业务项目的分层惯例（controller/service/mysql/model/router/ms/mw/mq/schedule）与 swagger 注解、静态编译部署与排障。当需要新建 MGin 微服务、给现有服务加接口/加数据源/加 MQ、排查「插件没初始化」「配置读不到」「泛型 DAO 报错」「探针 401/503」「调用超时或熔断」时使用。
---

# MGin 微服务开发 Runbook

框架仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`（权威文档：`docs/development/`）
参考业务项目：`/home/macro/Work/go/src/jihaihotpot.com/jihai/`（jh-ris-* 系列）
Go 版本：**1.25.7**

---

## 0. 第一步永远是：判版本

MGin 有两条并行的线，API 不兼容，选错会白写半天。

| 维度 | **v1（jh 分支，当前工作树）** | **v2（v2 分支）** |
|---|---|---|
| module | `github.com/maczh/mgin` | `github.com/maczh/mgin/v2` |
| 包路径 | `mgin/config`、`mgin/db`、`mgin/logs`、`mgin/db/dao`、`mgin/registry`、`mgin/utils` | `mgin/v2/pkg/config`、`/pkg/db`、`/pkg/logs`、`/pkg/db/dao`、`/pkg/registry`、`/pkg/utils` |
| 业务版本号 | `v1.25.x-jh` | `v2.0.x` |
| 配置文件 | 常放项目根 `<项目名>.yml`，`NewApp("", ...)` | 放 `conf/application.yml`，`NewApp("conf/application.yml", ...)` |
| 路由注册签名 | `Register(e *gin.Engine)` | `RegisterRoutes(app *mgin.App)` |
| DAO 实例化 | `dao.MySQLDao[T]{}`（**值**） | `&dao.MySQLDao[T]{}`（**指针**） |
| 插件契约 | `MginPlugin{Init([]byte); Close(); Check() error}` | `plugin.Plugin{Name/Order/Init(ctx)/Close(ctx)/Health/Enabled}`，**同时兼容 v1 的 `UsePlugin`** |
| 健康/指标/日志总线 | **已有** `health`、`metrics`、`logsink`（与 v2 能力对齐） | 同样有，且多 `otel` |
| v2 独有 | — | `pkg/otel`、`pkg/loadbalancer`、`pkg/internal`、`go.framework.{health,metrics,otel,shutdownTimeout,loadBalancer}`、`db.Mysql.UseCache()` |

**选型建议**：新服务优先 **v2**（脚手架产物、K8s 探针、指标、负载均衡都齐）；维护存量 jh-ris-* 老服务时沿用它在 go.mod 里已有的版本，**不要混用**——`jh-ris-booking` 的 go.mod 里 v1 和 v2 同时存在是因为公共库 `jh-ris-common` 还没迁完，这是历史债，不是范例。

判版本最快的方法：
```bash
grep -n "maczh/mgin" <项目>/go.mod
grep -rn "mgin/v2" <项目>/main.go
```

---

## 1. 铁律（违反必踩坑）

1. **HTTP 状态码恒 200**。404 / panic / 限流 / 业务失败全都返回 200，成败由响应体的 `status`（框架）或 `code`（jihai 业务）表达，**`== 1` 才是成功**。
2. **日志占位符是 `{}` 不是 `%s`**：`logs.Info("订单{}创建成功", orderId)`。traceId 由框架自动注入前缀。
3. **`UsePlugin` 的 name 必须出现在 `go.config.used` 里**，且 `go.config.prefix.<name>` 必须存在，否则框架只打一行 `logs.Error("加载{}失败，配置文件中未使用")` 然后静默跳过——插件不会初始化，调用时 nil panic。
4. **模型必须实现 `TableName() string`**（DAO 泛型约束 `schema.Tabler`）。
5. **MongoDB `GetConnection()` 后必须 `defer db.Mongo.ReturnConnection(db)`**。
6. **跨服务调用只用 `client.Call` / `CallT[T]` / `CallResultT[T]` / `CallPageT[T]`**（v1 `github.com/maczh/mgin/client`，v2 `.../pkg/client`）。`utils` 里没有。它会自动透传 trace 三头。
7. **定时任务的 cron 存在数据库 `mgin_job_info` 表里**，不在 yml；yml 的 `go.job.*` 只控制调度器本身。
8. **没有 `mgin.MGin` 包级变量，也没有 `mgin.Default()`**。一律 `app.MGin.UsePlugin(...)`。文档里写 `mgin.MGin.Use(...)` 的地方是按字面编译不过的（见 `pitfalls.md` §1）。
9. 组件配置文件名固定 **`<prefix>-<env>.yml`**（如 `mysql-test.yml`），由 `go.config.prefix.<name>` + `go.config.env` 拼出。
10. **`NewApp` 不再返回 nil**：`-v` 分支已改 `os.Exit(0)`。`if app == nil` 可留可删，但真正要判空的是 `app.MGin`（框架内部才有这种场景）。
11. **`WithContext(&ctx)` 改的是接收者本身**，并发复用同一 DAO 会串上下文；并发场景先复制 DAO。
12. **`Updates(entity)` 不带 WHERE**，条件更新用 `dao.DB().Model().Where(...).Updates(...)`。
13. **`WithCache` 的 `TTL<=0` 表示永不过期**，必须显式设 TTL；`RedisCache.Clear()` 是 **FLUSHDB**。
14. **i18n 与 VictoriaLogs 配置错误会在启动时 panic**（不是降级）：`i18n.Init()`、`logs.InitVictoriaLogs()`。
15. **探针/指标优先用配置开关**（`go.health.enabled` / `go.metrics.enabled`），框架会在 `baseRouter` 最前面注册；用 `app.EnableHealth()` 会带上当时已挂的中间件，可能被 jwt/casbin 拦成 401。
16. **热重载（`PUT /cfg/reload`）不重建路由/中间件/端口/TLS**，这些只能重启。

---

## 2. 新建项目：先用官方脚手架

脚手架在框架仓库的 `cmd/` 下，是**独立 module**（v1: `github.com/maczh/mgin/cmd/mgin`；v2: `.../mgin/v2/cmd/mgin`），不会污染主模块：

```bash
# v1 脚手架（当前 jh 工作树）
cd /home/macro/Work/go/src/github.com/maczh/mgin/cmd && go build -o /tmp/mgin .

# v2 脚手架：先切分支再构建
cd /home/macro/Work/go/src/github.com/maczh/mgin && git worktree add /tmp/mgin-v2 v2
cd /tmp/mgin-v2/cmd && go build -o /tmp/mgin .
```

⚠️ 当前工作树在 `jh` 分支，本地还有 `master-1.19` / `master-1.25` / `v2` 分支，别切错。

可用选项：

```
--module <path>          Go module 路径（默认 github.com/maczh/<工程名>）
--port <port>            默认 8080
--db  <list>             mysql,postgres,sqlite,mongodb,redis,clickhouse,elasticsearch
--mq  <list>             nats,kafka,mqtt,rabbit（可多选）
--registry <type>        nacos,consul,etcd,none
--config-center <type>   nacos,consul,etcd,polaris,springconfig,file,none
--i18n / --jwt / --casbin / --sys（sys 仅 v1 master）
--output <dir> / --force / --mgin-version <ver>
```

```bash
/tmp/mgin new myservice \
  --module jihaihotpot.com/jihai/myservice \
  --db mysql,redis --mq nats,kafka \
  --port 8080 --registry etcd --config-center file
```

产出：`main.go version.go Makefile plugins.go(选了 MQ) go.mod README.md .gitignore`、`router/ controller/ service/ dao/ model/`、`conf/{application.yml,<db>-test.yml,...}`。

⚠️ **脚手架已知问题**：`--db postgres|clickhouse` 会生成 `dao.PostgresDao` / `dao.ClickhouseDao`，而 `db/dao` 只有 `MySQLDao` / `MgoDao` → **生成即不可编译**；`--db sqlite|mongodb|redis|elasticsearch` 不生成 `dao/`，service 用内存示例。`--db kafka` 不是合法值（kafka 属于 `--mq`）。

**但脚手架产物跟 jihai 实际项目的惯例不同**，第二件事就是按 §3 调整骨架。

---

## 3. 目标骨架（脚手架 + jihai 惯例融合）

```
<project>/
├── main.go                 # mgin.NewApp + 插件注册 + 中间件 + 路由 + [MarkStarted] + app.Run()
├── version.go              # Version/BuildNum/BuildTime/GitHash（-ldflags 注入，禁止在代码里赋值）
├── Makefile                # tidy/build/linux/run/test/test-race/cover/lint/clean
├── conf/                   # ★ v2 规范；v1 老项目放根目录
│   ├── application.yml
│   └── mysql-test.yml  redis-test.yml  kafka-test.yml ...
├── consts/const.go         # 业务状态常量（订单/支付/预订状态机）
├── model/
│   ├── entity.go           # 实体：gorm tag + json 小驼峰 + TableName()；（双写时再加 bson + CollectionName()）
│   ├── ro/req.go           # Request Object：入参
│   └── vo/                 # View Object：出参 + result.go（统一响应）
├── mysql/enter.go          # ★ DAO 层：var XxxDao = &dao.MySQLDao[model.Xxx]{}（v1 可不用指针）+ MySQLInit()
├── mongo/enter.go          # Mongo DAO（可选）
├── service/                # enter.go 聚合单例 + xxx.go，返回 vo.Response[any] / vo.ResponsePage[any]
├── controller/             # enter.go 聚合单例 + xxx.go
├── router/router.go        # RegisterRoutes(app *mgin.App) 或 Register(e *gin.Engine)
├── ms/                     # 跨服务调用：client.go + 各下游服务封装（超时/兜底写在这里）
├── mw/jwt.go               # 鉴权中间件（可选）
├── mq/                     # MQ 订阅集中注册（可选）
├── schedule/               # cron 定时任务（可选）
└── docs/                   # swag 生成的 docs.go / swagger.json / swagger.yaml
```

可选补充（脚手架不生成）：`dto/`（请求/响应 DTO）、`internal/`（不对外）、`testdata/`、`migrations/`（框架不提供迁移器，自己管）。

**命名坑（真实项目里不统一，新项目请统一成左列）**：
- DAO 目录叫 **`mysql/`**（jihai 全系），不叫 `dao/`（脚手架模板叫 `dao/`）
- 常量目录：**`consts/`**（booking/print）vs `constants/`（order）→ 统一 `consts/`
- MQ 目录：`mq/`（order）vs `mqx/`（store-order）vs `mqttsub/`（booking）→ 统一 `mq/`

---

## 4. 分层职责（一句话一层）

| 层 | 入参 | 返回 | 允许做什么 |
|---|---|---|---|
| **controller** | `*gin.Context` | `vo.Response[any]` / `vo.ResponsePage[any]` | 只做：绑定参数 → 校验 → 调 service → 返回。**不写业务、不碰 DAO** |
| **router** | `*gin.RouterGroup` / `*mgin.App` | — | `prefix.POST("/x", func(c){ c.JSON(http.StatusOK, controller.X.Y(c)) })` |
| **service** | 业务参数 / `context.Context` | `vo.Response[any]` 或 `error` | 业务逻辑、事务编排（`db.MySQLTx`）、调 DAO、调 `ms/`、发 MQ |
| **mysql/ (DAO)** | — | `(data, error)` | 泛型 DAO 调用 + `MySQLInit()` 做 AutoMigrate + 绑定缓存 |
| **model** | — | — | 实体 + `TableName()`；`ro/` 入参、`vo/` 出参 |
| **ms/** | 业务参数 | `(data, error)` | 服务发现 + `client.CallT`，转 `error`；**超时/兜底集中在这里** |
| **mq/** | — | — | 订阅注册（`Register()` 集中调用）+ 发送封装 |

**service 的错误返回有例外**：被 controller 调用时返回 `vo.Response`；被 MQ handler / job / 内部函数链式调用时返回 `error`。同一个 service 里两种签名并存是正常的。

**面向生产的最低要求**：每个出站调用给 `Timeout`；关键只读查询可配 `Fallback`；需要 SLO 时挂 `metrics.Middleware()`；异常路径别用 `panic`（框架会转成 200 + `status=1001`）。

---

## 5. 参考文件路由表

| 你要做什么 | 读哪个文件 |
|---|---|
| 写 main.go、配 application.yml、读配置、热重载、连 DB、用 DAO/缓存/事务、统一响应、i18n、跨服务调用、日志、定时任务、中间件、注册中心、utils | `references/framework-api.md` |
| 接 Kafka / MQTT / NATS / RabbitMQ / MSSQL / OpenSearch / HBase / Hive / InfluxDB，或自己写插件 | `references/plugins.md` |
| 健康探针、Prometheus 指标、trace 透传、日志多目标与 logsink、配置热重载、部署与优雅关闭、测试命令与已知约束、故障速查 | `references/observability-deploy-testing.md` |
| 照抄一套可直接编译的分层代码（controller/service/dao/model/router/ms/mw/vo/swagger/Makefile） | `references/code-templates.md` |
| 想知道 jihai 实际项目怎么组织、`vo.Response` 长什么样、鉴权怎么做、swagger 怎么挂 | `references/project-conventions.md` |
| 编译不过 / 插件没生效 / 配置读不到 / 缓存不命中 / 探针 401 / 文档说谎 | `references/pitfalls.md` |

---

## 6. 最短上手路径（10 分钟）

1. `cd mgin/cmd && go build -o /tmp/mgin . && /tmp/mgin new myservice --db mysql,redis --registry etcd`
2. 补齐 `conf/application.yml` 的 `go.config.used` 与 `go.config.prefix.*`（见 `framework-api.md` §2）
3. 按需打开 `go.health.enabled` / `go.metrics.enabled`，在 main.go 里挂 `metrics.Middleware()`、初始化完成后 `health.MarkStarted()`
4. 把 `dao/dao.go` 改名挪到 `mysql/enter.go`，补 `MySQLInit()` 做 AutoMigrate（见 `code-templates.md` §4）
5. 建 `model/vo/result.go`——**每个项目复制一份**，见 `code-templates.md` §1
6. 写 `model/ro` + `service` + `controller` + `router`，照 `code-templates.md` §2–§6 抄；出站调用在 `ms/` 里统一给 `Timeout`
7. `make build && ./myservice`；`-v` 看版本，`/api/.../docs/index.html` 看 swagger，`/health/ready` 看依赖

---

## 7. 与项目文档的关系

权威细节在仓库 `docs/development/`，按「快速上手 → 概念 → 开发 → 配置 → 治理 → 可观测 → 部署 → 参考 → 高级 → 贡献 → 附录」组织。最常用的几篇：

| 文档 | 用途 |
|---|---|
| `docs/development/03-development/01-project-layout.md` | 目录与分层规范 |
| `docs/development/03-development/04-data-access.md` | DAO/缓存/事务/多库 |
| `docs/development/04-configuration/*` | 配置模型、加载规则、热重载 |
| `docs/development/05-governance/01-discovery-and-load-balancing.md` | 注册中心能力矩阵与发现缓存 |
| `docs/development/08-reference/{variables,types,api}` | 配置键、类型、包级 API |
| `docs/development/11-appendix/03-migration-and-known-issues.md` | 迁移改动 + 已知问题（15 条） |

⚠️ 文档本身也有过期和不一致（`mgin.MGin`、`client` 无超时、缓存默认值、`logsink` 签名等），**冲突时一律以代码为准**，并把新发现的偏差补进 `references/pitfalls.md` §1。
