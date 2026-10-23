# 协议与数据格式

本章定义 MGin 服务对外的**事实协议**：HTTP 响应、错误体、探针、指标、头部，
以及 `pkg/client` 的四种调用协议。

---

## 1. HTTP 通用约定

| 项 | 约定 |
|----|------|
| 编码 | UTF-8 |
| 请求体 | JSON（`application/json`）或表单 |
| 响应体 | JSON 对象（除文件下载、`/metrics`） |
| 时间 | RFC3339Nano（`srvTime`、日志 time） |
| 字段命名 | 以结构体 tag 为准；`models` 用 `status/msg/data/page`，`vo` 用 `code/data/msg/srvTime` |

---

## 2. 响应体

### 2.1 `models.Result[T]`（v1 兼容 / i18n）

```json
{
  "status": 1,
  "msg": "Success",
  "data": {"id": 7},
  "page": {"count": 5, "index": 1, "size": 20, "total": 97}
}
```

- `status=1` 成功；其他为业务错误码；
- `data`/`page` 为 `omitempty`，可能不出现；
- 错误示例：`{"status":1004,"msg":"订单不存在"}`。

### 2.2 `vo.Response[T]`

```json
{"code":1,"data":{"id":7},"msg":"ok","srvTime":"2026-10-23T21:00:00.123+08:00"}
```

- `code=1` 成功；`999` 未登录；`500` 通用错误；
- `srvTime` 由服务端生成。

### 2.3 `vo.ResponsePage[T]`

```json
{
  "code": 1,
  "msg": "ok",
  "srvTime": "...",
  "data": {"list": [], "total": 0, "page": 1, "pageSize": 20}
}
```

### 2.4 框架级错误

| 场景 | HTTP | 响应体 |
|------|------|--------|
| 路由不存在 | 404 | `{"status":1000,"msg":"<i18n 文案或 key>"}` |
| panic | 500 | `{"status":1001,"msg":"..."}` |
| JWT 失败 | 401 | **空响应体** |
| Casbin 无 claims | 401 | `{"status":401,"msg":"用户未登录"}` |
| Casbin 无权限 | 403 | `{"status":403,"msg":"无访问权限"}` |
| 限流 | 规则 `httpStatus`（默认 200） | `{"status":1011,"msg":"请求过于频繁，请稍后再试"}` |

> JWT 的 401 是 `AbortWithStatus`，没有 JSON 体；客户端需同时判断状态码。

---

## 3. 健康探针

| 路径 | 方法 | 成功 | 失败 |
|------|------|------|------|
| `/healthz` | GET | 200 | — |
| `/health/live` | GET | 200 | — |
| `/health/ready` | GET | 200 | 503 |
| `/health/startup` | GET | 200 | 503 |

存活/别名响应：

```json
{"code":1,"msg":"success","srvTime":"2026-10-23T21:00:00.1+08:00",
 "data":{"status":"UP","version":"1.0.0","uptime":86400}}
```

就绪：

```json
{"status":"ok","dependencies":{"mysql":"ok","redis":"fail"}}
```

启动：

```json
{"status":"ok"}
```

- `/ready` 的 `dependencies` 只包含 `go.config.used` 启用的 mysql/mongodb/redis；
- 没有任何数据源时 `dependencies` 被省略；
- `/startup` 不返回依赖字段。

---

## 4. 指标端点

`GET /metrics`，`text/plain; version=0.0.4`（Prometheus 格式）。

指标名与标签见 [指标](../06-observability/01-metrics.md)。

---

## 5. 头部约定

### 5.1 入站

| 头 | 用途 | 缺失时 |
|----|------|--------|
| `traceparent` | W3C 追踪上下文 | 生成 |
| `X-Trace-Id` | v1 兼容 trace id | 从 traceparent 解析或生成 |
| `X-Request-ID` | 请求标识 | 生成 16 hex |
| `X-Real-IP` | 真实客户端 IP | 取 `X-Forwarded-For` 或 `ClientIP` |
| `X-Lang` | i18n 语言 | 用默认语言 |
| `Authorization` | `Bearer <token>` | 401（挂 JWT 时） |

### 5.2 出站（client 自动注入）

