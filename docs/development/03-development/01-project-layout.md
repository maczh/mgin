# 项目结构

## 1. 脚手架生成的标准布局

```bash
./cmd/bin/mgin new demo --module example.com/demo --port 8080 --db mysql --output /tmp
```

生成结构（`--db mysql` 时）：

```
demo/
├── main.go              # 入口：NewApp → RegisterRoutes → Run
├── version.go           # Version/BuildTime/GitHash（-ldflags 注入）
├── router/router.go     # 路由与中间件注册
├── controller/controller.go
├── service/service.go
├── dao/dao.go           # 仅 mysql/postgres/clickhouse 时生成
├── model/model.go       # GORM 模型（实现 TableName）
├── conf/
│   ├── application.yml
│   └── mysql-test.yml
├── Makefile             # tidy/build/linux/run/test/lint/clean
├── go.mod
├── .gitignore
└── README.md
```

`--db` 为 `sqlite/mongodb/redis/elasticsearch`（或未启用 SQL 数据源）时，**不生成 `dao/`**，`service` 使用内存示例实现。

> ⚠️ `--db postgres` / `--db clickhouse` 会生成 `dao.PostgresDao` / `dao.ClickhouseDao`，这两个类型在本分支不存在，工程无法编译。需要这两种数据库时选 `mysql` 后自行改写数据访问层。

## 2. 目录职责

| 目录 | 放什么 | 不该放什么 |
|------|--------|-----------|
| `main.go` | 配置路径、版本、探针/指标开关、路由注册、`Run` | 业务逻辑 |
| `router/` | 路由分组、鉴权/限流中间件挂载 | SQL、事务 |
| `controller/` | 参数绑定/校验、调用 service、组装 `models.Result` | 直接访问数据库 |
| `service/` | 业务规则、事务边界、跨服务调用 | `gin.Context` 耦合 |
| `dao/` | 查询条件、缓存配置、数据访问 | 业务判断 |
| `model/` | 表结构映射、`TableName()` | 请求/响应 DTO |
| `conf/` | `application.yml` + `<prefix>-<env>.yml` | 真实密钥 |
| `job` 执行器 | 建议放 `job/` 或 `service/`，启动时注册 | 在 `init()` 中注册 |
| `migrations/` | SQL 迁移（框架不提供迁移器） | 运行时动态建表 |

推荐扩展：

```
internal/      # 业务私有，禁止外部 import
pkg/           # 可被其他服务复用的业务库
dto/           # 请求/响应 DTO（与 model 分离）
testdata/      # 测试夹具
scripts/       # 构建/发布脚本
```

## 3. module 与依赖

```go
module example.com/demo

go 1.25

require github.com/maczh/mgin v1.25.27-jh
```

- 本分支模块路径**不带** `/v2`；`github.com/maczh/mgin/v2` 属于 `master` 线，二者不能互换。
- 不要在提交的 `go.mod` 中保留指向本地路径的 `replace`（本地联调可临时使用，提交前删除）。
- 脚手架是独立 module（`cmd/go.mod`），在 `cmd/` 下构建。

### 多服务单仓库

```
repo/
├── go.work            # 本地开发用，不入发布产物
├── svc-order/go.mod
├── svc-user/go.mod
└── shared/go.mod
```

建议：每个服务独立 module、独立发布；公共库版本化；CI 使用 `GOWORK=off` 验证真实依赖。

## 4. 分层与依赖规则

```
router → controller → service → dao/db
                         └→ client（调用其他服务）
```

- 上层可以依赖下层，反之禁止。
- `service` 不接收 `*gin.Context`，需要请求信息时由 controller 提取后传入。
- `controller` 不直接 `db.Mysql.GetConnection()`。
- 跨服务调用统一走 `client`，不要在业务里手写 `http.Client`（会丢 trace 头与注册发现）。

## 5. 入口文件

```go
package main

import (
	"github.com/maczh/mgin"
	"example.com/demo/router"
)

func main() {
	app := mgin.NewApp("conf/application.yml", "demo", Version, false)

	app.EnableHealth()   // 探针（建议紧跟 NewApp）
	app.EnableMetrics()  // /metrics

	router.RegisterRoutes(app)

	app.MarkHealthStarted() // startup 探针转 200
	app.Run()
}
```

顺序约束：

1. `EnableHealth/EnableMetrics` 越早越好（它们挂载在默认链之前或紧随其后）；
2. `RegisterRoutes` 在启用探针之后；
3. `MarkHealthStarted` 在业务初始化完成后；
4. `Run()` 之后不要再注册路由（并发写 Gin 引擎不安全）。

## 6. 配置目录约定

```
conf/
├── application.yml     # 必需
├── mysql-test.yml      # prefix.mysql=mysql, env=test
├── redis-test.yml
├── mongodb-test.yml
├── nacos-test.yml
└── casbin.conf         # 模型文件（相对配置目录）
```

- 子配置文件名 = `<prefix>-<env>.yml`；
- 切换环境只改 `go.config.env`；
- SQLite 的 `prefix` 值是文件名，不是前缀；
- 生产环境建议把 `casbin.conf` 与配置一起纳入版本管理。

## 7. 构建与版本注入

脚手架生成的 Makefile 目标：`tidy`、`build`、`linux`、`build-multi-os`、`run`、`test`、`test-race`、`cover`、`lint`、`clean`。

```make
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitHash=${GIT_HASH}"
```

`version.go` 中的变量必须是包级 `string`，否则 `-X` 不生效。

## 8. 反例

| 反例 | 问题 |
|------|------|
| 在 `init()` 读配置/连库 | 配置未加载，无法测试 |
| controller 里 `gorm.Open` | 绕过池、重连与缓存 |
| 把 `*mgin.App` 传进 service | 业务与 Web 框架耦合 |
| 每个 handler 自己拼 `gin.H{"code":...}` | 响应协议漂移 |
| 把 `conf/*.yml` 的密码提交仓库 | 安全风险 |
| 删除 `version.go` | `-X` 失效，`/health/live` 版本为空 |
