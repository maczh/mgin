# QA 第二轮独立复核与最终定级报告

- 复核人：严过关（Yan）· QA 工程师
- 日期：2026-11-25（第二轮独立复核）
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，`HEAD = aaae6a3`，module `github.com/maczh/mgin/v2`
- 环境：`go version go1.25.7 linux/amd64`（`export PATH=/usr/local/go/bin:$PATH`，`GOPROXY=https://goproxy.cn,direct`）
- 立场声明：**不采信 04/05 的任何结论**。本报告所有数值均来自我自己写的 `zz_qa2_*` 探针（6 个文件、24 个用例，与工程师的 `zz_verify_*` 无任何复用），跑完已全部删除；未改动任何生产代码、未 commit、未动 01–05 报告。
- 收尾状态（已核验）：
  ```
  $ git status --short           →  M .gitignore   （唯一，进入任务前即存在）
  $ go test -count=1 ./pkg/db/... ./pkg/plugin/...  → exit 0
  $ go test -race -count=1 ./pkg/db/... ./pkg/plugin/... → exit 0
  $ go vet ./pkg/db/... ./pkg/plugin/...            → exit 0
  ```

---

## 0. 最终结论（先给判断，证据在后）

### 0.1 三个必须回答的问题

**Q1：上一轮的 P0 是否真的闭环？—— 是，闭环成立。**
我自己用最小 RESP 假服务（`PING→+PONG`，真协议）让 Redis 的 `Init` 真实成功，从而让「`Close` 关 client」与「另一 goroutine 拿到同一 client 并 `Ping()`」真实交错：`-race -count=3` 三轮全部通过、**零 DATA RACE、零 fatal**（成功 114/119/118 次，失败 138/127/132 次，失败全部来自关停窗口的正常 error，不是崩溃）。Mongo 用 `invalid uri` 把 `initMu→mu` 的完整锁路径压了 14934/14247/15675 次 + 3×10s 的 `Close/Check/GetConnection/Init` 混跑（34 万 ops/轮），同样零 race、零 fatal、无死锁。
**判定：上轮 P0（Redis/Mongo 无锁共享 map → `fatal error: concurrent map read and map write`）已闭环，不可再触发。**

**Q2：是否与 P0 同量级的新问题？—— 有 P1 级，但「同量级」这个措辞我只部分同意。**
- 我**不同意**把 X1 简单定性为「与 P0 同量级」。P0 是**无条件**的：只要并发就必然 `fatal`，进程直接死，无缓解手段。X1 是**条件性**的：只有当下游 Redis/Mongo 变慢/不可达时才触发；下游健康时它不外显。
- 但我**要补一条比 X1 更该立刻修的**：**X3（Mongo `GetConnection` 用写锁 + 每次都在锁内 `Ping`）是「无条件」的稳态损伤**——它不依赖任何故障，每条 DAO 调用都要过一次全局互斥量 + 一次网络往返，吞吐上限被钉死在 `1/RTT`。**这条是修复本身引入的性能回归，性质上比 X1 更接近 P0**（P0 是「随时崩」，X3 是「随时只有 1/RTT 的吞吐」）。
- **X2（Mongo 关闭后复活）**我判 P1：它在「关停期 + 在途请求」这个窗口内**必然**触发 10s/次重拨号，且被 `app.go:387/395` 的**关停顺序倒置**（先 `SafeExit` 关 DB、后 `Shutdown` 排空 HTTP）放大到整个 HTTP 排空期——`shutdownTimeout` 默认 5s，必然超时被 SIGKILL。

**Q3：本轮必修清单是什么？—— 见 §4 定级表，4 条 P1 + 6 条 P2。**

### 0.2 一句话总览

> 锁把「崩」修成了「不崩」，这一点我独立复跑认同。但修复的**形状**照搬了 `mu/initMu` 却丢了 `MysqlClient` 的**纪律**（锁外建连 + 锁内发布 + `closed` 三态 + 锁外 I/O）：结果是 Redis/Mongo 在**稳态**（X3：每次调用都串行 + Ping）和**故障期**（X1：锁护航）两个方向都出现可用性损伤，Mongo 还多出「关停后复活」（X2）。`plugin` 的 recover 加固漏了 `Order()`/`Name()` 两个元信息方法（X5/X6），`sqlite` 的 R4 只修了一半（X10），`Redis` 仍无 `closed` 语义（X7，比 04 报告说的更严重：**从未 Init 也报健康**）。**没有新 P0。**

---

## 1. 独立复跑表（我自己的探针与数值）

> 全部为 `package <pkg>` 内探针，直接访问私有字段。文件已删；重建方法见 §6。
> 数值列：**我的实测**（04 架构师 / 05 工程师）——三者并列便于看差异。

### 1.1 正向回归（本轮必须证明「P0 已闭环」）

| 编号 | 用例 | 构造要点 | 我的实测 | 与 04/05 对比 | 结论 |
|---|---|---|---|---|---|
| **R1** | Redis 并发 `Init/Close/GetConnection/Check` | 自建最小 RESP 服务（真协议 `PING→+PONG`、`SCAN/GET/DEL/SELECT`），`Init` 真实成功；8 读协程（拿到 client 后**故意在用** `Ping()`）与 40 次 `Init+Close` 并发 | `-race -count=3` 三轮全 PASS；成功 114/119/118、失败 138/127/132、`accept` 194/196/198 | 05: 成功 311/313/312（其 `count` 配置不同，量级一致） | **无 DATA RACE / 无 fatal** |
| **R2** | Mongo 并发 `Init/Close/Check/GetConnection/ListConnNames` | `uri: not-a-mongo-url`（`DialWithTimeout` 立即失败，不引 10s 等待），压完整锁路径 | 1s × 8 协程：14934 / 14247 / 15675 次调用，`-race -count=3` 全 PASS；另加 3×10s 混跑（340907/339612/339957 ops）无死锁 | 一致 | **无 map race / 无 fatal / 无死锁** |

> **R2 的诚实缺口（与 05 同）**：`conn.conn` 是 `*mgo.Session`，全字段不导出，唯一构造入口 `DialWithTimeout` 必须握手成功；本机无 `mongod`。因此 `mgo.go:238/254` 的 `Copy()→DB()` 路径**本轮仍未动态覆盖**，只有静态可读 + `copy == nil` 守护。

### 1.2 X 系列独立复跑