`traceparent`、`X-Trace-Id`、`X-Request-ID`、`X-Real-IP`、`X-User-Agent`。

### 5.3 响应

框架**不回写** trace 头；需要时业务显式设置。
限流开启 `headers` 时返回：

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
X-RateLimit-Policy: token_bucket
Retry-After: 1
```

CORS 默认：`Access-Control-Allow-Origin: *`、`Allow-Credentials: false`。

---

## 6. 调用协议（pkg/client）

| Protocol | 常量 | Content-Type | 参数位置 | 默认方法 |
|----------|------|--------------|----------|----------|
| `x-form` | `CONTENT_TYPE_FORM` | `application/x-www-form-urlencoded` | `Data` → body；`Query` → query | POST |
| `json` | `CONTENT_TYPE_JSON` | `application/json` | `Json` → body；`Query` → query | POST |
| `restful` | `CONTENT_TYPE_RESTFUL` | `application/json` | `Path` 替换 `{k}`；`Json` → body | POST |
| `file` | `CONTENT_TYPE_FILE` | `multipart/form-data`（不手动设置） | `Files` + `Data` | POST（强制） |

规则：

1. `Protocol` 为空 → 取 `go.discovery.callType`；
2. `Method` 为空 → `POST`；
3. `Files` 非空 → 强制 `file`；
4. URL = 实例地址 + `uri`（`uri` 应以 `/` 开头）；
5. 文件协议固定用 POST，忽略 `Method`。

### 6.1 路径参数

```go
uri := "/api/v1/orders/{id}"
Path: map[string]string{"id": "123"}
// → /api/v1/orders/123（url.PathEscape）
```

### 6.2 响应解析

| 函数 | 期望结构 | 解析失败 |
|------|----------|----------|
| `Call`/`CallCtx` | 任意字符串 | 不解析 |
| `CallT` | `vo.Response[T]` | `Code=-1` + 错误文案 |
| `CallPageT` | `vo.ResponsePage[T]` | `Code=-1` |
| `CallCtxT`/`CallResultT` | `models.Result[T]` | `Status=-1` |

`decodeResponse` 要求响应是 JSON **对象**（以 `{` 开头），否则报错。

---

## 7. 状态码映射

| 业务场景 | 建议 HTTP | 业务码（内置） |
|----------|-----------|----------------|
| 成功 | 200 | 1 |
| 参数缺失/错误 | 400 | 1003 / 1014 |
| 未认证 | 401 | 1005/1006/1008/1009 |
| 无权限 | 403 | — |
| 不存在 | 404 | 1000/1004 |
| 冲突 | 409 | — |
| 限流 | 429 | 1011 |
| 服务/依赖不可用 | 503 | 1002/1010 |
| 服务端错误 | 500 | 1001/1012/1013 |

完整定义表见 [errcode](../03-development/02-service-development.md#52-注册业务错误码) 与
[api/10-i18n-errcode.md](api/10-i18n-errcode.md)。

---

## 8. 国际化头与回退

```
请求 X-Lang: en-US
   ↓
字典 <app>_i18n 查 (message_id, en-us)
   ├─ 命中 → 文本
   └─ 未命中 → 回退默认语言（go.i18n.default）
                └─ 仍未命中 → 返回 message_id 原文
```

语言标签规范：`^[a-z]{2,8}(-[a-z0-9]{1,8})*$`，最长 35；`_` 会被规范为 `-`。

---

## 9. 文件与流

- 文件下载：业务直接写 `c.Data`/`c.File`；postlog 只记录摘要（zip/pdf/xlsx 等）；
- 文件上传：通过 `client.Files` 或业务自行处理 multipart；
- 不支持 WebSocket、gRPC、SSE 的框架级封装（gin 原生能力仍可用）。

---

## 10. 兼容性注意

| 变化 | 影响 |
|------|------|
| v2 起 404/500 不再返回 HTTP 200 | 依赖 200 的旧客户端需调整 |
| `Call` 不把 4xx/5xx 当错误 | 必须解析响应体 |
| JWT 401 无 JSON 体 | 客户端需容错 |
| `X-Lang` 不自动出站 | 跨服务多语言需显式传 |
| 限流默认 HTTP 200 | 建议业务配置 429 |
