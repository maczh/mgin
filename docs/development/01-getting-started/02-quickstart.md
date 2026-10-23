# 快速上手：5 分钟跑通第一个服务

本章**只有一条路径**，不讨论选型。需要 MySQL/Redis/注册中心时再进入
[04 配置](../04-configuration/01-config-model.md) 与 [05 治理](../05-governance/01-discovery-and-lb.md)。

---

## 1. 环境准备

### 1.1 必需

```bash
go version        # 需要 go1.25.7 或更高
git --version
```

Go 版本低于 1.25.7 时，可让 Go 自动下载工具链：

```bash
go env -w GOTOOLCHAIN=auto
```

国内网络建议设置代理（脚手架探测版本也用它）：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

### 1.2 可选

| 场景 | 需要 |
|------|------|
| 使用 SQLite（`go.config.used` 含 `sqlite`） | GCC/Clang（CGO）+ `CGO_ENABLED=1` |
| 使用 MySQL/Redis/Mongo | 本地或远程实例；Docker 命令见 [04 配置](../04-configuration/02-config-guide.md) |
| 服务注册发现 | Nacos / Consul / etcd / Polaris 之一 |

**第一个服务不需要任何中间件。**

---

## 2. 安装脚手架

```bash
go install github.com/maczh/mgin/v2/cmd/mgin@latest
mgin version        # 输出 mgin scaffold 2.x.x
```

如果 `mgin` 不在 PATH，确认 `$(go env GOPATH)/bin` 已加入 PATH。
也可从源码构建：

```bash
git clone <repo> && cd mgin/cmd
go build -o "$(go env GOPATH)/bin/mgin" .
```

---

## 3. 生成并启动第一个服务

### 3.1 生成

```bash
mgin new hello \
  --module example.com/hello \
  --port 8080 \
  --health \
  --metrics \
  --output /tmp
```

生成内容（实测）：

```
/tmp/hello/
├── main.go                # 入口：NewApp + EnableHealth/EnableMetrics + Run
├── version.go             # 版本变量，Makefile 通过 -ldflags 注入
├── router/router.go       # 路由注册
├── controller/controller.go
├── service/service.go
├── dao/dao.go
├── model/model.go
├── conf/application.yml
├── conf/mysql-test.yml    # 模板默认总会生成（即使未选 --db）
├── Makefile
├── go.mod
└── README.md
```

> `dao/dao.go` 只在 `--db` 选中 `mysql`（默认）/`postgres`/`clickhouse` 时生成；
> 其中 `postgres`、`clickhouse` 两个取值**当前无法编译**（v2 未提供
> `dao.PostgresDao` / `dao.ClickhouseDao`，也没有对应连接层）。需要这两种数据库时，
> 请在 `dao/` 中自行实现数据访问。

> ⚠️ **模板的一个已知偏差**：未传 `--db` 时，生成的 `conf/application.yml`
> 仍然会写 `used: "mysql"`，并生成 `conf/mysql-test.yml`。
> 若本机没有 MySQL，启动会打印连接失败日志，`/health/ready` 也会返回 503。
> **快速上手请先把 `used` 清空**（见下一步）。

> ⚠️ 另一个模板偏差：不传 `--health` 时，`main.go` 仍会 `import .../pkg/health`
> 却不调用 `health.MarkStarted()`，`go build` 直接报
> `"github.com/maczh/mgin/v2/pkg/health" imported and not used`。
> 现有版本的规避方式是始终带上 `--health`（本页命令已带）。

### 3.2 改为零依赖并启动

```bash
cd /tmp/hello

# 关闭模板默认启用的 MySQL（本机无 MySQL 时必须）
sed -i 's/^    used: "mysql"/    used: ""/' conf/application.yml

make tidy
make build
./hello
```

预期日志（节选，逐字来自代码）：

```
读取配置文件:/tmp/hello/conf/application.yml
Prometheus 指标端点已挂载: /metrics
健康检查探针已挂载: /healthz /health/live /health/ready /health/startup
```

