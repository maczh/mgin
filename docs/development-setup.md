# MGin v2 开发环境搭建指南

目标：从零到"能编译、能跑测试、能启动一个最小服务"，预计 15 分钟。

---

## 1. 环境要求

| 组件 | 版本 | 必需 | 说明 |
|------|------|------|------|
| Go | **1.25.7+** | ✅ | `go.mod` 的 `go` 指令为 1.25.7 |
| Git | 2.x | ✅ | 拉取代码、查看提交 |
| GCC / Clang | 任意可用版本 | ⚠️ | 使用 SQLite（`go.config.used` 含 `sqlite`）时需要 CGO |
| Make | 3.81+ | ⚠️ | 生成的业务工程使用 Makefile；框架仓库本身无 Makefile |
| MySQL | 5.7 / 8.0 | 可选 | 使用 mysql / job / casbin / i18n |
| Redis | 5.0+ | 可选 | DAO 缓存、业务缓存 |
| MongoDB | 4.4+ | 可选 | postlog 落库、MgoDao |
| Nacos / Consul / etcd / Polaris | 见各后端 | 可选 | 注册发现与配置中心 |
| upx | 任意 | 可选 | 生成工程 `make linux` 压缩二进制 |

检查：

```bash
go version          # go version go1.25.7 linux/amd64
go env GOPATH GOPROXY CGO_ENABLED GOOS GOARCH
gcc --version       # 仅 SQLite 场景需要
```

---

## 2. 获取代码与首次构建

```bash
# 仓库约定放在 GOPATH 风格的路径下（非强制，但与 import 路径一致便于排查）
mkdir -p ~/Work/go/src/github.com/maczh
cd ~/Work/go/src/github.com/maczh
git clone <repo-url> mgin
cd mgin
git checkout v2

# 建议设置代理（国内环境）
go env -w GOPROXY=https://goproxy.cn,direct

go mod download
go build ./...
```

预期：`go build ./...` 无输出（成功）。框架根目录**没有** Makefile，构建直接用 `go` 命令。

> **基线告警**：`go vet ./...` 当前会报 3 处 `unreachable code`
> （`pkg/middleware/xss/xss.go:513`、`pkg/utils/rsautil.go:369`、`pkg/utils/rsautil.go:406`），
> `gofmt -l .` 会列出 14 个历史文件。这是既有状态，不影响构建；
> 新增代码只需保证自身 `gofmt` 干净且不新增 vet 告警。

> `cmd/` 是**独立 module**（`github.com/maczh/mgin/v2/cmd/mgin`），
> 根目录的 `go build ./...` **不会**编译它，需要单独进入 `cmd/`。

---

## 3. 运行测试

```bash
# 1) 核心包（无外部依赖）
go test ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
        ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... \
        ./pkg/logs/... ./pkg/cache/... -count=1

# 2) 全量（需要外部中间件，缺服务会失败）
go test ./... -count=1

# 3) 竞态
go test -race ./pkg/client/... ./pkg/loadbalancer/... -count=1

# 4) 覆盖率
go test ./pkg/health/... ./pkg/client/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

已知依赖外部服务的测试：

| 包 | 依赖/状态 |
|----|-----------|
| `pkg/config` | 远程配置中心；无服务时约 90s 超时失败 |
| `pkg/registry/consul` | Consul |
| `pkg/registry/etcd` | etcd |
| `pkg/registry/polaris` | Polaris |
| `pkg/i18n/mysql_test.go` | MySQL（缺失时跳过/失败） |
| `pkg/logsink` | **无需外部服务，但 `TestQueueKeepsNewestAndFIFO` 当前失败**：测试断言上限 100，代码已改为 1024 |
| `pkg/job/job_test.go` | 使用纯 Go SQLite（`glebarez/sqlite`），无需外部服务 |

---

## 4. 构建并使用脚手架

```bash
cd cmd
go build -o "$(go env GOPATH)/bin/mgin" .
# 确保 $(go env GOPATH)/bin 在 PATH 中
mgin version     # mgin scaffold 2.1.0
mgin help
```

生成最小工程：

```bash
mgin new demo --module github.com/me/demo --db sqlite --output /tmp
cd /tmp/demo
make tidy build
./demo            # Makefile 的 BINARY 为工程名，产物在工程根目录
```

`--db sqlite` 时不需要外部数据库；首次启动会在可执行文件目录生成 `demo.db`。

---

## 5. 手工搭建一个最小可运行服务

如果不想用脚手架，按下面 4 个文件即可跑通。

### 5.1 `go.mod`

```bash
mkdir -p /tmp/mgin-demo && cd /tmp/mgin-demo
go mod init github.com/me/mgin-demo
go get github.com/maczh/mgin/v2@latest
go mod tidy
```

### 5.2 `conf/application.yml`

```yaml
go:
  application:
    name: demo
    port: 8080
    debug: true
  config:
    used: ""
    env: dev
  logger:
    level: info
    out: console
  health:
    enabled: true
    autoStarted: true
