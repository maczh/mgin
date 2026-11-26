# 工程师对抗性复现与回归报告（修复后代码 · 第二轮）

- 执行人：寇豆码（Kou）· 工程师
- 日期：2026-11-25
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，`HEAD = aaae6a3`，module `github.com/maczh/mgin/v2`
- 环境：`go version go1.25.7 linux/amd64`（`export PATH=/usr/local/go/bin:$PATH`，`GOPROXY=https://goproxy.cn,direct`）
- 纪律：**未修改任何生产代码、未 commit**；只写临时探针 `zz_verify_*_test.go`（5 个文件、21 个用例），跑完**全部删除**；01–04 报告未改动。
- 判定口径：只有「真跑出数值 / 真打出 panic」才写「成立」；依赖真实外部依赖无法构造的写「未验证 + 原因」；架构师结论与实测不符的单独列出。

---

## 0. 结论摘要

### 0.1 三句话

1. **上轮 P0 确实修掉了**：Redis/Mongo 并发 `Init/Close/GetConnection/Check` 在 `-race -count=3` 下零 DATA RACE、零 fatal、零 panic（Redis 场景还实打实发生了「Close 正在关 client 时 GetConnection 仍成功取到 client」的交错）。
2. **架构师 X 系列动态结论 12 条中 11 条被我独立复现，数值与其报告一致到小数第二位**（Redis 2.50s / 1.70s，Mongo 9.70s / 10.00s）。唯一需要修正的是 **X10 的根因表述**：panic 不在「`ConnPool` 为 nil」，而在 **`gorm.DB` 的嵌入字段 `*Config` 为 nil 导致字段提升解引用**——我用对照用例把这一点钉死了（`&gorm.DB{Config:&gorm.Config{}}` 不 panic，返回 `ErrInvalidDB`）。
3. **我新提的一个假设被自己推翻**：`PSubscribe` 订阅确认失败的分支**不是忙轮询**（3.5s 内只发出 1 次 `PSUBSCRIBE`），但它的实际行为更糟——`Receive()` 直接不返回，等于**无超时、无退避、无失败感知**。

### 0.2 总判定表

