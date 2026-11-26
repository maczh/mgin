# 架构师第三轮 panic 风险复审报告（核心高风险区 · 全量通读）

- 复审人：高见远（Gao）· 架构师
- 日期：2026-11-25（第三轮）
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，`HEAD = 4f1d9de`，module `github.com/maczh/mgin/v2`
- 环境：go1.25.7（`export PATH=/usr/local/go/bin:$PATH`，`GOPROXY=https://goproxy.cn,direct`）
- 基线：`aaae6a3..4f1d9de` 共 9 个提交（29e2a28 / 6ba617c / b0becba / 792497b / 7fb2617 / 5c17214 / 7fc1f2f / 10fe036 / 4f1d9de），14 文件 +331/−87
- 方法：
  1. **通读当前完整文件**（非只看 diff）：`app.go`、`mgin.go`、`pkg/plugin/{plugin,builtins,plugin_panic_test}.go`、`pkg/db/{db,transaction}.go`、`pkg/db/mysql/mysql.go`、`pkg/db/redis/redis.go`、`pkg/db/mongo/mgo.go`、`pkg/db/sqlite/sqlite.go`、`pkg/db/dao/{mysql,mgo,redis_cache}.go` 及全部新增 `*_lifecycle_test.go`；对照 `git diff aaae6a3..HEAD -- <file>` 理解改动意图，并核对依赖库 `mgo@v1.0.4`、`gorm@v1.31.0` 源码。
  2. 逐条回归上轮 X1–X16 + X5b + 上轮 P0（无锁 map）。
  3. 重点找**修复本身引入的新 panic/死锁/回归**。
  4. 写了 1 个独立 Go 探针（关停 nil-channel 死锁）+ 2 个包内临时测试（Redis Init 后 Close 语义、Init 持锁 Ping），**全部跑完即删**；未改任何生产代码、未 commit。
- 收尾：`git status --short` → 仅 ` M .gitignore`（进入任务前既有）；未改动 01–06 报告。

---

## 0. 结论摘要（先给判断）

### 0.1 三个必答问题

**Q1：本轮是否还有 P0？——没有新的 P0。**
上轮 P0（Redis/Mongo 无锁共享 map → `fatal error: concurrent map read and map write`）**继续闭环**：`clients/cfgs/conns/confData`（Redis）与 `conns/tags/confData`（Mongo）的全部读写点仍在 `mu` 内；`mgoCheck`（唯一 unlocked map 写）已沦为**无调用者的死代码**（`grep` 全仓仅 1 处定义）。没有发现新的、无条件的、进程级不可恢复 fatal 路径。

**Q2：修复是否引入新的高危问题？——是，发现 1 条新的 P1 关停死锁（本轮头号问题）。**
`app.go` 为「等待健康检查 goroutine 退出」新增的 `checkWG.Wait()` 与 `close(checkDone); checkDone = nil` 组合，在「SIGTERM 恰好落在 checkAll 执行窗口」时，让 goroutine 下一轮 select 读到 **nil channel**（`<-nil` 永久阻塞），`checkWG.Wait()` **永不返回** → 进程收 SIGTERM 后卡死，`Shutdown` 与 `SafeExit` 都不会执行。**已用独立探针实测复现**（500ms 超时下确认永久阻塞）。这是 9 个提交里 `10fe036` 直接引入的回归。

**Q3：上轮 P1/P2 是否闭环？——大部分闭环，但 X1 只修了一半，Redis 订阅泄漏仍未闭环。**
- ✅ 已闭环：X2、X3、X4、X5、X5b、X6、X9-1、X10、X13、X14、上轮 P0
- 🟡 部分闭环：X1（Check/GetConnection 已锁外 I/O，但 **Init 仍持写锁 Ping/Dial**，实测 `GetConnection` 被阻塞 2.80s）、X7（假阳性已修，**订阅泄漏仍未修**：`PSubscribe` 恒用 `Background()`，Close 无取消入口）、X16（新增 `InitWithError`，但 Redis/Mongo 的 Close 后 Init 返回 nil 假成功，与 Mysql 的 `ErrClosed` 仍不一致）
- ❌ 未修（维持 P3）：X8、X11、X12（已修，见下，实为闭环）、X15

### 0.2 一句话总览

> 这一轮把「锁内 I/O」修到了**读路径**（Check/GetConnection 快照后解锁再 Ping，做得对），但**写路径 Init 原地未动**（Redis Ping、Mongo Dial 仍在 `mu.Lock()` 里，实测 2.8s 阻塞）；plugin 的 `name/order` 缓存是**教科书式的正解**，X5/X5b/X6 一并消除且注册期二次 panic 被 `safeName/safeOrder` 挡住；sqlite 的 `validateSqlite` 也终于判到了 `Config`，X10 闭环。**唯一的新增 P1 是 app.go 关停等待的 nil-channel 死锁**——一处 `checkDone = nil` 把「等 goroutine 退出」变成了「等一个永不发生的信号」。本轮 P0=0、P1=1（新）、P2=5（3 新 + 2 残）、P3=6。