```

### 5.3 `main.go`

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin/v2"
    "github.com/maczh/mgin/v2/pkg/metrics"
)

func main() {
    app := mgin.NewApp("conf/application.yml", "demo", "0.1.0", false)
    app.EnableMetrics()
    app.Router.Use(metrics.Middleware())

    app.Router.GET("/api/v1/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"msg": "pong"})
    })

    app.Run()
}
```

### 5.4 运行与验证

```bash
go run .                 # 或 go build -o demo . && ./demo
curl -s localhost:8080/health/live
curl -s localhost:8080/health/ready
curl -s localhost:8080/api/v1/ping
curl -s localhost:8080/metrics | head -20
```

> **路径坑**：`NewApp("conf/application.yml", ...)` 中的相对路径是相对**可执行文件目录**，
> 不是当前工作目录。`go run .` 时会在临时目录下找配置而失败。
> 本地调试建议传绝对路径：`mgin.NewApp("/tmp/mgin-demo/conf/application.yml", ...)`，
> 或直接 `go build` 后在项目根目录运行二进制并将 conf 放在二进制同级。

---

## 6. 本地中间件

### 6.1 Docker 快速启动

```bash
# MySQL
docker run -d --name mgin-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=demo mysql:8.0

# Redis
docker run -d --name mgin-redis -p 6379:6379 redis:7-alpine

# MongoDB
docker run -d --name mgin-mongo -p 27017:27017 mongo:6

# etcd
docker run -d --name mgin-etcd -p 2379:2379 -p 2380:2380 \
  quay.io/coreos/etcd:v3.5.4 \
  /usr/local/bin/etcd --advertise-client-urls http://0.0.0.0:2379 \
  --listen-client-urls http://0.0.0.0:2379

# Consul
docker run -d --name mgin-consul -p 8500:8500 consul:1.15 agent -dev -client=0.0.0.0

# Nacos（单机）
docker run -d --name mgin-nacos -p 8848:8848 -p 9848:9848 \
  -e MODE=standalone nacos/nacos-server:v2.2.0
```

### 6.2 组件最小配置

`conf/mysql-test.yml`：

```yaml
go:
  data:
    mysql:
      multidb: false
      mysql: "root:root@tcp(127.0.0.1:3306)/demo?charset=utf8mb4&parseTime=True&loc=Local"
    mysql_debug: true
    mysql_pool:
      max: 50
      total: 20
      timeout: 30
      life: 3600
```

`conf/redis-test.yml`：

```yaml
go:
  data:
    redis:
      uri: "redis://127.0.0.1:6379/0"
    redis_pool:
      max: 50
      min: 5
      idleTimeout: 300
      timeout: 5
```

`conf/mongodb-test.yml`：

```yaml
go:
  data:
    mongodb:
      multidb: false
      uri: "mongodb://127.0.0.1:27017"
      db: "demo"
```

主配置对应片段：

```yaml
go:
  config:
    used: "mysql,redis,mongodb"
    env: test
    prefix:
      mysql: mysql
      redis: redis
      mongodb: mongodb
  discovery:
    registry: ""
    callType: json
```

---

## 7. IDE 配置

### 7.1 GoLand / IntelliJ IDEA

- GOROOT 指向 1.25.7；GOPATH 与 `go env GOPATH` 一致。
- 打开仓库根目录即可；`cmd/` 作为第二个 module 打开（File → Open → 选择 `cmd/go.mod`）。
- Run Configuration：
  - Program: `go run .`（工作目录设为项目根，或改用绝对配置路径）
  - Environment: `GOPROXY=https://goproxy.cn,direct`

