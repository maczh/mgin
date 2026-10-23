# 安全：认证、授权与输入防护

> 本章只描述框架提供的能力与**必须由业务补齐的部分**。安全是端到端的，
> 框架不替代网关、WAF、密钥管理和审计。

---

## 1. 能力清单

| 能力 | 包 | 启用方式 | 默认 |
|------|----|----------|------|
| JWT 认证 | `middleware/jwt` | 业务 `Use` | 关闭 |
| Casbin 接口授权 | `middleware/casbin` + `pkg/casbin` | 配置 + `Use` | 关闭 |
| 会话 | `middleware/session` | 业务 `Use` | 关闭 |
| XSS 净化 | `middleware/xss` | 业务 `Use` | 关闭 |
| IP 白名单 | `middleware/iplimit` | 业务 `Use` | 关闭 |
| 并发保护 | `middleware/limit` | 业务 `Use` | 关闭 |
| 限流 | `middleware/ratelimit` | 配置启用 | 关闭 |
| HTTPS | `app.Run` | `cert/key` | 关闭 |
| CORS | `middleware/cors` | 恒启用 | 允许 `*` |

---

## 2. JWT

### 2.1 中间件行为

```go
app.Router.Use(jwt.JwtAuthorize())
```

1. `/docs/`、`/swagger/`、`go.sys.swagger.uri` 路径直接放行；
2. 读取 `Authorization`，支持 `Bearer <token>` 或裸 token；
3. `go.jwt.secret` 为空 → 401；
4. 仅接受 HMAC 签名（HS256/384/512）；
5. 解析失败/过期 → 401（**空响应体**，不是 JSON）。

### 2.2 关键限制

> **`JwtAuthorize` 只校验，不写入 `claims`。**

如果后续要用 `casbin.CasbinHandler()` 或业务需要用户信息，必须在 JWT 之后
自行解析并 `c.Set("claims", claims)`：

```go
func Claims() gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
        token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
            return []byte(config.Config.Jwt.Secret), nil
        })
        if err == nil && token.Valid {
            if claims, ok := token.Claims.(jwt.MapClaims); ok {
                c.Set("claims", claims)
            }
        }
        c.Next()
    }
}

app.Router.Use(jwt.JwtAuthorize(), Claims())
```

### 2.3 签发

```go
import "github.com/maczh/mgin/v2/pkg/utils"

token, err := utils.GenerateToken(jwt.MapClaims{
    "userId": 1001,
    "roleId": 2,
    "exp":    time.Now().Add(2 * time.Hour).Unix(),
})
```

`GenerateToken` 默认 HS256、`exp=+24h`（若 claims 未带 exp）。生产建议：

- 短有效期 access token + refresh token；
- `secret` 从密钥管理/环境注入，不要提交仓库；
- 校验 `iss/aud`（框架不校验）。

---

## 3. Casbin

### 3.1 模型

`casbin.conf`：

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act
```

**重要**：matcher 没有使用 `g(...)`，因此这不是 RBAC 继承，而是
“主体 ID 直连路径”的平铺 ACL。`g` 定义不参与鉴权。

### 3.2 中间件行为

```go
app.Router.Use(casbin.CasbinHandler())
```

1. `go.casbin.enabled=false` → 全部放行；
2. `Casbin.UnAuthPath` 白名单（路径 + 方法完全匹配）放行；
3. Swagger 路径放行；
4. 从 `c.Get("claims")` 取 `userId`、`roleId`（必须是 `jwt.MapClaims`，且为数值）；
5. 分别以 userId、roleId 调 `Enforce(sub, path, method)`；
6. 任一通过 → 放行；否则 403（JSON：`models.Error(403, "无访问权限")`）。

注意：

- 缺 `claims` → 401；
- 每次请求都 `LoadPolicy()`，策略多时有性能开销；
- 路径用 `keyMatch2`，`/api/v1/users/:id` 可匹配 `/api/v1/users/123`。

### 3.3 策略维护

```go
import "github.com/maczh/mgin/v2/pkg/casbin"

// 查询角色策略
infos, _ := casbin.Casbin.GetPolicyPathByRoleId(2)