---

## 1. 上轮 X 系列逐条回归（核心交付 1）

> 证据均为**当前 HEAD 文件**的 `file:line`；结论分：✅闭环 / 🟡部分闭环 / ❌未修 / ⚠️修复引入新风险。

| 编号 | 上轮问题 | 本轮判定 | 当前证据（file:line）与说明 |
|---|---|---|---|
| **P0** | Redis/Mongo 无锁共享 map，`fatal: concurrent map read and map write` | **✅闭环** | `redis.go:19-32` `mu` 覆盖 `clients/cfgs/conns/confData/confData` 全部读写；`mgo.go:17-29` `mu` 覆盖 `conns/tags/confData`。`mgo.go:175-199` 的 `mgoCheck` 含 `m.conns[tag]=`（:192）裸写，但**全仓无调用者**（`grep mgoCheck` 仅 1 处定义）——已是死代码，不构成可达 race，但见 R-08。 |
| **X1** | 锁内网络 I/O（Redis Init/Check、Mongo Init/mgoCheck） | **🟡部分闭环** | **读路径已修**：`redis.go:253-266` Check 快照 clients 后 `RUnlock`，`:275` 锁外 Ping；`mgo.go:216-225` Check 快照 sessions 后解锁，`:234` 锁外 Ping；`mgo.go:256-257` GetConnection 改 `RLock` 且删除 `mgoCheck`（`:276/:289` 直接 `Copy()`）。**写路径未修**：`redis.go:43-46` `Init` 持 `initMu`+`mu.Lock()` 到返回，`:185` 在锁内 `rc.Ping()`——探针实测 `GetConnection` 被阻塞 **2.80s**；`mgo.go:44-47` `Init` 同样持双锁，`:103/:124` 在锁内 `DialWithTimeout(10s)`。见 R-02。 |
| **X2** | Mongo Close 后 `needsInit` 重新拨号复活 | **✅闭环** | `mgo.go:149-156` Close 置 `closed=true` 且 `confData=nil`；`:206-209`/`:246-249`/`:258-260` 三处 `if m.closed return ErrClosed` 早退；`needsInit=len(conns)==0 && len(confData)>0`（`:210/:250`）在 Close 后因 `confData=nil` 恒 false。`Check/GetConnection` 不再调 `mgoCheck`，无重拨号路径。`mongo_lifecycle_test.go` 覆盖 Close 后 ErrClosed。 |
| **X3** | Mongo `GetConnection` 用写锁 + 每次 Ping | **✅闭环** | `mgo.go:256` `m.mu.RLock()` + `defer RUnlock`，`:276/:289` 仅 `conn.conn.Copy()`（本地 `mongo.Connect`，不 Ping），`:280/:293` `copy.DB()`。`mgoCheck` 已从 GetConnection 移除。写锁+锁内 Ping 均消失。 |
| **X4** | 冗余 Init 擦除 `m.tags` | **✅闭环** | `mgo.go:62-63` `m.tags = make([]string,0)` 已移入 `if len(m.conns)==0 {` 块内（对比 diff：原 `:52` 在判断之前）。冗余 Init 不进该分支，tags 保留。 |
| **X5** | `Order()` 在 recover 外，三入口穿透 | **✅闭环** | `plugin.go:61-65` Register 期 `safeOrder(p)`；`:72` 缓存 `order`；`:124-137` `sortedByOrder()` 排序只读 `ps[i].order`（缓存 int），**运行期不再调 `Order()`**。`plugin_panic_test.go` 的 `Order()` panic 插件注册被拒（不进表）。 |
| **X5b** | `Name()` 在 recover 外（注册期/运行期） | **✅闭环** | `plugin.go:56-60` Register 期 `safeName(p)`，panic 则 `:58` 记日志并**不注册**（不进表）；`:72` 缓存 name；`:148/157/162/182/194/205/209/223/229` 全部用 `item.name`；`:198` recover 分支用缓存 `item.name`。运行期插件无 `Name()` 调用点。 |
| **X6** | recover 内二次 `Name()` panic 替换原始 panic | **✅闭环** | `plugin.go:243-268` `safeInit/safeClose/safeHealth` 签名新增 `name string` 参数，recover 分支 `:246/:255/:264` 用形参 `name`（字符串），不再调接口方法。调用方 `:158/:205/:229` 传 `item.name`。 |
| **X7** | Redis 无 closed、Check 假阳性、订阅泄漏 | **🟡部分闭环** | **已修**：`redis.go:30` `closed` + `:37` `ErrClosed`；`:254-261` Check 在 closed/空 clients 时返回 error（不再恒 nil）；`:310-317` GetConnection closed 早退；`redis_lifecycle_test.go` 覆盖。**未修**：`:363-365` `PSubscribe` 恒传 `context.Background()`，`:208-231` `Close()` 不保存/取消任何订阅 ctx，goroutine 在 Close 后 `GetConnection` 拿到 `ErrClosed` → `:385` 每秒打一条错误日志、`:378-382` ctx 永不 Done → **泄漏依旧**；`:405` `pubsub.Receive()` 无超时（X7b 仍在）。见 R-04。 |
| **X8** | `{db}` 只替换一次 | **❌未修(P3)** | `redis.go:375` `localChannels := append([]string(nil), channels...)` 副本，`:401-403` 每轮 `ReplaceAll(localChannels[i], "{db}", n)`——第一轮就把模板里的 `{db}` 抹成数字，后续 db 号变化无模板可替。 |
| **X9-1** | `(*MySQLDao)(nil).WithTx(nil)` panic | **✅闭环** | `dao/mysql.go:61-63` `if m == nil { return nil }`，与 `Debug/WithContext/WithCache/connection` 对齐。 |
| **X9-2** | DAO 副本语义 BREAKING | **❌未修(P3, 预期)** | 副本语义保留（`dao/mysql.go:64-66/102-104/118-120` 均为 `clone := *m`）。属设计决定，需 release note，非 panic。 |
| **X10** | sqlite 零值 `*gorm.DB` 解引用 panic | **✅闭环** | `sqlite.go:24-33` `validateSqlite` 判 `conn==nil \|\| Config==nil \|\| Statement==nil \|\| Error!=nil`，再 `conn.DB()` 判 `db==nil`；`:73/:90/:106` Close/Check/GetConnection 三处接入。已按 QA 修正意见判到 `Config`。`sqlite_lifecycle_test.go` 覆盖 nil receiver/未打开（但未覆盖半初始化 `&gorm.DB{}`，见 T-02）。 |
| **X11** | redis.go 注释残骸（RLock 内写 map 的雷） | **❌未修(P3)** | `redis.go:54-76`、`:233-247`（`redisCheck` 整段注释，含 `r.clients[dbName]=rc`）、`:280-302`、`:330-333` 注释仍在。当前不可达，但恢复注释即复发 P0。 |
| **X12** | Mongo `GetConnection` 原地改写入参 slice | **✅闭环** | `mgo.go:265-270` 用局部 `tag := dbName[0]`；空 tag 时 `tag = m.tags[0]`，不再写 `dbName[0]`。 |
| **X13** | `CloseAll` 中 `Enabled()` panic → 静默漏关 | **✅闭环** | `plugin.go:180-189` `safeEnabled` 出错时仍调 `safeClose(p,ctx,item.name)`（`:183`）并记日志「仍尝试关闭」；`:195-211` 另有 defer/recover 双保险。正常/异常插件都会被 Close。 |
| **X14** | `RedisCache.Range(nil)` 回调 nil panic | **✅闭环** | `dao/redis_cache.go:80` `if c==nil \|\| c.Client==nil \|\| f==nil { return }`；同时 `:21/27/34/42/52/66/102` 全部加了 nil receiver 守护。 |
| **X15** | `SafeExit` 的 5s ctx 装饰性 | **❌未修(P3)** | `mgin.go:203` `ctx, cancel := WithTimeout(5s)` 传给 `plugin.CloseAll(ctx)`（`:211`），但 `builtins.go:103/119/134/149` 的 `closeFn` 均 `func(ctx) error { db.X.Close(); return nil }` 忽略 ctx。 |
| **X16** | 三套数据源生命周期契约不一致 | **🟡部分闭环(P3)** | 进展：三套均有 `InitWithError`（`mysql.go:51-53`、`mgo.go:139-147`、`redis.go:198-206`、`sqlite.go:39-64`）、`closed` 三态；builtins `initFn` 已传播错误（`builtins.go:117/132/147`）。残余：Redis/Mongo `Init()` 在 closed 时 `:48-50`/`:49-51` 静默 return、`initErr` 保持 nil → `InitWithError` 返回 **nil 假成功**（探针确认），而 Mysql 返回 `ErrClosed`（`mysql.go:63-66`）。见 R-05。 |