| 编号 | 我的实测数值 | 04 架构师 | 05 工程师 | 判定 |
|---|---|---|---|---|
| **X1(a)-1** Redis `Init` 持写锁 Ping → `GetConnection` 阻塞 | **2.500784731s**（`Init` 总 **3.001937608s**） | 2.50s | 2.5001s | **成立**（三方一致，误差 <0.04%） |
| **X1(a)-2** Redis `Check` 持读锁 Ping → `Close` 阻塞 | **1.700751137s** | 1.70s | 1.7009s | **成立**（三方一致） |
| **X1(b)** Mongo `Init` 持写锁 Dial → `ListConnNames` 阻塞 | **9.701160471s**（`Init` 总 **10.001262911s**） | 9.70s | 9.7010s | **成立**（三方一致） |
| **X2** Mongo `Close()` 后复活 | `GetConnection` **10.000324228s** / `Check` **10.000200957s**，错误均为 `MongoDB connection not found` | 10.001s / 10.000s | 10.0004s / 10.0011s | **成立**（三方一致；`ErrClosed` 哨兵不存在） |
| **X3** Mongo `GetConnection` 用写锁 | 他人持 `RLock` 期间 `GetConnection` 阻塞 >300ms | 静态确认 | 同法实测 | **成立**（写锁，`mgo.go:219`） |
| **X4** 冗余 `Init` 擦除 `tags` | `[a b]` → `[]` | `[]` | `[]` | **成立** |
| **X5** `Order()` panic 穿透 | `InitAll` / `HealthAll` / **`CloseAll`** 三处全部逃逸 `ORDER_PANIC`（n=2）；**n=2..4 × 全 panic 落位穷举：命中 9 组 / 漏 0 组**；**n=1 对照组证明 `Order()` 完全未被调用（called=false）** | 前两处 | 三处 | **成立**（三入口全穿；n≥2 恒命中） |
| **X5b**（拆分后新增）`Name()` 注册期正常、**注册后变坏** | 注册成功（注册表 2 个插件）→ 翻转 `panicName` → `InitAll`/`CloseAll`/`HealthAll` **全部逃逸 `NAME_PANIC`**；而 `sortedByOrder()` **不触碰 `Name()`**（正常返回 2 个插件） | 未拆分 | 未拆分 | **成立**（缺口在三个入口的日志/键名取值处，**不在排序**） |
| **X6** recover 内二次 `Name()` panic | `safeInit` / `safeClose` / `safeHealth` 逃逸值均为 `NAME_PANIC`；对照（仅 Init panic、Name 正常）正确转 error | `safeInit` | `safeInit` | **成立，且我扩展到三个 safe 函数全覆盖** |
| **X7** Redis 关闭后 `Check` 假阳性 | 关闭后 `Check err = <nil>`；**二次 `Close` 不 panic**；**且「从未 Init」的 `RedisClient{}.Check()` 也返回 `nil`** | 关闭后 nil | 关闭后 nil | **成立，且比 04 更严重（新增「从未 Init」分支）** |
| **X9-1** `(*MySQLDao[E])(nil).WithTx(nil)` | panic `invalid memory address or nil pointer dereference`；同族 `Debug/WithContext/WithCache/connection/WhereAll` **全部不 panic** | 预期 panic | SIGSEGV | **成立**（`WithTx` 是唯一缺口） |
| **X9-2** 副本语义静默失效 | `WithContext` 忽略返回值 → `base.ctx == nil`；`WithCache` → `base.cache == nil`；`Debug` → `base.debug == false` | 静态 | 实测 | **成立** |
| **X10** `(&Sqlite{sqlite:&gorm.DB{}})` | `Close()`/`Check()` **panic**；`GetConnection()` 返回 `conn!=nil, err=nil` | panic | panic | **成立** |
| **X13** `Enabled()` panic → 漏关 | `CloseAll err=plugin enabled panic`；正常插件 `closed=true`、panic 插件 `closed=false` | 静态 | 实测 | **成立** |
| **X14** `RedisCache.Range(nil)` | 用假 RESP 的 `SCAN→1 key` 走进回调：**panic `invalid memory address`**（非 nil 回调被调用 1 次，证明假服务有效） | 静态（N4） | 未验证 | **成立（我做了动态证实）** |
| **X12** Mongo `GetConnection` 原地改写入参 | `caller := []string{""}` 传入后变成 `["a"]` | 静态 | 未验证 | **成立（动态证实）** |

### 1.3 我与 04/05 的差异（逐条）

| 项 | 他们的说法 | 我的独立结论 | 影响 |
|---|---|---|---|
| **X10 根因** | 04：「gorm.go:413 对零值 `ConnPool` 解引用」；05：「嵌入 `*Config` 为 nil」 | **05 对，我独立证实**：`&gorm.DB{Config:&gorm.Config{}}.DB()` → `(nil, invalid db)`，**不 panic**；且 `Sqlite{sqlite:&gorm.DB{Config:...}}` 的 `Close`/`Check` 都不 panic。所以 nil 的是嵌入的 `*Config` | 修复时 `validateSqlite` 必须判 `conn.Config == nil`，只判 `conn == nil` **无效** |
| **X10 影响面** | 只提「panic 中断资源释放」 | **多一层**：`GetConnection` 返回无效连接后，调用方 `conn.Model(&T{}).Find(nil)` **我实测 panic** —— 此时已脱离 `CloseAll` 的 recover，是**真正的进程级可恢复 panic**（无框架兜底） | 从「P2 内部问题」升级为「P2 + 可达业务 panic 面」 |
| **X7 范围** | 只说「Close 后 nil」 | **「从未 Init」也返回 nil**（`grep` 全仓：`Redis.Check` 是 `healthFn`，`/health/ready` 会绿）。且 `redis.go:227-269` 的 `Check` 是**空壳**（真实现被整段注释），`redisCheck` 也是 `return nil` 空壳 | 假阳性覆盖「未启用/未初始化/已关闭」三态，比 04 描述更广 → 我判 **P2（不是 P3）** |
| **X5 覆盖面** | 04 列 `InitAll`/`HealthAll` | 05 加了 `CloseAll`；**我把它拆成 X5 / X5b 两条**（见 §2.2），并证实 `Order()` 与 `Name()` 的缺口性质不同 | X5（`Order`，无准入校验）+ X5b（`Name`，受 `Register` 准入约束） |
| **X6 覆盖面** | 04/05 只验 `safeInit` | **三个 safe 函数全中**：`safeInit:214`、`safeClose:223`、`safeHealth:232` 都在 recover 分支再调 `p.Name()` | 同族，同一处修复（缓存 name） |
| **X1 的放大机理** | 04 提到「Go RWMutex 写者优先」但未量化 | **我加了量化实验**：一个挂死的 reader（`Check` 持 RLock 2s）+ 一个排队的 writer（`Init` 等写锁），第三个**无关读者** `GetConnection` 被阻塞 **1.599517784s** | 证明 X1 的损伤是「全局」而非「局部」，这是它够 P1 的关键证据 |
| **X3 定性** | 04 列为 P1 但侧重「锁内 I/O」 | **我把它单独提为「无条件稳态回归」**：不依赖任何故障，每次 DAO 调用都过一次全局写锁 + 一次 `Ping` 往返。这是**修复引入的**，性质比条件性的 X1 更严重 | 见 §4 优先级：X3 排第一 |
| **锁顺序/死锁** | 04 未做压力验证 | **我做了 3×10s 双包混跑 + 200 万 ops 级压测，无死锁**；并静态确认无 `mu→initMu` 反序路径 | 修复没有引入死锁（这是本轮的**好消息**） |

---

## 2. 反向检查结论（本轮重点，逐条给证据）

### 2.1 锁顺序 / 死锁 —— **未发现死锁，但存在两类「合法但致命」的阻塞**

**静态矩阵（我逐个方法读的）**