// 覆盖式更新
_ = casbin.Casbin.UpdateCasbin(2, []casbin.CasbinInfo{
    {Path: "/api/v1/orders", Method: "GET"},
    {Path: "/api/v1/orders/:id", Method: "GET"},
})

// 接口路径变更时同步
_ = casbin.Casbin.UpdateCasbinApi("/api/v1/old", "GET", "/api/v1/new", "GET")
```

策略存于 `casbin_rule` 表（gorm-adapter 自动建表）。

### 3.4 建议

| 建议 | 理由 |
|------|------|
| 用 `sys_api` 维护接口清单，再同步策略 | 避免路径漂移 |
| 不要在 matcher 里直接信任 header | 可伪造 |
| 定期审计 `p` 策略 | 删除接口后策略残留会导致越权 |
| 高并发场景缓存 Enforce 结果 | 每请求 LoadPolicy 有成本 |

---

## 4. 会话

```go
app.Router.Use(session.New())
```

| API | 说明 |
|-----|------|
| `session.New(opt ...session.Option)` | 默认配置 |
| `session.NewWithConfig(cfg, opt...)` | 自定义 StoreKey/ManageKey/Skipper/错误处理 |
| `session.FromContext(c)` | 取当前会话 |
| `session.Refresh(c)` | 刷新有效期 |
| `session.Destroy(c)` | 销毁 |

默认基于 cookie + 内存/文件存储（go-session 默认），多副本需换 Redis 等共享存储，
否则会话不共享。

---

## 5. 输入防护（XSS）

```go
xssMw := &xss.XssMw{}
app.Router.Use(xssMw.RemoveXss())
```

- 对 JSON、form、multipart、query 做 bluemonday 净化；
- 会修改请求体，下游读到的是净化后内容；
- 不适合需要保留原始内容的场景（如富文本、代码提交）——按路由挂载，不要全局。

---

## 6. IP 与并发

```go
// 只允许内网访问管理接口
admin := app.Router.Group("/admin", iplimit.CIDR("10.0.0.0/8,127.0.0.1/32"))

// 限制重资源接口并发
export := app.Router.Group("/export", limit.MaxAllowed(2))
```

- `iplimit.CIDR` 是**白名单**：不在列表 → 403；
- 代理后需设置 `iplimit.TrustedHeaderField = "X-Real-IP"`，并确保该头由可信代理覆盖；
- `limit.MaxAllowed` 达到上限会阻塞，需配合超时/限流。

---

## 7. 传输安全

```yaml
go:
  application:
    port_ssl: 8443
    cert: certs/server.crt
    key: certs/server.key
```

- 证书路径相对可执行文件目录；
- 配置后同时监听 HTTP 与 HTTPS（不是只开 HTTPS）；
- 不支持 mTLS、证书热更新；需要时在入口层（网关/Mesh）做。

---

## 8. CORS

```go
app.Router.Use(cors.Cors("Content-Type", "Authorization", "X-Request-ID"))
```

默认行为：

- `Access-Control-Allow-Origin: *`；
- `Allow-Credentials: false`；
- 允许方法 `POST/GET/OPTIONS/PUT/DELETE/PATCH/HEAD/UPDATE`；
- 额外头来自 `go.cors.headers` 或参数。

带 Cookie 的跨域需要显式回显 Origin 并允许 credentials，当前中间件不支持——
请在网关处理。

---

## 9. 安全检查清单

- [ ] `go.jwt.secret` 来自密钥管理，长度 ≥ 32 字节
- [ ] JWT 之后有中间件写入 `claims`（否则 Casbin 永远 401）
- [ ] Casbin 策略与接口清单一致，无残留旧路径
- [ ] 管理接口有 IP 白名单或独立鉴权
- [ ] 登录/短信/导出接口有限流
- [ ] 生产关闭 Swagger 或加鉴权
- [ ] `go.log.get` 不记录密码/Token
- [ ] 错误响应不泄露 SQL/堆栈
- [ ] 多副本会话使用共享存储
- [ ] 反向代理场景正确设置可信 IP 头
