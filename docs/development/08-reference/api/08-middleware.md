# HTTP 中间件

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> Gin 中间件：追踪、日志、CORS、鉴权、限流、会话、XSS。

## `github.com/maczh/mgin/v2/pkg/middleware/trace`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/trace"
```

### 函数

#### `CopyPreHeaderToCurRoutine`

```go
func CopyPreHeaderToCurRoutine(preRoutineId uint64)
```

从其他协程克隆headers到当前协程的缓存

#### `GetClientIp`

```go
func GetClientIp() string
```

#### `GetGoroutineID`

```go
func GetGoroutineID() uint64
```

#### `GetHeader`

```go
func GetHeader(header string) string
```

#### `GetHeaders`

```go
func GetHeaders() map[string]string
```

#### `GetRequestId`

```go
func GetRequestId() string
```

#### `GetUserAgent`

```go
func GetUserAgent() string
```

#### `Headers`

```go
func Headers() gin.HandlerFunc
```

#### `PutRequestId`

```go
func PutRequestId(c *gin.Context)
```

#### `SetHeader`

```go
func SetHeader(key, value string)
```

#### `TraceId`

```go
func TraceId() gin.HandlerFunc
```

## `github.com/maczh/mgin/v2/pkg/middleware/postlog`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/postlog"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `MongoSinkName` | = "mongodb" | MongoSinkName 内置 MongoDB 落库 handler 的注册名。 |

### 函数

#### `Close`

```go
func Close()
```

Close 停止全部日志处理器并尽力排空队列，进程退出前调用（mgin.SafeExit 已内置调用）。

#### `RegisterHandler`

```go
func RegisterHandler(h logsink.Handler, opts ...logsink.Option) error
```

RegisterHandler 挂接一个接口日志处理器，等价于 logsink.Register。
外部插件（mgkafka、elasticsearch 插件等）或业务代码在应用启动时调用即可接收全量接口日志。

postlog.RegisterHandler(logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
return mgkafka.Kafka.Send(topic, utils.ToJSON(e))
}))

#### `RequestLogger`

```go
func RequestLogger() gin.HandlerFunc
```

## `github.com/maczh/mgin/v2/pkg/middleware/cors`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/cors"
```

### 函数

#### `Cors`

```go
func Cors(headers ...string) gin.HandlerFunc
```

## `github.com/maczh/mgin/v2/pkg/middleware/xlang`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/xlang"
```

### 函数

#### `GetCurrentLanguage`

```go
func GetCurrentLanguage() string
```

#### `RequestLanguage`

```go
func RequestLanguage() gin.HandlerFunc
```

## `github.com/maczh/mgin/v2/pkg/middleware/jwt`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/jwt"
```

### 函数

#### `JwtAuthorize`

```go
func JwtAuthorize() gin.HandlerFunc
```

JwtAuthorize JWT认证中间件

## `github.com/maczh/mgin/v2/pkg/middleware/casbin`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/casbin"
```

### 函数

#### `CasbinHandler`

```go
func CasbinHandler() gin.HandlerFunc
```

CasbinHandler 拦截器

## `github.com/maczh/mgin/v2/pkg/middleware/session`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/session"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `DefaultConfig` | = Config{ ErrorHandleFunc: func(ctx *gin.Context, err error) { ctx.AbortWithStatusJSON(http.StatusInternalServerError, models.Error(500, err.Error())) }, StoreKey: "github.com/go-session/gin-session/store", ManageKey: "github.com/go-session/gin-session/manage", Skipper: func(_ *gin.Context) bool { return false }, } | DefaultConfig is the default Recover middleware config. |

### 类型

#### `ErrorHandleFunc`

```go
type ErrorHandleFunc func(*gin.Context, error)
```

ErrorHandleFunc error handling function

#### `Config`

```go
type Config struct
```

Config defines the config for Session middleware

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ErrorHandleFunc` | ErrorHandleFunc |  | error handling when starting the session |
| `StoreKey` | string |  | keys stored in the context |
| `ManageKey` | string |  | keys stored in the context |
| `Skipper` | func(*gin.Context) bool |  | defines a function to skip middleware.Returning true skips processing the middleware. |

### 函数

#### `Destroy`

```go
func Destroy(ctx *gin.Context) error
```

Destroy a session

#### `FromContext`

```go
func FromContext(ctx *gin.Context) session.Store
```

FromContext Get session storage from context

#### `New`

```go
func New(opt ...session.Option) gin.HandlerFunc
```

New create a session middleware

#### `NewWithConfig`

