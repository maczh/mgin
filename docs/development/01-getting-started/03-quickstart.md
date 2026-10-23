# 快速上手：5 分钟跑通第一个服务

本文只走一条路：**不连任何中间件**，启动一个带探针与指标的服务，再演示服务间调用需要补什么。

前置：Go 1.25.7、可访问 Go module 代理。详细环境说明见 [环境准备](02-environment.md)。

## 1. 创建工程

```bash
mkdir -p /tmp/mgin-hello/conf && cd /tmp/mgin-hello

cat > go.mod <<'EOF'
module example.com/hello

go 1.25

require github.com/maczh/mgin v1.25.27-jh
EOF
```

> 版本号可用 `go list -m -versions github.com/maczh/mgin` 查看；本分支模块路径**不带** `/v2`。

## 2. 入口文件

```go
// main.go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin"
	"github.com/maczh/mgin/i18n"
)

func main() {
	// 参数：配置文件、应用名、版本、是否启用 i18n
	app := mgin.NewApp("conf/application.yml", "hello", "1.0.0", false)

	app.EnableHealth()  // /healthz、/health/live、/health/ready、/health/startup
	app.EnableMetrics() // /metrics

	app.Router.GET("/ping", func(c *gin.Context) {
		// 统一返回结构：{"status":1,"msg":"success","data":...}
		c.JSON(200, i18n.Success(gin.H{"pong": true}))
	})

	app.MarkHealthStarted() // 让 /health/startup 返回 200
	app.Run()
}
```

## 3. 最小配置

```yaml
# conf/application.yml
go:
  application:
    name: hello
    port: 8080
    project: hello
    debug: true
  logger:
    level: debug
    out: console
  config:
    used: ""      # 不启用任何外部组件
    env: test
```

`used: ""` 表示不初始化 MySQL/Redis/Mongo 等组件，因此本机无需安装任何中间件。

## 4. 构建与启动

```bash
go mod tidy
go build -o hello .
./hello
```

启动后终端会打印读取的配置路径与启动表格。**必须用编译后的二进制运行**：`NewApp` 对不含路径分隔符的配置文件名会拼到可执行文件目录，`go run .` 会去临时目录找配置。

## 5. 验证

```bash
curl -s localhost:8080/ping
# {"status":1,"msg":"success","data":{"pong":true}}

curl -s localhost:8080/health/live
# {"code":1,"msg":"success","srvTime":"...","data":{"status":"UP","version":"1.0.0","uptime":3}}

curl -s localhost:8080/health/ready
# {"status":"ok"}

curl -s localhost:8080/health/startup
# {"status":"ok"}

curl -s localhost:8080/metrics | head
# mgin_build_info{...} 1
# mgin_http_requests_total{...}
```

停止：`Ctrl+C`（或 `kill -TERM <pid>`）。日志应出现 `Shutdown Server ...` 与 `Server exiting`。

## 6. 第一个调用方（可选）

服务调用依赖注册中心。先在 `conf/application.yml` 中启用 Nacos（或 Consul/etcd）：

```yaml
go:
  config:
    used: "nacos"
    prefix:
      nacos: nacos
  discovery:
    registry: nacos
    callType: json
```

`conf/nacos-test.yml`：

```yaml
go:
  nacos:
    server: 127.0.0.1
    port: 8848
    group: DEFAULT_GROUP
```

调用方代码：

```go
type User struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

resp := client.CallT[User]("hello", "/api/v1/users/{id}", &client.Options{
	Method:   http.MethodGet,
	Protocol: "restful",
	Path:     map[string]string{"id": "1"},
})
if resp.Code != vo.SUCCESS {
	logs.Error("调用失败: {}", resp.Msg)
	return
}
logs.Info("用户: {}", resp.Data.Name)
```

要求：被调服务（`go.application.name: hello`）已注册到同一个 Nacos；调用方进程已初始化注册中心。`CallT` 解析 `{code,msg,data,srvTime}`，旧协议用 `CallResultT`（`{status,msg,data}`）。

## 7. 常见问题（快速版）

| 现象 | 原因 | 处理 |
|------|------|------|
| 启动报配置读取失败 | 用 `go run .`，配置被拼到临时目录 | 先 `go build`；或给 `NewApp` 传绝对路径 |
| `/health/ready` 503 | `used` 里启用了不可用的 MySQL/Mongo/Redis | 清空 `used` 或先启动对应中间件 |
| `/metrics` 404 | 只配置了 `go.metrics.enabled` 但未调用 `EnableMetrics` | 两者任一即可；代码里调用最稳 |
| 调用报「服务注册中心未初始化」 | 未配置 `used`/`discovery.registry` | 补配置并重启 |
| 限流返回 200 | 默认用 HTTP 200 + `status=1011` | 在规则中配置 `httpStatus: 429` |

完整排障见 [FAQ](../11-appendix/01-faq.md) 与 [Runbook](../07-deployment/02-runbook.md)。

## 8. 接下来

- 需要数据库/缓存：[数据访问](../03-development/04-data-access.md)
- 需要 JWT/Casbin：[安全治理](../05-governance/03-security.md)
- 需要指标与日志接入：[可观测性](../06-observability/)
- 需要查函数签名：[API 参考](../08-reference/api/README.md)