**回归统计**：✅闭环 13 项（含 P0/X2/X3/X4/X5/X5b/X6/X9-1/X10/X12/X13/X14）｜🟡部分 3 项（X1/X7/X16）｜❌未修 4 项（X8/X9-2/X11/X15，均 P3）｜配置项差异已记录。

---

## 2. 本轮新发现风险（核心交付 2 · 按 P 级排序）

> 后果分类：fatal throw / 可恢复 panic / 死锁长阻塞 / 泄漏 / 行为回归。可信度：静态确认 / 动态证实 / 需动态验证。

### 🔴 R-01（P1，NEW，动态证实）—— app.go 关停 nil-channel 死锁：SIGTERM 后进程卡死，`Shutdown`/`SafeExit` 都不执行

- **位置**：`app.go:117-130`（生产者 goroutine）、`app.go:384-391`（关停端）
- **代码事实**：
  ```go
  // NewApp
  app.checkDone = make(chan struct{})           // :118
  app.checkWG.Add(1)                            // :119
  go func() {
      defer app.checkWG.Done()
      for {
          select {
          case <-app.checkTicker.C: app.MGin.checkAll()   // :125
          case <-app.checkDone:     return               // :126
          }
      }
  }()
  // Run（收到信号后）
  app.checkTicker.Stop()                        // :385
  if app.checkDone != nil {
      close(app.checkDone)                       // :388
      app.checkDone = nil                        // :389  ← 致命
  }
  app.checkWG.Wait()                            // :391  ← 永不返回
  ```
