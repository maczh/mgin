# 协议参考

## 1. 总览

| 通信 | 协议 | 编码 | 说明 |
|------|------|------|------|
| 客户端 ↔ 服务 | HTTP/1.1（HTTPS 可选） | JSON / form / multipart | 无 gRPC、无 WebSocket 封装 |
| 服务 ↔ 服务 | HTTP | 四种调用协议 | 见 §5 |
| 服务 ↔ 注册中心 | 各产品 HTTP API / etcd v3 | — | 见 [服务发现](../05-governance/01-discovery-and-load-balancing.md) |
| 服务 ↔ 配置中心 | HTTP / etcd | YAML | 见 §7 |
| 日志 → MongoDB | MongoDB wire protocol | BSON | 接口日志 |
| 日志 → VictoriaLogs | HTTP | JSON lines | 运行/调用日志 |

统一约定：

- 字符集 UTF-8；请求体大小未限制，需在网关控制。
- 时间：接口返回的时间字段由业务决定；框架内部 `SrvTime` 为 `time.Time`（RFC3339 序列化）。
- 分页：`page` 从 1 开始，`size` 缺省 20。

---

## 2. 业务响应信封

### 2.1 成功

```json
{
  "status": 1,
  "msg": "success",
  "data": {"id": 1, "name": "alice"}
}
```

分页：

```json
{
  "status": 1,
  "msg": "success",
  "data": [{"id": 1}],
  "page": {"count": 1, "index": 1, "size": 20, "total": 42}
}
```

### 2.2 业务失败

```json
{"status": 1004, "msg": "数据不存在", "data": null}
```

HTTP 状态通常为 **200**。错误码见 §4。

### 2.3 路由不存在

```json
{"status": 1000, "msg": "404", "data": null}
```

HTTP 200；由 `NoRoute` 产生。`msg` 为 i18n 字典键 `404` 的翻译结果（未启用 i18n 时为 `404`）。

### 2.4 未捕获异常

```json
{"status": 1001, "msg": "system_error", "data": null}
```

HTTP 200；由 `nice.Recovery` 的 `recoveryHandler` 产生，堆栈只写日志。

### 2.5 限流

默认 HTTP 200：

```json
{"status": 1011, "msg": "请求过于频繁，请稍后再试", "data": null}
```

配置 `httpStatus: 429` 时 HTTP 状态变为 429，body 结构不变。

### 2.6 鉴权失败

| 场景 | HTTP | Body |
|------|------|------|
| JWT 缺失/非法 | 401 | 空 |
| Casbin 无策略 | 403 | `models.Error(403, ...)` |
| Casbin 服务不可用 | 500 | `models.Error(500, ...)` |
| IP 不在网段 | 403 | 纯文本 |
| 并发限制超时 | 499 | 空 |

---

## 3. 请求头

| Header | 方向 | 说明 |
|--------|------|------|
| `X-Request-ID` | 双向 | 链路 ID；无则生成 16 位 hex |
| `X-Real-IP` | 双向 | 客户端 IP；优先 `X-Forwarded-For` 首段 |
| `X-User-Agent` | 双向 | 客户端标识 |
| `X-Lang` | 请求 | 语言，如 `zh-cn`/`en-us`；`_` 会归一化为 `-` |
| `Authorization` | 请求 | `Bearer <token>` 或裸 token |
| `X-MGin-Reload-Token` | 请求 | 配置重载令牌（或 `Authorization: Bearer`） |
| `Origin` | 请求 | CORS 来源 |
| `Access-Control-Allow-*` | 响应 | CORS 响应头 |
| `X-RateLimit-*` | 响应 | `ratelimit.headers: true` 时输出 |

CORS 允许方法固定：`POST, GET, OPTIONS, PUT, DELETE, PATCH, HEAD, UPDATE`。有 `Origin` 时回显来源并允许凭证；否则 `*` 且不允许凭证。

---

## 4. 错误码

| 码 | 含义 | 典型来源 |
|----|------|----------|
| 1000 | 路由不存在 | 404 |
| 1001 | 系统异常 | panic |
| 1002 | DB 连接/查询失败 | 业务/DAO |
| 1003 | 参数缺失 | `ParamLostError` |
| 1004 | 数据不存在 | 业务 |
| 1005 | 用户不存在 | 业务 |
| 1006 | 密码错误 | 业务 |
| 1007 | 验证码错误 | 业务 |
| 1008 | Token 无效 | 业务 |
| 1009 | 认证失败 | 业务 |
| 1010 | 服务不可用 | 业务 |
| 1011 | 限流 | ratelimit |
| 1012 | 任务失败 | job |
| 1013 | 存储错误 | 业务（无对象存储实现） |
| 1014 | 参数格式错误 | `ParamError` |

