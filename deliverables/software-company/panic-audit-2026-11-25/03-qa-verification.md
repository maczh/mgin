# QA 独立复核与定级报告：pkg/db/ 与 pkg/plugin/ panic 风险

- 复核人：严过关（Yan）· QA 工程师
- 日期：2026-11-25
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，module `github.com/maczh/mgin/v2`
- 环境：`go version go1.25.7 linux/amd64`（需前置 `export PATH=/usr/local/go/bin:$PATH`）；命令统一加 `GOPROXY=https://goproxy.cn,direct`
- 方法：**不采信前序报告结论，自行编写临时 `zz_qa_*_test.go`（`package <pkg>` 以访问私有字段）逐个真跑**，记录命令与输出节选；跑完逐一删除。全程未修改任何生产代码、未 git commit。
- 收尾状态：`git status --short` → 仅 `?? deliverables/`；删除全部临时文件后 `go test ./pkg/db/... ./pkg/plugin/...` 退出码 0（全绿），与基线一致。

---

## 0. 一句话总览

前序两份报告的主结论**成立且未被夸大**：Redis / Mongo 的「无锁 + 可写共享 map」是唯一能触发 Go runtime **不可恢复 `fatal error`（`throw`，recover 无效）** 的进程级崩溃，确为 P0；其余为「可恢复 panic / 竞态 / 逻辑缺陷」。QA 独立复跑 7 项关键复现全部命中，**新增 2 条被前序漏列的逻辑风险**（`enabled()` 子串误启用、`CloseAll` 静默吞 panic），并对多处定性做了**下调/澄清**。两处未决问题的结论：**R2 的 L211 链式 nil panic 属静态确认 + 表达式级动态证明、端到端需真实 MongoDB（建议保留但并入 R2 修复，不单列 P0）**；**R4 的工程师修正成立（panic 在 L41/gorm.go:413；生产 Init 失败路径不触发，触发概率下调）**。

---

## 1. 独立复跑结果表

| 编号 | 临时测试（已删除） | 确切命令（节选） | 关键输出 | 独立结论 |
|---|---|---|---|---|
| R1 | `pkg/db/redis/zz_qa_redis_test.go::TestQAR1MapRace`（8 读 goroutine `GetConnection()` + 1 写 goroutine 重建/delete map） | `go test -race ./pkg/db/redis/ -run TestQAR1MapRace -count=1 -v` | `WARNING: DATA RACE` @ `redis.go:256`（×5）；`fatal error: concurrent map read and map write` | **复现**，与工程师一致 |
| R2(map) | `pkg/db/mongo/zz_qa_mongo_test.go::TestQAR2MapRace`（8 读 `GetConnection("tag")` + 1 写 `Close()`/重建） | `go test -race ./pkg/db/mongo/ -run TestQAR2MapRace -count=1 -v` | `DATA RACE` @ `mgo.go:129/143/144/72/42/174/207/213`；`fatal error: concurrent map read and map write` | **复现**，与工程师一致 |
| R4 | `pkg/db/sqlite/zz_qa_sqlite_test.go`（Close 零值 gorm.DB、gorm.Open 失败状态、正常 Close） | `go test ./pkg/db/sqlite/ -run TestQAR4 -count=1 -v` | `panic` 栈：`gorm.(*DB).DB gorm.go:413` ← `sqlite.go:41`；`gorm.Open(invalid): db==nil? false; db.DB(): sqldb==nil? false, dberr=<nil>` | **复现且两处修正均成立**（见 §2.2） |
| R3 | `pkg/plugin/zz_qa_plugin_test.go::TestQAR3InitAllPanicPropagates`（panic 插件 order=1 + 正常插件 order=2，替换全局注册表） | `go test ./pkg/plugin/ -run TestQAR3 -count=1 -v` | `recovered="QA_R3_PANIC" laterInited=false` | **复现**：panic 穿透且后续插件未初始化 |
| R7 | `pkg/plugin/zz_qa_plugin_test.go::TestQAR7RegisterTypedNil`（`var p *funcPlugin=nil; Register(p)`） | `go test ./pkg/plugin/ -run TestQAR7 -count=1 -v` | `panic: invalid memory address or nil pointer dereference` | **复现**（L46 `p.Name()`） |
| R8 | `pkg/db/dao/zz_qa_dao_test.go::TestQAR8DAONilReceiver`（`(*MySQLDao[E])(nil).Create(...)`） | `go test ./pkg/db/dao/ -run TestQAR8 -count=1 -v` | `panic: invalid memory address or nil pointer dereference` | **复现**（L292） |
| R5（附加） | `pkg/db/dao/zz_qa_r5_test.go::TestQAR5SharedDAORace`（写者 `WithContext`/写 Tag，读者 `All`） | `go test -race ./pkg/db/dao/ -run TestQAR5 -count=1 -v` | `DATA RACE` @ `mysql.go:70/73/418/421`（`m.Tag`/`m.ctx`） | **QA 独立复现**（架构师 P1 成立，原报告未动态验证） |