- **触发条件/调用链**：SIGTERM 到达时，若 goroutine 正停在 `:125 app.MGin.checkAll()` 内（checkAll→`plugin.HealthAll()`→Mongo `Check()` 锁外 Ping 可达 30s、Redis Ping 可达 3–60s，见 §3），或在 `:126` select 重新求值前，`:389` 已把字段置 nil。Go 的 `select` **每次进入都重新求值通道操作数**，此时 `case <-app.checkDone` 变成 `case <-nil` → **该分支永久禁用**；`checkTicker.C` 已被 `Stop()`（且 Stop 不关闭通道，`:124` 再无值）。goroutine 卡死 → `:391 Wait()` 永不返回 → `:397-404` 的 HTTP/HTTPS Shutdown 与 `:406 SafeExit()` 全部跳过。
- **后果分类**：**死锁长阻塞 + 资源不释放**（进程级服务不可用，需 SIGKILL；DB 连接、订阅、日志 handler 均不关）。非 fatal throw，但等价于关停卡死。
- **可信度**：**动态证实**。独立探针精确复现该 select 模式：让读取端停在"checkAll"里 → close(done); done=nil → 放行 → 500ms 后读取端仍未返回，打印 `DEADLOCK: goroutine stuck in select with disabled <-nil case`。另注：`app.checkDone` 字段在 goroutine（读，:126）与 Run（写 nil，:389）间**无同步**，同时构成 data race（`-race` 在关停竞态窗口可捕获）。
- **修复建议**：删除 `:389` 的 `app.checkDone = nil`（Run 本就只执行一次，双重 close 不是问题；若担心重复 Run，改用 `sync.Once` 保护 close）。保留通道为已关闭状态，`:126` 的 `<-app.checkDone` 恒立即可读，goroutine 稳定退出。另给 `Wait()` 加超时兜底：`done := make(chan struct{}); go func(){ app.checkWG.Wait(); close(done) }(); select { case <-done: case <-time.After(shutdownTimeout): logs.Error("health checker did not stop") }`，避免坏 Health() 拖死关停。
- **一句话可证伪预测**：在 `checkAll()` 内加 200ms 睡眠、信号在睡眠期到达，**当前代码**必然在 `app.checkWG.Wait()` 挂起且 `SafeExit` 无日志；删除 `app.checkDone = nil` 后同一实验必然在 <250ms 内走完关停。

### 🟠 R-02（P2，NEW，动态证实）—— Redis/Mongo `Init` 仍持写锁做网络 I/O（X1 未修完的一半）

- **位置**：`redis.go:43-46`+`:183-194`、`mgo.go:44-47`+`:103/:124`
- **代码事实**：`Init` 全程 `initMu.Lock()` + `mu.Lock()`（defer 到函数返回）；`redis.go:185` 在锁内 `rc.Ping()`（多库逐库串行，`:183-194`），`mgo.go:103/:124` 在锁内 `DialWithTimeout(10s)`。
- **触发/调用链**：`builtins.go` 的 `initFn` → `InitWithError` → `Init`；或运行期 `Check/GetConnection` 的 `needsInit` 懒初始化（`mgo.go:213-215/:253-255`）再次进入 `Init`。锁内 Ping/Dial 期间，所有 `GetConnection`/`Check`/`Close` 被同一把写锁挡住。
- **后果分类**：**死锁长阻塞 / 可用性退化**（下游慢/不可达时，单库 Redis ReadTimeout 3s、`redis_pool.max>1` 时 DialTimeout 缺省 60s、多库累加；Mongo 单库 10s）。非 panic，但与上轮 X1 同性质，且**上一轮已判"必修"却只修了读路径**。
- **可信度**：**动态证实（Redis）/静态确认（Mongo）**。临时探针：hung TCP（只 Accept 不回包）+ `Init` 于 goroutine，200ms 后主 goroutine 计时 `GetConnection()` → **阻塞 2.8023s**（等于 Init 剩余时长），证明 Ping 在写锁内。Mongo 侧 `DialWithTimeout(10s)` 在 `mu.Lock()` 内为确定代码事实（上轮数值 9.70s 已三方吻合）。
- **修复建议**：抄 `mysql.go:55-103` 的 RCU 范式到两个 Init：`initMu` 内、**不持 mu** 构造新 client/session 并 Ping/Dial（失败关临时连接）→ 拿 `mu.Lock()` 仅做「确认未 closed + 一次性发布 clients/conns/cfgs/confData」→ 解锁 → 锁外关旧连接。特别注意与前一条 `needsInit` 懒初始化的交互：懒初始化也要走同一 RCU 路径。
- **一句话可证伪预测**：hung TCP 场景下将 Init 的 Ping 移到锁外后，`GetConnection` 阻塞应从 ~2.8s 降到 <10ms（立即返回"连接不可用"）。