| 编号 | 我的判定 | 关键实测数值 | 与架构师结论的差异 |
|---|---|---|---|
| **A1 Redis 并发回归** | **无 DATA RACE / 无 fatal**（P0 已修） | `-race -count=3` 3 次通过；期间 `GetConnection` 成功 311/313/312 次、失败 40/46/44 次 | 一致（架构师为静态+间接推断，我给出了「并发真的发生」的量化证据） |
| **A2 Mongo 并发回归** | **无 map race / 无 fatal**（P0 已修） | `-race -count=3` 通过；6 并发 × 20 轮 + 20 次 `Close` | 一致；**但真实 `*mgo.Session` 的 `Copy()/DB()` 路径未覆盖**（见 §4） |
| **X1(a) Redis Init 锁内 Ping** | **成立** | `GetConnection` 被阻塞 **2.500103118s**（第二次 2.501845597s）；`Init` 总耗时 **3.000833302s** | 数值与架构师 2.50s 一致 |
| **X1(a) Redis Check 锁内 Ping** | **成立** | `Close()` 被阻塞 **1.700973088s**（第二次 1.700188886s）；`Check` 总 **2.00167721s** | 与其 1.70s 一致 |
| **X1(b) Mongo Init 锁内 Dial** | **成立** | `ListConnNames()` 被阻塞 **9.700979666s**；`Init` 总 **10.000990822s** | 与其 9.70s 一致 |
| **X2 Mongo 关闭后复活** | **成立** | `Close()` 后 `GetConnection` **10.000362972s**、`Check` **10.001085518s**；错误为 `MongoDB connection not found`（**不是** `ErrClosed`） | 一致（其 10.001s / 10.000s） |
| **X3 GetConnection 用写锁** | **成立**（锁类型直接实测） | 持有 `RLock` 时 `GetConnection` 被阻塞 >300ms（读锁本应并发进入） | 架构师是「静态确认」，我提供了**锁模式的可证伪实验** |
| **X4 冗余 Init 擦除 tags** | **成立** | `tags:[a b]` → `[]`；`GetConnection("")` → `MongoDB multidb has no available database` | 一致 |
| **X5 Order() panic 穿透** | **成立** | `InitAll` / `HealthAll` / `CloseAll` 三处全部 `ORDER_PANIC` 逃逸 | 一致，且我补齐了 `CloseAll` 也穿透这一条 |
| **X6 recover 内二次 panic** | **成立** | `safeInit` 逃逸值 = `NAME_PANIC`（原 `INIT_PANIC` 被替换）；对照：仅 `INIT_PANIC` 时正确转 error | 一致 |
| **X7 Redis 关闭后 Check 假阳性** | **成立** | 关闭后 `Check() err = <nil>` | 一致 |
| **X7 Redis 订阅 goroutine 泄漏** | **成立** | goroutine 数 base=6 → `Close()` 后 1.5s 仍 =7（`PSubscribeContext` 版本可回到 base） | 一致，且我做了「有 ctx / 无 ctx」对照 |
| **X7b（我新增假设）订阅失败忙轮询** | **不成立** | 300ms 内 1 次、3.5s 内仍 **1 次** `PSUBSCRIBE` | 新增发现：不是忙轮询，而是 `Receive()` 卡死（见 §3.9） |
| **X9 DAO 副本语义 + WithTx nil** | **成立** | `(*MySQLDao[E])(nil).WithTx(nil)` → SIGSEGV；`base.WithContext(&ctx)` 后 `base.ctx==nil = true` | 一致 |
| **X10 sqlite 零值 gorm.DB** | **成立（根因需修正）** | `Close()` / `Check()` 均 panic 于 `gorm.go:413`；`GetConnection()` 返回 `conn!=nil, err=nil` | **根因表述差异**：不是 `ConnPool` 解引用，而是嵌入 `*Config` 为 nil；对照用例 `&gorm.DB{Config:&gorm.Config{}}` 不 panic |
| **X13 Enabled panic 漏关** | **成立** | 正常插件 `closed=true`；Enabled panic 插件 `closed=false` | 一致 |
| **X8/X11/X12/X15/X16** | **未验证（静态结论不推翻，也不背书）** | — | 见 §4 |
| **Mongo `Ping()` 30s 上界** | **未验证** | — | 与其「假设 9」同结论 |
| **X3 吞吐线性 / QPS 上限** | **未验证** | — | 与其「假设 5」同结论 |

---

## 1. A. 正向回归：上轮 P0 是否真修掉

### 1.1 A1 Redis —— 并发 `Init/Close/GetConnection`（`-race -count=3`）

**构造难点与解法**：`Init` 现在持写锁并**真的 Ping**，如果指向不可达地址，每次 `Init` 要等 3s（ReadTimeout）甚至 60s（`go.data.redis_pool.timeout`），测试会退化。因此我写了一个**最小 RESP 假服务**（`net.Listen` + 解析 RESP 数组 + `PING` 回 `+PONG`、`PSUBSCRIBE` 回合法订阅确认），让 `Init` 的 Ping 走真协议、毫秒级成功，从而让「Close 与使用并发」真实发生。

```bash
go test -race -run 'TestZZVerifyRedisConcurrentLifecycleRace' -v -count=3 -timeout 300s ./pkg/db/redis/
```

关键输出（节选）：
```
[A1] Init 后 clients=1 conns=1（>0 证明假 RESP 服务生效，GetConnection 走真实分支）
[A1] GetConnection 成功 311 次 / 失败 40 次
[A1] 并发 Init/Close/GetConnection 完成，-race 未报错
--- PASS: TestZZVerifyRedisConcurrentLifecycleRace (0.08s)
（三个 count 分别：成功 311/313/312，失败 40/46/44；无 WARNING: DATA RACE）
```

**判定：A1 通过。** `mu` 已覆盖 `clients/cfgs/conns` 全部访问点；`Close()` 关 client 的同时另一 goroutine 取到同一 client 并 `Ping()`，返回 `ErrClosed` 类错误而**不 panic**（失败计数 40+ 全部来自这个窗口，而不是崩溃）。

### 1.2 A2 Mongo —— 并发 `GetConnection/Close/Check`（`-race -count=3`）

