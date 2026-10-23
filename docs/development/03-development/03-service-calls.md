# 服务间调用

本章覆盖 `pkg/client` 的全部调用方式、协议差异、上下文透传与错误语义。
注册中心与负载均衡的内部机制见 [05 · 服务治理](../05-governance/01-discovery-and-lb.md)。

---

## 1. 调用前必须成立的三件事

1. `go.config.used` 启用了注册中心（如 `nacos`），且 `conf/nacos-<env>.yml` 正确；
2. 被调服务已注册，服务名与 provider 的 `go.application.name` 一致；
3. 本进程能访问注册中心。

缺任何一条，`Call` 会返回“获取服务主机IP端口失败”。

---

## 2. 调用函数一览

| 函数 | 返回 | 适用 |
|------|------|------|
| `Call(service, uri, op)` | `(string, error)` | 兼容 v1；拿原始响应体 |
| `CallCtx(ctx, service, uri, op)` | `(string, error)` | **推荐**；支持取消/超时（上层兜底） |
| `CallT[T](service, uri, op)` | `vo.Response[T]` | 下游返回 `code/data/msg/srvTime` |
| `CallCtxT[T](ctx, service, uri, op)` | `models.Result[T]` | 下游返回 `status/msg/data` |
| `CallResultT[T](service, uri, op)` | `models.Result[T]` | 同上，无 ctx 版本 |
| `CallPageT[T](service, uri, op)` | `vo.ResponsePage[T]` | 下游返回 `data.list/total/page/pageSize` |
| `CallResilient(service, uri, op, ro)` | `(string, error)` | 需要超时/重试/熔断 |
| `CallResilientCtx(ctx, service, uri, op, ro)` | `(string, error)` | 同上，带 ctx |

`CallT` 的 `T` 是**业务数据类型**，不是响应包装类型：

```go
type Order struct{ Id int64 `json:"id"` }
resp := client.CallT[Order]("order", "/api/v1/orders/1", &client.Options{Method: "GET"})
// resp.Code, resp.Msg, resp.Data.Id
```

---

## 3. Options 字段语义

```go
type Options struct {
    Method       string
    Protocol     string
    Group        string
    Header       any
    Query        any
    Data         any
    Json         any
    Path         map[string]string
    Files        []grequests.FileUpload
    Retry        bool
    LoadBalancer string
}
```

| 字段 | 默认 | 作用 | 注意 |
|------|------|------|------|
| `Method` | `POST` | HTTP 方法 | 空值不是 GET |
| `Protocol` | `go.discovery.callType` | `x-form`/`json`/`restful`/`file` | `Files` 非空时强制 `file` |
| `Group` | 当前注册分组 | 选择实例分组 | 多分组时用于隔离环境 |
| `Header` | 空 | 额外请求头 | 支持 `map[string]string` 等，经 `utils.AnyToMap` 转换 |
| `Query` | 空 | URL query | 同上 |
| `Data` | 空 | `x-form` 表单 / `file` 表单字段 | `json` 协议**不用** `Data` |
| `Json` | 空 | `json`/`restful` 请求体 | 结构体/`map` 均可 |
| `Path` | 空 | `restful` 路径占位符 | `uri` 中写 `{id}`，`Path["id"]="1"` |
| `Files` | 空 | 文件上传 | 会强制 `file` 协议并使用 POST |
| `Retry` | false | 底层失败后**再试一次** | 朴素递归，非指数退避；用韧性层替代 |
| `LoadBalancer` | 默认轮询 | 策略注册名 | `round_robin`/`random`/`least_connections`/`consistent_hash` |

### 3.1 URI 规则

- `uri` 必须以 `/` 开头：最终 URL = `host + uri`；
- 路径参数只对 `restful` 生效，占位符格式 `{name}`；
- 不要把 query 拼进 `uri`，用 `Query`。

### 3.2 四种协议

```go
// 表单
client.CallCtx(ctx, "user", "/api/v1/login", &client.Options{
    Method: "POST", Protocol: client.CONTENT_TYPE_FORM,
    Data: map[string]any{"name": "tom", "pwd": "***"},
    Query: map[string]any{"v": "2"},
})

// JSON
client.CallCtx(ctx, "order", "/api/v1/orders", &client.Options{
    Method: "POST", Protocol: client.CONTENT_TYPE_JSON,
    Json: map[string]any{"userId": 1001, "amount": 9900},
})

// RESTful 路径参数
client.CallCtx(ctx, "order", "/api/v1/orders/{id}", &client.Options{
    Method: "GET", Protocol: client.CONTENT_TYPE_RESTFUL,
    Path: map[string]string{"id": "123"},
})

// 文件上传（自动强制 POST + multipart）
client.CallCtx(ctx, "file", "/api/v1/upload", &client.Options{
    Protocol: client.CONTENT_TYPE_FILE,
    Data:     map[string]any{"biz": "avatar"},
    Files: []grequests.FileUpload{{
        FileLocation:  "/tmp/a.png",
        FileName:      "a.png",
        FileFieldName: "file",
    }},
})
```

---

## 4. 超时、重试与熔断

```go
body, err := client.CallResilientCtx(ctx, "order", "/api/v1/orders/1",
    &client.Options{Method: "GET"},
    &client.ResilienceOptions{
        Timeout:    2 * time.Second,
        MaxRetries: 2,
        BaseDelay:  100 * time.Millisecond,
        MaxDelay:   2 * time.Second,
        Breaker:    client.GetBreaker("order"),
    })
```