### 🟠 R-03（P2，NEW-ish，行为回归）—— Redis/Mongo `InitWithError` 在 Close 之后返回 nil 假成功，与 Mysql 的 ErrClosed 不一致

- **位置**：`redis.go:47-50 + 198-206`、`mgo.go:48-51 + 139-147`；对照 `mysql.go:62-66/85-90`
- **代码事实**：`Init()` 开头 `r.initErr = nil` 后 `if r.closed { return }`——**直接 return 且 initErr 保持 nil**；`InitWithError` 随后 `return r.initErr` → nil。
- **触发/调用链**：任何在 `Close()` 之后再次调用 `InitWithError` 的路径（如关停期在途 goroutine、测试重入、 Felix 式重新 Init）。探针实测：`(&RedisClient{}).Close()` 后 `InitWithError(...)` → `err=<nil>`。
- **后果分类**：**行为回归 / 假阳性**（调用方以为重连成功，实际 `clients/conns` 为空，后续所有请求拿 `ErrClosed`；健康与连接状态不可信）。非 panic。
- **可信度**：**动态证实（Redis）/静态确认（Mongo，代码同构）**。
- **修复建议**：两处 `if r.closed { r.initErr = ErrClosed; return }`（mgo 同理），或在 `InitWithError` 内先 `RLock` 判 `closed` 直接返回 `ErrClosed`。与 `mysql.initialize` 的 `:63-66/:86-90` 双检对齐。
- **一句话可证伪预测**：`Close(); err := InitWithError(cfg)` 当前 `err==nil`；修复后必须 `errors.Is(err, ErrClosed)==true`。

### 🟠 R-04（P2，残存，动态证实过）—— Redis `PSubscribe` 订阅 goroutine 在 Close 后仍泄漏并每秒刷错误日志

- **位置**：`redis.go:363-365`（`PSubscribe` → `PSubscribeContext(context.Background(),...)`）、`:376-439`（goroutine）、`:208-231`（Close 无 cancel 入口）
- **代码事实**：`RedisClient` 无 `ctx/cancel` 字段；`Close()` 只置 `closed` 并关 client；goroutine 的退出条件只有 `ctx.Done()`（Background 永不 Done）与 `ch` 关闭。Close 后 `GetConnection` 返回 `ErrClosed` → `:385` 打错误 → `:386-395` 1s timer → 循环往复。`:405` `pubsub.Receive()` 无超时，握手异常分支可永久挂起（X7b）。
- **后果分类**：**泄漏 + 日志风暴**（每订阅每 1s 一条 error；上轮实测 goroutine 数 Close 后不回落）。
- **可信度**：上轮 QA/工程师已动态证实（base 6→7 不回落；`PSubscribeContext` 对照可回落），本轮通读确认**代码未引入 cancel**，结论延续。
- **修复建议**：给 `RedisClient` 加 `subCtx context.Context; subCancel context.CancelFunc`（构造或首次订阅时创建），`Close()` 调 `subCancel()`；把 `PSubscribe` 默认 ctx 指向 `subCtx`；`pubsub.Receive()` 改 `ReceiveTimeout(time.Second)` 并在超时分支检查 ctx；所有退出路径 `defer pubsub.Close()`。
- **一句话可证伪预测**：`PSubscribe` 后 `Close()`，1.5s 后 `runtime.NumGoroutine()` 应回落到基线且错误日志停止增长；当前代码两者都不满足。

### 🟠 R-05（P2，NEW，静态确认）—— 关停顺序虽已改正，但 `checkWG.Wait()` 无超时，坏 Health() 可拖死关停（与 R-01 叠加）

