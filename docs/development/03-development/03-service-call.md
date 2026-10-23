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

通用字段：

| 字段 | 说明 |
|---|---|
| `Method` | 默认 `POST` |
| `Header` | `any`，经 `utils.AnyToMap` 合并到 trace 头之上；`Call` 内部会深拷贝，不改调用方数据 |
| `Query` | URL Query 参数 |
| `Group` | 注册中心分组/命名空间；回填为实际命中的 group |
| `Retry` | 失败后递归重试**一次**（非退避重试） |
| `Timeout` | 整次调用的超时；`<=0` 时取 `go.client.timeout`（默认 `10s`） |
| `Fallback` | `func(error) (string, bool)`，返回 `(响应体, true)` 即视为成功 |
| `CircuitKey` | 熔断分组键，默认 `service + "\x00" + uri` |
| `CircuitFailures` | 熔断失败阈值，`<=0` 取 `go.discovery.circuitFailures`（默认 3） |
| `CircuitOpenWindow` | 熔断打开后的探测窗口，`<=0` 取 `go.discovery.circuitOpenWindow`（默认 `30s`） |

`Call` 会先做浅拷贝再改写字段，同一个 `Options` 可以并发复用。

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

- **超时**：`Options.Timeout` 优先；为 0 时读 `go.client.timeout`（支持 `5s`/`500ms` 或纯数字秒，默认 `10s`），最终落到 `grequests.RequestTimeout`。
- **`Retry=true` 只重试一次**：失败后递归调用一次（保留 `Timeout`），仍失败即返回；不区分错误类型。
- **HTTP 非 2xx 视为错误**：返回 `error`，`CallT` 的 `Code` 为 `-1`、`Msg` 为 `service returned HTTP xxx`。
- **非 JSON 响应**：`CallT`/`CallResultT`/`CallPageT` 统一返回 `"Service error"` / `"Service response error: ..."`。
- **服务未找到**：返回「微服务获取…主机IP端口失败」，不会降级到本地；`dial tcp` 类错误也会被归一成同一文案。
- **`Protocol` 非法**：直接返回 `unsupported client protocol "xxx"`，不发请求。

```go
// 显式超时 + 一次重试
resp := client.CallT[User]("user-service", "/api/v1/users/{id}", &client.Options{
	Method:   http.MethodGet,
	Protocol: "restful",
	Path:     map[string]string{"id": "42"},
	Timeout:  2 * time.Second,
	Retry:    true,
})
```

## 7. 熔断与兜底

`client.Call` 内置一个按 key 的轻量熔断器（`go.discovery.circuitFailures` / `go.discovery.circuitOpenWindow`），并对成功响应做本地留存，用于失败兜底。

状态机：

| 状态 | 进入条件 | 行为 |
|---|---|---|
| 关闭 | 初始 / 任意一次成功 | 正常放行；失败累计到阈值后打开 |
| 打开 | 连续失败 ≥ 阈值 | 直接返回 `circuit breaker is open`，不发请求 |
| 半开 | 打开后超过 `circuitOpenWindow` | 放行**一个**探测请求：成功则关闭，失败则继续打开 |

兜底（fallback）三种用法：

```go
// 1) 全局开启：失败时回放该 service+uri+参数 的上一次成功响应
//    go.client.fallback.enabled: true

// 2) 单次调用自定义
resp := client.CallT[User]("user-service", "/api/v1/users/{id}", &client.Options{
	Fallback: func(err error) (string, bool) {
		return `{"code":0,"data":{"id":42,"name":"缓存用户"}}`, true
	},
})

// 3) 只取缓存，不自定义逻辑
op := &client.Options{}
op.Fallback = client.CachedFallback("user-service", "/api/v1/users/{id}",
	map[string]any{"path": map[string]string{"id": "42"}})
```

注意：

- 兜底返回原始 JSON 字符串，因此必须与 `CallT`/`CallResultT` 期望的结构一致，否则解析失败。
- 兜底命中同样会记为「失败」并累加熔断计数，避免下游长期不可用时无限重试。
- 响应缓存是**进程内**的 `map`，无 TTL、无容量上限；key 为 `service + uri + 参数 SHA256`，只在成功且未走兜底时写入。
- 熔断状态也在进程内，多副本各自独立。

## 8. 调用日志

`go.log.victorialogs.client: true` 且配置了 `go.log.victorialogs.url` 时，每次 `client.Call` 通过 `logsink` 异步上报一条记录（`client <service> <method> <uri> (<耗时>ms)`，失败时级别为 error 并附错误信息），独立于 `go.logger.level`。运行日志另由 `go.logger.outputs` 配置。

## 9. 常见问题

| 现象 | 原因 | 处理 |
|------|------|------|
| 「服务注册中心未初始化」 | `registry.Registry` 为 nil | 用 `NewApp` 启动，或手动赋值 |
| 「微服务获取 X 服务主机IP端口失败」 | 未配置 registry / 服务名或分组不匹配 | 查 `go.discovery.registry` 与注册中心控制台 |
| 调用成功但 `Code=-1` | 下游返回非 2xx，或响应不是 JSON 对象 | 查看 `CallT` 的 `Msg` 与下游日志 |
| 路径参数没替换 | 协议不是 `restful` 或 `Path` key 不匹配 | 检查 `{id}` 与 `Path["id"]` |
| 文件上传失败 | 未填 `FileName`/`FileField` | 按 `grequests.FileUpload` 要求补齐 |
| 下游收不到请求 ID | 手工设置了同名 Header 覆盖 | 不要覆盖 `X-Request-ID` |
| 「circuit breaker is open」 | 熔断打开，尚未过探测窗口 | 修复下游或调 `circuitFailures`/`circuitOpenWindow` |
| 兜底数据一直是旧值 | 响应缓存无 TTL | 只在幂等查询上开启兜底 |
| 请求体被并发改写 | 直接复用同一个 `Options` 并改字段 | 每次调用新建 `Options`（`Call` 只做浅拷贝） |