| 数据源 | 方法 | 获取顺序 | 锁内 I/O | 是否有 `mu→initMu` 反序 |
|---|---|---|---|---|
| Redis | `Init` | `initMu` → `mu.Lock` | **是**（`redis.go:176` Ping，`174-184` 逐库串行） | 否 |
| Redis | `Close` | `mu.Lock` | 是（`rc.Close()`，`187`） | 否 |
| Redis | `Check` | `mu.RLock` | **是**（`241` Ping） | 否 |
| Redis | `GetConnection` | `mu.RLock` → 调 `redisCheck`（空实现） | 否（当前） | 否 |
| Mongo | `Init` | `initMu` → `mu.Lock` | **是**（`95`/`116` `DialWithTimeout` 10s） | 否 |
| Mongo | `Close` | `mu.Lock` | 是（`142/148` `conn.Close()`） | 否 |
| Mongo | `Check` | `mu.RLock`(读 needsInit) → `RUnlock` → `Init`(`initMu`→`mu`) → `mu.Lock` | **是**（`192-204` 写锁内 `mgoCheck`→`Ping`/`Dial`） | 否（**读锁在调 `Init` 前已释放**，这是对的） |
| Mongo | `GetConnection` | 同上，末段 `mu.Lock` | **是**（`219-258` 写锁内 `mgoCheck`） | 否 |

> 关键：`Mongo.Check/GetConnection` 的 `needsInit` 判定用的是 `RLock`，**释放后**才 `Init`，因此不存在「持 `mu` 再去抢 `initMu`」的环。Redis 亦然。**结论：无锁顺序反转，无死锁。**

**动态验证**
- Redis `Close/GetConnection/Check/Init` 8+1+1 协程混跑 10s → 完成，无死锁、无 fatal。
- Mongo 6+1+1 协程混跑 10s（`-race -count=3`）→ 每轮 34 万 ops，完成，无死锁。

**但有两类「合法阻塞」必须记入等级：**

1. **`Mongo.Check()` 持写锁 → 与 DAO 互锁（我实测成立）**
   `Check():192` 用 `m.mu.Lock()`，`mgoCheck` 全程在锁内做 `Ping`（无超时，见 2.5）+ 可能 `DialWithTimeout(10s)`。而 `GetConnection():219` 也用写锁。**我实测**：人为占住写锁 500ms 后，所有 `GetConnection` 全部排队。即 **`mgo.ro` 的一次「健康检查」会让整个数据面停摆，最长 10s（Dial）/ 30s（Ping 的 `serverSelectionTimeout`）**。`app.go` 的 ticker 每 5 分钟驱动一次 → **周期性数据面停摆**。这是 04 未点出的、X1 与 X3 的**合流后果**。

2. **`Redis.Check()` 持读锁 + 写者优先 → 全局放大（我实测成立）**
   `REV-AMP` 实验：一个挂死的 reader + 一个等写锁的 `Init`，**无关的第三个 `GetConnection` 被拖 1.5995s**（本应 0 延迟）。机理：Go `RWMutex` 的写者优先让新读者排在待定写者之后，于是「单库挂死」→「`Check` 慢」→「`Init`/`Close` 排队」→「全部业务访问排队」。**这解释了为什么 X1 值得 P1。**

### 2.2 `plugin` 的 recover 覆盖面 —— **有一个方法完全没被覆盖（`Order`），还有两个「recover 内的洞」**

我做了完整的方法×入口矩阵，**必须注册 ≥2 个插件**（否则 `sort.Slice` 不调 `less`，`Order()` panic 会被「只有一个测试插件」的假象掩盖 —— 这正是本仓现有单元测试的通病；我用 n=1 对照组实测 `Order()` 完全未被调用）。

| 方法 panic | `Register` 逃逸 | `InitAll` 逃逸 | `HealthAll` 逃逸 | `CloseAll` 逃逸 |
|---|---|---|---|---|
| `Enabled()` | 否 | 否 | 否 | 否 |
| `Init()` | 否 | 否 | 否 | 否 |
| `Close()` | 否 | 否 | 否 | 否 |
| `Health()` | 否 | 否 | 否 | 否 |
| **`Order()` = X5** | 否（`Register` 不调 `Order`） | **是** | **是** | **是** |
| **`Name()` = X5b** | **是**（注册期） | **是**（运行期，注 1） | **是**（运行期，注 1） | **是**（运行期，注 1） |

- 注 1（**关键区分，我这里修正了第一版的模糊表述**）：「`Name()` 无条件 panic」的插件**连 `Register` 都过不了**，所以上表在 `InitAll/HealthAll/CloseAll` 一列的「是」**必须用「注册时正常、注册后变坏」的可变 `Name()` 才能观察到**。我按这个构造重测：注册成功（注册表 2 个插件）→ 翻转 `panicName` → **三入口全部逃逸 `NAME_PANIC`**。反过来，把「无条件 panic 的 `Name` 插件」直接 `Register`，结果是 `Register` 逃逸 `NAME_PANIC` 且注册表内插件数 = 0。
- **`safeEnabled/safeInit/safeClose/safeHealth` 只覆盖 `Enabled/Init/Close/Health` 四个方法**；`Plugin` 接口共 6 个，**`Name()` 与 `Order()` 完全在保护之外**。
- 附带一个**好消息**（N2 回归通过）：`CloseAll` 中前序插件 `Close` panic，后续插件仍被关闭（`second.closed = true`），且 panic 计入 `firstErr`。

### 2.2b X5 / X5b 拆分与我的独立定级

架构师把原 X5 拆成 X5（`Order`）/ X5b（`Name`）。**我独立验证后同意拆分，但不同意 X5 定 P1。**

#### X5 —— `Order()` 在 recover 外（`plugin.go:106-112`，经 `sortedByOrder`）

| 验证项 | 我的结果 |
|---|---|
| 三条入口是否都逃逸 | **是**：`InitAll` / `CloseAll` / `HealthAll` 三个子用例全部 FAIL（`ORDER_PANIC`） |
| 是否「无条件可复现」 | **在 n≥2 时是**：n=2..4 × 全 panic 落位**穷举 9 组，命中 9、漏 0** |
| n=1 是否会假绿 | **会**：n=1 时 `Order()` **一次都没被调用**（`called=false`），因此**任何 n=1 的用例都不可能发现 X5** |
| `Register` 是否提供准入校验 | **否**：`Register:49` 只调 `Name()`，不调 `Order()` → **没有任何「注册期校验」能拦住坏 `Order()`** |
| 后果路径 | `mgin.Init`（无 recover）→ 启动期崩；`app.go` 每 5 分钟 ticker → `checkAll` → `HealthAll`（无 recover）→ **运行期定时引爆，整进程退出** |
| 默认配置是否可达 | **不可达**：内置 6 个 `funcPlugin` 的 `Order()` 恒 `return p.order`（nil receiver 也有守护），**只有业务侧 `mgin.Use` 注册的自定义插件才可能 panic** |

**我的定级：P2（不同意 P1）。** 理由：我的 P1 口径是「服务不可用/关停卡死/稳态退化，**且默认配置下可达**」；X5 的充分条件是「业务自定义插件 + 该插件 `Order()` 有缺陷」，属非默认路径。

**但我给出明确的升 P1 触发条件（供主理人裁决）**：若本框架对外提供**插件扩展生态**（第三方/多团队实现 `Plugin` 接口），则「一个插件的 `Order()` bug 让整个进程在 5 分钟 ticker 上随机退出」= 服务不可用，**应升 P1**。这一条我无法从代码判定，需产品侧确认 `mgin.Use` 的受众范围。