> R6（PSubscribe 改写调用方 slice）QA 未单独复跑，采纳工程师已动态证实的结论（内容污染 + `-race` 命中 L287），定级不变。

---

## 2. 两处未决问题的结论

### 2.1 R2 的 `mgo.go:211` 链式解引用：静态风险，动态端到端不可达（建议保留但并入 R2）

做了三种独立逼近，均无法在无真实 MongoDB 时走通端到端路径：

1. **表达式级证明（命中）**：`m.conns[dbName[0]].conn.Copy().DB(...)` 在 `conn==nil` 时直接执行 → `panic: invalid memory address or nil pointer dereference`。证明 L211 本身的解引用**确实**会 panic。
2. **零值 `&mgo.Session{}` 注入（命中，但非 L211）**：把零值 session 塞进 `conns["tag"]` 后调 `GetConnection("tag")`，panic 来自 **mgo 库内部**（零值 Session 的 `client==nil`，`mgoCheck` L150 `conn.conn.Ping()` → `s.client.Ping` 解引用 nil），**并非 L211** 的 `.Copy()`。说明 L147 的 `conn.conn == nil` 只是浅层指针判空，挡不住「非 nil 但内部失效」的 session——但生产路径中 session 只来自成功的 `DialWithTimeout`，不会自然产生零值，故非新增可达风险。
3. **并发 Close 交错探测（49/50 被提前拦下）**：多 goroutine `GetConnection("tag")` 与 `Close()` 交错时，49/50 次在 `mgoCheck` L147（`!ok || conn.conn == nil`）就返回 error，**到不了 L211**。根因：无真实 Mongo 时 `mgo.DialWithTimeout` 10s 超时返回 error，建不出可 Ping 的 session（实测 `mgo.DialWithTimeout(unreachable,1s)` → server selection error，耗时 1.0009s）。

**结论**：L211 的 nil panic 需满足「先有能通过 `mgoCheck` 的有效 session（真实 Mongo）→ 在 L207 `mgoCheck` 与 L211 之间另一 goroutine 执行 `Close()` 删除 `conns` 项 → L211 取到零值 `connection{}` → `.conn.Copy()` nil 解引用」。这是一个真实的 TOCTOU，但**动态端到端不可达/需真实依赖**；其根因与 R2 的「无锁 map」同源。建议：**保留为静态确认风险，并入 R2 的统一修复（RWMutex + L211 取本地变量并 nil 守护），不单列为 P0**。可信度标注为「静态确认 + 表达式级动态证明，端到端需真实 MongoDB」。

### 2.2 R4 触发概率：工程师的两处修正均成立

1. **panic 位置**：QA 独立栈回溯与工程师完全一致——`gorm.(*DB).DB(...) gorm.go:413` ← `(*Sqlite).Close sqlite.go:41`。即 panic 发生在 **L41 调用 `m.sqlite.DB()` 时（gorm 内部对零值 ConnPool 解引用）**，根本到不了架构师预测的 L42 `db.Close()`。修复守护必须覆盖 L41 的调用本身（判断 `m.sqlite.Config == nil || m.sqlite.Statement == nil` 等），而非只判断返回的 `db != nil`。
2. **生产 Init 失败路径不触发（修正成立）**：QA 独立验证 `gorm.Open(sqlite.Open("/no/such/dir/qa.db"), ...)` → `db==nil? false, err=unable to open database file`；随后 `db.DB()` → `sqldb==nil? false, dberr=<nil>`（两次调用一致）。即 gorm v1.31.0 + sqlite v1.6.0 下 `sql.Open` 惰性建连，无效路径要到首次 Ping/查询才暴露，Init 失败留下的是**非 nil 且 ConnPool 有效**的 `*gorm.DB`。**架构师"Init 失败必触发 R4"的假设被推翻**，触发概率从「高概率（特定输入必现）」下调为「低概率：仅人为/注入 `&gorm.DB{}`（ConnPool 完全未初始化）或非标准 ConnPool 替换时必现」。代码缺陷（丢弃 `DB()` error、缺 nil 守护、Init 失败未复位 `m.sqlite`）仍真实，但不宜列为高优先级线上崩溃项。