```bash
go test -race -run 'TestZZVerifyMongoConcurrentLifecycleRace|TestZZVerifyMongoCloseConcurrentWithCheck' -v -count=3 -timeout 300s ./pkg/db/mongo/
```
关键输出：
```
[A2] 并发 Init/Check/Close/GetConnection/ListConnNames 完成（-race 未报错即通过）
[REV] Close 与 Check/GetConnection 并发：无死锁、无 fatal
ok  github.com/maczh/mgin/v2/pkg/db/mongo  1.067s
```

**构造方式（按主理人允许的「包内字段注入」）**：`confData` 用 `uri: not-a-mongo-url`，`mgo.DialWithTimeout` 会立刻返回 `mongodb empty url or invalid url`，因此 `Init` 的完整锁路径（`initMu` → `mu` → 解析 → 建连失败 → 解锁）被执行到，但**不引入 10s 网络等待**；同时注入 `conns` 占位条目覆盖 `mgoCheck` 的 nil-conn 分支。

**判定：A2 通过（有限覆盖）。** 覆盖了 `conns/tags/multi/confData` 的并发读写与 `Close` 的删除操作；**未覆盖**真实 `*mgo.Session` 的 `Copy()/DB()`（原因见 §4.1）——这是本轮 A2 唯一的缺口，我明确标注而不编造。

---

## 2. B. 对抗性验证（逐条：命令 + 输出 + 判定）

> 所有探针均为 `package <pkg>` 内测试，可直接访问私有字段；文件已删除，重建方法见 §5。

### 2.1 X1(a)-1：Redis `Init` 持写锁做 Ping → 阻塞所有 `GetConnection`

**构造**：本地 `net.Listen` 只 `Accept` 不回包（挂死服务）；goroutine 调 `Init()`（单库、无 `redis_pool.max` 配置 → DialTimeout 默认 5s、ReadTimeout 默认 3s），500ms 后主 goroutine 计时 `GetConnection()`。

```bash
go test -run 'TestZZVerifyRedisInitHoldsWriteLock' -v -count=1 ./pkg/db/redis/
```
```
[X1a] Init 持写锁期间 GetConnection 被阻塞 = 2.500103118s (err=Redis connection failed.)
[X1a] Init 总耗时 = 3.000833302s
[X1a] 判定：成立
```

**判定：成立。** 阻塞 **2.5001s**（复跑 2.501845597s），等于 `Init` 剩余时长；`Init` 总耗时 **3.0008s** ≈ 一次 ReadTimeout（3s，超时**不**重试）。**架构师 2.50s 完全吻合。**

**补充（静态，但直接影响阻塞上界）**：`redis.go:149-173` 里 `go.data.redis_pool.max > 1` 时 `DialTimeout = go.data.redis_pool.timeout`（缺省 **60s**）。多库（`multidb=true`）时 `redis.go:174-184` 是**逐库串行 Ping**，所以 `Init` 的最坏阻塞 = `Σ 每库 Ping 上界`；在「SYN 被丢弃/黑洞地址」场景下单库可达 60s，**多库叠加即分钟级**，且期间所有 `GetConnection`（= 全部 `RedisCache` 链路）被同一把写锁挡住。

### 2.2 X1(a)-2：Redis `Check` 持读锁做 Ping → 阻塞 `Close` 的写锁

**构造**：按主司令人指令注入 `clients["0"] = redis.NewClient(&Options{Addr: 挂死地址, ReadTimeout: 2s, MaxRetries: -1})`，goroutine `Check()`，300ms 后 `Close()` 计时。

```
[X1a] Check 持 RLock Ping 期间 Close(写锁) 被阻塞 = 1.700973088s
[X1a] Check 总耗时 = 2.00167721s
[X1a] 判定：成立
```
复跑：`1.700188886s` / `2.000855637s`。

**判定：成立。** Close 被拖 **1.70s**，与架构师 1.70s 一致。注意这正是 `SafeExit` 的 5s ctx 被吃掉的机制（X15）。

### 2.3 X1(b)：Mongo `Init` 持写锁 `DialWithTimeout` → 阻塞所有读者

**构造**：`confData` 指向 `mongodb://127.0.0.1:1/zzverify`；goroutine `Init()`，300ms 后 `ListConnNames()` 计时。