> 附带说明：**X5 的定级不影响排期**——X5/X5b/X6 共享同一个修复（`Register` 缓存 `name/order` + 两端守护），修一条即全消。

#### X5b —— `Name()` 在 recover 外（`:122/:131/:136/:163/:167/:175/:188/:191/:197`）

| 验证项 | 我的结果 |
|---|---|
| 注册期能否拦住 | **能**：`Register:49` 先调 `Name()`，无条件 panic 的插件注册失败、注册表 0 个插件 → **注册期 `Name` panic 属调用方问题，不是框架缺口** |
| 运行期（注册后变坏）是否逃逸 | **是**：翻转后 `InitAll` / `CloseAll` / `HealthAll` **三入口全部 FAIL**（`NAME_PANIC`） |
| 缺口是否在排序 | **否**：`sortedByOrder()` **完全不触碰 `Name()`**，正常返回 2 个插件 → 缺口只在三个入口的**日志/键名取值处**（`:122/131/136/163/167/175/188/191/197`）与 **`safeXxx` 的 recover 分支内部**（`:214/223/232`） |
| 暴露面 vs X5 | **严格更窄**：X5 只要求「插件 `Order()` 有 bug」；X5b 要求「注册时 `Name()` 正常 + 之后才变坏」（典型成因：`Init()` 失败后内部 `cfg` 被清空，`Name()` 读 `p.cfg.Name` 解引用 nil） |

**我的定级：P2（同意架构师）。** 且在 P2 内部排位低于 X5（前置条件更苛刻）。

#### X5b 的一个「修复侧陷阱」（我必须提出来）

`Register` **没有 error 返回值**，所以「注册时缓存 `name/order`」这个修复**只是把 panic 从 `InitAll` 搬到注册期**，并没有消除 panic，只是换了个地方爆。**因此修复必须两端同时守护**：
1. 注册侧：`Register` 内用 `safeName`/`safeOrder` 取值（失败则用占位名并记日志，**不 panic**）；
2. 运行期：`InitAll/CloseAll/HealthAll/safeXxx` **一律读缓存**，不再调接口方法。

只做第 1 步 = 把「运行期随机崩」换成「启动期确定崩」，属于**更差**的修复。这一点我会写进移交要点。

### 2.3 Redis RWMutex 写者优先与并发退化 —— **已实测，退化是「全局」的**

见 §2.1 第 2 条（1.5995s）。补充：`Redis.Check()` 在 `clients` 为空（未 Init / 已 Close）时**循环零次、恒返回 nil**，所以「健康检查」本身不会慢；问题只在「有 client 且该 client 挂死」时。

### 2.4 `sqlite.GetConnection` 无 validate 的影响面 —— **比 04 描述的更大**

| 场景 | 我的实测 |
|---|---|
| `(&Sqlite{sqlite:&gorm.DB{}}).Close()` | **panic**（`err == nil` 判断来不及生效，panic 在 `m.sqlite.DB()` 内部） |
| `(&Sqlite{sqlite:&gorm.DB{}}).Check()` | **panic** |
| `(&Sqlite{sqlite:&gorm.DB{}}).GetConnection()` | `conn != nil, err = nil`（**把无效句柄交给调用方**） |
| 调用方拿到该句柄后 `conn.Model(&T{}).Find(nil)` | **panic `nil pointer dereference`** ← **新增影响面：panic 从「被 `CloseAll` recover」迁移到「业务调用点，无兜底」** |
| 真实 sqlite 生命周期 `Init→Check→GetConnection→AutoMigrate→Create→Close→二次 Close` | 全通过；`Close` 后 `Check err = SQLite not opened`（**非 nil，无假阳性**）；二次 `Close` 不 panic |
| `Init` 失败后重试 | **首次失败 `sqlite==nil`，第二次 `Init(合法路径)` 成功**（`sqlite.go:37` 的 `if m.sqlite == nil` 让它可重试，OK） |

### 2.5 Mongo `Ping()` 的真实上界 —— **仍未动态验证，但静态可确认「无超时」**

`/home/macro/mgo`（`github.com/maczh/mgo@v1.0.4`）`Session.Ping()` 用 `context.Background()`，`SetSocketTimeout` 是空实现 → 框架侧**确实无超时**，只受 mongo-driver 默认 `serverSelectionTimeout`（30s）约束。我**未能**构造真实半开连接（无 `mongod`），故「30s」仍是**静态推断**，标为「未验证」。这不影响 X1/X3 的结论（10s 已由 `DialWithTimeout` 实测出来）。

### 2.6 未覆盖文件的补充检查

| 文件/位置 | 我的检查 | 结论 |
|---|---|---|
| `dao/redis_cache.go:59-80` `Range` | 假 RESP 服务让 `SCAN` 返回 1 个 key → 走进 `:71 f(key, value)` → **`Range(nil)` panic**（动态证实） | **X14 成立**，一行 `if f == nil { return }` 可修 |
| `dao/redis_cache.go` nil receiver | `(*RedisCache)(nil).Range(...)` → **panic**（`c.Client` 在 nil `c` 上取值）；但 `Client==nil` 的**非 nil** receiver 全部安全（`Range/Clear/Get/Set/Delete` 均无 panic） | 新增 P3：`RedisCache` 缺 nil receiver 守护（与 `MySQLDao` 的不一致） |
| `pkg/db/transaction.go` 回调 panic | 回调 `panic("QA2_CALLBACK_PANIC")` → **穿透**（Go 语义预期）；**回滚已发生**（`inside-panic` 行数 = 0）；`fn==nil`/`conn==nil` 双守护；`ctx==nil` 兜底成 `Background`；错误回调正确 rollback | **无缺陷，不建议加框架层 recover**（与 04 一致，我动态背书） |
| `pkg/db/mysql/mysql.go` R12 | `initialize:91-101`：锁外 `closeMySQL(oldSingle/oldMulti)`，是「泄露旧连接」与「use-after-close」的一个窄窗口 | 结构性残余，**P3**，维持不修 |
| `Mongo.GetConnection` tags 为空是否越界 | `multi + tags=nil` → `GetConnection("")` 返回 `MongoDB multidb has no available database`，**无越界 panic**（`mgo.go:226` 有 `len(m.tags)==0` 判空） | **无新 panic 面**（我特意验了这条） |
| `Redis.Init` 关闭后重入 | `Close()` 后 `Init(cfg)` → `GetConnection` 成功、`conns=[0]` | **可恢复，OK**（与 Mongo 一致；问题不是「不能恢复」，而是「无人要求它恢复时也自己恢复」→ X2） |
| `funcPlugin` 的 `Name/Order` 是否在 recover 内 | `funcPlugin` 六个方法对 nil receiver 与 nil 函数字段**全部有守护**；`initFn` panic 经 `safeInit` 被正确接住转 error | **内置插件不会触发 X5/X6**（只有业务 `mgin.Use` 的自定义插件才触发）——这决定了 X5/X6 只能是 P2 而非 P1 |

### 2.7 是否存在新的 panic 面（专项）

