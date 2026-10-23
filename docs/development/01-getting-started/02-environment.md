# 开发环境搭建指南（jh 分支）

目标：从零把仓库编译起来、跑通最小服务、准备可选的本地中间件，并知道测试怎么跑。

---

## 1. 前置条件

| 项 | 要求 | 检查命令 |
|----|------|----------|
| Go | **1.25.7**（go.mod 声明；低版本会触发 toolchain 自动下载或报错） | `go version` |
| Git | 任意较新版本 | `git --version` |
| 网络/代理 | 能访问 Go module 代理 | `go env GOPROXY` |
| Docker（可选） | 本地 MySQL/Redis/MongoDB | `docker version` |
| CGO | **不需要**（SQLite 用纯 Go 驱动） | — |

国内网络建议设置代理：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
```

---

## 2. 获取代码与分支

```bash
git clone <repo-url> mgin
cd mgin
git checkout jh
git log --oneline -3
```

注意：`master` 分支的模块路径是 `github.com/maczh/mgin/v2`，API 与目录都不同；本文所有命令仅适用于 `jh`。

---

## 3. 编译

```bash
# 框架本体
go mod download
go build ./...            # 应无输出（exit 0）

# 脚手架（独立 module）
cd cmd
go build -o bin/mgin .
./bin/mgin version        # mgin scaffold x.y.z
cd ..
```

> 仓库没有 Makefile / CI 配置。若需要，可按上面的命令自行封装。

---

## 4. 跑通最小服务

### 4.1 手工创建

```bash
mkdir -p /tmp/mgin-demo/conf && cd /tmp/mgin-demo

cat > go.mod <<'EOF'
module example.com/demo

go 1.25

require github.com/maczh/mgin v1.25.27-jh
EOF

cat > main.go <<'EOF'
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin"
    "github.com/maczh/mgin/i18n"
)

func main() {
    app := mgin.NewApp("conf/application.yml", "demo", "1.0.0", false)
    app.EnableHealth()
    app.EnableMetrics()
    app.Router.GET("/ping", func(c *gin.Context) {
        c.JSON(200, i18n.Success(map[string]any{"pong": true}))
    })
    app.MarkHealthStarted()
    app.Run()
}
EOF

cat > conf/application.yml <<'EOF'
go:
  application:
    name: demo
    port: 8080
    project: demo
    debug: true
  logger:
    level: debug
    out: console
  config:
    used: ""
    env: test
EOF

go mod tidy
go build -o demo .
./demo
```

验证：

```bash
curl -s localhost:8080/ping
curl -s localhost:8080/health/live
curl -s localhost:8080/health/ready
curl -s localhost:8080/metrics | head
```

### 4.2 用脚手架生成

```bash
cd cmd && go build -o bin/mgin . && cd ..
./cmd/bin/mgin new demo --module example.com/demo --port 8080 --db mysql --output /tmp
cd /tmp/demo
go mod tidy && go build -o demo . && ./demo
```

- 生成工程默认启用 MySQL（`used: "mysql"`）。本机没有 MySQL 时，把 `conf/application.yml` 的 `used` 改为 `""` 再启动。
- `--db postgres` / `--db clickhouse` 生成的 DAO 类型不存在，编译失败（见 README §2.4）。

### 4.3 配置路径规则（最容易踩的坑）

`NewApp` 对不含 `/`、`\` 的文件名会拼到**可执行文件所在目录**：

| 运行方式 | 配置文件实际位置 |
|----------|------------------|
| `./demo`（二进制与 `conf/` 同目录） | `<工程目录>/conf/application.yml` ✔ |
| `go run .` | 临时构建目录下的 `conf/application.yml` ✘ |
| 显式绝对路径 | 传入什么用什么 ✔ |

`go run` 调试时请传绝对路径，或先 `go build` 再运行。

---

## 5. 本地中间件（可选）

```bash
# MySQL 8
docker run -d --name mgin-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=demo mysql:8.0

