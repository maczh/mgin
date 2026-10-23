# 链路追踪（Tracing）

MGin 的追踪分两层：

1. **标识透传**（默认开启）：`traceparent`、`X-Trace-Id`、`X-Request-ID`；
2. **Span 采集**（可选）：通过 `pkg/otel` 接入 OpenTelemetry SDK。

---

## 1. 默认行为：标识透传

`trace.TraceId()` 已在 `baseRouter` 中挂载，无需业务配置。

处理逻辑：

```
入站有 traceparent？
  ├─ 是 → 原样保留；若缺 X-Trace-Id，从 traceparent 解析
  └─ 否 → 生成 traceparent = 00-<32hex>-<16hex>-01，并写 X-Trace-Id
X-Request-ID 为空 → 生成 16 位 hex
X-Real-IP：优先 X-Forwarded-For，其次 X-Real-IP，最后 ClientIP
X-User-Agent：取 User-Agent
```

这些值存进**按 goroutine id 索引**的进程内缓存（TTL 5 分钟），供同协程内读取。

### 1.1 读取

```go
import "github.com/maczh/mgin/v2/pkg/middleware/trace"

requestID := trace.GetRequestId()
traceID   := trace.GetHeader("X-Trace-Id")
clientIP  := trace.GetClientIp()
headers   := trace.GetHeaders()   // 副本，可安全修改
```

### 1.2 跨协程传递

```go
go func(parent uint64) {
    trace.CopyPreHeaderToCurRoutine(parent)
    logs.Info("async task, request id={}", trace.GetRequestId())
}(trace.GetGoroutineID())
```

不调用 `CopyPreHeaderToCurRoutine`，新协程读不到请求头，日志会缺 request id。

### 1.3 限制

| 限制 | 影响 |
|------|------|
| 不写响应头 | 客户端拿不到 `X-Request-ID`，需要业务自行 `c.Header(...)` |
| 基于 goroutine id | goroutine 复用/销毁依赖 TTL；异步任务必须显式复制 |
| 进程内缓存 | 多副本不共享；跨服务靠 header 透传 |
| 无采样控制 | 每个请求都生成标识（开销极小） |

> 若需要客户端可见的 request id，在业务中间件里加：
> `c.Header("X-Request-ID", trace.GetRequestId())`。

---

## 2. 出站自动透传

`client.Call*` 会调用 `trace.GetHeaders()` 并注入到下游请求，包括
`traceparent`、`X-Trace-Id`、`X-Request-ID`、`X-Real-IP`。
业务不需要手工拷贝。

在非 HTTP 场景（job/MQ）调用时没有入站头，框架会生成新的标识；
如需与任务关联，显式放入 `Options.Header`：

```go
client.CallCtx(ctx, "order", "/api/v1/close", &client.Options{
    Header: map[string]string{"X-Request-ID": ctx.Value("requestId").(string)},
})
```

---

## 3. 接入 OpenTelemetry

### 3.1 设计

- 框架只依赖 `go.opentelemetry.io/otel` 与 `otel/trace`；
- 不引入 SDK/Exporter；
- 业务调用 `otel.SetTracerProvider(tp)` 后，`client` 自动创建 `mgin.client.call` span；
- 未设置时 `Tracer()` 为 noop，开销接近零。

### 3.2 最小接入

```bash
go get go.opentelemetry.io/otel/sdk
go get go.opentelemetry.io/otel/exporters/stdout/stdouttrace   # 示例
```

```go
import (
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

    mginotel "github.com/maczh/mgin/v2/pkg/otel"
)

func initTracer() (*sdktrace.TracerProvider, error) {
    exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
    if err != nil { return nil, err }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("order"),
        )),
    )
    mginotel.SetTracerProvider(tp)
    return tp, nil
}

func main() {
    tp, err := initTracer()
    if err != nil { panic(err) }
    defer tp.Shutdown(context.Background())
    // ... NewApp / Run
}
```

生产把 exporter 换成 OTLP（`otlptracehttp`），指向 collector。

### 3.3 手动埋点

```go
ctx, span := otel.StartSpan(ctx, "OrderService.Create",
    trace.WithAttributes(attribute.Int64("userId", req.UserId)))
defer span.End()

if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

### 3.4 已自动产生的 span

| 位置 | span 名 | 属性 |
|------|---------|------|
| `client.callInternal` | `mgin.client.call` | `rpc.service`、`http.method`、`rpc.uri` |

HTTP 入站没有自动 span（没有 otelgin 中间件）。需要时在业务中间件里创建：

```go
func OtelMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, span := otel.StartSpan(c.Request.Context(), c.FullPath())
        defer span.End()
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

---

## 4. 与日志关联

`pkg/logs` 每次输出都会读取 `trace.GetRequestId()` 并注入日志记录：

```json
{"time":"2026-10-23T21:00:00+08:00","level":"info","message":"...","requestId":"a1b2...","goroutineId":123}
```

因此：

- 同一次请求的所有日志可用 `requestId` 检索；
- 下游服务的日志带同一个 `X-Request-ID`（client 自动透传）；
- 与 OTel trace 关联需在日志里加 `trace_id`（当前未内置，业务可自行加字段）。

---

## 5. 常见问题

| 现象 | 原因 | 处理 |
|------|------|------|
| 下游日志 request id 不同 | 用了裸 `net/http` 而非 `client`，或跨协程未复制 | 用 `client`；异步任务 `CopyPreHeaderToCurRoutine` |
| 客户端看不到 X-Request-ID | 框架不回写响应头 | 业务 `c.Header(...)` |
| 有 traceparent 但后端无 trace | 只透传标识，未接 OTel SDK | 配置 TracerProvider + Exporter |
| span 缺失 | 只给 `otel.SetTracerProvider` 但 exporter 未启动 | 检查 exporter 与 `tp.Shutdown` |
| 异步任务日志没有 request id | goroutine 不同 | 复制 header 或显式传 ctx 值 |
