# 核心抽象

本文定义框架里反复出现、且最容易叫混的概念。术语的权威定义见 [术语表](../11-appendix/02-glossary.md)。

## 1. App：应用运行时

```go
app := mgin.NewApp(configFile, appName, version, xlang)
```

`*App` 持有 Gin 引擎（`app.Router`）、配置路径、i18n 开关与探针/指标挂载状态。它负责：

- 解析 `-f`/`-v` 参数，定位配置文件；
- 初始化配置、日志、数据源、注册中心、job；
- 装配默认中间件链；
- `Run()` 启动监听、处理信号、优雅关闭。

**约束**：全局单例（`config.Config`、`db.*`、`registry.Registry`）决定一个进程只能有一个 App；测试中需要串行或重置。

## 2. 组件（component）与子配置

`go.config.used` 中声明的资源称为**组件**，例如：

```yaml
go:
  config:
    used: "mysql,redis,nacos,job"
    prefix:
      mysql: mysql    # 子配置文件名前缀
      redis: redis
```

- 组件是否初始化由 `used` 的**子串匹配**决定。
- 组件的配置放在 `<prefix>-<env>.yml`（**子配置**）中，由组件自己解析。
- SQLite 是例外：`prefix.sqlite` 的值就是数据库文件名。

## 3. 门面（facade）

`db.Mysql`、`db.Mongo`、`db.Redis`、`db.Sqlite` 是包级单例客户端，封装连接、池、健康检查与重连。业务不直接 `gorm.Open`，而是通过门面拿连接。

```go
conn, err := db.Mysql.GetConnection()          // 单库
conn, err := db.Mysql.GetConnection("order")   // 多库必须指定库名
```

## 4. DAO：泛型数据访问对象

`db/dao` 提供 `MySQLDao[E]` 与 `MgoDao[E]`，把「表 → 结构体」的常用操作收敛为方法，并可选接入缓存与事务。

```go
type User struct {
	Id   int64  `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (User) TableName() string { return "user" }

var userDao = &dao.MySQLDao[User]{}
users, err := userDao.All(User{})
```

注意：`dao.Dao[E]` 接口只声明了 `Insert`，而 `MySQLDao` 的方法叫 `Create`，因此**只有 `MgoDao` 满足该接口**；业务直接用具体类型。

## 5. 中间件（middleware）

Gin 的 `gin.HandlerFunc`。框架默认装配 trace/postlog/cors/recovery；其余（jwt/casbin/xss/iplimit/limit/session/ratelimit）按需挂载。顺序即语义：casbin 依赖 jwt 写入的 `claims`，必须先 jwt 后 casbin。

## 6. 执行器（job handler）

`job` 模块把「调度配置」存在数据库，把「执行逻辑」注册在进程内：

```go
job.Register("syncUserJob", func(ctx *job.Context) error {
	ctx.Log("参数=%v", ctx.Param["bizDate"])
	return nil
})
```

数据库中的任务记录通过 `handler_name` 关联执行器。**任务**是库里的记录，**执行器**是代码里的函数，二者不要混用。

## 7. 探针（probe）

| 端点 | 语义 | 失败 |
|------|------|------|
| `/health/live`（别名 `/healthz`） | 进程存活 | 不失败 |
| `/health/ready` | 依赖实时可用（mysql/mongodb/redis） | 503 |
| `/health/startup` | 是否已 `MarkStarted()` | 503 |

探针在 `NewApp` 内**最先注册**，不会被业务鉴权/限流拦截。

## 8. 接口日志与运行日志

| 概念 | 结构 | 产生方 | 去向 |
|------|------|--------|------|
| 接口日志（PostLog） | `models.PostLog` | `middleware/postlog` | MongoDB（需配置 + 启用 mongodb）、`logsink` 自定义 handler |
| 运行日志（Record） | `logsink.Record` | `logs.*` | console、file、VictoriaLogs、自定义 `RecordHandler` |

两者都经过 `logsink` 的异步队列：每 handler 单协程 FIFO，队列满淘汰最旧。

## 9. 响应信封

| 结构 | 字段 | 使用场景 |
|------|------|----------|
| `models.Result[T]` | `status/msg/data/page` | 业务 HTTP 接口 |
| `vo.Response[T]` | `code/msg/srvTime/data` | 跨服务调用（`client.CallT`） |
| `models.ResultPage` | `count/index/size/total` | 分页 |

业务错误默认 HTTP 200，用 `status` 表达；协议层错误（401/403/429/503）才使用非 200。

## 10. 抽象之间的关系

```
App
├─ Router（Gin）─ 中间件链 ─ 业务路由
├─ 组件（used）─ 数据源门面 ─ DAO ─ 业务表
│                └ 注册中心 ─ client 调用
├─ i18n/errcode ─ 响应信封
├─ logs ─ logsink ─ 文件/VictoriaLogs/Mongo
├─ job ─ 数据库任务表 + 进程内执行器
└─ health/metrics ─ 探针与指标端点
```