```bash
go test -run 'TestZZVerifyMongoInitHoldsWriteLock' -v -count=1 ./pkg/db/mongo/
```
```
[X1b] Init 持写锁 Dial 期间 ListConnNames 被阻塞 = 9.700979666s (len=0)
[X1b] Init 总耗时 = 10.000990822s
[X1b] 判定：成立
```
复跑：`9.6994957s` / `10.000288259s`。

**判定：成立。** 一个只读的 `ListConnNames()` 被 `Init` 的 Dial 拖 **9.70s**，与架构师一致。

### 2.4 X2：Mongo `Close()` 之后「复活」

**构造**：`m := &Mongodb{confData: cfg, conns: map[string]connection{}}` → `m.Close()` → `GetConnection()` 计时 → `Check()` 计时。

```
[X2] Close 之后：conns len=0, confData len=107（Close 未清 confData、无 closed 标志）
[X2] Close 后 GetConnection err=MongoDB connection not found elapsed=10.000362972s
[X2] Close 后 Check        err=MongoDB connection not found elapsed=10.001085518s
[X2] 判定：成立
```
复跑：`10.000862186s` / `10.000657517s`。

**判定：成立。** 关闭后每次 `GetConnection`/`Check` 都**重新完整拨号 10s**；错误是 `MongoDB connection not found`（`mgo.go:160`），**不是** `ErrClosed`（代码里压根没有这个哨兵），因此调用方无法区分「未初始化」与「已关闭」——这正是 X2 修复建议中三态 `uninitialized/ready/closed` 的必要性所在。可达地址下这会留下**永不被释放的 `*mgo.Session`**（`mgo.go:95/116` 新建、`Close` 已跑完无人再关）。

### 2.5 X3：`GetConnection` 用**写锁**（并发串行化）—— 锁模式可证伪实验

架构师是「静态确认（`Lock` 语义确定）」。我把它变成实验：在同包内**先持 `RLock`**，再观察 `GetConnection` 是否能并发进入（若用 `RLock` 应立即返回）。

```bash
go test -run 'TestZZVerifyMongoGetConnectionUsesWriteLock' -v -count=1 ./pkg/db/mongo/
```
```
[X3] RLock 被持有期间 GetConnection 被阻塞 300ms → 使用写锁（mgo.go:219 m.mu.Lock）
[X3] 判定：成立（... err=MongoDB connection not found）
```
**判定：成立。** 一个有守护的 `GetConnection` 在另一读者持锁时无法进入 → 用的是**互斥量**，不是读锁。结合 `mgoCheck`（`mgo.go:162` 的 `Ping()`、`:166` 的 `DialWithTimeout`）在同一把写锁内执行这一代码事实，可确定 **Mongo 全库访问被全局串行化**。

### 2.6 X4：冗余 `Init` 擦除 `m.tags`

```
[X4] Init 前 tags=[a b]；冗余 Init 后 tags=[] (len=0)
[X4] 冗余 Init 后 GetConnection("") err=MongoDB multidb has no available database
[X4] 判定：成立
```
**判定：成立。** 根因精确定位在 `mgo.go:52` 的 `m.tags = make([]string, 0)` 位于 `:56 if len(m.conns)==0` **之前**——即使 `conns` 非空、`Init` 什么也不做，`tags` 也已被清空。后果不是 panic，而是多库模式下**所有不带 tag 的 DAO 调用直接失败**。

### 2.7 X5：`Order()` 在任何 recover 之外 → panic 穿透三个入口

**构造**：注册 2 个插件（1 正常 order=1 + 1 `Order()` panic order=2，满足 `sort.Slice` 触发 `less` 的 ≥2 元素条件），外层 `recover` 记录。

```
[X5] InitAll   中 Order() panic 穿透 = ORDER_PANIC
[X5] HealthAll 中 Order() panic 穿透 = ORDER_PANIC
[X5] CloseAll  中 Order() panic 穿透 = ORDER_PANIC
[X5] 判定：任一处穿透即成立（plugin.go:109 sort.Slice 的 less 调用 Order() 在任何 recover 之外）
```
**判定：成立。** 架构师只列了 `InitAll`/`HealthAll`，我实测 **`CloseAll` 同样穿透**（关停路径也会被一个自定义插件的 `Order()` 打穿）。这条比 R3 更危险：`HealthAll` 由 `app.go` 的 ticker 每 5 分钟调用，**运行期定时器 goroutine panic → 整进程退出**。

