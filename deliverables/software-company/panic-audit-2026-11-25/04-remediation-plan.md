# panic 风险修复计划

- 项目：`github.com/maczh/mgin/v2`
- 日期：2026-11-25
- 范围：`pkg/db/`、`pkg/plugin/builtins.go`，以及承载生命周期风险的 `pkg/plugin/plugin.go` 和 `pkg/db/dao/`
- 依据：架构评审、工程复现、QA 验证及本轮静态审查
- 当前状态：仅完成审查与验证，尚未修改生产代码

## 1. 目标与验收标准

本计划的目标是消除数据库客户端生命周期中的进程级崩溃风险，并使插件初始化、健康检查和关闭过程具备可观测、可回归验证的失败语义。

完成后必须满足：

1. Redis/Mongo 的 `Init`、`Check`、`GetConnection`、`Close` 并发运行时无 data race、无 `concurrent map read and map write`。
2. Mongo 重连失败或连接失效时只返回错误，不对 nil/已关闭 session 调用 `Copy().DB()`。
3. 插件的 `Init`、`Health`、`Close` panic 不穿透框架生命周期；panic 要记录插件名和堆栈，并转换为错误继续处理。
4. SQLite、DAO、Registry、Redis 订阅入口对 nil/错误状态返回明确错误，不发生可避免的 nil 解引用。
5. 所有新增回归测试通过普通测试、`-race` 和 `go vet`。
6. 修复不改变现有 v2 API；需要新增能力时优先新增方法或内部实现，不静默改变旧方法语义。

## 2. 风险分级

### P0：必须优先处理

| ID | 风险 | 主要位置 | 结果 |
|---|---|---|---|
| P0-1 | Redis 共享 map 无同步 | `pkg/db/redis/redis.go` | 并发读写可能触发不可 recover 的 runtime fatal |
| P0-2 | Mongo 共享 map/切片无同步 | `pkg/db/mongo/mgo.go` | 并发读写可能触发不可 recover 的 runtime fatal |
| P0-3 | Mongo 失效 session 的 `Copy().DB()` 链式调用 | `mgo.go:190-223` | 重连/关闭竞态下 nil pointer panic |

### P1：本轮一并修复

| ID | 风险 | 主要位置 | 结果 |
|---|---|---|---|
| P1-1 | `InitAll`/`HealthAll` 无 panic 隔离 | `pkg/plugin/plugin.go` | 自定义插件 panic 可中断启动或定时健康检查 |
| P1-2 | MySQL DAO 共享配置字段读写竞态 | `pkg/db/dao/mysql.go` | data race，窄窗口下 nil 解引用 |
| P1-3 | Redis `PSubscribe` 修改调用方 slice、失败重连泄漏 pubsub | `redis.go:273-304` | data race、连接泄漏、长期运行可能 OOM |
| P1-4 | SQLite `DB()` 错误被忽略 | `pkg/db/sqlite/sqlite.go:39-45` | 异常 GORM 对象下 nil 解引用 |

### P2/P3：低成本防御与后续排期

- typed-nil 插件注册。
- DAO nil receiver 入口保护。
- `MgoDao.Update` 的 nil connection 检查。
- `builtins.go` 对 `config.Config`、`registry.Registry` 的保护。
- `CloseAll` recover 静默吞错、`HealthAll`/`CloseAll` 重复求值 `Enabled()`。
- Redis/SQLite 空健康检查、初始化错误丢失。
- `enabled()` 的子串误匹配和 Mongo 入参 slice 改写。

## 3. 分阶段实施方案

### 阶段 A：Redis/Mongo 生命周期并发安全

#### A1. Redis

在 `RedisClient` 增加 `sync.RWMutex`、关闭状态和必要的初始化串行锁。覆盖 `clients`、`cfgs`、`conns`、`multi`、配置快照的所有读写。

实现约束：

- `Init` 在锁外解析配置、创建和 Ping 新 client；成功后在锁内一次性发布新状态。
- 发布失败或中途失败时关闭所有临时 client，不污染旧状态。
- `Close` 在锁内摘除状态并标记 closed，锁外关闭 client；重复关闭安全。
- `GetConnection` 只在读锁中取得本地 client 快照，网络检查不持锁执行。
- 关闭后返回明确的 `ErrClosed`，不得自动恢复已关闭状态。

#### A2. Mongo

在 `Mongodb` 增加读写锁和初始化串行锁，保护 `conns`、`tags`、`multi`、配置快照和重连替换。

实现约束：

- `Init` 使用临时 connection map，全部成功后原子发布；失败时关闭临时 session。
- `Close` 先摘除连接快照，再在锁外关闭 session。
- `GetConnection` 在锁内一次性取出 session/db 到局部变量，检查 `conn != nil` 后解锁，再执行 `Copy()`。
- 检查 `Copy()` 返回值；不能对 nil session/database 继续调用 `DB()`。
- `Check`/`mgoCheck` 的重连替换必须经过同一把锁；避免在 map 遍历时修改 map。
- 不允许健康检查隐式创建半初始化连接；如保留懒初始化，必须走统一的初始化状态机。

#### A3. 先行回归测试

- 多 goroutine `GetConnection` 与 `Close`/`Init` 交错。
- 健康检查重连与 DAO 取连接交错。
- 初始化部分成功、全部失败、关闭期间初始化完成三类场景。
- `go test -race` 必须无报告；不能只断言“没有 panic”。