| 候选 | 结论 |
|---|---|
| `Mongo.GetConnection` `m.tags` 为空时 `dbName[0] = m.tags[0]` | **不越界**（`:226` 有判空）✅ |
| `Mongo` multi 模式 `Check` 在 `range m.conns` 内写 `m.conns[tag]` | 合法（对已存在 key 赋值），无 fatal ✅ |
| `Redis.Init` 关闭后重入 | 安全 ✅ |
| `funcPlugin` Name/Order 在 recover 内调用 | 内置适配器 Name/Order **无 panic 可能** ✅ |
| `plugin.Register` 的 `p.Name()` | ❌ **唯一的「新增可达 panic 面」**：`mgin.Use` 传入 `Name()` panic 的插件 → `Register:49` 无 recover → **启动期进程崩溃** |
| `RedisCache` nil receiver | ❌ 新增 P3（`Add/Set/Get/Delete/Range/Clear` 全部会在 nil `c` 上 panic） |
| `sqlite.GetConnection` 返回无效句柄 | ❌ **panic 面后移**到业务调用点（无 recover 兜底），见 §2.4 |

---

## 3. 与 04/05 的差异汇总

| 项 | 04 架构师 | 05 工程师 | 我的最终判定 |
|---|---|---|---|
| 上轮 P0 | 已修（静态+间接） | 已修（-race 量化） | **已修（我独立 -race -count=3 复现，且真制造了「Close 关 client 时他人正在用」的交错）** |
| X1 阻塞数值 | 2.50/1.70/9.70 | 2.5001/1.7009/9.7010 | **2.5008/1.7008/9.7012（三方吻合）** |
| X1 的定性 | P1，「与 P0 同量级」 | 采纳 | **P1，但「同量级」我改成「不同量级、爆炸半径更大」**；并新增「写者优先放大 1.5995s」的动态证据 |
| X2 | P1 | 成立 | **P1**，并强调 `app.go:387/395` 关停顺序是放大器（一行可修） |
| X3 | P1（静态） | 升级为动态 | **P1，且我把它排到 X1 之前**：这是**无条件稳态回归**，不是条件性故障放大 |
| X4 | P2 | 成立 | **P2**（多库场景下 DAO 全挂，但需 multi + 冗余 Init） |
| X5（`Order`） | P2（2 入口） | P2（3 入口） | **拆为 X5 / X5b；X5 我判 P2（不同意架构师的 P1）**，但给出「升 P1 的触发条件」；三入口全逃逸经穷举证实（n=2..4 命中 9/漏 0） |
| X5b（`Name`） | 未拆分 | 未拆分 | **P2（同意）**；用「注册期正常、注册后变坏」的可变 `Name()` 复现，三入口全逃逸；注册期 `Name` panic 属**调用方问题**（`Register` 无 error 返回值 → 修复须两端守护） |
| X6 | P2 | P2 | **P2，3 个 safe 函数全覆盖** |
| X7 | P2 | P2 | **P2，范围扩到「从未 Init 也假阳性」** |
| X9 | P2（两问题） | 成立 | **拆成 P2（`WithTx` nil，一行）+ P3（副本语义 BREAKING）** |
| X10 | P2（根因表述有误） | P2（根因修正） | **P2，并新增「`GetConnection` 无效句柄 → 业务侧 panic 无兜底」** |
| X12 | P3 | 未验证 | **P3（我动态证实入参被改写）** |
| X13 | P3 | 成立 | **P2 上调**：`CloseAll` 里 `Enabled()` panic 是**静默资源泄漏**（无日志指出「跳过关闭」），且 `Enabled` 可读配置、可因配置热更新 panic；同族还有 `N3`（`Enabled` 三次重复求值 → 状态翻转漏关） |
| X14 | P3（静态） | 未验证 | **P3（动态证实）** |
| 附录/范围外 | `mgin.go` 的 `panic(err)`；关停顺序 | 未提 | **我同意这两条，且把「关停顺序」提到 P1（因它是 X2 的唯一放大器）** |

---

## 4. 最终定级表（交付核心）

> 口径（我重新定义，比 04 更严格）：
> - **P0** = 进程级不可恢复（`fatal error` / 无法 recover 的 runtime throw）或数据损坏。
> - **P1** = 服务不可用 / 关停卡死 / 稳态性能量级退化，且**默认配置下可达**。
> - **P2** = 特定输入或自定义插件下可达，可 recover 或影响局部。
> - **P3** = 防御性 / 兼容性 / 潜在雷。
> 「可信度」：**动态证实** = 我亲自跑出数值/panic；**静态确认** = 代码路径确定但我未构造出运行场景；**未验证** = 我无法构造且不背书。