### 2.8 X6：`safeXxx` 的 recover 分支再次调用 `Name()` → 二次 panic

**构造**：直接对 `safeInit` 传入「`Init()` panic + `Name()` panic」的插件；另做一组对照只 panic `Init()`。

```
[X6] safeInit(Init panic + Name panic) 逃逸出的 panic = NAME_PANIC
[X6] 对照：仅 Init panic 时 safeInit err = plugin zz-init-only init panic: INIT_PANIC
[X6] InitAll 中 p.Name() panic 穿透 = NAME_PANIC
```
**判定：成立。** 原始 `INIT_PANIC` 被 recover「吞掉」后被 `p.Name()` 的新 `NAME_PANIC` **替换并穿透**——加固代码本身成了 panic 放大器。另外 `InitAll:131` 的 `p.Name()` 早于 `safeInit` 执行，所以实际连守护都进不去。

### 2.9 X7：Redis 关闭后 `Check()` 假阳性 + 订阅 goroutine 泄漏

```
[X7] Init 后 ListConnNames=[0]
[X7] 关闭后 Check() err = <nil>
[X7] 判定：成立（clients 为空时 Check 恒 nil → 健康检查假阳性）
[X7] PSubscribeContext goroutine: base=5 start=6 afterCancel=5
[X7] PSubscribe(无 ctx)  goroutine: base=6 start=7 afterClose=7
[X7] 判定：成立（Close 后订阅 goroutine 仍存活，无框架级停止信号 → goroutine/日志泄漏）
```
**判定：成立（两条）。** ① `Check()` 在 `clients` 为空时循环零次恒返回 `nil`——「未启用」与「已关闭」都报健康，`/health/ready` 永远绿；② `PSubscribe`（内部 `context.Background()`）在 `Close()` 之后仍每秒重试、持续 `logger.Error`，而 `PSubscribeContext` 版本能干净退出（对照 base 5→6→5），**说明修复方向（内部 ctx + `Close` 里 cancel）已经验证可行**。

### 2.10 X7b（我新增假设）：订阅确认失败是否忙轮询 —— **自我推翻**

假设：`redis.go:368-372` 的失败分支 `continue` 无退避 → 忙轮询打爆 CPU/连接。
```
[X7b] PSUBSCRIBE 请求次数：300ms=1，3.5s=1
[X7b] 判定：不成立（重试节奏被 ReadTimeout 节流，不是忙轮询；但失败分支确实没有显式退避）
```
**判定：不成立（我的假设错了），但带出一个更隐蔽的问题**：服务端回非订阅确认应答时，`pubsub.Receive()` **3.5s 内不返回**——既没有走「错误 → continue」分支，也没有订阅成功，goroutine 静默挂起。即该失败路径 **无超时、无退避、无失败感知**，`PSubscribe` 调用方无法知道订阅从未生效。与架构师 X7 同源但机制不同（不是「1s 重试」，而是「卡在 Receive」）。

### 2.11 X9：DAO 副本语义（静默失效）+ `WithTx` 的 nil receiver

```
[X9] (*MySQLDao[E])(nil).WithTx(nil) recovered = runtime error: invalid memory address or nil pointer dereference
[X9] 判定：成立（WithTx 中 clone := *m 解引用 nil receiver）
[X9] WithContext 后：base.ctx==nil = true ; 返回值.ctx!=nil = true
[X9] 判定：成立（忽略返回值的旧调用静默丢弃 ctx：编译通过、运行期超时/链路追踪失效，无任何报错）
[X9] WithCache 忽略返回值后 base2.cache==nil = true（缓存静默关闭）
[X9] Debug 忽略返回值后 base3.debug = false（原对象调试日志静默关闭）
```
**判定：成立（两条）。** ① `dao/mysql.go:60-64` 的 `WithTx` 是**唯一**漏掉 nil 守护的方法（`Debug/WithContext/WithCache/connection` 全部有守护，实测不 panic，见反向检查）；② 「副本语义」使 `dao.WithContext(&ctx)`（丢弃返回值的旧写法）**静默失效**——这是比 panic 更难排查的行为回归。

### 2.12 X10：`(&Sqlite{sqlite:&gorm.DB{}})` 的 Close/Check/GetConnection