调用响应码：`1` 成功、`500` 失败、`999` 未登录（`vo`）。

---

## 5. 服务调用协议

### 5.1 `x-form`（默认）

```
POST /path?query
Content-Type: application/x-www-form-urlencoded
X-Request-ID: ...
X-Real-IP: ...
X-User-Agent: ...

a=1&b=2
```

字段来源 `Options.Data`；`Options.Query` 拼到 URL。

### 5.2 `json`

```
POST /path
Content-Type: application/json

{"sku":"A-1","qty":2}
```

body 来源 `Options.Json`（任意可序列化值）。GET 请求时 body 仍会发送，下游需支持。

### 5.3 `restful`

```
GET /api/v1/users/42
```

`uri` 中的 `{key}` 用 `Options.Path[key]` 替换，值经 `url.PathEscape`。缺 key 时保留占位符。

### 5.4 `file`

```
POST /upload
Content-Type: multipart/form-data; boundary=...

--boundary
Content-Disposition: form-data; name="file"; filename="report.xlsx"
```

`Options.Files` 非空即强制此协议。

### 5.5 响应解析

| 函数 | 期望响应 |
|------|----------|
| `Call` | 原始字符串 |
| `CallT[T]` | `{"code":1,"msg":"...","data":{...},"srvTime":"..."}` |
| `CallPageT[T]` | `{"code":1,"data":{"list":[],"total":0,"page":1,"pageSize":20}}` |
| `CallResultT[T]` | `{"status":1,"msg":"...","data":{...}}` |

非 2xx 视为错误（返回 error，`Code=0`）；JSON 解析失败也返回错误。

---

## 6. 管理端点

| 端点 | 方法 | 前置 | 响应 |
|------|------|------|------|
| `/healthz`、`/health/live` | GET | 探针启用 | `{"code":1,"msg":"success","srvTime":...,"data":{"status":"UP","version":"...","uptime":123}}` |
| `/health/ready` | GET | 探针启用 | 200 `{"status":"ok"}` / 503 `{"status":"fail","dependencies":{...}}` |
| `/health/startup` | GET | 探针启用 | 200 `{"status":"ok"}` / 503 |
| `/metrics` | GET | 指标启用 | Prometheus 文本 |
| `/cfg/reload` | PUT | `go.config.reload.token` 非空 | 成功 200；401/403/500 |
| `/job/list` | GET | job 启用 | 分页任务列表（`index`/`size`/`group`/`keyword`/`status`） |
| `/job/:id` | GET/PUT/DELETE | job | 任务详情/更新/删除 |
| `/job/:id/start`、`/stop`、`/trigger` | POST | job | 操作结果 |
| `/job/log` | GET | job | 执行日志分页（`jobId`/`jobName`/`status`/`index`/`size`） |
| `/job/handlers` | GET | job | 已注册执行器名 |
| `/i18n/list`、`/get`、`/save`、`/update`、`/delete` | GET/POST | `go.i18n.api.enabled` | 字典 CRUD |
| `/debug/pprof/*` | GET | **未内置**，需自行挂载 | — |

---

## 7. 配置中心寻址

| 类型 | 请求 |
|------|------|
| Nacos | `GET {server}/nacos/v1/cs/configs?group={project}&dataId={prefix}-{env}.yml` |
| Consul | `GET {server}/v1/kv/{project}/{prefix}-{env}.yml?raw=true` |
| etcd | `GET /config/{project}/{prefix}-{env}.yml` |
| Polaris | `GET .../configfiles/release?namespace=...&group=...&name={prefix}-{env}.yml&file=true`（带 token） |
| Spring Config | `GET {server}/{project}/{prefix}-{env}.yml` |
| file | 读取 `{path}/{prefix}-{env}.yml` |

`{prefix}` 支持 `prefix@v版本`（Polaris），用于取指定版本。

---

## 8. 兼容性

- 本分支模块路径 `github.com/maczh/mgin`（v0/v1）；`/v2` 属于 `master`，协议与 API 不同。
- `models.Result` 与 `vo.Response` 是两套信封：业务接口用前者，跨服务新代码用后者，不要在同一接口混用。
- 旧服务可能只返回 `{status,msg,data}`，用 `CallResultT` 适配。
- 响应字段只增不改；删除字段属于破坏性变更，需走 major 版本。
