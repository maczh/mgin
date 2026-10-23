# 服务调用

调用方使用 `client` 包，通过注册中心解析服务名，自动携带追踪头，并把响应解析为泛型结构。

## 1. 前置条件

1. 调用方启用了注册中心：

```yaml
go:
  config:
    used: "nacos"
    prefix: { nacos: nacos }
  discovery:
    registry: nacos
    callType: json
```

2. 被调服务已用同一个 `go.application.name` 注册。
3. `registry.Registry` 已初始化（`NewApp` 内部完成）；未初始化时调用直接返回「服务注册中心未初始化」。

## 2. 最小调用

```go
type User struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

resp := client.CallT[User]("user-service", "/api/v1/users/{id}", &client.Options{
	Method:   http.MethodGet,
	Protocol: "restful",
	Path:     map[string]string{"id": "42"},
})
if resp.Code != vo.SUCCESS {
	logs.Error("调用失败 code={} msg={}", resp.Code, resp.Msg)
	return
}
logs.Info("用户: {}", resp.Data.Name)
```

- `CallT` 解析 `{code,msg,srvTime,data}`（`vo.Response[T]`）。
- 旧协议（`{status,msg,data}`）用 `CallResultT[T]`。
- 分页用 `CallPageT[T]`，解析 `data.list/total/page/pageSize`。
- 需要原始字符串用 `Call`。

## 3. 协议与参数

| 协议 | `Options` 字段 | 说明 |
|------|----------------|------|
| `x-form` | `Data map[string]string` | `application/x-www-form-urlencoded`；默认协议 |
| `json` | `Json any` | JSON body（GET 也可用 query） |
| `restful` | `Path map[string]string` | 路径中的 `{key}` 会被 `PathEscape` 替换 |
| `file` | `Files []grequests.FileUpload` | 非空即强制 file 协议，忽略 `Protocol` |

通用字段：`Method`（默认 POST）、`Header`（合并到 trace 头之上）、`Query`、`Group`（Nacos 分组）、`Retry`。

```go
// POST JSON
resp := client.CallT[Order]("order-service", "/api/v1/orders", &client.Options{
	Method:   http.MethodPost,
	Protocol: "json",
	Json:     map[string]any{"sku": "A-1", "qty": 2},
})

// 上传文件
resp := client.Call("file-service", "/api/v1/upload", &client.Options{
	Files: []grequests.FileUpload{{
		FileContents: data,
		FileName:     "report.xlsx",
		FileField:    "file",
	}},
})
```

## 4. 协议选择建议

| 场景 | 建议 |
|------|------|
| 内部表单式接口、历史服务 | `x-form` |
| 现代 JSON API | `json` |
| REST 资源风格（`/users/{id}`） | `restful` |
| 文件上传/下载 | `file` |

同一服务内不要混用多种协议；`callType` 只提供默认值，显式 `Protocol` 优先。

## 5. 追踪与上下文

调用会自动带上 `trace.GetHeaders()`：

| Header | 来源 |
|--------|------|
| `X-Request-ID` | 当前请求 ID，没有则生成 16 位 hex |
| `X-Real-IP` | `X-Forwarded-For` → `X-Real-IP` → `ClientIP` |
| `X-User-Agent` | 请求头或 `User-Agent` |

`op.Header` 可以覆盖同名头。跨服务链路靠这三个头串起来，不要手工删除。

## 6. 超时、重试与错误

当前实现的真实行为（务必了解）：

- **没有内置超时**：`grequests` 未设置 `Timeout`，下游不响应会一直占用 goroutine。需要在调用方用 `context`/网关超时兜底。
- **`Retry=true` 只重试一次**：失败后递归调用一次，仍失败即返回。
- **HTTP 非 2xx 视为错误**：返回 `error`，`CallT` 的 `Code` 为 0。
- **服务未找到**：返回「微服务获取…主机IP端口失败」，不会降级到本地。

```go
// 需要可控超时：自行包一层 goroutine + context，或改用业务 HTTP 客户端
done := make(chan vo.Response[User], 1)
go func() {
	done <- client.CallT[User]("user-service", "/api/v1/users/{id}", op)
}()
select {
case r := <-done:
	handle(r)
case <-time.After(2 * time.Second):
	logs.Warn("调用 user-service 超时")
}
```

## 7. 调用日志

`go.log.victorialogs.client: true` 时，每次调用异步上报一条日志（含耗时与结果），用于链路分析。运行日志另由 `go.logger.outputs` 配置。

## 8. 常见问题

| 现象 | 原因 | 处理 |
|------|------|------|
| 「服务注册中心未初始化」 | `go.discovery.registry` 为空 | 配置并重启 |
| 调用成功但 `Code=0` | 下游返回非 2xx | 查看 `CallT` 的 `Msg` 与下游日志 |
| 路径参数没替换 | 协议不是 `restful` 或 `Path` key 不匹配 | 检查 `{id}` 与 `Path["id"]` |
| 文件上传失败 | 未填 `FileName`/`FileField` | 按 `grequests.FileUpload` 要求补齐 |
| 下游收不到请求 ID | 手工设置了同名 Header 覆盖 | 不要覆盖 `X-Request-ID` |
