# 链路追踪

MGin 不实现完整 Tracing（没有 Span、采样、上报后端），只提供**请求标识透传**：让同一请求在服务端日志、下游调用日志、MongoDB 访问日志中用同一个 ID 串起来。

## 1. 上下文载体

`middleware/trace` 在请求入口生成并写入：

| Header | 生成规则 |
|--------|----------|
| `X-Request-ID` | 优先使用上游传入值；否则 16 位 hex 随机 |
| `X-Real-IP` | `X-Forwarded-For`（取第一个）→ `X-Real-IP` → `ClientIP` |
| `X-User-Agent` | `User-Agent` 或 `X-User-Agent` |

这些值同时写入 gin context（`c.Set`），供日志与 `client` 使用。

## 2. 获取与设置

```go
reqID := trace.GetRequestId(c)       // 当前请求 ID
ip := trace.GetClientIp(c)
ua := trace.GetUserAgent(c)

trace.SetHeader(c, "X-Tenant-Id", tenantID) // 透传到下游
headers := trace.GetHeaders(c)              // map[string]string
```

`trace.Headers()` 返回固定的三个头名，供中间件初始化使用。

## 3. 跨协程传播

日志需要 `request_id` 时，异步协程不会自动继承 gin context：

```go
go func(c *gin.Context) {
	defer trace.CopyPreHeaderToCurRoutine(c)() // 恢复原协程上下文
	// 本协程内 logs.Info 会带上同一个 request_id
}(c)
```

实现方式：以 goroutine ID 为键的内存缓存（默认 5 分钟），不是 Go 原生 context。**不要**在协程里长期持有 `*gin.Context`。

## 4. 跨服务传播

`client` 自动带 `trace.GetHeaders()`，无需手工处理：

```
服务 A（入口生成 X-Request-ID）
  └─ client.Call → 下游 B（Header 携带同一 ID）
        └─ B 的 postlog / logs 记录同一 ID
```

若使用自建 `http.Client`，必须手动复制这三个头，否则链路断开。

## 5. 日志中的呈现

- 运行日志：消息前缀 `[requestId]`，字段 `request_id`（结构化输出）。
- 接口日志：`models.PostLog.RequestId`。
- 调用日志（VictoriaLogs）：`request_id` 字段。

## 6. 与 OpenTelemetry 的关系

当前**没有** OTel/W3C `traceparent` 支持。若公司已统一 OTel：

1. 在网关注入 `traceparent`；
2. 在业务中间件中解析并写入 `X-Request-ID`（可复用网关的 trace id）；
3. 用 OTel SDK 自行埋点，MGin 的 ID 作为属性透传。

不要假设 `X-Request-ID` 等于 OTel trace id。

## 7. 排障

| 症状 | 原因 | 处置 |
|------|------|------|
| 下游日志没有 request_id | 用了自建 HTTP 客户端 | 复制 `trace.GetHeaders()` |
| 同一请求 ID 变化 | 中间件顺序错误/重复生成 | 确认 `trace.TraceId()` 只挂一次且在业务前 |
| 异步日志 ID 丢失 | 未调用 `CopyPreHeaderToCurRoutine` | 按 §3 处理 |
| IP 恒为网关地址 | 未正确配置代理头 | 由可信代理写 `X-Forwarded-For` |
