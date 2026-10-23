# 项目结构

## 1. 脚手架生成的标准布局

`mgin new` 生成的结构（实测）：

```
hello/
├── main.go                 # 唯一入口：NewApp → 路由 → Run
├── version.go              # Version/BuildTime/GitHash，Makefile 注入
├── router/router.go        # 路由与业务中间件注册
├── controller/controller.go
├── service/service.go
├── dao/dao.go              # 仅 --db 选中 mysql/postgres/clickhouse 时生成
├── model/model.go          # 表模型（实现 TableName）
├── conf/
│   ├── application.yml     # 主配置
│   └── mysql-test.yml      # 数据源子配置（模板默认生成）
├── Makefile                # tidy/build/linux/run/test/lint/clean
├── go.mod
└── README.md
```

### 1.1 各目录职责

| 目录 | 放什么 | 不该放什么 |
|------|--------|-----------|
| `main.go` | 装配：配置路径、版本、中间件、路由、后台任务 | 业务逻辑 |
| `router/` | 路由分组、鉴权中间件挂载 | SQL、事务 |
| `controller/` | 参数绑定/校验、调用 service、组装响应 | 直接访问 DB |
| `service/` | 业务规则、事务边界、领域校验 | HTTP 上下文耦合 |
| `dao/` | 数据访问、查询条件、缓存配置 | 业务判断 |
| `model/` | 表结构映射、`TableName()` | 请求 DTO |
| `conf/` | 配置；命名 `<组件>-<env>.yml` | 密钥明文入库 |

> `pkg/plugin`、`pkg/client` 等**不要**复制到业务工程；直接 import 框架包。

### 1.2 建议的扩展目录（按需）

```
internal/          # 业务私有实现，禁止外部 import
pkg/               # 可被其他服务复用的业务库
dto/               # 请求/响应 DTO（与 model 分离）
migrations/        # SQL 迁移脚本（框架不提供迁移器）
testdata/          # 测试夹具
scripts/           # 构建/发布脚本
```

### 1.3 数据层支持状态（实测）

`--db` 决定是否生成 `dao/` 以及 `service/service.go` 用哪套实现：

| `--db` 取值 | 生成的 DAO | `dao/` | 实测编译 | 结论 |
|-------------|-----------|--------|---------|------|
| `mysql`（默认） | `dao.MySQLDao[model.Product]` | ✅ | ✅ | 可用 |
| `postgres` | `dao.PostgresDao[model.Product]` | ✅ | ❌ `undefined: dao.PostgresDao` | **不可用** |
| `clickhouse` | `dao.ClickhouseDao[model.Product]` | ✅ | ❌ `undefined: dao.ClickhouseDao` | **不可用** |
| `sqlite` / `mongodb` / `redis` / `elasticsearch` | 不生成 | ❌ | ✅ | 仅生成子配置，业务层为内存示例 |

原因：v2 的 `pkg/db` 只有 `mysql`、`mongo`、`redis`、`sqlite` 四个实现，
`pkg/db/dao` 只提供 `MySQLDao` 与 `MgoDao`；`PostgresDao`/`ClickhouseDao`
仅见于 README/文档与 `jh` 分支，v2 主干没有这些类型，也没有对应的连接层。

选择 `--db postgres`/`--db clickhouse` 时，脚手架仍会写入
`go.config.used: "postgres|clickhouse"` 与对应子配置，但框架不建连、不读子配置，
生成的 `dao/dao.go` 也无法编译。

补充约定：

- 数据层按 `mysql > postgres > clickhouse` 优先级**单选**（`cmd/scaffold.go`），同时传多个只取第一个；
- SQLite 与 MongoDB 不会切换数据层，需要 `MgoDao` 时在 `dao/` 中自行实例化；
- 未传 `--db` 时默认 `mysql`，因此 `dao/` 一定存在，即使随后把 `go.config.used` 清空。

---

## 2. module 与依赖管理

### 2.1 单服务单 module

```go
module example.com/hello

go 1.25.7

require github.com/maczh/mgin/v2 v2.1.0
```

- 版本号必须是 `v2.x.y`（module 路径含 `/v2`）。
- 不要用 `replace` 指向本地路径提交到仓库；本地联调可临时使用。
- `go.mod` 的 `go` 指令与框架保持一致，避免工具链行为差异。