---

## 3. 漏报 / 误报反向检查结论

| 检查项 | 独立验证 | 判定（是否构成风险） |
|---|---|---|
| **`builtins.go:52-54` `enabled()` 用 `strings.Contains(Used,name)` 子串误匹配** | 实测 `Used="mysql_backup,redis-cli"` → `enabled("mysql")=true`、`enabled("redis")=true`、`enabled("sqlite")=false`（用例全 PASS）。确认 **false-positive 误启用**（如 "notmysql" 也会启用 mysql）。 | **新增逻辑缺陷，但不触发未初始化 client 的 panic（对原假设的修正）**。逐条追踪误启用后的路径：Mysql `InitWithError(getCfg(空))` → `loadMySQLConnections` 返回 "mysql configuration is empty" error，`InitAll` 记 error 后继续；`Close`/`Check` 均有 nil/`closed` 守护；Redis/Mongo/Sqlite 空配置 `Init` 得到空 client map，`Close` 遍历空 map、`Check` 返回 nil。均无 panic。`Contains` 只会 false-enable、不会 false-disable。**定级 P3（配置正确性），非崩溃风险**；但建议改用逗号分词精确匹配，避免"误启用 → 无谓 Init + 日志噪音"。 |
| **`plugin.go:123-147` `CloseAll` 虽 recover 但吞错误不记日志 + `Enabled()` 关闭时重新求值** | 用例证明：panic 插件的 Close 被 recover 后 `CloseAll` 返回 `err=nil`（`recover` 分支仅 `_ = r`，无 `logs.Error`、不计入 `firstErr`）；另构造 `Enabled()` 在 Init 时 true、Close 时 false 的插件 → `closeCalled=false`（**漏关**）。 | **构成 P2/P3 风险，非 panic（它反而阻断 panic 穿透）**：① recover 吞 panic 无日志 → 运维无感知、该插件资源未释放，属可观测性 + 资源泄漏；② `Enabled()` 在 Init/Close 各求值一次，若插件 enabled 状态在两次之间变化（自定义有状态插件）→ 漏关。内置插件的 `config.Used` 静态，实际不受影响。**建议**：recover 时 `logs.Error` 并计入 `firstErr`；`InitAll` 阶段快照 enabled 列表供 `CloseAll`/`HealthAll` 复用。 |
| **`plugin.go:150-159` `HealthAll` 无 recover（R11）——内置路径是否真安全** | 用例证明：自定义 `Health()` panic 的插件使 `HealthAll()` panic 穿透。零值状态逐一测内置：`Mysql.Check` → "mysql configuration is empty"（无 panic）、`Sqlite/Redis/Job/Registry.Check` → nil、`(&Mongodb{}).Check()` → "MongoDB connection not found"（无 panic）。 | **架构师"内置路径安全"判断成立**：6 个内置 healthFn 在零值/未初始化下均不 panic（`Mysql.Check` 有 nil/`closed` 守护，Mongo `Check/mgoCheck` 有 nil 守护）。风险仅在**自定义插件** health panic：它经 `checkAll`（`mgin.go:175`，5 分钟定时器，`app.go` 无 recover）→ **进程级崩溃**（定时器 goroutine 无 HTTP 中间件兜底）。**维持 P2，建议与 R3 一并加 recover**。 |
| **`dao/redis_cache.go:59-80` `Range(f)` f==nil 且 Redis 有 key 是否 panic；`clearCache` 是否总传非 nil** | 用 `redis.NewScanCmdResult`/`NewStringResult` 造 1 个 key 的假 client：`Range(nil)` → `panic: invalid memory address`（L71 `f(key,value)`）；`Range(非nil)` 正常调用 1 次。全仓 grep `.Range(`：唯一内部调用方 `mysql.go:220 clearCache` 传的是非 nil 闭包。 | **真实 API 健壮性缺口但内部不可达**：`clearCache` 恒传非 nil，`cache.go`/`memcache.go`/`expire_cache.go` 的 `.Range` 是不同接口且均传非 nil。`RedisCache` 为导出类型（`NewRedisCache`），外部调用方传 nil 才会触发。**定级 P3（防御性），非当前风险**；建议 `if f == nil { return }`。架构师"未发现 panic 风险"在内部调用范围内成立。 |
| **`dao/mgo.go:202` `dbName[0]=m.tags[0]` 改写入参 slice 的副作用，是否比 R6 更严重** | 实测：`GetConnection(args...)`（slice 展开）→ `args[0]` 被改为 `"tag-a"`；`GetConnection("")`（单字符串）→ 调用方变量不变（编译器新建 slice）。 | **真实的入参副作用，但比 R6 更轻**：发生在调用方 goroutine（同步），且 DAO 调用均传 `m.Tag()` 单值（编译器建 slice）→ 生产无调用方可见污染。R6 在后台 goroutine 改共享 slice 且有跨 goroutine race + 内容污染，更严重。**定级 P3**，与 R6 同属"复制入参"防御建议。 |
| **`db/transaction.go` 回调 panic 时 GORM 是否回滚并重新 panic；框架层是否需 recover** | 用 sqlite `:memory:` 建表，回调插入 1 行后 `panic`：外层 `recovered="QA_TX_CALLBACK_PANIC"`，回滚后 `count=0`。另核验 gorm 源码 `finisher_api.go:622+`：`defer` 中 `if panicked || err != nil { tx.Rollback() }`，**不 recover**，panic 按 Go 语义穿透。 | **架构师"未发现风险"成立，无需框架层 recover**：GORM 已保证回滚，panic 穿透是预期语义（HTTP 路径有 `app.go:200 nice.Recovery` 兜底，后台 job 路径无兜底属标准 Go 行为，框架 recover 会吞掉业务错误）。**不建议改动**。 |
| **附加新发现：零值 `&mgo.Session{}` 经 `mgoCheck` L150 解引用 panic** | `TestQAR2ZeroValueSessionProbe` 命中 nil deref（见 §2.1-2）。 | **防御性缺口，非生产可达**：L147 仅判指针非 nil，不判内部 `client`；生产 session 只来自成功 `DialWithTimeout`，无零值来源。并入 R2 的"取本地变量 + nil 守护"修复即可，**不单列风险项**。 |

