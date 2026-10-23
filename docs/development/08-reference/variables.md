# 全局变量与单例参考

> 本文列出**包级导出变量**与框架内部全局单例。每个条目给出类型、默认值、作用与约束。
> 函数/方法签名见 [API 参考](api/01-app.md)，配置键见 [配置项参考](configuration.md)。

---

## 1. 配置

### `config.Config`

| 项 | 值 |
|----|----|
| 类型 | `*config`（未导出结构体指针） |
| 默认值 | `&config{}`，`Init` 前所有字段为零值 |
| 作用 | 全局配置快照；所有 `GetConfig*` 与组件读取的唯一入口 |
| 约束 | 只在 `NewApp`/`mgin.Init` 中初始化一次；并发只读安全；启动后修改 `Cnf` 不保证生效 |

```go
port := config.Config.GetConfigInt("go.application.port")
cfg := config.Config.App   // 结构体字段可直接读
```
---

## 2. 数据源

| 变量 | 类型 | 默认值 | 作用 | 约束 |
|------|------|--------|------|------|
| `db.Mysql` | `*mysql.MysqlClient` | 零值客户端 | MySQL 门面 | 未启用/未初始化时 `GetConnection` 报错 |
| `db.Mongo` | `*mongo.Mongodb` | 零值 | MongoDB 门面 | 连接必须 `ReturnConnection` |
| `db.Redis` | `*redis.RedisClient` | 同 `redis.Redis` | Redis 门面 | 多库必须指定库名 |
| `db.Sqlite` | `*sqlite.Sqlite` | 零值 | SQLite 门面 | 需 CGO |
| `redis.Redis` | `*redis.RedisClient` | `&RedisClient{}` | `pkg/db/redis` 自己的单例 | 与 `db.Redis` 指向同一对象 |

```go
conn, err := db.Mysql.GetConnection()
if err != nil { /* 未启用或连接失败 */ }
```

---

## 3. 注册与发现

### `registry.Registry`

| 项 | 值 |
|----|----|
| 类型 | `registry.RegistryClient`（接口） |
| 默认值 | `nil`，`mgin.Init` 中由 `NewRegistry()` 赋值 |
| 作用 | 全局注册中心客户端；`client` 与 `postlog` 等通过它取地址 |
| 约束 | 单例，只能有一个后端；测试中替换后需恢复 |

```go
urls, err := registry.Registry.GetServices("order")
```

---

## 4. 负载均衡

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `loadbalancer.RoundRobinLB` | `*RoundRobin` | 已注册 `round_robin` | 原子计数轮询 |
| `loadbalancer.RandomLB` | `*Random` | 已注册 `random` | 随机 |
| `loadbalancer.LeastConnectionsLB` | `*LeastConnections` | 已注册 `least_connections` | 需 `Inc` 维护计数 |
| `loadbalancer.ConsistentHashLB` | `*ConsistentHash` | 已注册 `consistent_hash`，`replicas=160`（未使用） | FNV-1a 取模 |
| `loadbalancer.ErrEmptyInstances` | `error` | `"实例列表为空"` | `Pick` 空列表返回 |

```go
host, err := loadbalancer.RoundRobinLB.Pick([]string{"a:80", "b:80"}, "")
```

约束：`Default()` 初始为轮询，可用 `SetDefault` 改；策略名必须用注册名。

---

## 5. 调用客户端

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `client.ErrAllInstancesCircuitOpen` | `error` | `"client: all instances circuit-open"` | 全部实例熔断 |
| `client.ErrCircuitOpen` | `error` | 中文描述 | 韧性层熔断打开 |
| `client.ErrCallTimeout` | `error` | 中文描述 | 韧性层超时 |
| `client.CONTENT_TYPE_FORM` | `string` | `"x-form"` | 表单协议 |
| `client.CONTENT_TYPE_JSON` | `string` | `"json"` | JSON 协议 |
| `client.CONTENT_TYPE_RESTFUL` | `string` | `"restful"` | 路径参数协议 |
| `client.CONTENT_TYPE_FILE` | `string` | `"file"` | 文件上传 |

```go
if errors.Is(err, client.ErrAllInstancesCircuitOpen) {
    return fallback()
}
```

---

## 6. 日志

