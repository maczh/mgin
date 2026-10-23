# 核心抽象

框架里需要长期记住的抽象只有 8 个。理解它们的位置，比记住函数名更重要。

---

## 1. 总览

| 抽象 | 接口/类型 | 解决的问题 | 扩展方式 |
|------|-----------|-----------|----------|
| 应用 | `mgin.App` | 装配路由、中间件、生命周期 | 持有 `Router` 自由扩展 |
| 插件 | `plugin.Plugin` | 组件初始化/关闭/健康检查 | `plugin.Register` |
| 配置 | `config.Config` | 多来源、点分键读取 | 无需扩展，按约定读键 |
| 注册中心 | `registry.RegistryClient` | 服务注册与发现 | `registry.Registry = 自定义实现` |
| 负载均衡 | `loadbalancer.LoadBalancer` | 从实例列表选一个 | `loadbalancer.Register` |
| 数据访问 | `dao.MySQLDao[E]` | 泛型 CRUD、事务、缓存 | `WithCache`/`WithTx` 组合 |
| 中间件 | `gin.HandlerFunc` | 横切逻辑 | 自行 `Use` |
| 错误定义 | `errcode.Definition` | 业务码 → HTTP 状态 → i18n 文案 | `RegisterDef` + 字典 |

---

## 2. 应用（App）

```go
app := mgin.NewApp(cfg, "order", "1.0.0", false)
app.Router.GET("/api/v1/orders", handler)   // 原生 gin，无额外抽象
app.Run()
```

职责边界：

- `NewApp` 完成配置、日志、注册中心实例、内置插件、基础中间件、内置路由的装配；
- 业务路由、业务中间件、业务后台任务由 `main` 自行注册；
- 框架**不扫描包、不做依赖注入**，没有“隐式注册”。

生命周期钩子：

| 方法/配置 | 时机 |
|-----------|------|
| `EnableHealth()` | 立即挂载探针（建议紧跟 `NewApp`） |
| `EnableMetrics()` | 立即挂载 `/metrics` |
| `MarkHealthStarted()` / `health.MarkStarted()` | 业务初始化完成后，让 startup 探针转 200 |
| `go.health.autoStarted` | `Run()` 启动监听后自动标记 |

---

## 3. 插件（Plugin）

```go
type Plugin interface {
    Name() string
    Order() int
    Init(ctx context.Context) error
    Close(ctx context.Context) error
    Health() error
    Enabled() bool
}
```

- `Order` 决定初始化顺序（升序）与关闭顺序（降序）。
- `Enabled` 是**惰性判定**：每次 `InitAll` 时求值，可读配置。
- `Init` 失败不返回给进程，只记录日志 —— 设计目标是“缺一个组件也能起服务”。
- `Health` 的返回值不会进入 `/health/ready`，只进 5 分钟巡检日志。

> 何时用插件：组件有连接、需要关闭、需要按配置启用。纯函数工具不要做成插件。

---

## 4. 注册中心与负载均衡

```go
type RegistryClient interface {
    Register(config []byte)
    GetServiceURL(service string, groups ...string) (url string, group string)
    GetServices(service string, groups ...string) ([]string, error)
    DeRegister()
}
```

- `Registry` 是**全局单例**，通过 `registry.NewRegistry()` 按配置创建，外层自动包一层
  `CachedRegistry`（TTL + 后台刷新 + 并发回源抑制）。
- `GetServiceURL` 返回一个地址（兼容 v1）；`GetServices` 返回全部实例（供负载均衡）。
- 后端差异：Nacos 过滤不健康实例、Consul passingOnly、etcd 按租约、Polaris 不过滤。

负载均衡：

```go
type LoadBalancer interface {
    Pick(instances []string, key string) (string, error)
}
```

- 注册名只有 4 个：`round_robin`、`random`、`least_connections`、`consistent_hash`。
- 调用时通过 `client.Options.LoadBalancer` 指定；未命中会**静默回落默认轮询**。
- `least_connections` 需要业务调用 `Inc` 维护计数，否则退化为选第一个。

详见 [05 · 服务治理](../05-governance/01-discovery-and-lb.md)。

---

## 5. 数据访问（DAO）

```go
var orderDao = &dao.MySQLDao[Order]{Tag: func() string { return "order_db" }}

list, err := orderDao.WithCache(dao.CacheConfig{Enabled: true, Store: cacheStore, TTL: time.Minute}).
    WhereAll("status = ?", 1)
```

三个正交维度：

| 维度 | 方法 | 说明 |
|------|------|------|
| 库 | `Tag` 字段 | 多库时选择连接；单库可省略 |
| 事务 | `WithTx(tx)` | 事务内禁用缓存 |
| 缓存 | `WithCache(cfg)` | 读缓存、写失效；非事务 |

DAO 不是 Repository 模式的完整实现：没有关联加载、没有软删除、没有钩子。
它只负责“把 SQL 与分页、缓存、事务组合起来”。

---

## 6. 中间件

中间件就是 `gin.HandlerFunc`，框架没有自定义抽象。需要记住的只有：

1. 会拦截请求的中间件（鉴权/限流）**不要**在探针注册之前 `Use`；
2. 全局中间件已在 `baseRouter` 中固定，业务中间件自行追加；
3. 请求级数据用 `c.Set(key, value)`，键名统一小写点分（如 `claims`、`lang`）。

---

## 7. 错误定义（Definition）

```go
type Definition struct {
    Code       int    // 业务错误码，如 1005
    HTTPStatus int    // HTTP 状态码，如 401
    MessageKey string // i18n 键，如 errcode.user_not_found
    Module     string // 归组，如 auth
}
```

三向映射是框架的错误处理核心：

```
业务代码/框架 ──► Definition ──► HTTPStatus（响应行）
                        └─────► MessageKey ──► i18n 文案（响应体）
```

- `New` 会把非法 HTTP 状态（0 或越界）修正为 500；
- `LookupDef` 未注册时返回 500 + `errcode.unknown`；
- 业务必须显式 `c.JSON(def.HTTPStatus, i18n.ErrorDef(def))`，框架不会代劳。

---

## 8. 响应模型

两套并存，别混用：

| 模型 | 字段 | 用途 |
|------|------|------|
| `models.Result[T]` | `status`(1 成功)/`msg`/`data`/`page` | v1 兼容、`i18n`、`client` 内部 |
| `vo.Response[T]` | `code`/`data`/`msg`/`srvTime` | 对外 HTTP API 推荐 |

`vo.SuccessPage` 的 `data` 是 `PageData{list,total,page,pageSize}`。
转换函数（`ToResponse`、`ResponseToT`）在类型不一致时会 panic，跨类型转换需保证 `Data` 实际类型。

---

## 9. 抽象之间的关系图

```
App
 ├─ Router ── Middleware[] ── Handler
 ├─ PluginRegistry
 │    ├─ DB ── DAO ── Cache
 │    ├─ Registry ── Client ── LoadBalancer ── Breaker
 │    └─ Job
 └─ (config / logs / i18n / errcode / metrics / otel) 全局单例
```

全局单例清单与默认值见 [08 · 变量参考](../08-reference/variables.md)。