# Redis 7
docker run -d --name mgin-redis -p 6379:6379 redis:7-alpine

# MongoDB 6
docker run -d --name mgin-mongo -p 27017:27017 mongo:6
```

对应子配置（放在 `conf/` 下，文件名与 `prefix` + `env` 对应）：

```yaml
# conf/mysql-test.yml
go:
  data:
    mysql: root:root@tcp(127.0.0.1:3306)/demo?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true
```

```yaml
# conf/redis-test.yml
go:
  data:
    redis: { host: 127.0.0.1, port: 6379, password: "", database: 0 }
```

```yaml
# conf/mongodb-test.yml
go:
  data:
    mongodb:
      uri: mongodb://127.0.0.1:27017/demo
      db: demo
```

主配置里同步启用：

```yaml
go:
  config:
    used: "mysql,redis,mongodb"
    env: test
    prefix:
      mysql: mysql
      redis: redis
      mongodb: mongodb
```

> 多库、连接池、注册中心与配置中心的完整键见 [数据结构设计](../03-development/04-data-access.md#2-数据源与配置键) 与 README §5。

---

## 6. 测试

```bash
# 不需要外部服务的包
go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ \
        ./middleware/cors/ ./middleware/limit/ ./middleware/ratelimit/

# 需要本地 MySQL/SQLite/Mongo/Redis 的包
go test ./db/... ./i18n/ ./job/ ./config/

# 注册中心相关（需要网络或本地注册中心，耗时较长）
go test ./registry/... ./client/
```

- `go test ./...` 一次性执行可能因注册中心用例耗时较长而超时，建议按包运行。
- `go vet ./...` 目前仍有历史告警（`middleware/xss`、`utils/rsautil.go` 的 unreachable code；`i18n/store_test.go` 的 copylocks），不影响构建。
- 详细策略、现有用例清单与实测基线见 [测试策略](../10-contributing/03-testing-strategy.md)。

---

## 7. 代码格式与 IDE

```bash
gofmt -l $(git ls-files '*.go' | grep -v '^.Trash-0/')   # 期望无输出
goimports -w <file>                                       # 可选
```

- VS Code 安装 Go 扩展，开启 `Format On Save`（gofmt），并启用 `gopls`。
- 调试：使用 `dlv debug` 或 IDE 的 Go 调试配置；注意工作目录设为工程根目录，避免配置路径问题。
- 仓库中的 `.Trash-0/` 是被误提交的回收站副本，不要在其上开发，也不要参照其中的代码。

---

## 8. 常见环境问题

| 现象 | 原因 | 处理 |
|------|------|------|
| `go: module github.com/maczh/mgin/v2 ...` 找不到 | 混用了 `master` 的模块路径 | `jh` 分支用 `github.com/maczh/mgin` |
| `undefined: grequests.FromRequestOptions...` | 依赖版本与调用方式不匹配 | 使用 `&grequests.RequestOptions{...}`（仓库已修） |
| 启动日志显示读取了 `/tmp/go-build.../conf/application.yml` | 用了 `go run .` | 改绝对路径或先 build |
| `bind: address already in use` | 端口被占用 | 改 `go.application.port` 或释放端口 |
| 连接 MySQL 报认证插件错误 | MySQL 8 默认 `caching_sha2_password` | DSN 加 `allowNativePasswords=true` 或改用 `mysql_native_password` |
| SQLite 打不开 | 路径相对可执行目录、目录不存在 | 用绝对路径或先建目录 |
| i18n 初始化 panic | 应用名不符合 `^[A-Za-z0-9_.-]{1,59}$` 或数据库不可用 | 修正 `go.application.name` / 检查 used |
| `go test` 卡住 | 注册中心用例访问外部网络 | 按包运行或跳过 `registry/*` |
| Git 状态出现大量未跟踪文件 | 生成的工程/日志在仓库目录 | 使用 `/tmp` 输出，或配置 `.gitignore` |