> 配置路径说明：`NewApp` 会把相对路径拼到**可执行文件所在目录**。
> `./hello` 在工程根目录运行，因此 `conf/application.yml` 能被正确找到。
> 用 `go run .` 时二进制在临时目录，需改用绝对路径，见
> [FAQ](../11-appendix/01-faq.md#q1-go-run-找不到配置)。

### 3.3 验证

```bash
curl -s localhost:8080/health/live      # {"code":1,"msg":"success","srvTime":"...","data":{"status":"UP",...}}
curl -s localhost:8080/health/ready     # {"status":"ok"}
curl -s localhost:8080/health/startup   # {"status":"ok"}
curl -s localhost:8080/metrics | head   # mgin_build_info...
```

三个探针的区别：

| 端点 | 检查内容 | 失败时 | K8s 用途 |
|------|----------|--------|----------|
| `/health/live` | 进程是否存活，不查依赖 | 200 | livenessProbe |
| `/health/ready` | 依赖（mysql/mongodb/redis）实时可用 | 503 | readinessProbe |
| `/health/startup` | 是否已 `MarkStarted`（`go.health.autoStarted` 可自动） | 503 | startupProbe |

### 3.4 加一个业务接口

模板已经生成 `/api/v1/products` 示例路由。打开 `router/router.go`，在 `v1` 组内加：

```go
v1.GET("/hello", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"msg": "hello, mgin"})
})
```

补上 `net/http` import，重新 `make build && ./hello`，验证：

```bash
curl -s localhost:8080/api/v1/hello
# {"msg":"hello, mgin"}
```

> **配置键提醒**：生成模板里的 `go.framework.health/metrics/shutdownTimeout/loadBalancer`
> 当前**不会被框架读取**。真正生效的是 `go.health.enabled`、`go.metrics.enabled`、
> `go.application.shutdownTimeout`。模板能工作是因为 `main.go` 显式调用了
> `EnableHealth()` / `EnableMetrics()`。详见
> [已知问题](../11-appendix/03-migration-and-issues.md#3-脚手架模板与框架配置键不一致)。

### 3.5 停止

`Ctrl+C`（SIGINT）或 `kill -TERM <pid>`。进程会：

1. 停止健康巡检；
2. 排空日志队列；
3. 逆序关闭插件（任务 → 注册中心 → Redis → DB）；
4. 关闭 HTTP/HTTPS server（默认 5 秒超时，`go.application.shutdownTimeout` 可调）。

---

## 4. 第一个调用方：服务间调用

### 4.1 最小闭环（直连，无需注册中心）

另建一个服务 `caller`：

```bash
mgin new caller --module example.com/caller --port 8081 --health --output /tmp
```

在 `router/router.go` 加（只用标准库，避免额外依赖）：

```go
import (
    "io"
    "net/http"
)

v1.GET("/call-hello", func(c *gin.Context) {
    resp, err := http.Get("http://127.0.0.1:8080/api/v1/hello")
    if err != nil {
        c.JSON(http.StatusBadGateway, gin.H{"msg": err.Error()})
        return
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    c.Data(resp.StatusCode, "application/json", body)
})
```

启动两个服务后：

```bash
curl -s localhost:8081/api/v1/call-hello
# {"msg":"hello, mgin"}
```

这验证了“两个进程 + HTTP 调用”的闭环，但地址是写死的。

### 4.2 框架调用（经注册中心）

生产用法是通过 `pkg/client` + 注册中心，由框架完成发现、负载均衡、超时与熔断：

```go
import (
    "time"

    "github.com/maczh/mgin/v2/pkg/client"
)

func CallHello(ctx context.Context) (string, error) {
    return client.CallResilientCtx(ctx, "hello", "/api/v1/hello", &client.Options{
        Method:   "GET",
        Protocol: client.CONTENT_TYPE_JSON,
    }, &client.ResilienceOptions{
        Timeout:    2 * time.Second,
        MaxRetries: 2,
    })
}
```

前置条件：

1. 两个服务都在 `go.config.used` 中启用同一注册中心（如 `nacos`）；
2. `conf/nacos-<env>.yml` 配置了地址与分组；
3. provider 已成功注册。

注册中心配置见 [05 · 服务发现](../05-governance/01-discovery-and-lb.md)；
调用参数、错误语义与协议见 [03 · 服务调用](../03-development/03-service-calls.md)。

---

## 5. 你刚刚获得了什么

| 能力 | 默认行为 |
|------|----------|
| 探针 | `--health` 生成即启用；也可用 `go.health.enabled` |
| 指标 | `--metrics` 生成 `EnableMetrics()`；否则仅配置不挂载 |
| 日志 | 控制台输出，带 trace id、耗时、状态码 |
| 错误响应 | 404 → HTTP 404 + 业务码 1000；panic → HTTP 500 + 业务码 1001 |
| 优雅停机 | SIGINT/SIGTERM/SIGHUP/SIGQUIT，默认 5s 超时 |
| 请求 ID | 自动生成/透传 `X-Request-ID`、`X-Trace-Id`、`traceparent` |

---

## 6. 下一步

- 想理解框架内部：[02 · 架构](../02-concepts/01-architecture.md)
- 想写完整业务：[03 · 开发指南](../03-development/02-service-development.md)
- 想接数据库/缓存：[04 · 配置](../04-configuration/02-config-guide.md)
- 想上线：[07 · 部署](../07-deployment/01-deployment.md)