### 2.2 多服务单仓库（monorepo）

```
repo/
├── go.work            # 本地开发用，不入发布产物
├── svc-order/go.mod
├── svc-user/go.mod
└── shared/go.mod      # 团队公共库
```

建议：

- 每个服务独立 module，独立发布；
- 公共库版本化，禁止服务之间 `replace` 互指；
- `go.work` 只用于本地同时修改，CI 用 `GOWORK=off` 验证真实依赖。

### 2.3 依赖原则

| 原则 | 说明 |
|------|------|
| 业务不直接依赖具体驱动 | 用 `db.Mysql` 门面 / DAO，不 import `gorm.io/driver/mysql` |
| 不引入第二个 Web 框架 | 路由统一走 `gin.Engine` |
| 日志统一走 `pkg/logs` | 便于级别控制与多目标输出 |
| 错误统一走 `errcode` + `i18n` | 避免 `c.JSON(500, gin.H{"error": ...})` 满天飞 |

---

## 3. 入口文件的正确写法

```go
package main

import (
    "github.com/maczh/mgin/v2"
    "github.com/maczh/mgin/v2/pkg/health"
    "example.com/hello/router"
)

func main() {
    app := mgin.NewApp("conf/application.yml", "hello", Version, false)
    if app == nil {           // 配置缺失时 NewApp 仍返回实例；保留判空以兼容未来
        return
    }

    app.EnableHealth()        // 探针（建议紧跟 NewApp）
    app.EnableMetrics()       // /metrics

    router.RegisterRoutes(app)

    health.MarkStarted()      // startup 探针转 200
    app.Run()
}
```

顺序约束（违反会出问题）：

1. `EnableHealth/EnableMetrics` 必须在业务 `Use` 之前；
2. `RegisterRoutes` 在中间件之后；
3. `MarkStarted` 在业务初始化完成后、`Run` 之前或之后均可；
4. `Run` 之后不要再去注册路由（并发写 `Engine` 不安全）。

---

## 4. 配置目录约定

```
conf/
├── application.yml       # 必需，主配置
├── mysql-test.yml        # go.config.prefix.mysql -> mysql, env=test
├── redis-test.yml
├── mongodb-test.yml
├── nacos-test.yml
├── etcd-test.yml
└── casbin.conf           # 相对路径以 application.yml 所在目录为基准
```

规则：

- 子配置文件名 = `{prefix}-{go.config.env}.yml`；
- `go.config.prefix.<name>` 指定前缀；sqlite 的 prefix 值是**文件路径**；
- 环境切换只改 `go.config.env`，不要复制多份 `application.yml`。

---

## 5. Makefile 与版本注入

脚手架生成的 Makefile 目标：

| 目标 | 作用 |
|------|------|
| `tidy` | `go mod tidy` |
| `build` | 先 tidy，再编译到工程根目录（`BINARY` = 工程名） |
| `linux` | 交叉编译 Linux 二进制（可选 upx 压缩） |
| `build-multi-os` | 多平台产物 |
| `run` | 本地运行 |
| `test` / `test-race` / `cover` | 测试与覆盖率 |
| `lint` | `go vet` + 可选 golangci-lint |
| `clean` | 清理产物 |

版本注入：

```make
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitHash=${GIT_HASH}"
```

`version.go` 中的变量必须是**包级 string 变量**，否则 `-X` 不生效。

---

## 6. 反例

| 反例 | 问题 |
|------|------|
| 在 `init()` 里读配置/连数据库 | 配置尚未加载；无法测试 |
| controller 里直接 `gorm.Open` | 连接泄漏、绕过 DAO 缓存与事务 |
| 把 `mgin.App` 传到 service 层 | 业务与 Web 框架耦合，无法复用 |
| 每个 handler 自己拼 `gin.H{"code":...}` | 响应协议漂移，客户端难适配 |
| 把 `conf/*.yml` 的密码提交到仓库 | 安全风险；用配置中心或环境注入 |
| 生成工程后删除 `version.go` | Makefile 的 `-X` 失效，`/health` 版本为空 |
