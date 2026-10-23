# 指标（Metrics）

## 1. 启用（两步，缺一不可）

```yaml
go:
  metrics:
    enabled: true
```

```go
app := mgin.NewApp(...)
app.EnableMetrics()                    // ① 挂载 /metrics
app.Router.Use(metrics.Middleware())   // ② 采集 HTTP 指标
metrics.SetBuildInfo(Version, GitHash, runtime.Version()) // ③ 可选
```

只做 ①：`/metrics` 可访问，但 HTTP 指标全为 0（除手动 Set 的）。
只做 ②：有数据，但没有暴露端点。
配置 `enabled` 单独不够——`EnableMetrics()` 才是挂载动作。

验证：

```bash
curl -s localhost:8080/metrics | grep '^mgin_'
```

---

## 2. 内置指标

| 指标 | 类型 | 标签 | 谁写入 | 说明 |
|------|------|------|--------|------|
| `mgin_http_requests_total` | CounterVec | `method,path,status` | `Middleware()` | 请求总数 |
| `mgin_http_request_duration_seconds` | HistogramVec | `method,path` | `Middleware()` | 时延，默认 buckets（5ms…10s） |
| `mgin_http_requests_in_flight` | Gauge | — | `Middleware()` | 当前并发 |
| `mgin_build_info` | GaugeVec | `version,commit,go_version` | `SetBuildInfo` | 恒为 1 |
| `mgin_plugin_health` | GaugeVec | `name` | `SetPluginHealth` | 1 健康 / 0 异常 |
| `mgin_dependency_up` | GaugeVec | `name` | `SetDependencyUp` | 1 可用 / 0 不可用 |

> 后三个**不会自动更新**。框架只在启动时提供 API，需业务在合适时机调用
>（如每 30s 巡检后写入）。

### 2.1 标签基数

`path` 使用 `c.FullPath()`（路由模板 `/api/v1/orders/:id`），未匹配路由为 `<nomatch>`。
不要自行用 `c.Request.URL.Path` 打标签，否则每个 ID 都会成为新时间序列。

`status` 是最终状态码，`c.Writer.Status()` 默认 200，panic 恢复为 500。

---

## 3. 手动写入

```go
metrics.SetBuildInfo("1.4.2", "a1b2c3d", runtime.Version())

for name, err := range plugin.HealthAll() {
    metrics.SetPluginHealth(name, err == nil)
}

conn, _ := db.Mysql.GetConnection()
metrics.SetDependencyUp("mysql", conn != nil)
```

典型位置：启动后一次 + 定时巡检（可复用 `job` 或自建 ticker）。

---

## 4. Prometheus 抓取

```yaml
scrape_configs:
  - job_name: mgin-service
    metrics_path: /metrics
    static_configs:
      - targets: ["order:8080"]
```

K8s 注解：

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

`/metrics` 在鉴权中间件之前挂载，不需要 token。

---

## 5. 常用查询

```promql
# QPS
sum(rate(mgin_http_requests_total[1m])) by (path)

# P99 时延
histogram_quantile(0.99,
  sum(rate(mgin_http_request_duration_seconds_bucket[5m])) by (le, path))

# 5xx 比例
sum(rate(mgin_http_requests_total{status=~"5.."}[5m]))
  / sum(rate(mgin_http_requests_total[5m]))

# 并发
mgin_http_requests_in_flight

# 依赖不可用
mgin_dependency_up == 0
```

---

## 6. 告警建议

| 告警 | 表达式（示例） | 级别 |
|------|----------------|------|
| 5xx 比例 > 5% | 见上，持续 5m | P1 |
| P99 > 2s | 按 path 聚合，持续 10m | P2 |
| QPS 突降 50% | 与 1h 前对比 | P2 |
| 依赖 down | `mgin_dependency_up == 0` | P1 |
| 并发逼近上限 | `mgin_http_requests_in_flight > 0.8*上限` | P2 |

阈值必须按业务基线调整，不要照搬。

---

## 7. 常见问题

| 现象 | 原因 | 处理 |
|------|------|------|
| `/metrics` 404 | 只配了 `go.metrics.enabled` | 调用 `EnableMetrics()` |
| 只有 go_* 指标，没有 mgin_* | 未挂 `metrics.Middleware()` | `app.Router.Use(...)` |
| `mgin_build_info` 为空 | 未调用 `SetBuildInfo` | 启动时调用 |
| path 标签爆炸 | 用了原始 URL | 用 `Middleware()`，不要自研 |
| 指标重复注册 panic | 同一进程 import 两份 metrics | 检查依赖树 |

---

## 8. 与日志、追踪的分工

| 问题 | 用指标 | 用日志 | 用追踪 |
|------|--------|--------|--------|
| 整体 QPS/错误率 | ✅ | | |
| 哪个接口慢 | ✅（聚合） | | ✅（单次） |
| 某次请求为什么失败 | | ✅（request id） | ✅（trace） |
| 下游依赖是否健康 | ✅ | ✅ | |

三者通过 `X-Request-ID`/`traceparent` 关联，见 [追踪](02-tracing.md) 与 [日志](03-logging.md)。