```
[X10] (&Sqlite{sqlite:&gorm.DB{}}).Close() recovered = runtime error: invalid memory address or nil pointer dereference
[X10] Close 后 m.sqlite==nil = false（panic 中断，未走到置 nil）
[X10] (&Sqlite{sqlite:&gorm.DB{}}).Check() recovered = runtime error: invalid memory address or nil pointer dereference
[X10] .GetConnection() recovered = <nil> ; conn!=nil = true err = <nil>
[X10] 对照（Config 非 nil、ConnPool nil）：Check recovered = <nil> ; DB()=(<nil>,invalid db)
```
panic 栈（首次跑出的崩溃现场，用于定位）：
```
panic: runtime error: invalid memory address or nil pointer dereference
gorm.io/gorm.(*DB).DB(...) gorm.go:413
github.com/maczh/mgin/v2/pkg/db/sqlite.(*Sqlite).Check(...) sqlite.go:70
```
**判定：成立，但根因与架构师表述不同。** 架构师写「gorm.go:413 对零值 `ConnPool` 解引用」；实测**根因是 `gorm.DB` 嵌入的 `*Config` 为 nil**：`gorm.go:413` 的 `db.ConnPool` 是**字段提升**，需先解引用 `db.Config`（nil → SIGSEGV `addr=0x80`）。我加的对照用例是决定性证据：`&gorm.DB{Config:&gorm.Config{}}`（Config 非 nil、ConnPool 仍 nil）**不 panic**，`DB()` 返回 `(nil, invalid db)`。结论对修复无影响（`validateSqlite` 仍是正解，建议判 `conn.Config == nil` 而不仅是 `conn == nil`），但对「根因归档」有意义。另：`GetConnection()` 返回 `conn!=nil, err=nil` 的无效连接，会把 panic 后移到 `db.Model()`，脱离 `CloseAll` 的 recover 保护。

### 2.13 X13：`CloseAll` 中 `Enabled()` panic → 静默漏关

```
[X13] CloseAll err = plugin enabled panic: ENABLED_PANIC
[X13] 正常插件 closed=true ; Enabled panic 插件 closed=false
[X13] 判定：成立（Enabled panic → continue 跳过 Close，资源静默泄漏）
```
**判定：成立。** 对比 N2 的收益很讽刺：panic 被记进 `firstErr` 了（N2 已修），但**该插件根本不关**（`plugin.go:153-159` 的 `continue`）——从「panic 被打断」变成「静默泄漏」。

### 2.14 反向检查（修复是否引入新 panic 面 / 是否漏关）

| 检查项 | 结果 | 判定 |
|---|---|---|
| sqlite 真实连接 `Init → Check → GetConnection → Close → 二次 Close` | `Check err=<nil>`；关闭后 `Check err=SQLite not opened`（**非 nil，无假阳性**）；二次 `Close` 幂等不 panic | 通过 |
| `Redis.Close()` 与 4 个 `PSubscribeContext` 并发 | `Close` 在 0.1s 内返回，无死锁、无 panic（`use of closed network connection` 由 go-redis 内部消化） | 通过 |
| `Mongo.Close()` 与 `Check()/GetConnection()` 并发（6 协程 × 20 轮 + 20 次 Close） | 无死锁、无 fatal（`-race` 亦通过） | 通过 |
| nil receiver 守护：`Debug/WithContext/WithCache/connection` | 4 个全部不 panic | 通过（`WithTx` 是唯一例外 → X9） |
| 未启用（`Enabled()==false`）插件 | `InitAll` 不 panic，`Close` 不被调用 | 通过 |

**结论：修复没有引入新的 panic 面**；X1/X2/X3/X4/X7/X9/X13 都是「不崩但卡/泄漏/静默失效」类问题，与架构师的分级（P1：可用性；P2：可 recover 或影响有限）一致。

---

## 3. 与架构师结论的差异汇总（逐条）

