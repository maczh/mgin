# 服务鉴权与安全

## 1. 认证：JWT

```go
app.Router.Use(jwt.JwtAuthorize())
```

| 项 | 行为 |
|----|------|
| Token 位置 | `Authorization`，支持 `Bearer ` 前缀 |
| 算法 | 仅接受 HMAC 家族（`*jwt.SigningMethodHMAC`） |
| 密钥 | `go.jwt.secret`；为空时所有请求 401 |
| 跳过路径 | `/docs/`、`/swagger/`、`go.sys.swagger.uri` |
| 成功 | `c.Set("claims", MapClaims)` |
| 失败 | `AbortWithStatus(401)`，**无响应体** |

生成 token（工具函数，HS256，24h）：

```go
token, err := utils.GenerateToken(map[string]any{
	"userId": 1001,
	"roleId": 2,
}, []byte(secret))
```

> 依赖 `dgrijalva/jwt-go`（已停止维护）。新系统若可自选，建议在业务层使用 `golang-jwt/jwt`，仅把框架中间件用于兼容存量。

## 2. 授权：Casbin

```go
app.Router.Use(jwt.JwtAuthorize())     // 先
app.Router.Use(casbin.CasbinHandler()) // 后
```

工作方式：

1. 从 `claims` 取 `userId` 与 `roleId`（兼容 float64/string）。
2. 每次请求 `LoadPolicy()`，分别用 userId、roleId 执行 `Enforce(path, method)`。
3. 任一通过即放行；都失败返回 403。
4. 白名单：`casbin.Casbin.UnAuthPath` 与 swagger 路径。

配置：

```yaml
go:
  casbin:
    enabled: true
    model_file: casbin.conf   # 相对配置目录；仓库只提供这个文件
```

模型（`casbin.conf`）：`keyMatch2(r.obj, p.obj)`，策略表 `casbin_rule`（gorm-adapter 自动建表）。

策略维护 API：

```go
casbin.Casbin.UpdateCasbin(roleID, []casbin.CasbinInfo{{Path: "/api/v1/users", Method: "GET"}})
casbin.Casbin.ClearCasbin(0, "role:2")
casbin.Casbin.FreshCasbin()
```

**约束**：Enforcer 使用 `db.Mysql.GetConnection()`（不带库名），多库模式下不可用；每次请求加载策略有性能开销，高并发场景应做缓存或改为定时刷新。

## 3. 传输安全

- HTTPS：`go.application.cert/key` + `port_ssl`，HTTP 与 HTTPS 同时监听。
- 框架不做 mTLS；需要时由网关/Service Mesh 终止。
- 重定向：不自动把 HTTP 跳 HTTPS，需在网关层处理。

## 4. 输入安全

### XSS

```go
xssMw := &xss.XssMw{FieldsToSkip: []string{"content"}} // password 自动跳过
app.Router.Use(xssMw.RemoveXss())
```

- 策略：`StrictPolicy`（默认）/`UGCPolicy`。
- 过滤范围：GET 的 query；POST/PUT 的 JSON、form、multipart 字段；**不处理请求头**。
- 富文本场景只对指定字段用 `UGCPolicy`，不要全局放开。

### IP 限制

```go
app.Router.Use(iplimit.CIDR("10.0.0.0/8,192.168.0.0/16"))
```

不在网段返回 403；CIDR 非法返回 500。`iplimit.TrustedHeaderField` 可指定从哪个头取真实 IP，但该头必须由可信代理写入。

### SQL 注入

`utils.CheckSqlValidate` 是关键字黑名单，**不能**作为注入防护。正确做法：

- 全部使用参数绑定（`Where("id = ?", id)`）；
- 动态排序字段使用白名单映射，不拼接用户输入；
- 最小权限数据库账号。

## 5. 敏感数据

| 数据 | 要求 |
|------|------|
| 配置中的密码/token | 不提交仓库；用 Secret/配置中心加密字段 |
| JWT secret | 长度 ≥ 32，随机生成，按环境隔离 |
| 日志 | 不打印 DSN、密码、完整 token、身份证/手机号 |
| 错误响应 | 只返回 messageId 与业务码，不返回堆栈/SQL |
| 缓存 key | 不要拼入密码、token |

## 6. CORS

```yaml
go:
  cors:
    headers: "X-Tenant-Id"
```

- 允许方法固定：`POST/GET/OPTIONS/PUT/DELETE/PATCH/HEAD/UPDATE`。
- 有 `Origin` 时回显来源并允许凭证；无 `Origin` 时返回 `*` 且不带凭证。
- 预检直接 204，不进入业务链。

## 7. 配置重载接口

`PUT /cfg/reload` 的安全约束见 [配置刷新](../04-configuration/03-refresh-and-diagnostics.md#22-安全约束)：内网来源 + 等长 token 常量时间比较。生产必须设置强随机 token，且不要暴露到公网。

## 8. 安全上线检查

- [ ] `go.jwt.secret` 已配置且非默认值
- [ ] Casbin 模型文件存在，策略已初始化
- [ ] 公开接口（`/i18n`、`/job`、`/metrics`）已评估是否需要鉴权
- [ ] XSS 策略与跳过字段经过确认
- [ ] 反向代理注入的 `X-Forwarded-For` 只在可信链路使用
- [ ] 日志与错误响应无敏感信息
- [ ] 依赖漏洞扫描（尤其 `dgrijalva/jwt-go`）
