# DB 与插件第二轮修复计划

- 项目：`github.com/maczh/mgin/v2`
- 日期：2026-11-25
- 复审基线：`aaae6a3`；当前实现包含其后的生命周期加固提交
- 依据：架构复审、工程复现、QA 复审及当前源码验证

## 1. 复审结论

上一轮 P0 已闭环：Redis/Mongo 共享 map 的竞态不再触发 `concurrent map read and map write`。但当前修复仍把部分网络操作放在全局锁内，并保留了若干生命周期和 panic 边界缺口。

本轮没有新的 P0。优先处理四类问题：

1. Mongo `GetConnection` 使用写锁并在锁内 Ping/重连，稳态访问被全局串行化。
2. Redis/Mongo 初始化、健康检查和重连在锁内执行网络 I/O，依赖异常时会阻塞请求和关闭。
3. Mongo 关闭后保留配置并可能复活连接；应用退出顺序又是先关 DB、后排空 HTTP。
4. 插件 `Name()`/`Order()` 仍可能在 recover 外调用；SQLite 无效 GORM 句柄仍可能外流。

## 2. 已完成项，不重复排期

- Redis/Mongo 共享 map 基础互斥保护。
- Mongo `Copy()` nil 检查。
- Redis 订阅 channel 复制、handler panic 隔离、`PSubscribeContext`。
- SQLite Close 基础错误检查。
- MySQL DAO 多数 nil receiver、Tag 竞态和 `WithTx` nil 防护。
- 插件 Init/Health/Close 基础 panic 隔离、typed-nil 和 Registry 防护。
- `go.config.used` 精确匹配。
- Redis/SQLite 基础健康检查。

## 3. 修复批次

### 批次一：Mongo/Redis 生命周期（P1）

#### Mongo

目标：`pkg/db/mongo/mgo.go`。

- 增加 `uninitialized/ready/closed` 生命周期状态和 `ErrClosed`。
- `Close` 设置 closed、清理 `confData`，摘出连接快照后锁外关闭 session。
- Close 后 `Check`/`GetConnection` 立即返回 `ErrClosed`，不得重新 Dial。
- `Init` 锁外解析配置和 Dial，锁内确认未关闭后一次性发布连接集合。
- `GetConnection` 用读锁取得 session/db 快照后立即解锁；Ping、Copy、重连不得持有全局写锁。
- 重连采用“锁外创建新 session、锁内替换、锁外关闭旧 session”。
- 将 `tags` 重置放入真正重建连接集合的分支；不得在冗余 Init 中擦除多库 tags。
- 使用局部 tag，不修改调用方 `dbName` slice。

验收：并发 `GetConnection/Check/Close/Init` 无 race；持有 RLock 时 GetConnection 不被写锁阻塞；Close 后连接调用在 50ms 内返回 ErrClosed。

#### Redis

目标：`pkg/db/redis/redis.go`。

- 增加 `closed bool` 和 `ErrClosed`，Close 幂等并取消内部订阅 context。
- Init 锁外构建/Ping 新 client，锁内一次性发布；失败时关闭临时 client。
- Check 只在锁内复制 client 快照，锁外 Ping；空集合返回错误。
- GetConnection 不在读锁中执行网络 I/O。
- 旧 `PSubscribe` 保持兼容，但必须由 Close 可停止；握手 Receive 等待期间也要响应取消，所有退出路径关闭 pubsub。
- 每次重连基于原始 channel 模板重新生成 `{db}` 替换结果。

### 批次二：插件与应用关停（P1/P2）

目标：`pkg/plugin/plugin.go`、`app.go`。

- Register 使用 `safeName`/`safeOrder`，捕获元信息方法 panic；注册表保存 `{plugin,name,order}` 条目。
- sortedByOrder、日志、错误、健康 map 和 recover 分支只使用缓存 name/order，不再调用不可靠的插件方法。
- safeInit/safeClose/safeHealth 的 recover 分支禁止再次调用插件接口。
- Enabled 失败要记录；Close 阶段不能静默跳过已注册插件，需保存启用快照或执行幂等 Close。
- 退出顺序调整为：停止接收新请求并 Shutdown HTTP → 等待健康检查退出 → SafeExit/CloseAll 关闭数据库和插件。

### 批次三：SQLite、错误传播与 DAO（P2）

- 为 SQLite 增加统一 `validateSqlite`，检查 `gorm.DB`、`Config`、`Statement`、`Error` 和底层 `*sql.DB`；Close/Check/GetConnection 均先校验，禁止无效句柄外流。
- 为 Redis、Mongo、SQLite 增加 `InitWithError`，保留旧 Init 作为兼容包装；builtins 的 initFn 传播初始化错误。
- `RedisCache.Range(nil)` 安全返回；RedisCache 导出方法增加 nil receiver 防护。
- 保留 DAO 副本语义，并补 release note 与调用示例，避免旧代码忽略返回值导致 context/cache/debug 静默失效。

## 4. 正式回归测试

此前 `zz_*` 探针已删除，必须将最小稳定用例纳入包测试：

- Mongo：Close terminal state、冗余 Init 保留 tags、读锁/锁外 Ping、并发生命周期。
- Redis：Close 后 Check 返回 ErrClosed、空 client 健康失败、context 取消和握手取消关闭 pubsub。
- SQLite：零值/半初始化 GORM DB 的 Close/Check/GetConnection 不 panic。
- Plugin：Order panic、Name 二次 panic、Register Name panic、Enabled panic 仍执行 Close。
- DAO：WithTx nil、RedisCache nil receiver、Range nil callback、链式配置副本语义。
- App：HTTP Shutdown 先于数据库关闭，健康检查 goroutine 有退出等待。

质量门禁：

```bash
export PATH=/usr/local/go/bin:$PATH
gofmt -w <changed-go-files>
go test ./pkg/db/... ./pkg/plugin/... -count=1
go test -race ./pkg/db/... ./pkg/plugin/... -count=1
go vet ./pkg/db/... ./pkg/plugin/...
go test ./... -timeout 20s
```

全量测试中的 Etcd/Consul 外部服务超时和既有 logsink 测试失败必须单独记录，不能误报为本计划回归。

## 5. 发布与回滚

按三批发布：Mongo/Redis 生命周期；插件元信息与应用关停；SQLite、错误传播和 DAO 防御性修复。每批观察请求 P95/P99、连接数、FD、goroutine、健康检查结果、重连次数和关闭耗时，日志不得输出 DSN、密码或 token。

出现关闭超时、连接数增长、健康检查异常、合法配置无法启动、race 或核心回归失败时回滚对应批次。

## 6. 交付清单

- [ ] Mongo 锁外 Ping/Dial、closed 三态、关闭后不复活。
- [ ] Redis 锁外 Ping、closed 三态、空集合健康失败、订阅可停止。
- [ ] 插件 name/order 缓存，运行期无不受保护元信息调用。
- [ ] Enabled 失败时关闭策略明确且可观测。
- [ ] HTTP Shutdown 先于 DB/插件关闭，健康检查有退出等待。
- [ ] SQLite 统一 validate，不外流无效 GORM 句柄。
- [ ] Redis/Mongo/SQLite 初始化错误可传播。
- [ ] RedisCache 防御性修复和 DAO 副本语义文档完成。
- [ ] 正式回归测试、race、vet 通过。

## 7. 暂不阻塞本轮

MySQL 极窄的旧连接关闭窗口、三套数据源生命周期抽象统一、Redis cluster 故障转移后的 `{db}` 变化和 Mongo Ping 的真实最长时延，暂列 P3/后续架构演进项。