### 7.2 VS Code

安装 Go 扩展，`settings.json` 建议：

```json
{
  "go.toolsManagement.autoUpdate": true,
  "go.formatTool": "gofmt",
  "go.useLanguageServer": true,
  "gopls": { "build.directoryFilters": ["-cmd"] }
}
```

`cmd/` 是独立 module，gopls 需要单独识别；可在 `cmd/` 目录下单独打开窗口。

### 7.3 调试

```bash
# Delve
dlv debug . -- -f /tmp/mgin-demo/conf/application.yml
```

`-v` 会打印版本后立即退出，调试时不要加。

---

## 8. 常用环境变量

| 变量 | 建议值 | 作用 |
|------|--------|------|
| `GOPROXY` | `https://goproxy.cn,direct` | 模块代理（脚手架探测版本也用它） |
| `GOFLAGS` | `-mod=mod` | 避免 vendor 干扰 |
| `CGO_ENABLED` | `1` | 使用 SQLite / 需要 cgo 的驱动 |
| `CGO_ENABLED` | `0` | 纯 Go 交叉编译（此时不要启用 `sqlite` 组件） |

---

## 9. 仓库结构与构建边界

| 路径 | 是否独立 module | 构建命令 |
|------|-----------------|----------|
| 根（`.`, `pkg/`, `logs/`） | 否 | `go build ./...` |
| `cmd/` | **是** | `cd cmd && go build ./...` |

没有 `go.work`，也不建议添加：`cmd/` 依赖已发布版本，不应隐式指向本地框架。

---

## 10. 常见环境问题

| 现象 | 原因 | 解决 |
|------|------|------|
| `go: go.mod requires go >= 1.25.7` | Go 版本过低 | 升级 Go；或用 `GOTOOLCHAIN=auto` 让 Go 自动下载 |
| `dial tcp ... i/o timeout` 拉依赖 | 网络 | 设置 `GOPROXY=https://goproxy.cn,direct` |
| `gcc: command not found` / `cgo: C compiler not found` | 使用了 SQLite 或 CGO | 安装 build-essential；或改用 MySQL/Mongo 并移除 `sqlite` |
| `undefined: sqlite3` 类错误 | `CGO_ENABLED=0` 但启用 SQLite | 设 `CGO_ENABLED=1` 并确保 gcc 可用 |
| 启动报 `读取配置文件错误: open ... no such file` | `-f` 缺省为可执行文件同名 yml | 显式 `-f` 绝对路径，或把 conf 放到二进制同级 |
| 端口被占用 | 已有进程 | `lsof -i :8080`；改 `go.application.port` |
| `/health/live` 404 | 未启用探针 | `go.health.enabled: true` 或 `app.EnableHealth()` |
| `/metrics` 404 | 仅配了 `go.metrics.enabled` | 调用 `app.EnableMetrics()` |
| 注册中心连接失败 | 地址/端口/分组不对 | 检查 `conf/<registry>-<env>.yml` 与 `go.config.env` |
| 生成的工程 `go build` 失败（`--db postgres`/`clickhouse`） | 框架无对应 DAO | 改用 `mysql`/`sqlite`/`mongodb`/`memory` |
| `make linux` 失败 | 缺少 upx | 安装 upx 或改用 `make build` |

---

## 11. 提交前自检脚本

```bash
set -e

# 1) 只检查本次改动的 Go 文件
changed=$(git diff --name-only -- '*.go' | tr '\n' ' ')
if [ -n "$changed" ]; then
  test -z "$(gofmt -l $changed)"
fi

# 2) 构建与核心测试
go build ./...
go test ./pkg/health/... ./pkg/client/... ./pkg/loadbalancer/... \
        ./pkg/metrics/... ./pkg/otel/... ./pkg/errcode/... -count=1

# 3) 改动包静态检查（全仓库 vet 有 3 处历史告警，不作为门禁）
for pkg in $(git diff --name-only -- '*.go' | xargs -r -n1 dirname | sort -u); do
  go vet "./$pkg/..."
done

# 4) 脚手架模块
(cd cmd && go build ./... && go vet ./...)
echo "OK"
```

建议后续将此脚本纳入 CI（当前仓库无 CI 配置）。