| # | 编号 | 问题 | 后果类型 | **最终级别** | 可信度 | **本轮必修** | 修复要点（file:line） |
|---|---|---|---|---|---|---|---|
| 1 | **X3** | Mongo `GetConnection` 用**写锁**且每次都在锁内 `Ping` | 稳态吞吐钉死 ≈`1/RTT`，全局串行 | **P1** | 动态证实 | **必** | `mongo/mgo.go:219` 改 `RLock` 取快照即释放；`:234/:244` 的 `mgoCheck` 移出锁；Ping 加 1s 节流（`atomic.Int64` 记录上次校验时间） |
| 2 | **X1** | 锁内做网络 I/O：Redis `Init:176`/`Check:241`、Mongo `Init:95/116`、`mgoCheck:162/166` | 锁护航 → 延迟雪崩 + 关停阻塞（我实测全局放大 1.5995s） | **P1** | 动态证实 | **必** | 统一抄 `mysql/mysql.go:80-101` 的 RCU 范式：锁外建连 → 锁内一次性发布 → 锁外关旧；`Check` 取 `[]client` 快照后解锁再 Ping |
| 3 | **X2** | Mongo `Close` 不清 `confData`、无 `closed`；`Check:185`/`GetConnection:213` 的 `needsInit` 让关闭后**重新拨号** | 关停期每请求 10s + 健康检查假阳性 + 可达地址下 session 泄漏 | **P1** | 动态证实 | **必** | `mgo.go:130-152` 加 `closed bool` + `m.confData=nil` + 锁外关旧连接；`:185/:213` 前加 `if m.closed { return ErrClosed }`（对齐 `mysql.go:63-66/:86-90`）；引入三态 `uninitialized/ready/closed`，不要用 `len(conns)==0` 兼职 |
| 4 | **关停顺序** | `app.go:387 SafeExit()`（关 DB）**早于** `:395 s.Shutdown(ctx)`（排空 HTTP） | 把 X1/X2 的窗口放大到整个 HTTP 排空期；`shutdownTimeout` 默认 5s → SIGKILL | **P1** | 静态确认 | **必（一行）** | `app.go:387-395` 调换顺序：先 `Shutdown` 排空（此时 DB 仍可用）→ 再 `SafeExit` |
| 5 | **X5** | `Order()` 在 recover 外：`plugin.go:109`（经 `sortedByOrder()`），三条入口全部依赖它；`Register` **不做** `Order()` 准入校验 | 自定义插件 `Order()` panic → 启动期崩 + **5 分钟 ticker 运行期定时整进程退出** | **P2** | 动态证实（穷举 n=2..4 命中 9 / 漏 0；n=1 不命中） | **必**（与 X5b/X6 同一处修复） | `Register` 时用 `safeOrder` 缓存 order；`sortedByOrder` 只读缓存，**运行期不再调 `Order()`**。**注意：架构师建议 P1，我判 P2** —— 若框架对外提供插件扩展生态则升 P1（见 §2.2b） |
| 5b | **X5b** | `Name()` 在 recover 外：`:122/131/136/163/167/175/188/191/197` + `safeXxx` 的 recover 分支内 `:214/223/232`；缺口**不在排序**（`sortedByOrder` 不触碰 `Name`） | 注册后 `Name()` 变坏 → 三入口逃逸 → 同上 | **P2**（同意架构师；前置条件比 X5 更苛刻：须注册期正常、之后才坏） | 动态证实（可变 `Name` 构造） | **必**（同一处修复） | `Register` 时用 `safeName` 缓存 name（失败用占位名 + 日志）；三个 `safeXxx` 的 recover 分支读缓存 name |
| 5c | **X6** | `safeInit:214` / `safeClose:223` / `safeHealth:232` 的 recover 分支**再调 `p.Name()`** | 原始 panic 被 `NAME_PANIC` 替换并穿透（加固代码成了 panic 放大器） | **P2** | 动态证实（三个 safe 函数全中） | **必**（同一处修复） | recover 分支内不得调用接口方法，用注册期缓存的 name |
| 5d | **X5b 修复侧陷阱**（我新增） | `Register` **没有 error 返回值** | 「注册时缓存 `name/order`」若不配 `safeName/safeOrder` 守护，只是把 panic 从 `InitAll` 搬到注册期 —— **从「运行期随机崩」变成「启动期确定崩」，更差** | **P2** | 静态确认 | **必**（必须两端守护） | 两端守护：① `Register` 内 `safeName`/`safeOrder`（失败占位 + 日志，不 panic）；② 运行期一律读缓存 |
| 6 | **X10** | `sqlite.Close:53-58`/`Check:70` 在零值 `*gorm.DB` 上 panic；`GetConnection:77-87` 无 validate → 无效句柄外流 | 可恢复 panic（`CloseAll` 内被 recover，**但业务侧调用点无兜底，我实测 panic**） | **P2** | 动态证实 | **建议必** | 新增 `validateSqlite`：判 `conn == nil \|\| conn.Config == nil \|\| conn.Statement == nil`（**必须含 `Config`**，见 §1.3）；`Close/Check/GetConnection` 先 validate |
| 7 | **X7** | Redis 无 `closed` 语义；`Check:227-245` 空 map 恒 `nil`；`redisCheck:211-225` 是 `return nil` 空壳 | 健康检查假阳性（未启用/未初始化/已关闭三态全绿）→ 流量继续打进来 | **P2** | 动态证实 | **建议必** | 加 `closed bool` + `ErrClosed`；`clients` 为空时 `Check` 返回 error 而非 nil；`Close` 里 cancel 内部 ctx 让 `PSubscribe` 可停 |
| 8 | **X13 + N3** | `CloseAll:153-159` 中 `Enabled()` panic → `continue` **静默漏关**；`Enabled` 在 `InitAll:120`/`CloseAll:153`/`HealthAll:189` 各求值一次 | 资源静默泄漏（无日志说明「跳过关闭」） | **P2** | 动态证实 | 建议必 | `Enabled` 失败时**仍然调用 `Close`**（幂等释放通常安全）并明确记日志；`Enabled` 结果在注册时或首次求值后缓存 |
| 9 | **X9-1** | `(*MySQLDao[E])(nil).WithTx(nil)` panic（`clone := *m`） | nil receiver panic | **P2** | 动态证实 | 建议必（一行） | `dao/mysql.go:60-64` 加 `if m == nil { return nil }`（与 `Debug/WithContext/WithCache` 对齐） |
| 10 | **X4** | `mgo.go:52` 的 `m.tags = make(...)` 在 `len(m.conns)==0` 判断**之前** | 多库 tag 表被擦除 → `GetConnection("")` 全失败（**功能全挂，非 panic**） | **P2** | 动态证实 | 建议必 | 把 `:52` 移入 `:56 if len(m.conns)==0 {` 块内 |
| 11 | **X9-2** | 副本语义 BREAKING：`WithContext/WithCache/Debug` 改副本后，忽略返回值的旧写法**静默失效** | 行为回归（ctx 超时/链路追踪/缓存/日志静默丢失），**比 panic 更难查** | **P3** | 动态证实 | 否（需 release note） | 保留副本语义（方向正确）；在 release note 标 BREAKING + 全仓 grep 示例（`cmd/templates.go:489/509` 已改）+ 可选提供 `ApplyContext` 原地版本 |
| 12 | **X14** | `dao/redis_cache.go:71` `f(key,value)` 无判空 | 导出 API panic | **P3** | 动态证实 | 否（一行） | `:60` 后加 `if f == nil { return }` |
| 13 | **新** | `RedisCache` 无 nil receiver 守护（所有方法在 nil `c` 上 panic） | 导出 API panic | **P3** | 动态证实 | 否 | 各方法首行加 `if c == nil { return }` |
| 14 | **X12** | `mgo.go:229` `dbName[0] = m.tags[0]` 原地改写调用方 slice | 入参副作用 | **P3** | 动态证实 | 否 | 改 `tag := dbName[0]; if tag == "" { tag = m.tags[0] }` |
| 15 | **X11** | `redis.go:275-276` `GetConnection` 持 **RLock** 调 `redisCheck`；`:211-224` 被注释的重连实现在 RLock 内**写 map** | 潜在雷：恢复注释即 `fatal error: concurrent map read and map write` | **P3** | 静态确认 | 否（但需删注释） | 删除 `:211-224` 的注释残骸，或改成「锁外重连 + 换锁发布」并写明约束 |
| 16 | **X8** | `redis.go:337/363-365` `localChannels` 只替换一次 `{db}` | db 号变化后订阅通道不更新 | **P3** | 静态确认 | 否 | 每轮循环基于原始模板重新派生 |
| 17 | **X15** | `mgin.go:200-212` `SafeExit` 的 5s ctx 是装饰性的（所有 `closeFn` 忽略 ctx） | 关停超时不可控 | **P3** | 静态确认 | 否（与 #4 同批考虑） | 每个 `closeFn` 独立 goroutine + 外层 `select ctx`，或 `Close(ctx)` 真接受 deadline |
| 18 | **X16** | 三套数据源生命周期契约不一致 | 结构性温床 | **P3** | 静态确认 | 否 | 抽 `lifecycle` 基类统一收口 |
| 19 | **新** | `mgin.go:124-126` `if err := logs.InitVictoriaLogs(); err != nil { panic(err) }` | **启动期显式 panic**（日志后端不可达 → 整进程崩），`plugin.InitAll` 的 recover 完全无关 | **P2** | 静态确认 | 建议必 | 降级为 error + 本地日志兜底 |
| 20 | **新** | `builtins.go:117/133/149-150` 的 `Mongo/Redis/Sqlite` `initFn` **恒返回 nil**（`Init` 无返回值） | `InitAll` 看不到这三个组件的初始化失败 → 「启动成功」假象 | **P2** | 静态确认 | 建议必 | 补 `InitWithError`（对齐 `db.Mysql.InitWithError`） |