| 项 | 架构师 | 我的实测 | 处理建议 |
|---|---|---|---|
| X1(a) Redis Init | 阻塞 2.50s | **2.5001s**（= ReadTimeout 3s 的剩余） | 采纳；**补一句**：多库时 `Init` 是逐库串行 Ping，`redis_pool.max>1` 时 `DialTimeout` 被抬到 **60s**（`redis.go:162-165`），最坏阻塞是「库数 × 60s」量级，比报告写的「库数 ×(5s+3s)」更严重 |
| X1(a) Redis Check | Close 被阻塞 1.70s | **1.7009s** | 一致 |
| X1(b) Mongo Init | 9.70s | **9.7010s** | 一致 |
| X2 | 10.001s / 10.000s，`ErrClosed` 缺失 | **10.0004s / 10.0011s**，错误是 `MongoDB connection not found` | 一致；**补**：错误文本暴露「未初始化/已关闭」不可区分，三态改造是必需而非可选 |
| X3 | 静态确认写锁 | **锁模式实验通过** | 采纳（升级为动态证据） |
| X4 | tags 被擦除 | 一致（`[a b]`→`[]`，并复现 `no available database`） | 采纳 |
| X5 | `InitAll`/`HealthAll` 穿透 | 一致，**另发现 `CloseAll` 也穿透** | 采纳 + 扩一条 |
| X6 | `NAME_PANIC` 二次 panic | 一致 | 采纳 |
| X7 | Check 假阳性 + 1s 重试泄漏 | 一致（`PSubscribeContext` 对照可退出） | 采纳 |
| **X7b（新增）** | — | 失败分支**不是忙轮询**，而是 `Receive()` 卡死不返回 | 建议并入 X7 的修复说明：`pubsub.Receive()` 需要超时 |
| X9 | 副本语义 + `WithTx` nil | 一致（两条都实测） | 采纳 |
| **X10 根因** | 「零值 `ConnPool` 解引用」 | panic 确实是 `gorm.go:413`，但**根因是嵌入 `*Config` 为 nil**；`Config!=nil, ConnPool==nil` 不 panic（返回 `ErrInvalidDB`） | 采纳结论，**修正根因**；`validateSqlite` 应判 `conn.Config == nil` |
| X13 | Enabled panic 静默漏关 | 一致（`closed=false`） | 采纳 |

---

## 4. 未验证项与原因（不编造）

### 4.1 Mongo 真实 `*mgo.Session` 的 `Copy()/DB()` 并发路径 —— **未验证**
**原因**：`github.com/maczh/mgo@v1.0.4` 的 `Session` 字段全不导出，且**唯一**构造入口 `DialWithTimeout`（`session.go:64-100`）在内部 `client.Ping(ctx)` 成功后才返回 `*Session`；本机无 `mongod`（`which mongod` 为空），而任何假 TCP 服务都过不了 mongo-driver 握手。因此 A2 只能覆盖 `conns/tags/multi/confData` 的 map 并发与 nil-conn 分支，**`mgo.go:238/254` 的 `Copy()→DB()` 路径没有动态覆盖**（该路径的 nil 守护是静态可读的，但与 R2b 的「真实行为」仍需真实 Mongo 才能闭环）。
**建议**：CI 起一个 `mongo:6` 容器后补 `TestMongodbGetConnectionConcurrent`。

### 4.2 `mgo.Session.Ping()` 的真实超时上界（架构师假设「30s」）—— **未验证**
**原因**：同上（拿不到 session 就无法 `Ping`）。**静态可确认的部分**：`session.go:89` 的 `s.ctx = context.Background()`，`session.go:221` 的 `Ping` 用的就是这个无超时 ctx，`SetSocketTimeout` 是空实现（`session.go:225-231`）→ **`Ping` 在框架侧确实没有超时**，只受 mongo-driver 默认 `serverSelectionTimeout`（30s）约束。这只影响「最坏阻塞时长」的数值，不影响 X1/X3 的结论。

### 4.3 X3 的吞吐/QPS 线性 —— **未验证**
**原因**：需要真实 Mongo + 可注入延迟。本轮给出的是**锁模式**证据（写锁 + 锁内 Ping），性能推导仍属静态。

### 4.4 X8（`{db}` 只替换一次）、X11（RLock 内写 map 的注释残骸）、X12（入参 slice 副作用）、X15（`SafeExit` ctx 装饰性）、X16（生命周期契约不一致）—— **未验证（静态结论，不推翻也不背书）**
**原因**：X11 的代码路径当前不可达（`redisCheck` 是空实现），X8 需要 db 号在重连后变化的真实故障转移场景，X15 需要真实关停链路（本轮 X1/X2 的阻塞数值已足以佐证「单组件就能吃掉 5s」）。