---

## 4. 最终定级表

区分三类：**进程级不可恢复崩溃（fatal/throw）** / **可恢复 panic** / **竞态·资源泄漏·逻辑缺陷（非直接 panic）**。

| 编号 | 风险 | 最终级别 | 崩溃类型 | 可信度 | 本轮建议 |
|---|---|---|---|---|---|
| R1 | Redis 无锁 map（`redis.go:66/67/186/235/256`） | **P0** | 进程级不可恢复（fatal throw） | 已动态证实（QA 独立） | **必修** |
| R2 | Mongo 无锁 map（`mgo.go:42/72/129/143/144/174/204/207`） | **P0** | 进程级不可恢复（fatal throw） | 已动态证实（QA 独立） | **必修** |
| R2b | `mgo.go:211` 链式 nil 解引用 | **P1**（并入 R2） | 可恢复 panic / 非 HTTP 路径进程级 | 静态确认 + 表达式级动态证明；端到端需真实 Mongo | 随 R2 一并修，不单列 P0 |
| R3 | `InitAll` 无 defer/recover | **P1** | 进程级（启动期 `mgin.Init`） | 已动态证实（QA 独立） | 建议本轮修 |
| R5 | 共享 DAO `m.ctx/m.Tag` 无锁读改写 | **P1** | 竞态 / 窄窗口 nil deref | 已动态证实 `-race`（QA 独立） | 建议本轮修（改返回副本） |
| R6 | `PSubscribe` 改写入参 slice + 重连不关 pubsub | **P1** | 竞态 + 连接泄漏（非直接 panic） | 已动态证实（工程师） | 建议本轮修 |
| R4 | `sqlite.Close` 丢弃 `DB()` error（panic 在 L41） | **P2**（概率已下调） | 可恢复 panic（`CloseAll` 内）；生产不可达 | 已动态证实（QA 独立） | 可本轮顺手修，不列高优先级 |
| R7 | `Register` 未拦 typed-nil | **P2** | 可恢复 panic（启动期） | 已动态证实（QA 独立） | 建议本轮修（复用 `isNilPlugin`） |
| R8 | DAO nil receiver 前置写 `Tag` | **P2** | 可恢复 panic（HTTP 可恢复） | 已动态证实（QA 独立） | 排期/防御性修复 |
| R9 | `MgoDao.Update` 缺 `conn==nil` 守护 | **P2** | 可恢复 panic | 静态确认 | 随 R2 加守护 |
| R10 | Registry 插件对 nil/typed-nil `registry.Registry` 解引用 | **P2** | 可恢复 panic（非标准启动） | 静态确认（QA 未动态） | 排期 |
| R11 | `HealthAll` 无 recover | **P2** | 进程级（仅自定义插件，定时器无兜底） | 自定义路径已动态证实；内置安全已验证 | 随 R3 一并加 recover |
| R12 | MySQL 发布后锁外关旧连接 use-after-close | **P3** | 逻辑风险（`sql.DB.Close` 返回 error 非 panic） | 静态确认 | 可不修 |
| R13 | `Check` 空实现掩盖故障 | **P2** | 逻辑缺口（非 panic） | 静态确认 | 排期（实现真实 Ping） |
| N1（新） | `enabled()` 子串误启用 | **P3** | 配置逻辑缺陷（**不触发 panic**） | 已动态证实（QA） | 可选：分词精确匹配 |
| N2（新） | `CloseAll` recover 吞 panic 无日志 | **P2** | 可观测性 + 资源泄漏（非 panic） | 已动态证实（QA） | 建议修：记日志 + 计入 firstErr |
| N3（新） | `CloseAll`/`HealthAll` 重复求值 `Enabled()` 漏关 | **P3** | 资源泄漏（非 panic） | 已动态证实（QA） | 可选：快照 enabled 列表 |
| N4（新） | `RedisCache.Range(f=nil)` 无守护 | **P3** | 可恢复 panic（内部调用方均非 nil） | 已动态证实（QA） | 可选：`if f==nil{return}` |
| — | `mgo.go:202` 改写入参 slice | **P3** | 入参副作用（比 R6 轻） | 已动态证实（QA） | 可选：复制入参 |