- **位置**：`app.go:384-406`、`mgin.go:170-194`、`mgo.go:234`（锁外 Ping，mgo 无超时）
- **代码事实**：`app.go:397-404` 已把 `Shutdown` 移到 `:406 SafeExit` 之前——**关停顺序修复正确**（DB 在 HTTP 排空后才关，上轮 P1 项闭环）。但 `:391 checkWG.Wait()` 在 `Shutdown` **之前**且无超时；`checkAll → HealthAll → db.Mongo.Check() → conn.Ping()`，`mgo@v1.0.4` `session.go:213-223` 的 `Ping` 用 `context.Background()`、`SetSocketTimeout` 空实现，框架侧无超时（仅 driver 默认 30s serverSelectionTimeout）。
- **后果分类**：**死锁长阻塞**（关停被拖 30s+，超过 `shutdownTimeout` 缺省 5s → SIGKILL）。
- **可信度**：静态确认（mgo Ping 无超时为库源码事实；30s 上界 QA 标"未动态验证"，不影响本条结论）。
- **修复建议**：见 R-01 的 `select+time.After(shutdownTimeout)` 兜底；并给 `Health()` 链路加 context/超时或跳过关停期的健康检查。
- **一句话可证伪预测**：注入一个 `Health()` 睡眠 10s 的插件，当前代码 `Run()` 从收到信号到 `Shutdown` 至少 10s；加超时兜底后应在 `shutdownTimeout`（5s）内继续关停。

### 🟡 R-06（P3，NEW，静态确认）—— 关停期 `mgoCheck` 是唯一残留的 unlocked map 写，虽当前不可达但为未来 P0 埋雷

- **位置**：`mgo.go:175-199`，尤其 `:192 m.conns[tag] = connection{...}`
- **代码事实**：`mgoCheck` 读写 `m.conns` 完全不加锁；上轮修复把它从 `Check/GetConnection` 摘除后变成**无调用者死代码**。
- **后果分类**：潜在 fatal（`fatal error: concurrent map read and map write`）/ 潜在连接泄漏。
- **可信度**：静态确认（无调用者为 `grep` 事实）。
- **修复建议**：直接删除 `mgoCheck` 整个函数（含其 `:183 Ping`、`:187 Dial`、`:192 写 map`）；若未来恢复重连逻辑，必须走"锁外建连 → 锁内换发布 → 锁外关旧"且全程 `mu` 保护。
- **一句话可证伪预测**：删函数后 `grep -c mgoCheck` 应为 0，且不影响任何现有调用点编译。

### 🟡 R-07（P3，残存，静态确认）—— X8 `{db}` 模板一次性擦除、X11 注释残骸、X15 ctx 装饰性

- 三条均维持上轮 P3 判定，证据见 §1 X8/X11/X15 行。不阻塞本轮，但 X11 的注释残骸（含 `r.clients[dbName] = rc` 的 RLock 内写 map）应在下次触碰 redis.go 时删除，避免"恢复注释即复发 P0"。

### 🟡 R-08（P3，NEW，静态确认）—— `mgo.Session.DB()` 内部对 `s.database` 的无锁写在极端并发下是库级 data race

- **位置**：依赖库 `github.com/maczh/mgo@v1.0.4` `session.go:166-184`：`func (s *Session) DB(name string)` 先 `if name != "" { s.database = name }`（**无锁写**），随后 `s.RLock()`；而 `Copy()/Close()/Ping()` 都用同一把 `sync.RWMutex`。
- **触发/调用链**：`mgo.go:280/:293` 对每个请求 `conn.conn.Copy()` 得到新 session 再 `copy.DB(conn.db)`——**每个请求是独立 Copy，通常不共享**；但若某调用方直接对同一 `*mgo.Session` 并发 `DB(不同 name)`，会与自身 RLock 读形成 race。
- **后果分类**：潜在 data race（非本仓代码，难在框架侧修复）。
- **可信度**：静态确认（库源码事实）；**当前框架用法下不可达**（每次 Copy 独立），标 P3 仅作登记。
- **修复建议**：不改本仓；若出现共享 session 用法，改为先 `Copy()` 再 `DB()`。不作为本轮必修。
- **一句话可证伪预测**：并发对同一 session 调 `DB("a")/DB("b")` 在 `-race` 下应报 `s.database` 写竞争；用 Copy 隔离后不报。

### 🟡 R-09（P3，NEW，静态确认）—— `Register` 与 `Unregister`/`All` 的 `plugins` 与 `meta` 双 map 一致性依赖"name 缓存"，但 `All()` 仍走原始 `plugins[n]`

- **位置**：`plugin.go:52-73`（Register 写 `plugins`+`meta`+`order`）、`:89-103`（Unregister 三处删）、`:113-121`（All 用 `order`+`plugins[n]`）、`:124-137`（sortedByOrder 用 `order`+`meta[name]`）
- **代码事实**：三处写都在 `mu.Lock()` 内，读都在 `RLock()` 内，一致性 OK。差异：`All()` 返回 `plugins[n]`（真插件），`sortedByOrder()` 返回 `meta[name]`（含缓存 name/order 的快照）。
- **后果分类**：行为一致性风险（若某条路径只更 `plugins` 不更 `meta`，两者会分叉）——当前代码无此路径，标 P3 登记。
- **可信度**：静态确认（当前无缺陷）。
- **修复建议**：统一 `All()` 也返回 `[]pluginMeta` 或让 `meta` 成为单一真相源；非阻塞。

