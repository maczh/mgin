# 序列化、协议与上下文透传

## 1. 序列化

框架只使用 `encoding/json`：

| 场景 | 编码 |
|------|------|
| 业务响应 | `c.JSON` → JSON |
| 服务调用 | `json` 协议为 JSON body；`x-form` 为表单；`file` 为 multipart |
| 配置 | YAML（koanf 解析） |
| 接口日志 | BSON（MongoDB） |
| 运行日志 | text/json（`outputs[].format`） |

没有 Protobuf/Thrift/MsgPack 的内置支持。需要时由业务自行序列化，用 `client.Options.Json` 传 `[]byte`/`json.RawMessage`。

### 1.1 JSON 约定

- 字段名使用小驼峰（`json:"requestId"`），与前端约定一致。
- 时间字段：业务自定义（字符串或 `time.Time`）。`vo.Response.SrvTime` 为 `time.Time`，序列化为 RFC3339。
- 数字溢出：`int64` 在前端可能丢精度，ID 建议返回字符串（由业务 DTO 控制）。
- `omitempty`：空字符串/0/nil 会被省略；需要返回 `0`/`""` 时不要加。

```go
type UserDTO struct {
	Id    string `json:"id"`              // 避免前端精度问题
	Name  string `json:"name"`
	Age   int    `json:"age,omitempty"`   // 0 时省略
	Email string `json:"email,omitempty"`
}
```

## 2. 协议选择

| 协议 | 请求编码 | 适用 | 不适用 |
|------|----------|------|--------|
| `x-form` | form-urlencoded | 传统内部接口、简单字段 | 嵌套结构、数组 |
| `json` | application/json | 现代 API、嵌套结构 | 超大文件 |
| `restful` | 路径参数 + query | 资源风格 | 复杂查询体 |
| `file` | multipart | 上传 | 普通 JSON |

`client.Options.Protocol` 优先；未设置时用 `go.discovery.callType`（默认 `x-form`）。同一个下游服务应固定一种协议，避免下游要写多套解析。

### 2.1 响应协议

- 新接口：`vo.Response{code,msg,srvTime,data}`，调用方用 `CallT`。
- 存量接口：`models.Result{status,msg,data}`，用 `CallResultT`。
- 不要在一个接口里同时返回 `status` 与 `code`。

## 3. 上下文透传

### 3.1 三种「上下文」

| 上下文 | 载体 | 生命周期 | 用途 |
|--------|------|----------|------|
| 请求上下文 | `*gin.Context` | 单次 HTTP 请求 | 参数、Header、写响应 |
| 标准上下文 | `context.Context` | 调用链 | 超时、取消、DB/客户端传递 |
| 追踪上下文 | `X-Request-ID` + goroutine 缓存 | 跨服务/跨协程 | 日志关联 |

### 3.2 从 gin 到 context

```go
func GetUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	user, err := userService.Get(ctx, id) // service 只依赖 context.Context
	_ = err
	_ = user
}
```

controller 负责提取参数与设置超时；service 不接收 `*gin.Context`。

### 3.3 跨服务透传

`client` 自动带 `X-Request-ID`/`X-Real-IP`/`X-User-Agent`。业务自定义头（租户、灰度标签）：

```go
trace.SetHeader(c, "X-Tenant-Id", tenantID)

resp := client.CallT[User]("user-service", "/api/v1/users/{id}", &client.Options{
	Method: http.MethodGet,
	Protocol: "restful",
	Path: map[string]string{"id": id},
	Header: map[string]string{"X-Tenant-Id": tenantID},
})
```

下游读取：

```go
tenant := c.GetHeader("X-Tenant-Id")
```

### 3.4 跨协程

```go
go func(c *gin.Context) {
	defer trace.CopyPreHeaderToCurRoutine(c)()
	logs.Info("异步任务开始") // 会带上同一个 request_id
}(c)
```

追踪上下文存在进程内的 goroutine ID → 缓存（5 分钟），不是 `context.Context`。异步任务不要把 `*gin.Context` 传得太深，也不要跨请求复用。

### 3.5 隐式参数清单

| 参数 | 载体 | 谁写入 | 谁读取 |
|------|------|--------|--------|
| request id / client ip / user agent | Header + gin context | `trace` | `logs`、`client`、`postlog` |
| 语言 | `X-Lang` | 客户端 | `xlang`、`i18n` |
| 用户/角色 | JWT claims | `jwt` | `casbin`、业务 |
| 租户 | 自定义头 | 网关/客户端 | 业务 |

## 4. 常见问题

| 问题 | 原因 | 处理 |
|------|------|------|
| 下游拿不到租户 | 只设置了 gin context，没设 Header | 用 `trace.SetHeader` 或 `Options.Header` |
| service 里用了 `c.Param` | 分层错误 | 由 controller 提取后传参 |
| 超时不生效 | 没有把 `ctx` 传进 DB/客户端 | 全链路使用派生 ctx |
| 异步日志没有 request_id | 未恢复上下文 | `CopyPreHeaderToCurRoutine` |
| 数字 ID 前端精度丢失 | int64 直接返回 | DTO 转字符串 |