**定级口径说明**：
- **进程级不可恢复崩溃**：仅 R1、R2（map 的 `fatal error` 是 runtime `throw`，`recover` 无法捕获）——这是唯一会"打挂进程且无法在框架层兜底"的两项，故独占 P0。
- **进程级可恢复/可兜底**：R3、R11（自定义插件）、R2b（非 HTTP 路径）——panic 可被 `recover` 捕获，但当前关键路径（启动期、定时器）缺兜底。
- **可恢复 panic**：R4/R7/R8/R9/R10/N4——Go 标准 panic，HTTP 请求路径有 `nice.Recovery` 兜底。
- **竞态/泄漏/逻辑**：R5/R6/R12/R13/N1/N2/N3——非直接 panic，但 R5 的 data race 属未定义行为、R6 的连接泄漏长期可致 OOM。

---

## 5. 修复优先级建议

1. **本轮必修（P0，进程级不可恢复）**：以 `mysql.MysqlClient` 为模板，为 `RedisClient`/`Mongodb` 增加 `sync.RWMutex`（建议 `initMu + mu` 分离）+ `closed` 幂等 + RCU 发布；`Init` 锁外建连/锁内发布，`Close` 写锁摘旧/锁外关闭；`GetConnection` 在 RLock 内取本地变量（含 `conn==nil` 守护）后立即解锁再做网络 I/O。一并关闭 R2b 的 L211 链式解引用。
2. **本轮强烈建议（P1，可兜底的进程级缺口 / 高概率竞态）**：
   - R3 + R11 + N2：`InitAll`/`HealthAll` 循环内仿 `CloseAll` 加 `defer/recover`，panic 转 error 记录并继续；`CloseAll` recover 分支补 `logs.Error` + 计入 `firstErr`。
   - R5：`WithContext`/`WithCache`/`Debug` 改为返回新副本（仿 `WithTx`），`connection()` 内把 `m.ctx/m.Tag` 读到局部变量，消除共享实例竞态。
   - R6：`PSubscribe` 复制 `channels` 局部切片 + `Receive` 失败先 `pubsub.Close()` + `handler==nil` 守护 + context 停止信号。
3. **可本轮顺手修（低成本防御）**：R4（L41 前加 `m.sqlite==nil || m.sqlite.Config==nil || m.sqlite.Statement==nil` 守护 + 检查 `DB()` error）、R7（`Register` 复用反射判空）、R9（补 `conn==nil`）、N4（`Range` 加 `f==nil`）、`mgo.go:202`（复制入参）、N1（`enabled` 分词精确匹配）、N3（快照 enabled 列表）。
4. **排期（P2/P3 逻辑）**：R8（DAO 全方法入口 nil 守护）、R10（registry nil 守护）、R13（实现真实健康检查）。
5. **可不修**：R12（`sql.DB.Close` 安全，仅偶发查询失败）。

