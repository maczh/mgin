# 参考手册

参考手册只回答事实性问题：**有什么、签名是什么、默认值是多少、异常是什么**。
“为什么这样设计”在 [02 · 概念](../02-concepts/01-architecture.md)，
“怎么做”在 [03](../03-development/02-service-development.md)/[05](../05-governance/01-discovery-and-lb.md)/[06](../06-observability/01-metrics.md)。

---

## 1. 索引

| 文档 | 回答的问题 |
|------|-----------|
| [api/](api/) | 每个包有哪些导出常量/变量/类型/字段/方法/函数，签名与注释 |
| [variables.md](variables.md) | 全局变量与单例：类型、默认值、作用、约束 |
| [configuration.md](configuration.md) | 每个配置键：类型、默认值、是否被读取 |
| [types.md](types.md) | 核心类型的关系、嵌套、零值语义、并发安全 |
| [protocols.md](protocols.md) | HTTP 响应、错误体、调用协议、头部约定 |
| [cli.md](cli.md) | `mgin` 脚手架命令与参数 |
| [compatibility.md](compatibility.md) | 版本、依赖、后端兼容与破坏性变更 |
| [examples.md](examples.md) | 每类能力的可运行示例与异常语义 |
| [../examples/](../examples/) | 可直接 `go run` 的示例工程 |

---

## 2. API 参考的组织方式

`api/` 下 13 个文件按领域划分：

| 文件 | 覆盖包 |
|------|--------|
| `01-app.md` | 根包 `mgin`、`pkg/plugin` |
| `02-config.md` | `pkg/config` |
| `03-db.md` | `pkg/db`、`mysql`、`sqlite`、`mongo`、`redis`、`dao` |
| `04-cache.md` | `pkg/cache` |
| `05-registry.md` | `pkg/registry` 及四种后端 |
| `06-client.md` | `pkg/client`、`pkg/loadbalancer` |
| `07-observability.md` | `health`、`metrics`、`otel`、`logs`、`logsink` |
| `08-middleware.md` | 11 个中间件包 |
| `09-data-models.md` | `models`、`models/sys`、`request`、`sys/vo`、`vo` |
| `10-i18n-errcode.md` | `pkg/i18n`、`pkg/errcode` |
| `11-job.md` | `pkg/job` |
| `12-casbin.md` | `pkg/casbin` |
| `13-utils.md` | `pkg/utils`（273 个导出符号） |

### 2.1 生成规则

这些文件由 AST 脚本生成，保证：

- 只包含**导出**符号（未导出类型/字段/方法已过滤）；
- 签名、字段类型、struct tag 与源码逐字一致；
- 代码注释原样保留；
- 方法挂在所属类型下，函数按字母序排列。

因此它们**不会**包含：

- 参数的业务语义（多数写在注释里）；
- 错误列表（见 [examples.md](examples.md) 与各主题章节）；
- 示例代码（见 [examples.md](examples.md)）。

### 2.2 阅读示例

```
#### `CallCtx`
func CallCtx(ctx context.Context, service, uri string, op *Options) (string, error)
CallCtx 是 v2 新增的"支持 context"版本微服务调用入口。...
```

含义：

- `####` 是符号名，`go` 块是完整签名；
- 下方是源码注释；
- `Options`/`ResilienceOptions` 等类型的字段在同文件靠前的“类型”小节。

---

## 3. 符号覆盖统计（当前代码）

| 包 | 导出符号数（go doc 行数） |
|----|---------------------------|
| `pkg/utils` | 273 |
| `pkg/logs` | 46 |
| `pkg/logsink` | 36 |
| `pkg/job` | 21 |
| `pkg/client` | 19 |
| `pkg/plugin` | 15 |
| `pkg/vo` | 15 |
| `pkg/loadbalancer` | 14 |
| `pkg/models/sys` | 14 |
| `pkg/middleware/ratelimit` | 13 |
| `pkg/models` | 12 |
| 其余 | 见各文件 |

合计约 650 个导出符号，全部收录。

---

## 4. 约定

### 4.1 零值语义

框架的公开类型大多遵循“零值可用”：

- `Options{}`、`ResilienceOptions{}` 表示不使用额外能力；
- `CacheConfig{}` 表示不启用缓存；
- `Definition{}` 不合法，必须用 `errcode.New` 构造；
- `Result[T]{}` 的 `Status=0` 不是成功（成功是 1）。

### 4.2 错误判断

优先用 `errors.Is`：

```go
errors.Is(err, client.ErrAllInstancesCircuitOpen)
errors.Is(err, client.ErrCircuitOpen)
errors.Is(err, client.ErrCallTimeout)
errors.Is(err, i18n.ErrNotFound)
errors.Is(err, logsink.ErrDuplicateHandler)
```

`client.Call` 对下游 4xx/5xx **不返回 error**，必须解析响应体。

### 4.3 并发安全

| 类型/函数 | 并发安全 |
|-----------|----------|
| `config.Config` 读 | ✅ |
| `client.GetBreaker` / `ResetBreakers` | ✅ |
| `loadbalancer` 注册表 | ✅ |
| `logsink.Emit` | ✅ |
| `job.GetManager` | ✅ |
| `MySQLDao` 本身 | ❌ 无状态但 `WithContext/WithCache` 会修改副本；不要跨协程共享可变 DAO |
| `casbin.Casbin` | ⚠️ 内部 SyncedCachedEnforcer，但中间件每请求 LoadPolicy |

### 4.4 版本

签名以 `v2` 分支当前 HEAD 为准；升级前查
[兼容性矩阵](compatibility.md) 与 [迁移说明](../../migration-v1-to-v2.md)。

---

## 5. 找不到符号时

1. 确认包路径：v2 全部在 `pkg/` 下（根包 `mgin` 除外）；
2. 用 `go doc` 对照：
   ```bash
   go doc github.com/maczh/mgin/v2/pkg/client
   go doc github.com/maczh/mgin/v2/pkg/client.Options
   ```
3. 可能是未导出符号——不属于公开 API，不应在业务中使用；
4. 如果文档缺失但代码存在，按 [贡献指南](../10-contributing/01-contributing.md) 提交补充。