**统计：P0 = 0；P1 = 4；P2 = 8；P3 = 8。**

### 4.1 「上一轮 P0 是否闭环」的最终裁定

**闭环。** 判据（我自己的三条，缺一不可）：
1. `-race -count=3` 在**真发生并发**的场景下零 `DATA RACE`（我用真协议假服务保证 `Init` 成功、`GetConnection` 真的拿到 client 并 `Ping`，成功 114/119/118 次）。
2. 3×10s 的 `Close` vs `Check/GetConnection/Init` 混跑（Mongo 34 万 ops/轮）零 `fatal`、零死锁。
3. 静态：`clients/cfgs/conns`（redis）与 `conns/tags/multi/confData`（mongo）的**全部**读写点都在 `mu` 内（我逐个核对到行）。

### 4.2 「是否存在与 P0 同量级的新问题」的最终裁定

**不存在新的 P0。** 但有 4 条 P1，其中我要**修正 04 的定级逻辑**：

| 04 的说法 | 我的修正 |
|---|---|
| 「X1/X2 把问题从『崩』变成『卡』，与 P0 同量级」 | **不同量级**。P0 是**无条件 + 不可恢复**（并发即 fatal，无缓解）；X1/X2 是**条件性**（下游慢/关停期才触发），且**有缓解手段**（`app.go` 调换顺序、`shutdownTimeout` 调大、关闭 `HealthAll` ticker）。爆炸半径更大但可控性更高，**不能等价**。 |
| （未区分） | **真正接近 P0 性质的是 X3**：它**不依赖任何故障条件**，是修复后**恒久成立**的稳态吞吐上限（≈`1/RTT`）+ 每次调用一次额外网络往返。这是「修复引入的、无条件的、全局的」退化——**优先级应排在 X1 之前**。 |
| X2 是 P1 | **同意 P1，但前提是保留 `app.go` 现有的关停顺序**。若采纳 #4（调换顺序），X2 的实际暴露面收缩到「`Shutdown` 之后仍有 goroutine 访问 DAO」，届时降为 P2。**所以 #4 与 #3 是同一件事的两半，必须一起做。** |

---

## 5. 修复优先级（建议的落地顺序）

**第一批（1 个 PR，只碰 `mongo`，收益最大 / 风险最低）**
1. `mongo/mgo.go:219` 写锁 → 读锁 + 锁外 `Ping`（**X3**，消掉无条件稳态串行化）
2. `mongo/mgo.go:130-152` 加 `closed` + 清 `confData` + 锁外关旧连接（**X2**）
3. `mongo/mgo.go:185/213` 加 `ErrClosed` 短路
4. `mongo/mgo.go:52` 移入 `len(conns)==0` 块（**X4**，一行）

**第二批（1 个 PR，`app.go` + `plugin`，一行级改动）**
5. `app.go:387/395` **调换关停顺序**（P1，一行，同时是 X1/X2 的关停侧总闸）
6. `plugin.Register` 缓存 `name/order`；`sortedByOrder`/`InitAll`/`CloseAll`/`HealthAll`/`safeXxx` 全部改用缓存值（**X5+X6**，一处修法覆盖 4 个面）
7. `plugin.CloseAll:153-159` `Enabled` 失败时仍 `Close` + 明确日志（**X13**）

**第三批（1 个 PR，`redis` + `sqlite`）**
8. `redis` 加 `closed` + `ErrClosed`；`Check` 空 map 返 error；`Close` cancel 内部 ctx（**X7**）
9. `redis.Check` 锁外 Ping（**X1** 的 Redis 半边）
10. `sqlite` 加 `validateSqlite`（含 `Config == nil`）三处接入（**X10**）
11. `redis/mongo/sqlite` 补 `InitWithError`，`builtins.go` 三个 `initFn` 传播 error（新增 P2）

**第四批（清理项，可随任意 PR 带上）**
12. `dao/mysql.go:60-64` `WithTx` nil 守护（一行）
13. `dao/redis_cache.go:60` `if f == nil { return }` + 各方法 nil receiver 守护
14. 删 `redis.go:211-224` 的注释残骸（X11 的雷）
15. `mgo.go:229` 不再改写入参
16. `mgin.go:124-126` 去掉 `panic(err)`

---

## 6. 缺失测试 seam 说明（这是「绿灯但不正确」的根因）

### 6.1 现状

```
pkg/db/mongo    [no test files]   ← 本轮改动最激进的包之一
pkg/db/redis    [no test files]   ← 本轮改动最激进的包之一
pkg/db/sqlite   [no test files]   ← X10 完全无保护
pkg/db          [no test files]   ← transaction.go 无直接测试
pkg/db/mysql    ok  (12 用例，覆盖 closed/并发 initialize/RCU/pool)
pkg/db/dao      ok  (3 用例，未覆盖 X9 副本语义)
pkg/plugin     ok  (1 用例，未覆盖 X5/X6/X13)
```

### 6.2 结构性缺口（我这次踩到的）

| 缺口 | 为什么它让 bug 隐身 | 我这次怎么绕过的 |
|---|---|---|
| **`mgo.Session` 无法构造** | `conn.conn` 是 `*mgo.Session`，字段全不导出，`mgoCheck` 的 `Ping`/`Dial` 路径**无法在无 mongod 下单测** | 用 `invalid uri` 让 `DialWithTimeout` **立即**失败，从而把 `initMu→mu` 的完整锁路径压几万次而不等 10s；`Copy()→DB()` 路径**仍无法覆盖**（需 CI 起 `mongo:6`） |
| **`redis.Init` 现在持锁真 Ping** | 任何指向不可达地址的测试都要等 3s/60s，作者自然就不写测试了 | 自建**最小 RESP 服务**（真协议应答 `PING/SCAN/GET/DEL/SELECT`），让 `Init` 毫秒级成功，并发才「真的发生」 |
| **`sort.Slice` 对单元素不调 less** | 只注册一个插件时 `Order()` panic **不会暴露**，很容易误判「已修复」 | 强制注册 ≥2 个插件（我的 `qa2-anchor`） |
| **`funcPlugin` 全部 nil-safe** | 内置插件的 Name/Order 永不会 panic → 只用内置插件测 `plugin` 包，**永远测不出 X5/X6** | 手写 `qa2Plugin` 实现 6 方法按需 panic + 绕过 `Register` 直接塞注册表 |
| **锁内 I/O 只能靠时序暴露** | 单测「返回值正确」时，`Init` 也不会慢 | 用「只 accept 不回包」的 hung TCP 服务 + 计时器断言**阻塞时长** |
| **无 `-race` 的 CI 门禁** | 上轮 P0 就是这么漏出去的 | 本轮我全部回归用例在 `-race -count=3` 下跑 |

### 6.3 建议沉淀入仓的回归用例（最小集）