---

## 6. 哪些风险没有正确测试 seam（diagnosing-bugs Phase 5 要求）

以下风险缺少**能稳定、端到端复现生产真实链路**的测试 seam，当前只能靠静态审查或合成夹具逼近，修复后需补充相应 seam 才能真正回归验证：

| 风险 | 缺失的 seam | 当前替代手段 | 修复后应补的测试 |
|---|---|---|---|
| R2b `mgo.go:211` nil panic | 需**真实 MongoDB** 提供可通过 `mgoCheck` L150 `Ping()` 的 session，并在 L207→L211 之间并发 `Close()` 删除 `conns` 项 | 表达式级测试 + 零值 session 注入（panic 落在 mgo 库内部）+ 并发交错探测（被 L147 提前拦下） | 用容器化 Mongo（testcontainers）或可注入 `Ping`/`Copy` 的 session 接口，精确制造 TOCTOU 窗口 |
| R1/R2 生产真实并发窗口 | 需**完整 mgin 应用 + 配置 + 在途 HTTP 请求 + `checkAll` 定时器** 交错；本地仅能用合成 goroutine 制造 map 竞态 | `-race` 合成读/写 goroutine（已命中 fatal）；关停顺序由工程师代码走查确认（先关 DB 后 `Shutdown HTTP`） | 集成测试：启动真实 router，在途请求中触发 `SafeExit`，断言无 fatal |
| R5 真实共享用法 | 需业务以"共享一个 `*MySQLDao` 并并发重配置"的方式使用；本地用合成写者/读者 | `-race` 合成 `WithContext`/`Tag` 写 vs `All` 读（已命中 race） | 并发单元测试固定 `WithContext` 返回副本后断言无 race |
| R10 Registry typed-nil / R3 非标准启动 | 需绕过 `mgin.Init:127` 的 `NewRegistry()` 赋值、注入 typed-nil `registry.Registry`；需访问 `registry` 包私有全局 | 静态审查（依赖全局初始化顺序） | 在 `registry` 包内加可注入 seam 的测试 |
| R4 生产 Init 失败路径 | 需特定 dialector 把 `ConnPool` 换成非 `*sql.DB` 类型；gorm 版本行为相关 | `&gorm.DB{}` 注入（ConnPool 完全未初始化）+ `gorm.Open` 无效路径状态探针 | 构造自定义 `ConnPool` 的 dialector，验证修复后 `Close` 不 panic |
| R12 use-after-close 窗口 | 需在 RCU 发布后、旧连接关闭前的极窄窗口取到旧 `*gorm.DB`；且 `sql.DB.Close` 后操作返回 error 非 panic，难以稳定观测为失败 | 静态审查 | 给连接加代际编号/引用计数后，用脚本化 driver 精确控制时序 |
| R11 定时器路径 | 需真实 5 分钟 `checkAll` 定时器触发自定义插件 health panic；本地仅验证 `HealthAll()` 直接调用穿透 | 自定义插件直接调 `HealthAll()`（已证实 panic 穿透） | 注入短周期 ticker + 自定义 panic 插件，断言 recover 后进程存活 |

---

## 7. QA 复核结论

- **前序报告无夸大**：R1/R2 的 P0、R3/R7/R8 的定性全部经 QA 独立动态证实；R2 的 L211、R4 的两处修正经独立验证属实。
- **前序报告无重大漏报的 panic 风险**：新增的 N1/N2/N3/N4 均为 P2/P3 的逻辑/可观测性/防御性缺口，不改变"仅 Redis/Mongo map 为 P0"的格局。
- **可信度总览**：已动态证实 9 项（R1/R2/R3/R4/R5/R7/R8/N1/N2/N3/N4 中的主要项）、静态确认 4 项（R2b 端到端/R9/R10/R12）、表达式级证明 1 项（R2b 单点）。
- **最终路由**：本轮以 P0（R1/R2）为修复核心，P1（R3/R5/R6 + R2b 并入）紧随；其余按 §5 排期。**QA 侧无未通过测试**（临时测试全部达成预期后已删除，基线全绿）。

（报告完）
