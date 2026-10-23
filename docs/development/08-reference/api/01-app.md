# 应用入口与插件

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 根包 `mgin` 负责装配；`pkg/plugin` 提供统一组件生命周期。

## `github.com/maczh/mgin/v2`

```go
import "github.com/maczh/mgin/v2"
```

### 类型

#### `App`

```go
type App struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Router` | *gin.Engine |  |  |
| `MGin` | *mgin |  |  |

**方法**

```go
func (app *App) EnableHealth()
```

EnableHealth 显式启用健康检查探针，等价于配置 go.health.enabled=true。

建议在挂载 casbin / jwt 等鉴权中间件之前调用。由于 NewApp() 中 baseRouter() 已执行，
此时挂载的探针会带上 baseRouter() 中已注册的无害中间件（trace / postlog / cors / recovery），
但不会带上本方法之后才注册的中间件；若需要完全干净的调用链，请直接使用 go.health.enabled 配置。
重复调用或与配置开关同时使用都是安全的，探针路由只会挂载一次。

```go
func (app *App) EnableMetrics()
```

EnableMetrics 显式启用 Prometheus /metrics 端点，等价于配置 go.metrics.enabled=true。
建议在 NewApp() 之后立即调用，确保端点先于业务路由注册。

```go
func (app *App) MarkHealthStarted()
```

MarkHealthStarted 标记应用已完成启动，使 /startup 探针返回 200。
等价于直接调用 health.MarkStarted()。

```go
func (app *App) Run()
```

Run 方法用于启动 HTTP 和 HTTPS 服务器，并监听系统信号以实现优雅关闭。

```go
func (app *App) GetVersion() string
```

#### `MginPlugin`

```go
type MginPlugin interface
```

MginPlugin 是 v1 时代暴露的"组件适配接口"，保留以兼容存量项目。
业务也可改为直接实现 plugin.Plugin 接口（新接口），用 plugin.Register 注册。

| 方法/内嵌 | 签名 |
|-----------|------|
| `Init` | func(configData []byte) |
| `Close` | func() |
| `Check` | func() error |

### 函数

#### `Init`

```go
func Init(configFile string)
```

Init 是框架统一初始化入口。v2 行为：
1. 加载配置（application.yml / nacos / consul / etcd / polaris / springconfig）
2. 创建全局 registry.Registry 实例（保留 v1 行为，让 client.Call 可用）
3. 注册全部内置组件到 plugin.PluginRegistry
4. 按 Order 升序调用 plugin.InitAll，依次初始化内置组件
5. 兼容旧 initPlugins：把 s3 等组件用 m.UsePlugin 纳入旧生命周期

#### `NewApp`

```go
func NewApp(configFile, appName, version string, xlang bool) *App
```

NewApp 创建一个新的 MGin App 实例。
参数:
- configFile: 配置文件的路径。如果为空，将尝试从命令行参数获取。
- appName: 应用程序的名称。
- version: 应用程序的版本号。
- xlang: 是否启用国际化支持。

返回值:
- *App: 新创建的 App 实例指针。

## `github.com/maczh/mgin/v2/pkg/plugin`

```go
import "github.com/maczh/mgin/v2/pkg/plugin"
```

### 类型

#### `Plugin`

```go
type Plugin interface
```

Plugin 是 mgin v2 的统一组件契约。

字段/方法语义：
- Name：唯一名称，如 "mysql" "redis" "kafka" "nacos" "job" "s3"。
- Order：启动顺序，数值越小越先 Init；Close 时按逆序执行。
- Init：框架启动时调用，完成组件连接/初始化；失败返回 error。
- Close：框架退出时调用，释放资源；失败返回 error（不应 panic）。
- Health：供 /health/ready 自报健康；无依赖可返回 nil。
- Enabled：是否由配置启用（通常读 go.config.used）。

| 方法/内嵌 | 签名 |
|-----------|------|
| `Name` | func() string |
| `Order` | func() int |
| `Init` | func(ctx context.Context) error |
| `Close` | func(ctx context.Context) error |
| `Health` | func() error |
| `Enabled` | func() bool |

### 函数

#### `All`

```go
func All() []Plugin
```

All 返回全部已注册插件（按注册顺序）。

#### `CloseAll`

```go
func CloseAll(ctx context.Context) error
```

CloseAll 按 Order 逆序对 Enabled()==true 的插件调用 Close。
设计要点（防漏关）：
- 单插件 Close 返回 error 仅记入首个错误并继续关闭其余插件；
- 单插件 Close 若 panic，由 defer+recover 兜底，保证不会中断整条关闭链。

#### `GetByName`

```go
func GetByName(name string) Plugin
```

GetByName 按名称获取已注册插件，未找到返回 nil。

#### `HealthAll`

```go
func HealthAll() map[string]error
```

HealthAll 汇总各 Enabled()==true 插件的 Health() 结果，供 /health/ready 使用。

#### `InitAll`

```go
func InitAll(ctx context.Context) error
```

InitAll 按 Order 升序对 Enabled()==true 的插件调用 Init。
任一插件 Init 失败仅记录首个错误并继续，不会中断其余插件的初始化
（与旧 mgin.Init 的 if 链“不中断”语义一致）。

#### `Job`

```go
func Job() Plugin
```

#### `Mongodb`

```go
func Mongodb() Plugin
```

#### `Mysql`

```go
func Mysql() Plugin
```

#### `Redis`

```go
func Redis() Plugin
```

#### `Register`

```go
func Register(p Plugin)
```

Register 注册一个插件。按 Name 去重：同名插件后注册者覆盖先注册者，
但注册顺序（order 切片）仅在首次出现时记录。

#### `RegisterBuiltins`

```go
func RegisterBuiltins()
```

RegisterBuiltins 注册全部内置插件。在 mgin.Init 中、配置加载之后调用，
之后由 plugin.InitAll 统一驱动初始化。

#### `Registry`

```go
func Registry() Plugin
```

---- 注册中心 适配 ----
仅处理旧 mgin.Init 中实际注册的三类（nacos/etcd/consul），与既有行为保持一致。

#### `Sqlite`

```go
func Sqlite() Plugin
```

#### `Unregister`

```go
func Unregister(name string)
```

Unregister 按名称注销插件。