### 阶段 B：插件生命周期和错误边界

修改 `pkg/plugin/plugin.go`：

- `InitAll` 对每个插件建立局部 `defer/recover` 边界，将 panic 转换为带插件名的 error，继续处理后续插件。
- `HealthAll` 同样隔离 panic，返回每个插件对应的错误。
- `CloseAll` 的 recover 分支记录错误和堆栈，并计入 `firstErr`，不能静默吞掉。
- 对 `Register` 增加 typed-nil 检测，复用反射判空逻辑；`Name()` 之前不得调用可能为 nil 的对象。
- 对 `Enabled`、`Name`、`Order` 的异常是否需要隔离，应统一决定并覆盖测试；至少不能让 typed-nil 在 `Name()` 处崩溃。

修改 `pkg/plugin/builtins.go`：

- `config.Config == nil` 时返回明确错误或安全的 disabled 状态。
- Registry 初始化、注册和关闭前检查 nil/typed-nil。
- MySQL 之外的内置 DB 初始化应尽量提供错误返回，避免日志后仍被报告为“初始化成功”。

回归测试：注册一个 enabled 的 panic 插件，验证 `InitAll`/`HealthAll` 返回错误、继续处理其他插件且测试进程存活。

### 阶段 C：DAO、SQLite 和 Redis 订阅

#### C1. MySQL DAO

- 所有公开方法在访问 `Tag` 前先处理 nil receiver。
- `WithContext`、`WithCache`、`Debug` 使用 clone-and-return，不原地修改共享 DAO。
- `connection()` 将 `Tag`、`ctx` 复制到局部变量，避免重复解引用共享字段。
- 保持 `WithTx` 的现有副本语义。

#### C2. SQLite

- `Init`、`Close`、`GetConnection` 增加 nil receiver和状态保护。
- `Close` 检查 `DB()` error 及 `*sql.DB` 是否为 nil；无论关闭结果如何都清理内部引用。
- `Init` 失败时不保留无效 `*gorm.DB`。
- 如支持运行时重载，增加 mutex；否则明确初始化完成后不可并发重载。

#### C3. Redis PSubscribe

- 入口拒绝 nil handler。
- 复制 `channels` 后只修改局部副本。
- 每轮订阅使用 `defer` 或显式关闭 pubsub；`Receive()` 失败后先关闭再重试。
- 增加 context/stop 信号，关闭客户端时终止后台重试 goroutine。
- handler panic 应隔离并记录，不能直接打挂进程。

## 4. 测试与质量门禁

每个阶段都必须先补测试再改实现，测试应覆盖真实 seam，而不是只测试一个 nil 表达式。

建议命令：

```bash
go test ./pkg/db/... ./pkg/plugin/...
go test -race ./pkg/db/... ./pkg/plugin/...
go vet ./pkg/db/... ./pkg/plugin/...
```

必要的集成测试：

- 使用容器或可注入 seam 验证真实 Mongo session 的 Ping/Copy/Close 时序。
- 启动真实应用和在途请求，再执行优雅关闭，验证 DB 关闭不会与请求产生 fatal。
- 使用短周期 ticker 验证健康检查中的自定义插件 panic 被隔离。

当前已知限制：此前复现用的临时 `zz_repro_*` 文件已被外部进程移除；后续应将关键复现正式迁移到受版本控制的测试文件中，避免验证证据丢失。

## 5. 发布、回滚与观测

发布顺序：

1. 先发布 Redis/Mongo 锁与连接状态修复。
2. 再发布插件生命周期和订阅 goroutine 修复。
3. 最后发布 DAO/SQLite/防御性修复。

上线观测：

- 统计初始化、健康检查、重连和关闭错误，包含组件名和数据库名，不输出 DSN、密码或 token。
- 观察 Redis/Mongo 连接数、FD、goroutine 数和关闭耗时。
- 关注 `concurrent map read and map write`、nil pointer、pubsub 重连次数。
- 保留旧 API 行为；新增错误只在原本无法安全继续时返回。

回滚条件：

- 连接池无法初始化或频繁重连；
- 关闭耗时显著增加；
- 业务请求出现新增连接不可用错误；
- race/集成测试未通过。

回滚应按提交阶段进行，不回退已经独立验证的错误日志和测试资产。

## 6. 交付清单

- [ ] Redis/Mongo 生命周期锁、原子发布、幂等关闭完成。
- [ ] Mongo `Copy().DB()` 全链路 nil/error 检查完成。
- [ ] `InitAll`、`HealthAll`、`CloseAll` panic/error 语义统一。
- [ ] DAO/SQLite/PSubscribe 防御性修复完成。
- [ ] 正式回归测试覆盖 P0/P1，且 `-race` 通过。
- [ ] `go vet` 和相关包测试通过。
- [ ] 日志脱敏、无 DSN/token 泄露。
- [ ] 更新提交说明，记录实际修复的根因与验证命令。

## 7. 不纳入本轮核心修复的项目

MySQL 发布新连接后锁外关闭旧连接属于低概率 use-after-close/逻辑窗口，当前证据显示 `sql.DB.Close` 本身不会产生 panic，暂不阻塞本轮交付。Redis/SQLite 的真实健康检查、`enabled()` 精确分词和全量 DAO nil receiver 规范可作为后续增强，但 P0/P1 修复完成前不得用这些项目替代核心并发修复。
