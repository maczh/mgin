# 指标（Metrics）

## 1. 启用

两种方式，等价：

```go
app.EnableMetrics()   // 代码
```

```yaml
go:
  metrics:
    enabled: true     # 配置（已验证可单独生效）
```

挂载后暴露 `GET /metrics`（Prometheus 文本格式）。探针与 `/metrics` 在默认中间件链之前注册，不受鉴权/限流影响。

**注意**：HTTP 请求指标需要额外挂载收集中间件：

```go
app.Router.Use(metrics.Middleware())
```

不挂载时，`/metrics` 只有构建/插件/依赖指标，没有 `mgin_http_*`。

## 2. 指标清单

| 指标名 | 类型 | 标签 | 含义 |
|--------|------|------|------|
| `mgin_http_requests_total` | Counter | `method`、`path`、`status` | 请求总数；未匹配路由的 `path` 为 `<nomatch>` |
| `mgin_http_request_duration_seconds` | Histogram | `method`、`path` | 时延分布（默认桶） |
| `mgin_http_requests_in_flight` | Gauge | — | 当前处理中的请求数 |
| `mgin_build_info` | Gauge | `version`、`commit`、`go_version` | 构建信息，值恒为 1 |
| `mgin_plugin_health` | Gauge | `name` | 1 健康 / 0 异常 |
| `mgin_dependency_up` | Gauge | `name` | 依赖可用性（1/0） |

设置方式：

```go
metrics.SetBuildInfo(Version, GitHash, runtime.Version())
metrics.SetPluginHealth("nacos", true)
metrics.SetDependencyUp("mysql", true)
```

## 3. 示例输出

```
# HELP mgin_http_requests_total Total number of HTTP requests.
# TYPE mgin_http_requests_total counter
mgin_http_requests_total{method="GET",path="/api/v1/users/:id",status="200"} 128
mgin_http_requests_total{method="GET",path="<nomatch>",status="404"} 3

# HELP mgin_http_request_duration_seconds HTTP request latency.
# TYPE mgin_http_request_duration_seconds histogram
mgin_http_request_duration_seconds_bucket{method="GET",path="/api/v1/users/:id",le="0.05"} 90
...
mgin_build_info{commit="abc123",go_version="go1.25.7",version="1.2.0"} 1
```

## 4. 告警与看板建议

| 指标 | 建议 |
|------|------|
| 5xx 比例 | `sum(rate(mgin_http_requests_total{status=~"5.."}[5m])) / sum(rate(...)) > 0.01` |
| P99 时延 | `histogram_quantile(0.99, sum(rate(mgin_http_request_duration_seconds_bucket[5m])) by (le))` |
| `<nomatch>` 增长 | 客户端路径错误或扫描；持续增长需排查 |
| in_flight 持续高 | 下游阻塞或连接池不足，结合 `db` 指标/日志判断 |
| `mgin_dependency_up == 0` | 依赖不可用，联动 `/health/ready` |
| `mgin_plugin_health == 0` | 插件异常，查启动日志 |

> 标签 `path` 使用 Gin 路由模板（如 `/users/:id`），不是原始 URL，基数可控。

## 5. 约束

1. 一个进程一套指标；多实例需在 Prometheus 侧按 `instance` 聚合。
2. 指标未做多租户/服务维度拆分，服务名需从 `job`/`target` 标签获取。
3. `in_flight` 是进程级，仅在 `Middleware()` 挂载后更新。
4. 不使用 OpenTelemetry SDK；接入 OTel 需在网关或业务侧转换。