```go
func NewWithConfig(config Config, opt ...session.Option) gin.HandlerFunc
```

NewWithConfig create a session middleware

#### `Refresh`

```go
func Refresh(ctx *gin.Context) (session.Store, error)
```

Refresh a session and return to session storage

## `github.com/maczh/mgin/v2/pkg/middleware/limit`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/limit"
```

### 函数

#### `MaxAllowed`

```go
func MaxAllowed(n int) gin.HandlerFunc
```

## `github.com/maczh/mgin/v2/pkg/middleware/iplimit`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/iplimit"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `DisableLogging` | bool = | DisableLogging set up logging. default is false (logging) |
| `TrustedHeaderField` | string = | TrustedHeaderField is a header field that developer trusted in their env. e.g. Upstream proxy server's special header that only server can setup need to avoid use common forgry-able header fields. |

### 函数

#### `CIDR`

```go
func CIDR(CIDRs string) gin.HandlerFunc
```

CIDR is a middleware that check given CIDR rules and return 403 Forbidden
when user is not coming from allowed source. CIDRs accepts a list of CIDRs,
separated by comma. (e.g. 127.0.0.1/32, ::1/128 )

## `github.com/maczh/mgin/v2/pkg/middleware/ratelimit`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/ratelimit"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `AlgoTokenBucket` | Algorithm = "token_bucket" | AlgoTokenBucket 令牌桶：允许突发流量，按固定速率补充令牌 |
| `AlgoSlidingWindow` | Algorithm = "sliding_window" | AlgoSlidingWindow 滑动窗口：精确统计窗口内请求数，无临界突刺问题 |
| `AlgoConcurrency` | Algorithm = "concurrency" | AlgoConcurrency 最大并发数：信号量控制同时在处理的请求数 |
| `DimGlobal` | Dimension = "global" | DimGlobal 全局共享一个限流器 |
| `DimIP` | Dimension = "ip" | DimIP 按客户端 IP 分别限流 |
| `DimPath` | Dimension = "path" | DimPath 按请求路径分别限流 |
| `DimIPPath` | Dimension = "ip_path" | DimIPPath 按 客户端IP + 请求路径 组合分别限流 |
| `DimHeader` | Dimension = "header" | DimHeader 按指定请求头的值分别限流（如 X-User-Id、Authorization） |

### 类型

#### `Algorithm`

```go
type Algorithm string
```

Algorithm 限流算法

#### `Dimension`

```go
type Dimension string
```

Dimension 限流维度，决定限流计数器的分组维度

#### `Rule`

```go
type Rule struct
```

Rule 一条限流规则。
全局规则由 go.ratelimit 下的一级配置项构成；
独立规则由 go.ratelimit.rules 数组构成，按声明顺序优先匹配，命中即用该规则，不再叠加全局规则。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Name` | string | koanf:"name" | Name 规则名，用于限流器 key 前缀与统计展示；为空时自动以路径生成 |
| `Path` | string | koanf:"path" | Path 匹配路径。支持三种写法： 精确匹配 /api/v1/user/info 前缀匹配 /api/v1/sms/* Gin 路由 /api/v1/user/:id 为空表示匹配所有路径 |
| `Methods` | []string | koanf:"methods" | Methods 匹配的 HTTP 方法，为空表示不限方法 |
| `Algorithm` | Algorithm | koanf:"algorithm" | Algorithm 限流算法，默认 token_bucket |
| `Dimension` | Dimension | koanf:"dimension" | Dimension 限流维度，默认 global |
| `HeaderKey` | string | koanf:"headerKey" | HeaderKey Dimension 为 header 时读取的请求头名称 |
| `Rate` | float64 | koanf:"rate" | Rate 令牌桶：每秒生成令牌数；滑动窗口：一个窗口内允许的请求数 |
| `Burst` | int | koanf:"burst" | Burst 令牌桶容量（可突发的最大请求数），为 0 时取 Rate 向上取整 |
| `Window` | int | koanf:"window" | Window 滑动窗口时长（秒），默认 1 |
| `WindowMs` | int | koanf:"windowMs" | WindowMs 滑动窗口时长（毫秒），优先于 Window |
| `MaxConcurrent` | int | koanf:"maxConcurrent" | MaxConcurrent concurrency 算法的最大并发请求数 |
| `Wait` | bool | koanf:"wait" | Wait 触发限流时是否排队等待而非立即拒绝 |
| `WaitTimeoutMs` | int | koanf:"waitTimeoutMs" | WaitTimeoutMs 排队等待的最长时间（毫秒），超时后拒绝 |
| `Code` | int | koanf:"code" | Code 被限流时返回的业务状态码，默认 errcode.TOO_MANY_REQUESTS |
| `Message` | string | koanf:"message" | Message 被限流时返回的提示信息 |
| `HttpStatus` | int | koanf:"httpStatus" | HttpStatus 被限流时的 HTTP 状态码，默认 200（框架统一用业务码表达错误） |

#### `Config`

```go
type Config struct
```

Config 限流中间件配置，对应 application.yml 中 go.ratelimit 节点

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Enabled` | bool | koanf:"enabled" | Enabled 是否启用限流 |
| `Whitelist` | []string | koanf:"whitelist" | Whitelist 白名单路径（前缀匹配），命中则完全跳过限流 |
| `WhiteIps` | []string | koanf:"whiteIps" | WhiteIps 白名单 IP，命中则完全跳过限流 |
| `IdleTimeout` | int | koanf:"idleTimeout" | IdleTimeout 空闲限流器回收时间（秒），默认 600 |
| `Headers` | bool | koanf:"headers" | Headers 是否在响应头中输出 X-RateLimit-* 信息 |
| `Rules` | []Rule | koanf:"rules" | Rules 独立规则列表 |

#### `Manager`

```go
type Manager struct
```

Manager 单例限流管理器。
全进程唯一，集中持有所有维度的限流器实例，并通过后台协程定期回收空闲限流器，
避免按 IP / 路径维度限流时限流器无限增长造成内存泄漏。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (m *Manager) Reload()
```

Reload 重新从配置中心/配置文件加载限流配置并重置所有限流器，用于配置热更新

```go
func (m *Manager) ReloadWith(cfg *Config)
```

ReloadWith 使用给定配置替换当前配置，并清空已有限流器

```go
func (m *Manager) Config() *Config
```

Config 返回当前生效的限流配置

```go
func (m *Manager) Stop()
```

Stop 停止回收协程，通常无需调用（进程退出即可）

```go
func (m *Manager) Count() int
```

Count 返回当前活跃限流器数量，可用于监控

```go
func (m *Manager) Handler() gin.HandlerFunc
```

Handler 生成该管理器对应的 gin 中间件

```go
func (m *Manager) Allow(key string, rule Rule) (bool, func())
```

Allow 提供非 HTTP 场景（如 MQ 消费、定时任务、RPC 调用）的编程式限流入口。
key 为自定义限流维度标识，rule 为限流规则。返回 true 表示放行。
使用 concurrency 算法时，务必在处理结束后调用返回的 release 函数归还槽位。

### 函数

#### `Allow`

```go
func Allow(key string, rule Rule) (bool, func())
```

Allow 单例管理器的编程式限流入口，等价于 GetManager().Allow(...)

#### `GetManager`

```go
func GetManager() *Manager
```

GetManager 获取单例限流管理器，首次调用时自动从 application.yml 加载配置并启动回收协程

#### `LoadConfig`

```go
func LoadConfig() *Config
```

LoadConfig 从 application.yml 的 go.ratelimit 节点读取限流配置。
未配置 go.ratelimit 节点时返回 Enabled=false 的配置，中间件将直接放行。

#### `LoadConfigFrom`

```go
func LoadConfigFrom(prefix string) *Config
```

LoadConfigFrom 从指定配置前缀读取限流配置，便于同一进程内挂载多组互不干扰的限流策略

#### `RateLimit`

```go
func RateLimit() gin.HandlerFunc
```

RateLimit 返回限流中间件，配置从 application.yml 的 go.ratelimit 节点读取。

app.Router.Use(ratelimit.RateLimit())

未配置或 enabled=false 时中间件直接放行，无任何性能开销。

#### `RateLimitWith`

```go
func RateLimitWith(rules ...Rule) gin.HandlerFunc
```

RateLimitWith 使用代码指定的规则构建限流中间件（不读取 yml），
适用于对某个路由组单独施加限流策略的场景：

api.Use(ratelimit.RateLimitWith(ratelimit.Rule{
Algorithm: ratelimit.AlgoConcurrency, MaxConcurrent: 10,
}))

## `github.com/maczh/mgin/v2/pkg/middleware/xss`

```go
import "github.com/maczh/mgin/v2/pkg/middleware/xss"
```

### 类型

#### `XssMw`

```go
type XssMw struct
```

Config struct for passing options

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `FieldsToSkip` | []string |  | List of fields to not filter. i.e. password, created_on, created_at, etc password is set to skip by the system i.e. FieldsToSkip = []string{"password", "cre_date"} |
| `BmPolicy` | string |  | Bluemonday comes with two default policies Two options StrictPolicy // the default UGCPolicy or you can specify you own policy define it somewhere in your package so that you can call it here see https://github.com/micro… |

**方法**

```go
func (mw *XssMw) RemoveXss() gin.HandlerFunc
```

XssMw implements the Gin Middleware interface.

```go
func (mw *XssMw) GetBlueMondayPolicy() *bluemonday.Policy
```

Get which Bluemonday policy

```go
func (mw *XssMw) XssRemove(c *gin.Context) error
```

Receives an http request object, processes the body, removing html and returns the request.

Headers (and other parts of the request) are passed through unaltered.

Request Method must be "POST" or "PUT"

```go
func (mw *XssMw) HandleGETRequest(c *gin.Context) error
```

HandleGETRequest handles get request

```go
func (mw *XssMw) HandleXFormEncoded(c *gin.Context) error
```

Handles Content-Type "application/x-www-form-urlencoded"

Has been tested with basic param=value form fields only:

comment=<img src=x onerror=alert(0)>
&cre_at=1481017167
&email=testUser@example.com
&flt=2.345
&id=2
&password=TestPass
&user=TestUser

has not been tested on file/data uploads

```go
func (mw *XssMw) HandleMultiPartFormData(c *gin.Context, ctHdr string) error
```

Handles Content-Type "multipart/form-data"

skips sanitizing if file upload

Content-Disposition: form-data; name="" filename=""

tries to determine Content-type for form data file upload, defaults
to application/octet-stream if unknown

handles basic form field POST request for example:

--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
Content-Disposition: form-data; name="flt"

2.345
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
Content-Disposition: form-data; name="user"

TestUser
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
Content-Disposition: form-data; name="email"

testUser@example.com
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
281         }
Content-Disposition: form-data; name="password"

!@$%^ASDF
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
Content-Disposition: form-data; name="comment"

&gt;&#39;&gt;\&#34;&gt;
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
Content-Disposition: form-data; name="cre_at"

1481017167
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909
Content-Disposition: form-data; name="id"

2
--3af5c5b7adcb2142f404a8e1ce280c47c58e563e3d4c1e172490737c9909--

NOTE: form-data name 'password' is skipped (not sanitized)

```go
func (mw *XssMw) HandleJson(c *gin.Context) error
```

Handles request Content-Type = application/json

The four types of data handled.

* 1st type filter - basic key:value - most common

map[string]interface {}{"updated_by":"534", "updated_at":"1480831130", "id":"1", "name":"foo"}

* 2nd type an id with associated ids list

map[string]interface {}{"project_id":"1", "talent_ids":[]interface {}{"1", "4", "8"}}

- NOTE changes from ["1", "4", "8"] to [1,4,8]

* 3rd type an "array of records"

[]interface {}{
map[string]interface {}{"name":"asd", "url":"/data/1/as",
"user_id":"537", "username":"Test User ©", "created_by":"537", "id":"286",
"fqdn":"audio class", "project_id":"1", "path":"/tmp/store/1/as",
"updated_at":"1480791630", "status":"NEW",
"updated_by":"537", "created_at":"1480450694"},
map[string]interface {}{"name":"asd2", "url":"/data/2/as", etc... },
map[string]interface {}{"name":"asd3", "url":"/data/3/as", etc... },
...
}

* 4th type "complex array/nested records"

map[string]interface {}{
"id":"1",
"users":[]interface {}{
map[string]interface {}{"id":"1", "flt":"1.345", "user":"TestUser1", "email":"testUser1@example.com", "password":"!@$%^ASDF<html>1", "comment":"<img src=x onerror=alert(0)>", "cre_at":"1481017167"},
map[string]interface {}{"cre_at":"1481017167", "id":"2", "flt":"2.345", "user":"TestUser2", "email":"testUser2@example.com", "password":"!@$%^ASDF<html>2", "comment":"<img src=x onerror=alert(0)>"}
}
}

```go
func (mw *XssMw) SetRequestBodyJson(c *gin.Context, buff bytes.Buffer) error
```

encode processed body back to json and re-set http request body

```go
func (mw *XssMw) ConstructJson(xmj XssMwJson, buff bytes.Buffer) bytes.Buffer
```

De-constructs the http request body
removes undesirable content
keeps the good content to construct
returns the cleaned http request
Map to Bytes (struct to json string...)

#### `XssMwJson`

```go
type XssMwJson map[string]interface{}
```