| 变量 | 类型 | 默认值 | 说明 | 约束 |
|------|------|--------|------|------|
| `logs.NoColor` | `bool` | `!isatty(stdout)` | 控制台是否禁用颜色 | 全局，影响所有输出 |
| `logs.Output` | `*ansicolor.AnsiColorWriter` | 包装 `os.Stdout` | 控制台写出目标 | 可替换为自定义 writer（测试） |

```go
logs.NoColor = true           // 容器/CI 环境
logs.Output = ansicolor.NewAnsiColorWriter(os.Stderr)
```

---

## 7. 日志投递

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `logsink.ErrDuplicateHandler` | `error` | `"同名 handler 已注册"` | `Register` 重名 |
| `logsink.ErrInvalidHandler` | `error` | `"handler 不能为空"` | 非法 Handler |

```go
if err := logsink.Register(h); errors.Is(err, logsink.ErrDuplicateHandler) { ... }
```

---

## 8. 国际化

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `i18n.ErrNotInitialized` | `error` | 字典未初始化 | 未调用 `Init`/无连接 |
| `i18n.ErrInvalidEntry` | `error` | 条目非法 | messageId/语言/文本不合法 |
| `i18n.ErrNotFound` | `error` | 条目不存在 | CRUD 查询未命中 |
| `i18n.ErrConflict` | `error` | 主键冲突 | 重复创建 |

```go
if errors.Is(err, i18n.ErrNotFound) { /* 回退默认文案 */ }
```

---

## 9. 鉴权

### `casbin.Casbin`

| 项 | 值 |
|----|----|
| 类型 | `*casbinService`（未导出） |
| 默认值 | `&casbinService{UnAuthPath: []}` |
| 字段 | `UnAuthPath []CasbinInfo`（可写，白名单） |
| 作用 | 全局 Enforcer 与策略维护 |
| 约束 | 需先 `Init`（插件/中间件触发）；每次 Enforce 前 `LoadPolicy` |

```go
casbin.Casbin.UnAuthPath = append(casbin.Casbin.UnAuthPath,
    casbin.CasbinInfo{Path: "/api/v1/login", Method: "POST"})
```

---

## 10. 中间件

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `iplimit.DisableLogging` | `bool` | `false` | 是否关闭“IP 不允许”日志 |
| `iplimit.TrustedHeaderField` | `string` | `""` | 从哪个头取真实 IP（如 `X-Real-IP`） |
| `session.DefaultConfig` | `session.Config` | StoreKey/ManageKey/Skipper 已填 | 会话默认配置 |

```go
iplimit.TrustedHeaderField = "X-Real-IP"
app.Router.Use(iplimit.CIDR("10.0.0.0/8"))
```

> 设置 `TrustedHeaderField` 前必须确保该头由可信代理覆盖，否则可被伪造绕过白名单。

---

## 11. 工具库

| 变量 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `utils.RSA` | `*RSASecurity` | `&RSASecurity{}` | RSA 加解密辅助对象 |
| `utils.LimitQueue` | `*LimitQueeMap` | 空 map | 本地频率限制队列（单机） |
| `utils.UUIDSnippet` | `string` | UUID 正则片段 | 用于自定义校验 |
| `utils.ErrDataToLarge` 等 | `error` | RSA 相关错误 | `errors.Is` 判断 |

```go
enc, err := utils.RSA.PublicEncrypt(pub, []byte("secret"))
```

---

## 12. 无导出全局状态（需要知道但无法直接访问）

| 状态 | 位置 | 访问方式 |
|------|------|----------|
| 插件注册表 | `pkg/plugin` | `Register/All/GetByName` |
| 负载均衡注册表 | `pkg/loadbalancer` | `Register/Get/SetDefault` |
| 熔断器表 | `pkg/client` | `GetBreaker/ResetBreakers` |
| 指标注册表 | `pkg/metrics` | `SetBuildInfo/SetPluginHealth/SetDependencyUp` |
| 日志 Handler 表 | `pkg/logsink` | `Register/Emit/Stats/Close` |
| 请求头协程缓存 | `pkg/middleware/trace` | `GetHeaders/SetHeader/CopyPreHeaderToCurRoutine` |
| job 管理器 | `pkg/job` | `GetManager/Start/Stop` |
| i18n Store | `pkg/i18n` | `GetStore/Init/Close` |

这些状态是**进程级**的：测试之间会互相影响，多副本之间不共享。