---

## 3. 修复引入的新 panic/死锁风险专项核查（任务重点逐条回应）

1. **`closed` 三态与 `ErrClosed` 语义是否正确？**
   - `Close` 幂等：✅ 三套 Close 均先判 `closed/nil` 早退（`mysql.go:281-285`、`redis.go:212-214` 置 true 后二次进仍安全、`mgo.go:153-155`）。
   - 关闭后 Init 可否复活：**Redis/Mongo 不可复活（正确）但返回 nil 假成功（错）**——见 R-03。Mysql 正确返回 `ErrClosed`。
   - `ErrClosed` 哨兵：三套均已定义并使用于 `Check/GetConnection`（`mgo.go:38/206/246`、`redis.go:37/254/310`、`mysql.go:35`）。✅
2. **锁外建连/锁内发布 RCU 是否引入 use-after-close？**
   - Mysql 的 RCU（`mysql.go:85-101`）保持上轮纪律：锁内发布后锁外关旧，属已知极窄窗口（上轮 P3，不重复排期）。
   - Redis/Mongo **未采用 RCU**（Init 全程持锁），故不产生"锁内发布+锁外关旧"型 use-after-close；但读路径改为"快照后锁外 Ping"后，**Close 可在 Ping 期间关掉同一 client/session**：go-redis 与 mgo 的 Close/Ping 内部都有各自 mutex（mgo `session.go:200/213` 用 `s.Lock/RLock`），返回 error 而非 panic，属**良性 use-after-close**（可观测为关停期 error 增多）。未发现新 panic。
3. **新增 `InitWithError` 错误传播路径是否有 nil 解引用？**
   - `mysql.go:56-58`（m==nil→ErrClosed）、`mgo.go:140-142`、`redis.go:199-201`、`sqlite.go:40-42` 均先判 nil；`builtins.go:98-100/114-116/129-131/144-146` 判 `config.Config==nil`。**无 nil 解引用**。
   - 语义缺陷仅 R-03（Close 后假成功）与 `initErr` 可能被并发 Init 覆写（低危，不影响 panic）。
4. **`app.go` 关停顺序调整后 health checker 的 WaitGroup/通道是否 close 两次、nil channel、等待死锁？**
   - close 两次：Run 只执行一次（`signalChan` 单次接收 `:378`），且 `:387` 有 nil 判定 → 无双关。
   - **nil channel：有——R-01，P1，已动态复现**。
   - 等待死锁：**有——R-01（nil channel）+ R-05（无超时）**。
   - WaitGroup：`Add(1)`（`:119`）与 `Done()`（`:121`）配对，计数正确；`checkTicker` 非 nil 才 Stop（`:384`），无 nil 解引用。
5. **plugin 缓存 name/order 后 Register 与运行期是否一致？**
   - 缓存时机：Register 期 `safeName/safeOrder`（`:56/61`），运行期全读缓存（`:72/148/157/.../229`）——**一致，X5/X5b/X6 全消**。
   - typed-nil：`isNilPlugin`（`:75-86`）用 reflect 判 6 类 nil kind，安全。
   - `Name()` 在注册期 panic 的边界：`safeName` recover 后 `:58` 记日志并**不注册**（不进 `plugins/meta/order`）——符合 QA 的"两端守护"要求，注册失败不留残表。
6. **builtins `initFn` 传播 error 后 `InitAll` 语义？**
   - `builtins.go:101/117/132/147` 四个数据源均返回 `InitWithError`；`plugin.go:158-165` 把 error 记入 `firstErr` 并继续；`mgin.go:141-145` 记日志但不中断启动——与"旧 Init 不中断"语义一致，✅。
   - 附带：MySQL 的 `initFn`（`:101`）上轮已是 `InitWithError`，本轮只是补齐另三个。
7. **新增生命周期测试是否真覆盖修复点？**——见 §5 测试评估。

---

## 4. 继续发现的新潜在 panic 面（任务重点 3）

- **R-01（P1）**：nil-channel 关停死锁——本轮唯一"高"级新风险，已在 §2 详述。
- **R-02/R-05（P2）**：Init 锁内 I/O + Wait 无超时导致的长阻塞（非 panic，但可致 SIGKILL）。
- **无新的 nil 指针 / slice 越界 / 类型断言 panic**：
  - `mgo.go:266-270` multi + 空 tags 有 `len(m.tags)==0` 守护，不越界（与上轮 QA 一致）。
  - `redis.go:398-400` 类型断言 `conn.(*redis.Client)` 用 `ok` 形式，安全。
  - `plugin.go:133-135` `sort.Slice` 比较缓存 int，无接口方法调用。
  - `dao/mysql.go` 全部 receiver 方法首行 nil 守护齐备；`Pager:595` 判 `receiver==nil||conn==nil`。
  - `transaction.go:15/24-26` `fn==nil`、`conn==nil` 双守护，`ctx==nil` 兜底 `Background`，回调 panic 按 Go 语义穿透且已回滚（上轮结论延续，**不建议加 recover**）。
