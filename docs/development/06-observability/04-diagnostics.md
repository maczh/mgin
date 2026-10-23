# 运行时诊断

框架没有内置 pprof、QoS 命令或线程栈采样端点。可用的自助诊断手段是：探针、指标、日志、健康检查、配置重载与管理接口。

## 1. 探针

```bash
curl -s localhost:8080/health/live     # 存活
curl -s localhost:8080/health/ready    # 依赖实时状态
curl -s localhost:8080/health/startup  # 是否 MarkStarted
```

`/health/ready` 返回：

```json
{"status":"ok"}
```

失败时 HTTP 503，响应为 `{"status":"fail","dependencies":{"mysql":"..."}}`，并可能触发一次重连。

## 2. 周期性自检

`NewApp` 启动后每 **5 分钟**执行 `checkAll`：

- 遍历 `used` 中的 MySQL/MongoDB/Redis（**不含 SQLite**）调用 `Check()`；
- 失败时打 `liveness check failed` 日志并触发重连；
- 自检结果没有导出查询接口，以日志为准。

`health.CheckDependencies()` 可手动触发一次并拿到 `(ok, map[string]string)`。

## 3. 指标与日志联动

| 现象 | 先看指标 | 再看 |
|------|----------|------|
| 请求变慢 | `mgin_http_request_duration_seconds` | 接口日志 `responseTime` |
| 5xx 上升 | `mgin_http_requests_total{status=~"5.."}` | `logs.Error` + 下游调用日志 |
| 依赖异常 | `mgin_dependency_up` | `/health/ready`、连接日志 |
| 请求堆积 | `mgin_http_requests_in_flight` | 下游超时、连接池 |
| 日志丢失 | `logsink.Stats()`（需业务接入） | 队列配置、VictoriaLogs 连通性 |

## 4. 配置与重载

```bash
curl -s -X PUT localhost:8080/cfg/reload -H "X-MGin-Reload-Token: $TOKEN"
```

用于：切换数据源、启用/停用组件、调整日志与限流配置。**不适用于**端口、TLS、路由、中间件变更。详见 [配置刷新](../04-configuration/03-refresh-and-diagnostics.md)。

## 5. 管理接口

| 接口 | 条件 | 用途 |
|------|------|------|
| `GET /health/*` | `EnableHealth`/`go.health.enabled` | 探针 |
| `GET /metrics` | `EnableMetrics`/`go.metrics.enabled` | 指标 |
| `PUT /cfg/reload` | `go.config.reload.token` 非空 | 重载 |
| `/job/*` | `go.job.enabled` | 任务列表、手动触发、执行日志 |
| `/i18n/*` | `go.i18n.api.enabled` | 字典维护 |

`/job` 与 `/i18n` 没有内置鉴权，生产需在网关或路由组上自行加保护。

## 6. 临时性能分析

需要 goroutine/CPU profile 时，在**开发或临时排障**环境自行挂载：

```go
import (
	"net/http"
	"net/http/pprof"
	"github.com/gin-gonic/gin"
)

func registerPprof(r *gin.Engine) {
	g := r.Group("/debug/pprof")
	g.GET("/", gin.WrapH(http.HandlerFunc(pprof.Index)))
	g.GET("/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
	g.GET("/heap", gin.WrapH(http.HandlerFunc(pprof.Handler("heap").ServeHTTP)))
}
```

```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

生产不要长期暴露；若必须，走内网 + 鉴权 + 短期开关。

## 7. 常见诊断脚本

```bash
# 版本与启动参数
curl -s localhost:8080/health/live | jq .data

# 依赖状态
curl -s -w '\n%{http_code}\n' localhost:8080/health/ready

# 指标快照
curl -s localhost:8080/metrics > /tmp/metrics.txt

# 错误日志按 requestId 聚合
awk -F'[][]' '{print $2}' logs/app-$(date +%F).log | sort | uniq -c | sort -rn | head

# 端口与进程
ss -lntp | grep :8080
```

## 8. 诊断边界

- 没有分布式追踪后端，跨服务只能靠 `X-Request-ID` 关联日志。
- 没有运行时路由列表/线程池状态查询接口；需要时从代码或日志确认。
- `job` 的调度状态以数据库为准，接口返回的是库中记录，不是内存中的实时状态。