### 4.1 语义

| 能力 | 行为 | 约束 |
|------|------|------|
| 超时 | `goroutine + timer`，到点返回 `ErrCallTimeout` | 底层 HTTP **不会取消**，连接可能残留 |
| 重试 | 指数退避 + 全抖动：`min(Base*2^n, Max)`，等待 `[d/2, d)` | 仅幂等方法（GET/HEAD/OPTIONS/TRACE）默认重试；POST/PUT/DELETE/PATCH 不重试 |
| 熔断 | 每个 `service@host` 一个熔断器；打开后快速失败 | 全部实例熔断返回 `ErrAllInstancesCircuitOpen`，**不回退**单实例 |
| 状态码重试 | `RetryOnStatus` 回调 | 当前 `Call` 不把非 2xx 当错误，回调通常拿不到状态码 |

默认值：

| 参数 | 默认 |
|------|------|
| `Timeout` | 0（不超时） |
| `MaxRetries` | 0（不重试） |
| `BaseDelay` | 100ms |
| `MaxDelay` | 2s |
| 熔断 `maxFailures` | 5 |
| 熔断 `interval` | 60s |
| 熔断 `timeout`（Open→HalfOpen） | 30s |
| 半开允许并发 | 1 |

### 4.2 选择建议

| 场景 | 建议 |
|------|------|
| 查询类 GET | Timeout 500ms–2s，MaxRetries 1–2 |
| 创建订单等写操作 | Timeout 2–5s，**不重试**；用幂等键在业务层保证 |
| 文件上传 | 不设置过短超时；不用自动重试 |
| 下游已配重试（网关/Mesh） | 客户端关闭重试，避免放大 |

---

## 5. 上下文与请求头透传

框架自动完成：

| 头 | 来源 | 用途 |
|----|------|------|
| `traceparent` | 入站透传或生成 | W3C 链路追踪 |
| `X-Trace-Id` | 同 trace id | 日志检索 |
| `X-Request-ID` | 入站或生成 | 单次请求标识 |
| `X-Real-IP` | 入站 | 下游获取真实客户端 IP |
| `X-Lang` | 入站 | 下游 i18n（如果业务透传） |

出站时 `client` 通过 `trace.GetHeaders()` 自动带上当前请求的头，
不需要手工从 `*gin.Context` 拷贝。若在非 HTTP 场景（job/MQ）中调用，
头为空是正常的，可显式传 `Header`。

> `X-Lang` 不会被自动注入到出站请求；需要跨服务多语言时显式放入 `Header`。

---

## 6. 错误语义

| 场景 | 返回 |
|------|------|
| `op == nil` | `error: client: Options 参数不能为空` |
| 注册中心无实例 | `error: 微服务获取X服务主机IP端口失败` |
| 连接失败（`dial tcp`） | 同上（错误文本被归一化） |
| 所有实例熔断 | `client.ErrAllInstancesCircuitOpen` |
| 韧性层熔断打开 | `wrapped client.ErrCircuitOpen` |
| 韧性层超时 | `wrapped client.ErrCallTimeout` |
| 下游返回 4xx/5xx | **`err == nil`，body 原样返回**（需自行判断） |
| 响应不是 JSON 对象 | `CallT`/`CallCtxT` 返回 `code=-1`/`status=-1` + 错误文案 |

因此：**不要只判断 error**，业务错误通常在响应体里。

```go
resp := client.CallT[Order]("order", "/api/v1/orders/1", op)
if resp.Code != vo.SUCCESS {
    return fmt.Errorf("order service: code=%d msg=%s", resp.Code, resp.Msg)
}
```

---

## 7. 在非 HTTP 场景调用

```go
func CloseTimeoutOrders(ctx context.Context) error {
    body, err := client.CallResilientCtx(ctx, "order", "/api/v1/orders/close-timeout",
        &client.Options{Method: "POST", Protocol: client.CONTENT_TYPE_JSON,
            Json: map[string]any{"before": time.Now().Add(-30 * time.Minute)}},
        &client.ResilienceOptions{Timeout: 5 * time.Second})
    if err != nil {
        return err
    }
    var r models.Result[int]
    utils.FromJSON(body, &r)
    return nil
}
```

此时没有入站 trace 头，框架会自动生成新的 trace id；
如果需要与任务日志关联，把 `job.Context.LogId` 放进 `Header`。

---

## 8. 常见问题

| 现象 | 原因 | 处理 |
|------|------|------|
| 永远“获取主机IP端口失败” | 注册中心未启用/服务名不一致/分组不一致 | 查 `go.config.used`、`go.config.env`、`Group` |
| 调用返回 HTML/空串 | 下游 404/网关错误页 | 打印 `body` 与状态；`Call` 不把非 2xx 当错误 |
| 超时后下游仍在处理 | 超时是上层兜底，未取消底层 | 下游做幂等；或缩短下游自身耗时 |
| POST 被重试导致重复数据 | 用了 `Call` 的 `Retry:true` | 关闭 `Retry`，用业务幂等键 |
| 熔断后一直失败 | 所有实例熔断，等待 `timeout` 半开 | 检查下游健康；必要时 `ResetBreakers()`（仅测试/应急） |
| `CallT` 解析出零值 | 下游用 `models.Result` 而非 `vo.Response` | 对齐响应模型：`CallT` ↔ `vo`，`CallCtxT` ↔ `models` |