### 4.5 生产配置相关（X13 的「漏启用」）—— **未验证**
**原因**：没有线上 `go.config.used` 样本，无法判断是否存在依赖子串匹配的旧配置。

---

## 5. 探针重建方法（供 QA 独立复核）

| 用例 | 文件（临时） | 关键构造 | 预计耗时 |
|---|---|---|---|
| A1 Redis 并发回归 | `pkg/db/redis/zz_verify_redis_test.go` | 最小 RESP 假服务（PING→`+PONG`、PSUBSCRIBE→合法确认）+ 8 读协程与 40 次 `Init/Close` 并发 | 0.1s/次 |
| X1a Redis Init | 同上 | `net.Listen` 只 accept 不回包 + `Init` 于 goroutine，500ms 后计时 `GetConnection` | 3s |
| X1a Redis Check | 同上 | 注入 `ReadTimeout:2s, MaxRetries:-1` 的 client + `Check` 于 goroutine，300ms 后计时 `Close` | 2s |
| X7 / X7b | 同上 | 拒绝连接地址（走 1s timer 分支）/ 假服务回 `+OK` 触发异常分支；`runtime.NumGoroutine()` 与命令计数取证 | 2.2s / 3.6s |
| A2 / X1b / X2 / X3 / X4 | `pkg/db/mongo/zz_verify_mongo_test.go` | `invalidCfg`（`uri: not-a-mongo-url`，立即失败）压锁路径；`unreachableCfg`（`127.0.0.1:1`）测 10s；X3 用「先持 `RLock` 再调 `GetConnection`」判锁模式；X4 用包内字段注入 `multi/conns/tags` | 0.01s / 10s / 20s / 0.3s / 0.01s |
| X5 / X6 / X13 | `pkg/plugin/zz_verify_plugin_test.go` | 自定义插件实现 6 方法，按需 panic；`recoverOf` 包裹 `InitAll/HealthAll/CloseAll`；X6 直接调 `safeInit` 隔离「recover 内二次 panic」 | 0.02s |
| X9 | `pkg/db/dao/zz_verify_dao_test.go` | `MySQLDao[zzEntity]`（`zzEntity` 实现 `schema.Tabler`）；nil receiver 与「忽略返回值」两种调用写法 | 0.01s |
| X10 | `pkg/db/sqlite/zz_verify_sqlite_test.go` | `&Sqlite{sqlite:&gorm.DB{}}` 与对照组 `&gorm.DB{Config:&gorm.Config{}}`；另用 `t.TempDir()` 跑真实 sqlite 生命周期 | 0.02s |

一键复原（跑完记得删文件）：
```bash
export PATH=/usr/local/go/bin:$PATH
export GOPROXY=https://goproxy.cn,direct
go test -v -count=1 -timeout 400s ./pkg/db/sqlite/ ./pkg/db/dao/ ./pkg/plugin/ ./pkg/db/mongo/ ./pkg/db/redis/
go test -race -count=3 -run 'TestZZVerifyRedisConcurrentLifecycleRace' ./pkg/db/redis/
go test -race -count=3 -run 'TestZZVerifyMongoConcurrentLifecycleRace' ./pkg/db/mongo/
```
21 个用例本轮**全部 PASS**（`ok pkg/db/mongo 30.315s`、`ok pkg/db/redis 10.974s`、其余 <0.05s）。

---

## 6. 收尾状态

```
$ git status --short
 M .gitignore            # 进入本任务前即存在，非我改动
$ go test -count=1 ./pkg/db/... ./pkg/plugin/...      → exit 0（dao/mysql/plugin ok，其余 no test files）
$ go test -race -count=1 ./pkg/db/... ./pkg/plugin/... → exit 0
$ go vet ./pkg/db/... ./pkg/plugin/...                → exit 0
```
- 5 个临时探针文件已**全部删除**，无残留；01–04 报告未删除、未改动；无 commit。
- 唯一遗留：`pkg/db/{redis,mongo,sqlite}` 仍然**没有正式测试文件**（`[no test files]`）——这正是「绿灯不代表正确」的根源，建议把 §5 的 A1/A2 正例沉淀为入仓回归用例（X 系列的「失败用例」在当前代码上会红，需先修再入仓）。

（报告完）