- **锁顺序 ABBA 检查**：Redis/Mongo 均为 `initMu → mu`，`Check/GetConnection` 的懒初始化是"先 RLock 读 needsInit → **RUnlock** → 再 Init"(`mgo.go:205-215/245-255`)，无"持 mu 抢 initMu"的反序 → **无 ABBA 死锁**。本轮新增的 R-01 是 channel 层死锁，与锁序无关。

---

## 5. 新增测试有效性评估（红灯绿灯都看退出码与完整输出）

| 测试文件 | 覆盖了什么 | 未覆盖 / 隐患 |
|---|---|---|
| `pkg/db/mongo/mongo_lifecycle_test.go`（14 行） | `Close` 后 `GetConnection/Check` 返回 `ErrClosed`——直接覆盖 X2 终态 | 未覆盖 R-03（Close 后 `InitWithError` 假成功）、X4（冗余 Init 保 tags）、X3（读锁不被写锁阻塞）、并发 race |
| `pkg/db/redis/redis_lifecycle_test.go`（14 行） | `Close` 后 `ErrClosed`——覆盖 X7 closed 语义 | 未覆盖 R-03、R-04（订阅泄漏）、X8、X1（Init 锁内 Ping） |
| `pkg/db/sqlite/sqlite_lifecycle_test.go`（17 行） | nil receiver、`&Sqlite{}` 未打开的 `GetConnection/Check` 报错 | **未覆盖 X10 真修复点**：`&Sqlite{sqlite:&gorm.DB{}}`（Config nil 的半初始化）——这是 QA 修正后的关键判据，当前用例即使 `validateSqlite` 退化成只判 `conn==nil` 也会绿（假绿） |
| `pkg/plugin/plugin_panic_test.go`（28 行） | `Name()`/`Order()` panic 的插件**注册被拒**、不进表——覆盖 X5/X5b 注册期边界 | 未覆盖 X6（recover 分支二次 Name）、X13（Enabled panic 仍 Close）、运行期可变 Name/Order（已因缓存而不可能，应补一条"注册后改方法不生效"固化缓存契约） |

**T-02（P3，测试缺口）**：sqlite 生命周期测试是典型"绿灯但没测到修复点"。建议补 `TestSqliteHalfInitializedGormDB`：`client := &Sqlite{sqlite: &gorm.DB{}}`，断言 `Close/Check/GetConnection` 均返回 error 且不 panic——当前 `validateSqlite` 会因 `Config==nil` 正确返回 error（会绿），但若有人把校验删回 `conn==nil` 则该用例转红，能真正锁住 X10。

---

## 6. 收尾状态与探针说明

```
$ git status --short        →  M .gitignore   （唯一，进入任务前既存，非我改动）
$ 未 git commit、未改任何生产代码、未改 01–06 报告
```

- **临时探针（全部已删除，无残留）**：
  1. `deliverables/.../probes/gao_shutdown_probe.go`（说明性文件，已删目录）——配合 `/tmp/gao_nil_select.go` 复现 R-01，输出 `DEADLOCK: ... checkWG.Wait() never returns`，探针已删。
  2. `pkg/db/redis/zz_gao_probe_test.go`（2 用例：InitAfterClose 假成功、Init 持锁 Ping 2.8023s）——跑完即删，`git status` 确认无残留。
- 本轮**未**跑全量 `go test/-race/go vet`（由主理人统一跑基线）；本报告所有"动态证实"均来自上述自写探针，退出码 0、输出已逐字核对，输出中无敏感信息（地址均为 `127.0.0.1` 本地环回）。

---

## 7. 给主理人的三句话

1. **没有新 P0，上轮 P0 继续闭环**；plugin 的 `name/order` 缓存 + `safeName/safeOrder` 是本轮最干净的修复，X5/X5b/X6/注册期 panic 四处一起消掉，sqlite `validateSqlite` 判到 `Config` 也把 X10 钉死了。
2. **本轮头号问题是 `10fe036` 引入的新 P1 关停死锁**（`app.go:389 checkDone=nil` + `:391 Wait()`）：SIGTERM 撞上 checkAll 执行窗口就卡死，Shutdown/SafeExit 全跳过——删掉那一行 `checkDone = nil` 即可，另加 `Wait()` 超时兜底。**这是唯一需要立即修的**。
3. **X1 只修了一半**：读路径快照锁外 Ping 做得对，但 Redis/Mongo 的 `Init` 仍持写锁 Ping/Dial（实测 2.8s 阻塞）；加上 Redis 订阅泄漏（PSubscribe 无 cancel）、Init-after-Close 假成功，这三条建议与 P1 同批修，按 `mysql.go:55-103` 的 RCU 范式统一收口。

（报告完）