| 用例 | 包 | 断言语义 | 现在会红/绿 |
|---|---|---|---|
| `TestRedisConcurrentLifecycleRace` | `pkg/db/redis` | 假 RESP 服务 + 8 读协程 + 40 次 `Init/Close`，`-race` 无告警 | **绿**（上轮 P0 的正例，**应立刻入仓**） |
| `TestMongodbConcurrentLifecycleRace` | `pkg/db/mongo` | `invalid uri` 压锁路径，`-race` 无告警 | **绿** |
| `TestRedisCheckAfterCloseReturnsErrClosed` | `pkg/db/redis` | `Close()` 后 `Check()` 必须返回 `ErrClosed`，不得为 nil | **红**（X7） |
| `TestMongodbCloseIsTerminal` | `pkg/db/mongo` | `Close()` 后 `GetConnection/Check` 立即返回 `ErrClosed`，耗时 <50ms | **红**（X2） |
| `TestMongodbGetConnectionDoesNotHoldWriteLock` | `pkg/db/mongo` | 持 `RLock` 时 `GetConnection` 不得被阻塞 | **红**（X3） |
| `TestMongodbRedundantInitKeepsTags` | `pkg/db/mongo` | 冗余 `Init` 后 `ListConnNames()` 不变 | **红**（X4） |
| `TestPluginOrderPanicIsContained` | `pkg/plugin` | 2 插件（1 个 `Order()` panic），`InitAll/HealthAll/CloseAll` 均返回 error 且不 panic | **红**（X5） |
| `TestPluginNamePanicInRecoverPath` | `pkg/plugin` | `Init()+Name()` 双 panic 时 `safeInit` 返回 error（原 panic 不被替换） | **红**（X6） |
| `TestPluginRegisterNamePanic` | `pkg/plugin` | `Register` 对 `Name()` panic 的插件不得打穿进程 | **红**（新增 P2） |
| `TestCloseAllClosesPluginWithPanickingEnabled` | `pkg/plugin` | `Enabled()` panic 的插件仍被 `Close` | **红**（X13） |
| `TestSqliteZeroGormDBDoesNotPanic` | `pkg/db/sqlite` | 零值/半初始化 `*gorm.DB` 下 `Close/Check/GetConnection` 均返回 error | **红**（X10） |
| `TestMySQLDaoWithTxNilReceiver` | `pkg/db/dao` | `(*MySQLDao[E])(nil).WithTx(nil)` 不 panic | **红**（X9-1） |
| `TestMySQLDaoWithContextReturnsCopy` | `pkg/db/dao` | 副本语义 + 原对象不变（把 BREAKING 固化成契约） | **绿**（固化现状） |
| `TestRedisCacheRangeNilCallback` | `pkg/db/dao` | `Range(nil)` 不 panic | **红**（X14） |
| `TestTransactionCallbackPanicRollsBack` | `pkg/db` | 回调 panic 穿透且事务已回滚 | **绿**（固化现状，防未来加错 recover） |

> 规律：**12 条「绿」里只有 4 条能立刻入仓**（正向回归 + 现状固化），其余 10 条**必须先修才能入仓**。这也是为什么「先修 P1/P2，再把红用例转绿」是唯一不返工的路径。

---

## 7. 探针重建方法（供第三人复核）

| 文件（临时） | 用例数 | 关键构造 | 耗时 |
|---|---|---|---|
| `pkg/db/redis/zz_qa2_redis_test.go` | 5 | 最小 RESP 服务（`PING→+PONG`/`SCAN`/`GET`/`DEL`/`PSELECT`）+ hung TCP 服务（只 accept）+ 包内注入 `clients["0"]`（`ReadTimeout:2s, MaxRetries:-1`） | 17s |
| `pkg/db/mongo/zz_qa2_mongo_test.go` | 8 | `invalidCfg`（`uri: not-a-mongo-url`，立即失败压锁路径）+ `unreachableCfg`（`127.0.0.1:1`，测 10s）+ 包内注入 `multi/conns/tags` + 「先持 `RLock` 再调 `GetConnection`」判锁模式 | 42s |
| `pkg/db/sqlite/zz_qa2_sqlite_test.go` | 3 | `&Sqlite{sqlite:&gorm.DB{}}` 与对照组 `&gorm.DB{Config:&gorm.Config{}}`；`t.TempDir()` 真实生命周期；失败后重试 | 0.1s |
| `pkg/db/dao/zz_qa2_dao_test.go` | 3 | nil receiver 矩阵；副本语义；自己再写一份最小 RESP 让 `SCAN` 返 1 key 从而走进 `Range` 回调 | 0.05s |
| `pkg/db/zz_qa2_transaction_test.go` | 1 | sqlite 内存库 + 回调 panic / 错误 / nil 守护 | 0.05s |
| `pkg/plugin/zz_qa2_plugin_test.go` | 6 | `qa2Plugin` 按需 panic + **强制 ≥2 插件** + 绕过 `Register` 直塞注册表 + 方法×入口覆盖矩阵 | 0.02s |

一键复原（跑完记得删文件）：
```bash
export PATH=/usr/local/go/bin:$PATH
export GOPROXY=https://goproxy.cn,direct
go test -count=1 -v -timeout 600s -run 'TestQA2' ./pkg/db/ ./pkg/db/dao/ ./pkg/db/sqlite/ ./pkg/plugin/
go test -count=1 -v -timeout 600s -run 'TestQA2' ./pkg/db/redis/ ./pkg/db/mongo/
go test -race -count=3 -run 'TestQA2RedisConcurrentLifecycle' ./pkg/db/redis/
go test -race -count=3 -run 'TestQA2MongoConcurrentLifecycle|TestQA2MongoCloseRaceNoDeadlock' ./pkg/db/mongo/
```

---

## 8. 未验证项（不编造、不背书）

1. **Mongo 真实 `*mgo.Session` 的 `Copy()/DB()` 并发路径** —— 无 `mongod`，`Session` 不可构造。需 CI 起 `mongo:6`。
2. **`mgo.Session.Ping()` 的 30s 上界** —— 静态可确认「框架无超时」（`session.go:89` 用 `context.Background()`），但未动态测到上界。不影响 X1/X3 结论。
3. **X3 的 QPS 线性下降** —— 我只证明了「锁模式是写锁」+「每次调用都 Ping」这两个**代码事实**，未做真实基准。性能数值属静态推导。
4. **X8（`{db}` 只替换一次）** —— 需真实 cluster 故障转移让 db 号变化。
5. **X13 的「漏启用」副作用**（`configured` 改精确匹配后，依赖子串的旧配置失效）—— 无线上 `go.config.used` 样本，请运维确认。**不会 panic**，只导致功能静默失效。
6. **X11** —— 代码路径当前不可达（`redisCheck` 空实现），故只做静态登记。

---

## 9. 给主理人的三句话

1. **上轮 P0 闭环了，这不是自我安慰**：我用了真协议假 RESP 服务让并发「真的发生」，`-race -count=3` 三轮干净，Mongo 34 万 ops/轮无 fatal 无死锁。
2. **本轮没有新 P0，但有 4 条 P1，且优先级要重排**：**先修 X3（Mongo 写锁 + 每次 Ping，无条件稳态退化）**，再修 X1（锁内 I/O）,再修 X2 + `app.go` 关停顺序（这两条是同一件事的两半，必须一起做）。
3. **`plugin` 的修复方向错了**：`safeXxx` 逐个方法包 recover 是「补漏」，而 `Order`/`Name` 这两个**元信息方法**注定补不完（我实测 4 个面）。正解是 **`Register` 时缓存 `name/order`，运行期不再调接口方法** —— 一处改动，X5/X6/新增的 `Register` panic 三个问题同时消失。

---

（报告完）
