# 架构师第二轮 panic 风险复审报告（修复后代码）

- 复审人：高见远（Gao）· 架构师
- 日期：2026-11-25（第二轮）
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，module `github.com/maczh/mgin/v2`
- 环境：`go version go1.25.7 linux/amd64`（`export PATH=/usr/local/go/bin:$PATH`，`GOPROXY=https://goproxy.cn,direct`）
- 基线：`HEAD = aaae6a3`；修复窗口 `9825b9b..HEAD`（8 个提交，+455/−103，7 个文件）
- 方法：
  1. **通读最新完整文件**（非只看 patch）：`pkg/plugin/{plugin,builtins}.go`、`pkg/db/{db,transaction}.go`、`pkg/db/{mysql,redis,mongo,sqlite}/*.go`、`pkg/db/dao/{mysql,mgo,define,redis_cache}.go` 及全部现有测试；对照 `git diff 9825b9b..HEAD -- <file>` 理解改动意图。
  2. 逐条回归上轮 R1–R13 / N1–N4。
  3. **重点审查修复本身引入的新风险**（锁顺序/重入、锁内网络 I/O、closed 语义、RCU 发布、订阅停止信号、适配器 nil 安全、enabled 匹配、DAO 副本语义、Mongo 懒初始化）。
  4. 为 12 条新假设写了**临时探针测试**（`pkg/db/{mongo,redis,sqlite}/zz_gao_probe_test.go`、`pkg/plugin/zz_gao_probe_test.go`，`package <pkg>` 以访问私有字段），真跑取证后**全部删除**；未修改任何生产代码、未 commit。
- 收尾状态：`git status --short` → 仅 ` M .gitignore`（进入本任务前即存在，非我改动）；`go test ./pkg/db/... ./pkg/plugin/...`、`go test -race`、`go vet` 均退出码 0。

---

## 0. 结论摘要

### 0.1 一句话总览（最重要的判断）

上一轮 **P0 的根因（Redis/Mongo 无锁共享 map）确实已被修掉**——`RWMutex` 覆盖了全部 map 访问点，`-race` 与 `concurrent map read and map write` 的触发路径已不可达。**但修复的方式把「数据竞争」换成了「锁内网络 I/O + 写锁串行化 + 缺失 closed 语义」**：新的问题不再是 `fatal error`（不可恢复崩溃），而是**全局锁护航 → 延迟雪崩 / 关停卡死 / 关闭后复活重连 / 资源泄漏**，其线上杀伤力与 P0 同量级（服务不可用 vs 进程崩溃），只是表现形式从"崩"变成"卡"。此外 **`plugin` 的 recover 加固仍留两个缺口（`Order()` / `Name()` 在 recover 之外被调用）**，`sqlite` 的 R4 只做了象征性修补，`Redis` 仍无 `closed` 语义。

### 0.2 上轮 17 项回归总判定（R1–R13 + N1–N4；其中 R2b 为上轮 R2 的子项，本表单列一行，故下表共 18 行）

| 编号 | 上轮定性 | 本轮判定 | 依据（当前 file:line） |
|---|---|---|---|
| R1 Redis 无锁 map | P0 | **已修复** | `redis.go:20-21` 增 `mu/initMu`；`:42,:192,:231,:275,:310,:319` 全部读写入锁；`-race` 场景下 map 访问不再并发 |
| R2 Mongo 无锁 map | P0 | **已修复**（但引入 X2/X3/X4） | `mgo.go:18-19` 增锁；`:43,:134,:184,:192,:212,:219,:273,:282` 全覆盖 |
| R2b `mgo.go:211` 链式解引用 | P1 | **已修复** | 现 `mgo.go:238-241`（multi）/`:254-257`（单库）拆为「取 `Copy()` → 判 nil → 再 `.DB()`」，且 `mgoCheck`（`:234/:244`）在同一次写锁内先保证 `conn != nil`；`mgo.Session.Copy()` 在 `isClosed` 时返回 nil（依赖库 `session.go:132-138`），新增 `copy == nil` 守护正好兜住 |
| R3 `InitAll` 无 recover | P1 | **部分修复（有缺陷）** | `plugin.go:132` 改用 `safeInit`（`:211-218`）；但 `sortedByOrder():106-112` 的 `Order()`（`:109`）与 `InitAll` 内 `p.Name()`（`:122,:131,:136`）仍在任何 recover 之外 → **X5 / X5b** |
| R4 `sqlite.Close` 丢弃 `DB()` error | P2（概率已下调） | **部分修复（有缺陷）** | `sqlite.go:53-58` 改为 `if db, err := m.sqlite.DB(); err == nil && db != nil`——但 **panic 发生在 `m.sqlite.DB()` 调用内部**（gorm.go:413；根因为嵌入的 `db.Config == nil`，见 §10.1），`err == nil` 判断根本来不及生效 → **X10**（已动态复现） |
| R5 共享 DAO 字段无锁读改写 | P1 | **已修复（但有兼容性缺陷）** | `mysql.go(dao):95-118` `WithContext/WithCache` 改为返回副本（`:99-101,:115-117`），`Debug():50-56` 同构；`connection():70-72` 把 `tx/Tag/ctx` 读到局部变量，`:89` 不再重复解引用 `m.ctx` → **X9**（忽略返回值的旧调用静默丢 ctx） |
| R6 `PSubscribe` 改写入参 slice | P1 | **已修复** | `redis.go:337` 复制局部切片；`:370` `Receive` 失败先 `pubsub.Close()`；`:331` `handler == nil` 守护；`:341/:350/:377/:394` ctx 停止信号 → 残留 **X8**（`{db}` 只替换一次） |
| R7 `Register` typed-nil | P2 | **已修复** | `plugin.go:46` 改用 `isNilPlugin`（`:58-69`，反射判空），与 `mgin.go:103-114` 同构 |
| R8 DAO nil receiver 前置写 `Tag` | P2 | **已修复** | 所有 `receiver.Tag = notag` 写入已删除；`connection():67-68` 统一 nil 守护，`tag == nil` 时用局部 `notag`（`:75-77`）→ 残留 **X9**（`WithTx:61 clone := *m` 无 nil 守护） |
| R9 `MgoDao.Update` 缺 `conn==nil` | P2 | **已修复** | `dao/mgo.go:112-114` 已补（本轮唯一 +3 行的改动） |
| R10 Registry typed-nil 解引用 | P2 | **已修复** | `builtins.go:176-178`（init）/`:196-198`（close）改用 `isNilRegistry`（`:205-216`） |
| R11 `HealthAll` 无 recover | P2 | **部分修复（有缺陷）** | `plugin.go:197` 改用 `safeHealth`（`:229-236`）；但 `sortedByOrder()`/`p.Name()`（`:188/:191/:197`）仍在 recover 外 → **X5 / X5b** |
| R12 MySQL 发布后锁外关旧连接 | P3 | **未修复（上轮判定可不修）** | `mysql/mysql.go:91-101` 结构未变；`sql.DB.Close()` 后操作返回 error 非 panic |
| R13 `Check` 空实现 | P2 | **部分修复** | sqlite 已实现真实 Ping（`sqlite.go:74`）✓；**redis 仍是空实现**（`redis.go:211-225` 全注释），且 `Check():234-245` 在 `clients` 为空时恒返回 nil → 未初始化/已关闭时**假阳性健康**（探针6 复现） |
| N1 `enabled` 子串误启用 | P3 | **已修复（但有行为变更）** | `builtins.go:218-225` 改为逗号分词 + `TrimSpace` + `EqualFold` → **X13**（依赖子串的旧配置会漏启用；且与 `mgin.go:90-100` 区分大小写的实现语义分叉） |
| N2 `CloseAll` 吞 panic 无日志 | P2 | **已修复** | `plugin.go:166-172` recover 分支补 `logs.Error` + `debug.Stack` + 计入 `firstErr` |
| N3 `Enabled()` 重复求值 | P3 | **未修复** | `InitAll:120` 与 `CloseAll:153`、`HealthAll:189` 各求值一次；状态翻转 → 漏关/漏查 |
| N4 `RedisCache.Range(nil)` | P3 | **未修复** | `dao/redis_cache.go:59-80`，`f` 无判空，`:71 f(key, value)` 仍会 panic |

**统计：完全修复 11 项（R1/R2/R2b/R5/R6/R7/R8/R9/R10/N1/N2）、部分修复 4 项（R3/R4/R11/R13）、未修复 3 项（R12/N3/N4），合计 18 项与上表行数一致。**

### 0.3 本轮新发现风险总表（X 系列）

> 分级口径：**P1** = 可致服务不可用/关停卡死/资源泄漏，且默认配置下可达；**P2** = 特定输入或自定义插件下可复现，可 recover 或影响有限；**P3** = 防御性/兼容性/潜在雷。
>
> 本表共 **17 条**（X1–X16 + 由原 X5 拆分出的 X5b）。其中 **P1 共 4 条**（X1 / X2 / X3 / X5）、**P2 共 6 条**（X4 / X5b / X6 / X7 / X9 / X10）、**P3 共 7 条**（X8 / X11 / X12 / X13 / X14 / X15 / X16）。

| 编号 | 位置 | 问题 | 后果类型 | 级别 | 可信度 |
|---|---|---|---|---|---|
| **X1** | `redis.go:42-43,176` / `:231-232,241` / `mongo/mgo.go:43-44,95,116` / `:219-220,234,244,162,166` | **锁内做网络 I/O**（Init 持写锁 Ping/Dial、Check 持读锁 Ping、Mongo GetConnection 持写锁 Ping+Dial）；Redis 上界 = **库数 × 60s**（见 §10.3） | 锁护航、关停阻塞（分钟级）、延迟雪崩 | **P1** | 已动态证实（探针 2/3/4/5） |
| **X2** | `mongo/mgo.go:130-152`（Close 不清 `confData`、无 `closed`）+ `:185-190,:213-218`（needsInit） | **Mongo 关闭后复活**：Close 后 `GetConnection`/`Check` 因 `len(conns)==0 && len(confData)>0` 重新 `Init` | 关停期在途请求重拨 10s、新 session 永不释放、关停拖死 | **P1** | 已动态证实（探针2，10.001s / 10.000s） |
| **X3** | `mongo/mgo.go:219-220` | **GetConnection 用写锁而非读锁**，且每次调用都在锁内做 `Ping()` 网络往返 | 全库访问串行化，吞吐上限 ≈ 1/RTT | **P1** | 静态确认（`Lock` 语义确定）+ 探针3 佐证锁内阻塞 |
| **X4** | `mongo/mgo.go:52` | `m.tags = make([]string,0)` 在 `len(m.conns)==0` 判断**之前**，冗余 `Init` 会擦除多库 tag | 多库 `GetConnection("")` 报 "no available database"、`ListConnNames()` 空 | **P2** | 已动态证实（探针1） |
| **X5** | `plugin.go:106-112`（`Order()` 在 `:109`） | **元信息方法 `Order()` 在任何 recover 之外**被调用（`sortedByOrder`） | 打穿 `InitAll`/`CloseAll`/`HealthAll` **三条主链路**（含关停）→ 启动期/运行期定时器/关停期进程级崩溃 | **P1**（介于 P1/P2 之间，按"三链路全裸且无条件可复现"取 P1） | 已动态证实（三入口 `ORDER_PANIC` 全部逃逸） |
| **X5b** | `plugin.go:122,:131,:136`（`p.Name()`）、`:163,:167,:175`、`:188,:191,:197` | **`Name()` 在 recover 之外**被调用；但 `Register:49` 自己也调 `p.Name()`，故"无条件 panic 的 Name()"**根本进不了注册表** | 真实暴露面是"注册后 `Name()` 才变坏"（Init/Close 失败后内部状态被清空，如配置指针置 nil）→ 启动/关闭/健康检查二次崩溃 | **P2**（需注册后状态劣化） | 已动态证实（"先正常 Register，再让条目状态劣化"才复现 `NAME_PANIC`） |
| **X6** | `plugin.go:214,223,232` | `safeInit/safeClose/safeHealth` 的 recover 分支**再次调用 `p.Name()`** | `Name()` panic 造成二次 panic（原 panic 被吞、新 panic 穿透） | **P2** | 已动态证实（探针8） |
| **X7** | `redis.go:188-209`（无 `closed`）、`:234`、`:325-327,:345-357` | Redis 无 `closed` 语义：Close 后 `Check()` 恒 nil（假阳性）；无 ctx 的 `PSubscribe` 在 Close 后**永久 1s 重试** | 健康检查失真、goroutine/日志泄漏、无框架级停止信号 | **P2** | 已动态证实（探针6）+ 静态确认 |
| **X8** | `redis.go:337,363-365` | `localChannels` 跨重连复用，`{db}` 只替换一次 | db 号变化（故障转移）后订阅通道不更新，收不到消息 | **P3** | 静态确认 |
| **X9** | `dao/mysql.go:60-64`（`WithTx`）、`:95-118`（副本语义） | `WithTx` 仍 `clone := *m` 无 nil 守护；`WithContext/WithCache/Debug` 改副本后**忽略返回值的旧调用静默失效** | nil receiver panic（R8 同族残留）；ctx/缓存静默丢失（行为回归） | **P2** | 静态确认 |
| **X10** | `sqlite.go:53-58`（`Close`）、`:70`（`Check`）、`:77-87`（`GetConnection`） | R4 未彻底：零值 `*gorm.DB{}` 下 `Close`/`Check` 仍 panic；`GetConnection` 不做 validate | 可恢复 panic（`CloseAll` 内被 recover，但中断 sqlite 资源释放） | **P2** | 已动态证实（探针9） |
| **X11** | `redis.go:275-276` + `:284` + 注释 `:213-223` | `GetConnection` 持 **RLock** 调用 `redisCheck`；注释保留的重连实现会在 RLock 内写 `r.clients[dbName]` | 潜在雷：一旦恢复该实现，即是 `fatal error: concurrent map read and map write`（写 map 无需另加锁的前提下）或直接死锁 | **P3** | 静态确认（当前 `redisCheck` 为空实现） |
| **X12** | `mongo/mgo.go:229` | `GetConnection` 仍原地改写调用方 slice（`dbName[0] = m.tags[0]`） | 入参副作用（同步发生、无跨 goroutine 竞态） | **P3** | 静态确认（上轮已列，仍未修） |
| **X13** | `plugin.go:153-159`、`builtins.go:218-225`、`mgin.go:90-100` | ① `CloseAll` 中 `Enabled()` panic → `continue` **静默漏关**；② `enabled()` 改精确匹配后，依赖子串的旧配置（如 `used: "redis-cluster"`）不再启用 redis；③ `plugin.configured`（EqualFold）与 `mgin.configuredComponent`（区分大小写）语义分叉 | 资源泄漏 / 功能静默失效（**非 panic**） | **P3** | 静态确认 |
| **X14** | `dao/redis_cache.go:59-80`（`:71`） | N4 未修：`Range(f=nil)` 仍 panic | 导出 API 健壮性（内部调用方恒非 nil） | **P3** | 静态确认 |
| **X15** | `mgin.go:200-212` + `builtins.go` 全部 `closeFn` | `SafeExit` 的 5s ctx 是**装饰性的**（各 closeFn 完全忽略 ctx），叠加 X1/X2 关停可能远超 5s | 关停超时不可控（K8s 侧 SIGKILL） | **P3** | 静态确认 |
| **X16** | 三套数据源 | 生命周期契约不一致：mysql 有 `closed`+读锁+锁外建连；redis 无 `closed`+全锁+锁内 I/O；mongo 无 `closed`+**写锁**+锁内 I/O+懒初始化复活 | 结构性（后续同类 bug 的温床） | **P3** | 静态确认 |

---

## 1. P1 新风险详解（本轮核心）

### X1 — 修复把「无锁」换成了「锁内做网络 I/O」，导致锁护航与关停阻塞

这是本轮**最需要立刻处理**的问题。三处把网络 I/O 放进了临界区：

**(a) `RedisClient.Init` 持写锁做 Ping/建连**
- `pkg/db/redis/redis.go:40-43`：`initMu.Lock()` + `mu.Lock()`，两个锁一直持有到函数结束（`defer`）。
- `redis.go:174-184`：`for dbName, rds := range r.cfgs { rc := redis.NewUniversalClient(&rds); rc.Ping() ... }` —— **在写锁内逐库 Ping**。
- `RedisClient.Check` 持读锁做 Ping：`redis.go:231-232`（`RLock` + `defer RUnlock`）+ `:241`（`client.Ping().Err()`）。
- 影响：`Init` 期间所有 `GetConnection`（即所有 Redis 业务访问、`RedisCache` 全链路）被阻塞。阻塞上界原文我写作「库数 ×（DialTimeout 5s + ReadTimeout 3s）」——**经工程师实测修正（我已复核源码，见 §10.3）：`redis.go:162-165` 在 `go.data.redis_pool.max > 1` 时会把 `DialTimeout` 抬到 `go.data.redis_pool.timeout`，缺省 60s；而 `:174-184` 是逐库串行 Ping**。因此真实上界是 **库数 × 60s**，比我原先估计的最坏情况差一个数量级。`Check` 期间 `Close()`/`Init()` 的写锁被反向阻塞，**并且在 Go 的 RWMutex 语义下，一个等待中的写者会阻塞后续所有新读者**——于是关停瞬间所有 `GetConnection` 一起排队。

**(b) `Mongodb.Init` 持写锁做 `DialWithTimeout`**
- `pkg/db/mongo/mgo.go:43-44`（写锁）→ `:95`（multi 每库 10s）/ `:116`（单库 10s）。

**(c) `Mongodb.GetConnection` / `Check` 持**写锁**做 Ping + Dial**
- `mgo.go:219-220`：`m.mu.Lock(); defer m.mu.Unlock()` —— **这是 DAO 热路径**（每次 `MgoDao.*` 调用一次）。
- `:234`/`:244` → `mgoCheck` → `:162 conn.conn.Ping()` + `:166 mgo.DialWithTimeout(uri, 10*time.Second, m.max)`。
- `mgo.Session.Ping()` 用的是 `context.Background()`（依赖库 `session.go:89` `ctx: context.Background()`），**没有超时** → 走 mongo-driver 默认 `serverSelectionTimeout`（30s）。即：Mongo 不可达时，**每一次 DAO 调用都可能持写锁阻塞约 30s**，且全局串行。

**动态证据（探针已删，输出留存）**
```
probe3: Init 拨号期间 ListConnNames 被阻塞 9.70025656s
probe4: Check 持读锁 Ping 期间，Close 写锁被阻塞 1.700135764s
probe5: Init 持写锁 Ping 期间，GetConnection 被阻塞 2.501997607s
probe2: Close 后 GetConnection err=MongoDB connection not found elapsed=10.001287373s
```
（探针用「只 accept 不回包的本地 TCP 服务」模拟 Redis 挂死、用不可达地址模拟 Mongo 不可达。）

**后果**：不 panic、不 fatal，但表现为：① 服务 P99 暴涨甚至"假死"；② `SIGTERM` 后关停卡死（`SafeExit` 的 5s ctx 无效，见 X15）被 K8s 强杀；③ Mongo 抖动时形成请求堆积 → 内存/连接数雪崩。

**修复建议（代码级）**
1. 一律「锁内只做指针/切片拷贝，锁外做 I/O」：
   ```go
   // Mongodb.GetConnection 正例
   m.mu.RLock()
   c, ok := m.conns[tag]
   m.multi, tags := m.multi, m.tags
   m.mu.RUnlock()
   if !ok || c.conn == nil { return nil, errNotFound }
   if err := pingWithTimeout(c.conn, 2*time.Second); err != nil { /* 走受控重连（单飞+退避），成功后再取写锁发布 */ }
   cp := c.conn.Copy(); ...
   ```
2. `Init` 改为 `mysql.MysqlClient.initialize` 的 RCU 范式：锁外建连 → 锁内一次性发布 → 锁外关旧（`mysql/mysql.go:55-103` 已经是标准答案，直接抄）。
3. `redis.Check`/`Check` 内的 Ping 必须移出锁：`RLock` 取 `[]UniversalClient` 快照 → 解锁 → 逐个 Ping。
4. 给 `mgo` 的 `Ping` 加超时：`mgo.Session` 未暴露 ctx 参数，建议改为 `conn.conn.Run(bson.M{"ping":1}, ...)` 或在 `mgoCheck` 外包 `context.WithTimeout` + 独立 goroutine（注意 goroutine 泄漏），或 **在 DialWithTimeout 时给 Session 设置 `socketTimeout`**（依赖库 `Session.socketTimeout` 字段存在但 `SetSocketTimeout` 是空实现，需评估是否自行 fork）。

**可证伪预测**：若 X1 成立，则「启动一个 `pkg/db/mongo` 的 `Mongodb{}`，`confData` 指向不可达 URI，goroutine A 调 `GetConnection()`，300ms 后主 goroutine 调 `ListConnNames()`」应观察到主 goroutine 被阻塞 ≈9.7s（我已实测 9.70s）；把 `Init`/`GetConnection` 改为「锁外建连 + 锁内仅发布」后，`ListConnNames()` 应立即返回（<1ms）。

---

### X2 — Mongo「关闭后复活」：`closed` 语义缺失 + 懒初始化在 `Close` 之后仍然生效

**位置**：`pkg/db/mongo/mgo.go:130-152`（`Close` 只删 `conns`，**不清 `confData`、不设 `closed`**）+ `:185-190`（`Check` 的 `needsInit`）+ `:213-218`（`GetConnection` 的 `needsInit`）。

```go
// GetConnection（mgo.go:212-218）
m.mu.RLock()
needsInit := len(m.conns) == 0 && len(m.confData) > 0   // ← Close 之后该条件恒为 true
confData := append([]byte(nil), m.confData...)
m.mu.RUnlock()
if needsInit { m.Init(confData) }                        // ← 关闭后重新拨号（10s）
```

**触发链路（本轮是代码事实，不是推测）**：
`app.go:387 app.MGin.SafeExit()` → `mgin.go:211 plugin.CloseAll(ctx)` → `builtins.go:136 db.Mongo.Close()`（清空 `conns`）
→ 但 `app.go:395` 的 `s.Shutdown(ctx)` **在其之后**，HTTP server 仍在线处理在途请求
→ 在途请求 → `dao.MgoDao.*` → `db.Mongo.GetConnection()`（`mgo.go:208`）→ `needsInit == true` → `m.Init(confData)` → **`mgo.DialWithTimeout` 10s，成功后把新 session 写回 `m.conns`，而 `Close` 已经跑完，无人再关它**。

**动态证据**：
```
probe2: Close 后 GetConnection err=MongoDB connection not found elapsed=10.001287373s
probe2: Close 后 Check err=MongoDB connection not found elapsed=10.000137706s
```
（不可达地址下两次都是 10s 整，证明 `Init` 被 Close 之后再次调用；可达地址下则会在 Close 之后建立**永不被释放**的 session。）

**后果**：
1. 关停期每个 Mongo DAO 请求被拖 10s（`app.go` 的 `shutdownTimeout` 默认 5s，必然超时）→ 强制 `SIGKILL`。
2. 若 Mongo 可达：`Close` 之后新建的 `*mgo.Session`（各自持一个 mongo-driver client + 连接池）永远不会被 `Close()` → **FD/连接泄漏**。生产上进程退出会释放，但"进程不退出"的场景（测试、热重载、库被复用、以及先 `CloseAll` 后长时间等待 HTTP 排空的窗口）会实打实泄漏。
3. `plugin.HealthAll()` 从 `checkAll`（`mgin.go:170`）调用时同样会复活，并**返回 nil**（`mgoCheck` 成功）→ 关闭后仍报健康。

**修复建议**
```go
type Mongodb struct {
    ...
    closed bool
}
func (m *Mongodb) Close() {
    m.mu.Lock()
    if m.closed { m.mu.Unlock(); return }   // 幂等
    m.closed = true
    old := m.conns
    m.conns = map[string]connection{}
    m.tags = nil
    m.confData = nil                        // ← 关键：切断复活燃料
    m.mu.Unlock()
    for _, c := range old { if c.conn != nil { c.conn.Close() } }  // 锁外关
}
```
并在 `Check`/`GetConnection` 的 `needsInit` 判断前加 `if m.closed { return ErrClosed }`（与 `mysql/mysql.go:63-66,:86-90` 对齐）。**注意**：`aaae6a3 "preserve mongo lazy initialization"` 的意图（保留懒初始化）应只覆盖"从未 Init 过"的情形，不能与"已 Close"混淆——正确做法是引入三态 `uninitialized / ready / closed`，而不是用 `len(conns)==0` 兼职判断。

**可证伪预测**：若 X2 成立，`m := &Mongodb{}; m.confData = []byte(...); m.conns = map[string]connection{}; m.Close(); _ = m.Check()` 耗时应 ≈10s（已实测 10.000s）；在 `Close` 中把 `confData` 置 nil 并加 `closed` 短路后，同一调用应立即返回 `ErrClosed`（<1ms）。

---

### X3 — `Mongodb.GetConnection` 用**写锁**且每次都 Ping：全库串行化

**位置**：`pkg/db/mongo/mgo.go:219-220`（`m.mu.Lock()` 而非 `RLock`）。

**为什么这是新问题**：修复前 `GetConnection` 是无锁的，并发 DAO 调用可以真并发；修复后**所有 Mongo DAO 调用被一把互斥量串行化**，且每次都在持锁状态下完成 `Ping()` 的一次网络往返（`:162`）。这意味着：

- 单进程 Mongo 吞吐上限 ≈ `1 / RTT`（同机房 0.3–1ms → ~1000–3000 QPS 封顶，且随并发升高只会排队不会提升）。
- `Ping` 结果**从不被用来做任何缓存/短路**：即使连接完全健康，每个 DAO 调用也照做一次 `Ping`。
- Mongo 抖动 100ms → 所有 Mongo 请求串行排队，排队长度 = 并发数；并发 100 时尾部请求要等 10s。
- `mgoCheck` 的重连（`:165-175`）也在同一把写锁内 → 单库重连期间所有库全停。

**修复建议**：见 X1 建议 1；另外：
- `Ping` 结果加 `sync.Map`/`atomic.Int64` 缓存的"上次校验时间"，做**1s 节流**（同一 tag 每秒最多 Ping 一次），或改为「出错才 Ping」。
- 重连做 singleflight（每 tag 一个 `sync.Once`/`singleflight.Group`），避免 N 个 goroutine 排队各自 Dial 10s。

**可证伪预测**：若 X3 成立，用一个 clock 可注入的 session 桩（或真实 Mongo + 人为 50ms 延迟）测 100 并发 `GetConnection`，总耗时应 ≈ `100 × (RTT×2)`（含 Ping），近似线性；改为 RLock + 锁外 Ping + Ping 节流后，总耗时应回落到 ≈ `RTT×2 + 排队`（并行）。

---

## 2. P2 新风险详解

### X4 — 冗余 `Init` 擦除 `m.tags`（多库 tag 表被清空）

**位置**：`pkg/db/mongo/mgo.go:52` `m.tags = make([]string, 0)` —— 这行在 `:56 if len(m.conns) == 0 {` **之前**无条件执行。

**触发**：任何第二次 `Init` 调用（`builtins.go:133` 的 initFn 重复调用、`Check`/`GetConnection` 的 `needsInit` 触发、或业务直接调 `db.Mongo.Init`）都会清空 `tags`，而此时 `conns` 非空、`multi` 为 true。

**动态证据**：
```
probe1: Init 后 tags=[] len=0
probe1 复现：冗余 Init 擦除 m.tags，多库 tag 列表丢失
```

**后果**：`GetConnection("")`（DAO 默认不传 tag 的路径，`dao/mgo.go:32` 传的是 `m.Tag()` 可能是 `""`）在 multi 模式下走 `mgo.go:225-230`：`if dbName[0] == "" { if len(m.tags) == 0 { return nil, errors.New("MongoDB multidb has no available database") } }` → **所有 Mongo DAO 操作直接失败**（不 panic，但功能全挂）。`ListConnNames()` 也变空。注意：这是**修复前就存在的缺陷**（原代码也有 `m.tags = make(...)` 在函数开头），但本轮加重了触发概率——因为 `GetConnection`/`Check` 现在会主动调 `Init`（`mgo.go:217`、`:189`），并发/重复触发变得常见。

**修复建议**：把 `m.tags = make([]string,0)` 移进 `if len(m.conns) == 0 {` 块内（与 `m.multi` 赋值同层），或在块尾用局部 `tags` 构建后锁内一次性发布（RCU）。

**可证伪预测**：若 X4 成立，`m := &Mongodb{multi:true, conns:{a,b}, tags:[a,b]}; m.Init(nil); m.ListConnNames()` 应为空（已实测空）；把该行移入 `if len(m.conns)==0` 后应返回 `[a b]`。

### X5 / X5b — `plugin` 的 recover 加固仍留 `Order()` / `Name()` 两个缺口（R3/R11 未彻底）

> **两条分开定级（工程师第二轮反馈后我采纳，理由见 §10.6）**：`Order()` 无条件可复现、三入口全裸 → **X5 = P1**；`Name()` 需"注册后状态劣化"才可复现（`Register:49` 自己会先调一次 `Name()`，无条件 panic 的插件进不了注册表）→ **X5b = P2**。二者修复手段相同（注册时缓存 name/order），但**风险等级不同，不要合成一条**。

**位置**：
- **X5**：`pkg/plugin/plugin.go:106-112` `sortedByOrder()`：`sort.Slice(ps, func(i,j int) bool { return ps[i].Order() < ps[j].Order() })` —— `:109` 的 `Order()` **在任何 recover 之外**，且被 `InitAll`/`CloseAll`/`HealthAll` 三个入口共同依赖。
- **X5b**：`plugin.go:122`（`logs.Error(..., p.Name(), err)`）、`:131`（`logs.Info("正在初始化{}", p.Name())`）、`:136`、`:163`、`:167`、`:175`、`:188/:191/:197` —— `p.Name()` 全部在 recover 之外。注意 `Register:49 name := p.Name()` 在注册表锁**外**执行，因此"`Name()` 无条件 panic"的插件在 `Register` 处即失败（属调用方问题，不会污染注册表）。

**动态证据**：
```
probe7: InitAll 中 Order() panic 是否穿透 = ORDER_PANIC
probe7: HealthAll 中 Order() panic 是否穿透 = ORDER_PANIC
probe8（X5b）: 先正常 Register 再令条目 Name() 劣化 → recovered = NAME_PANIC
```
（注意：`sort.Slice` 对 n=1 不调用 `less`，所以只有**≥2 个插件**时才复现——我第一次探针就因单插件得到 `recovered=nil` 的假绿。工程师独立踩同一个坑。测试写法要求与断言建议见 §10.6。）

**后果**：`mgin.Init`（`mgin.go:140-145`）无 recover → 启动期进程崩溃；`checkAll`（`mgin.go:170`，由 `app.go` 的 ticker 每 5 分钟驱动）也无 recover → **运行期定时器 goroutine 崩溃 → 整个进程退出**。这条路径比 R3 更隐蔽：它不在启动期，而在任何时刻。**另经工程师补齐（我已复核）：`CloseAll` 同样在 `plugin.go:149 ps := sortedByOrder()` 处被打穿，即三个入口 `InitAll`/`CloseAll`/`HealthAll` 全部可逃逸——关停路径也保不住。**

**修复建议**：把「取元信息」也纳入守护，最小改动：
```go
func safeOrder(p Plugin) (n int) {
    defer func(){ if r:=recover(); r!=nil { n = math.MaxInt } }()
    return p.Order()
}
func safeName(p Plugin) (name string) {
    defer func(){ if r:=recover(); r!=nil { name = fmt.Sprintf("unknown-plugin-%p", p) } }()
    return p.Name()
}
```
`sortedByOrder` 改用 `safeOrder`，三个入口统一改用 `safeName`。

**⚠️ 修复建议补丁（工程师第二轮指出、我复核成立）：必须「两端都守护」，否则 panic 只是搬家。**
`func Register(p Plugin)` **没有 error 返回值**（`plugin.go:45`）。因此我原先写的「在 `Register` 时就把 name/order 缓存进注册表条目（注册时 panic 由调用方负责）」这条**有洞**：在 `Register` 里直接读 `p.Order()`，panic 会从 `InitAll` **搬到注册期**，而 `Register` 无法表达失败（只能 panic 或静默吞掉）。正确形态是：
1. **注册侧**：`Register` 内部也走 `safeOrder/safeName`（recover + 降级值），把 name/order 缓存进注册表条目；`Register` 保持 `func Register(p Plugin)` 签名不变（不破坏 API）。
2. **运行期侧**：`sortedByOrder` 与三个入口一律读**注册表缓存**，不再调用接口方法（或至少都套 `safeXxx`）。
3. 只做其中一端都不够：只做注册侧 → 注册后状态劣化（X5b）仍会炸；只做运行期侧 → 每次排序都重复 recover，且 `Order()` 语义（排序稳定性）在坏插件上不可恢复。

**可证伪预测**：若 X5 成立，注册一个 `Order()` 内 `panic("ORDER_PANIC")` 的插件（**≥2 个插件**使 sort 触发 less），`plugin.InitAll(ctx)` / `CloseAll(ctx)` / `HealthAll()` 三个入口都会 panic 而不是返回（已实测 `recovered=ORDER_PANIC`，三入口全中）；改用 `safeOrder` 后三个入口均**不 panic**（注意：修复后 `InitAll` 可能返回 **nil** 而非 error，见 §10.8 的断言说明——不要把"返回 error"写进断言）。

### X6 — `safeXxx` 的 recover 分支自身会二次 panic

**位置**：`plugin.go:214`（`err = fmt.Errorf("plugin %s init panic: %v", p.Name(), r)`）、`:223`、`:232` —— recover 分支内再调 `p.Name()`。

**动态证据**：`probe8: InitAll 中 Name() panic = NAME_PANIC`（原 `INIT_PANIC` 被替换为新 panic）。

**后果**：本意"把 panic 转成 error"的守护反而变成 panic 的放大器（在 defer 中 panic 会替换原 panic）；且 `InitAll:131` 的 `p.Name()` 先于 `safeInit` 执行，实际连 `safeInit` 都进不去。

**修复建议**：recover 分支内不要调用接口方法；用注册表里缓存的 name（见 X5 修复建议）。

### X7 — Redis 缺 `closed` 语义：关闭后健康检查假阳性 + 订阅 goroutine 永久重试

**位置**：
- `redis.go:188-209` `Close()` 只 `delete` map，不设 `closed`、不清 `conf`。
- `redis.go:227-245` `Check()`：`clients` 为空时循环零次 → **恒返回 nil**。
- `redis.go:325-327` `PSubscribe` 用 `context.Background()` → `:345-357` 失败后 1s 重试，**永不退出**；`Close()` 不 cancel 任何订阅。

**动态证据**：
```
probe6: 二次 Close 未 panic；关闭后 GetConnection err=Redis connection failed.
probe6: 关闭后 Check err=<nil>（空 map 恒返回 nil，健康检查假阳性）
```

**后果**：
1. **健康检查失真**：Redis 从未 Init（配置缺失/精确匹配改动导致漏启用，见 X13）或已 Close 时，`Redis.Check()` 全部报健康 → `/health/ready` 绿 → 流量继续打进来 → 后续 `RedisCache` 静默不缓存、`GetConnection` 全 error。这与 R13 同源，属"逻辑缺口"但与 panic 审计同属健壮性范畴。
2. **goroutine/日志泄漏**：用 `PSubscribe`（非 ctx 版本，框架内置用法）订阅后，`Close()` 之后 goroutine 仍每秒重试一次并 `logger.Error`；进程若继续存活（优雅退出等待、或被当库复用），将永久泄漏 + 日志洪水。

**修复建议**：加 `closed bool` + `ErrClosed`，`Check`/`GetConnection` 提前短路；`RedisClient` 持一个内部 `ctx, cancel`（`Close()` 里 `cancel()`），`PSubscribe` 用该 ctx 代替 `Background()`，使 `Close()` 成为「停订阅 + 关连接」的单一入口。同时 `Close` 后 `Check` 应返回 `ErrClosed` 而非 nil，避免假阳性。**另补（工程师实测，我已复核 `redis.go:367`）**：`pubsub.Receive()` **全程无超时**——工程师 3.5s 窗口内只观测到 1 次 `PSUBSCRIBE`，说明订阅确认卡住时 `Receive()` 直接不返回（**不是忙轮询**），于是既不重连、也不退避、也不报错，比"1s 重试"更难发现。建议改用 `pubsub.ReceiveTimeout(d)` 或把 `Receive()` 放进带 deadline 的 goroutine。

### X9 — DAO 副本语义的兼容性缺陷 + `WithTx` 仍无 nil 守护

**位置**：`pkg/db/dao/mysql.go:95-118`（`WithContext`/`WithCache` 返回副本）、`:46-57`（`Debug`）、`:60-64`（`WithTx`）。

**问题 1（兼容性）**：R5 的修复方向是对的（不可变配置对象），但**改变了调用契约**：
```go
dao.WithContext(&ctx)        // 旧代码：原地生效，返回值常被忽略
dao.All(entity)              // 新代码：ctx 完全丢失（副本被 GC）
```
本仓的 `cmd/templates.go:489/509` 已把示例改成链式写法，说明作者知道这个变化；但**存量业务代码不会跟着改**，结果是 context 超时/链路追踪静默失效（不是 panic，但比 panic 更难查）。同类问题在 `WithCache`（缓存静默关闭）和 `Debug`（日志静默关闭）上同样存在。

**问题 2（R8 同族残留）**：`WithTx:61 clone := *m` 无 nil 守护 → `(*MySQLDao[E])(nil).WithTx(tx)` **panic**。`Debug():47-49` 有守护、`WithContext/WithCache` 有守护，唯独 `WithTx` 漏了，不一致。

**修复建议**：
1. 保留副本语义（正确方向），但**在 `WithContext/WithCache/Debug` 上加编译期/运行期纠错**：把旧字段设为不导出并 `// Deprecated:` 注释；或提供一个 `ApplyContext(ctx)` 原地版本供旧代码迁移；最实用的是在 release note 里明确 BREAKING 并全仓 grep 修示例。
2. `WithTx` 补 `if m == nil { return nil }`。
3. 补一条单元测试：`WithContext(&ctx)` 后原 dao 的 `ctx` 应为 nil、返回值应带 ctx。

**可证伪预测**：`d := &MySQLDao[E]{}; _ = d.WithContext(&ctx); d.ctx == nil` → true（副本语义），若业务按旧写法调用则 ctx 丢失；`(*MySQLDao[E])(nil).WithTx(nil)` → panic（未修）。

### X10 — R4 只修了一半：零值 `*gorm.DB` 下 `Close`/`Check` 仍然 panic

**位置**：
- `sqlite.go:53-58`：`if m.sqlite != nil { if db, err := m.sqlite.DB(); err == nil && db != nil { ... } }` —— **panic 发生在 `m.sqlite.DB()` 调用内部**（gorm v1.31.0 `gorm.go:413`），`err == nil` 判断无效。
- **根因修正（工程师实测 + 我已复核 `gorm.DB` 结构体）**：`type DB struct { *Config; Error error; RowsAffected int64; Statement *Statement; clone int }`（`gorm.go:104-110`），`ConnPool` 是 `Config`（`gorm.go:66-67`）的字段，因此 `gorm.go:413 connPool := db.ConnPool` 是**对嵌入的 `db.Config` 的解引用**——`&gorm.DB{}` 的 `Config == nil` 才是 SIGSEGV 的真正原因（addr=0x80），**不是 `ConnPool` 为 nil**。决定性对照：`&gorm.DB{Config: &gorm.Config{}}`（`Config` 非 nil、`ConnPool` 仍 nil）**不 panic**，`DB()` 走到 `gorm.go:431` 返回 `(nil, ErrInvalidDB)`，此时我原先写的 `err == nil && db != nil` 守护**是有效的**。→ 所以修复必须覆盖 `Config == nil`，只判 `conn == nil` 完全不够。
- `sqlite.go:70`：`Check()` 同构，同样 panic。
- `sqlite.go:77-87`：`GetConnection()` 只判 `m.sqlite == nil`，**不做 validateMySQL 一类的校验**，会把无效 `*gorm.DB` 交给调用方（后续 `db.Model()` 直接 panic，脱离了 `CloseAll` 的 recover 保护）。

**动态证据**：
```
probe9: Close 仍 panic = runtime error: invalid memory address or nil pointer dereference
probe9: Check 仍 panic = runtime error: invalid memory address or nil pointer dereference
probe9: GetConnection 返回 conn!=nil=true err=<nil>（未做 validate）
```

**修复建议**（与 `mysql.validateMySQL` 对齐）：
```go
func validateSqlite(conn *gorm.DB) error {
    if conn == nil || conn.Config == nil || conn.Statement == nil { return errors.New("sqlite connection is not initialized") }
    return nil
}
// Close: if validateSqlite(m.sqlite) == nil { if db,err := m.sqlite.DB(); err==nil && db!=nil { _=db.Close() } }
// Check/GetConnection: 先 validateSqlite
```

**可证伪预测**：`(&Sqlite{sqlite:&gorm.DB{}}).Close()` 现在仍 panic（已实测）；加 `validateSqlite` 后不 panic。

---

## 3. P3 新风险（简述）

- **X8 `redis.go:337,363-365`**：`localChannels` 仅复制一次，重连循环内 `ReplaceAll` 只对首次有效。若 db 号变化（cluster 故障转移、`multi` 切换），订阅通道名不更新。建议每次循环重新基于"原始模板"派生。
- **X11 `redis.go:275-276,284` + 注释 `:213-223`**：`GetConnection` 持 **RLock** 调用 `redisCheck`；`redisCheck` 目前是空实现，但注释里的原实现会在 RLock 下执行 `r.clients[dbName] = rc`（**写 map**）、`r.clients[dbName].Close()`。**任何人恢复这段注释即立刻引入 `fatal error: concurrent map read and map write` 或 `RWMutex` 自我死锁**。建议：删除这段注释，或把重连改成「锁外做 + 换锁发布」的正确形态并写明约束。
- **X12 `mongo/mgo.go:229`**：`dbName[0] = m.tags[0]` 仍原地改写调用方 slice（上轮 R6 同族，仍未修）。建议 `tag := dbName[0]; if tag == "" { tag = m.tags[0] }`。
- **X13 `plugin.go:153-159`**：`CloseAll` 中 `safeEnabled` 返回 err 时 `continue` → **该插件不会被关闭**（静默漏关，比"panic 被记日志"更隐蔽）。建议 Enabled 失败时保守地**仍然调用 Close**（幂等资源释放通常安全），或至少在日志里明确"跳过关闭"。
- **X13 之二 `builtins.go:218-225` vs `mgin.go:90-100`**：`plugin.configured` 用 `EqualFold`（大小写不敏感），`mgin.configuredComponent` 区分大小写 → 同一份 `go.config.used` 在两套路径下可能得到不同启用结果（v1 兼容路径与 v2 插件路径行为分叉）。
- **X14 `dao/redis_cache.go:71`**：`f(key,value)` 无判空（N4 未修）。`if f == nil { return }` 一行即可。
- **X15 `mgin.go:200-212`**：`SafeExit` 建了 5s 超时 ctx，但 `builtins.go` 里所有 `closeFn` 完全忽略 `ctx`（如 `:103 db.Mysql.Close()`、`:136 db.Mongo.Close()`、`:152 db.Redis.Close()`）。叠加 X1/X2，单个组件即可吃掉 10s。建议要么让 `Close` 接受 deadline 并内部提前返回，要么把 5s 改成真实可中断（例如每个 `closeFn` 在独立 goroutine 里跑、外层 `select` ctx）。
- **X16**：三套生命周期契约不一致（对照矩阵见 §5），建议抽一个 `lifecycle` 基类统一收口，长期降低回归成本。

---

## 4. 逐文件精读记录（当前文件全貌）

| 文件 | 精读结论 |
|---|---|
| `pkg/db/db.go` | 4 个全局单例均为 `&T{}`，非 nil。**无风险**。 |
| `pkg/db/transaction.go` | `fn == nil` / `conn == nil` / `ctx == nil` 三重守护（`:15,:23-26`）；gorm `Transaction` 内部保证回滚且不 recover（panic 按 Go 语义穿透，属预期）。**未发现风险，不建议加框架层 recover**。 |
| `pkg/db/mysql/mysql.go` | **本仓最佳实践模板**：`initMu + mu` 分离、`closed` 幂等（`:282-286`）、锁外建连锁内发布（`:80-101`）、`validateMySQL`（`:224-232`）、`closeMySQL` 双重守护（`:262-269`）、`mySqlsCheck` 的 RUnlock→initialize→RLock 序列正确（`:368-382`）。仅 R12（use-after-close 窗口）为理论残余。**可作为 redis/mongo 的对齐目标**。 |
| `pkg/db/mysql/mysql_test.go` | 12 个用例覆盖了 `closed`、并发 initialize、RCU、pool 配置、脚本化 driver。质量好。**未覆盖** R12 的窄窗口（可接受）。 |
| `pkg/db/redis/redis.go` | R1 已修（全锁覆盖，`:275` 的 `redisCheck` 调用需注意 X11）。新问题：X1(a)、X7、X8、X11。`Init` 里 `:166-172` 在 range 中改 `r.cfgs[k]`（更新已有 key，合法）；`:51 if len(r.clients)==0` 的重复 Init 短路仍在（不会擦 tags，与 mongo 不同）。 |
| `pkg/db/mongo/mgo.go` | R2/R2b 已修。新问题：X1(b)(c)、X2、X3、X4、X12。另注意 `Check():194-201` 在 multi 下 `err` 只保留**最后一个** tag 的错误（`:196` 覆盖），诊断信息丢失；`:162` 的 `Ping` 无超时是最危险的单点。 |
| `pkg/db/sqlite/sqlite.go` | X10（R4 未彻底）。另：`Init` 失败已复位 `m.sqlite = nil`（`:41`）✓；`Check` 已实现真实 Ping（`:74`）✓；`GetConnection` 缺 validate。**无测试文件**。 |
| `pkg/plugin/plugin.go` | R3/R7/R11/N2 的主体修复到位（`:46,:132,:166-172,:197`）。缺口：X5（`Order`/`Name` 在 recover 外）、X6（recover 内再调 `Name`）、N3（Enabled 重复求值）、X13（Enabled panic 静默漏关）。`All()` 返回切片副本（`:95-103`）✓；`Unregister` 的 `append(order[:i], order[i+1:]...)` 在锁内，安全。 |
| `pkg/plugin/builtins.go` | `funcPlugin` 六个方法对 nil receiver 与 nil 函数字段**全部有守护**（`:26-73`）✓ → R7 的"适配器 nil 安全"目标达成。`enabled/getCfg` 对 `config.Config == nil` 有守护（`:77,:85`）✓。`isNilRegistry`（`:205-216`）✓ → R10 达成。`configured` 精确匹配（`:218-225`）✓ → N1 达成（有 X13 的兼容性提醒）。`Registry().enabledFn` 读的是**值类型** `Discovery`（`configure.go:35`），无 nil 风险。**遗留**：`Mysql/Sqlite/Mongodb/Redis` 的 `initFn` 返回的 error 永远为 nil（`Init` 无返回值），Init 失败对 `InitAll` 不可见——建议 `Redis/Mongo/Sqlite` 补 `InitWithError`。 |
| `pkg/plugin/builtins_test.go` | 仅 1 个用例（mysql 插件 init 错误传播）。**未覆盖**：enabled 精确匹配、typed-nil、`Order/Name` panic、`CloseAll` recover/漏关、`isNilRegistry`。 |
| `pkg/db/dao/mysql.go` | R5/R8 主体已修。X9（副本语义兼容性 + `WithTx` nil）。其余精读无新增 panic：`cacheQueryString/cacheKey/loadCached/storeCached`（`:149-226`）均 comma-ok + `cacheEnabled` 守护；`Pager`（`:591-634`）`size` 有下界（`:602-604`）、`math.Ceil` 无除零。性能提示：`All/One/Exists/Count` 在 `cacheEnabled()==false` 时**仍然**完整执行 `cacheKey` 的两次 JSON 序列化（`:463,:507,:539,:566` → `:140-147` → `:149-182`），纯浪费。 |
| `pkg/db/dao/mgo.go` | R9 已修（`:112-114`）。所有方法**值接收者**，`m.Tag = notag` 只改副本，安全。`conn == nil` 守护齐全（`:37/:62/:87/:112/:137/:164/:193`）。无新增风险。 |
| `pkg/db/dao/redis_cache.go` | N4 未修（X14）。其余每个方法都有 `c.Client == nil` 守护（`:32/:39/:50/:60/:83`）。`var _ cacheStore.ICache` 断言在位（`:109`）。 |
| `pkg/db/dao/mysql_cache_test.go` / `mysql_lifecycle_test.go` | 3 个用例，覆盖 cacheKey 稳定性、缓存按表失效、无效连接拒绝。无风险，但**未覆盖 R5 的副本语义**。 |
| `pkg/db/dao/CACHE_README.md` | `L42` 的示例仍未检查 `GetConnection` 的 err（nil client 已被 `RedisCache` 守护，不会 panic，仅缓存静默失效）——建议顺手修文档。 |

---

## 5. 三套数据源生命周期契约对照（X16 的证据）

| 维度 | MySQL（模板） | Redis（本轮修复后） | Mongo（本轮修复后） |
|---|---|---|---|
| 互斥量 | `mu RWMutex` + `initMu` | `mu RWMutex` + `initMu` | `mu RWMutex` + `initMu` |
| `closed` 标志 | **有**，`Close` 幂等（`:282-286`） | **无** | **无** |
| `Close` 是否清配置 | **清**（`confData=nil`，`:292`） | 不清 | 不清 → **X2 复活** |
| `Init` 建连位置 | **锁外**（RCU，`:80` 前） | **锁内**（`:42` 起全程） | **锁内**（`:43` 起全程） |
| `GetConnection` 锁类型 | RLock 取快照后立即释放（`:434-447`） | RLock（`:275`）+ 调 `redisCheck` | **写锁**（`:219`）→ **X3** |
| 锁内是否有网络 I/O | 否（`pingMySQL` 在锁外 `:330`） | **是**（`Check:241`、`Init:176`）→ **X1** | **是**（`mgoCheck:162/166` 在写锁内）→ **X1/X3** |
| 关闭后 `GetConnection` | `ErrClosed` ✓ | error（"connection failed"）✓ | **重新拨号** ✗ → **X2** |
| 关闭后 `Check` | `ErrClosed` ✓ | **nil（假阳性）** ✗ | 重新拨号（可能 nil）✗ |
| 锁外关旧连接 | ✓（`:100-101`） | ✗（锁内 `:197`） | ✗（锁内 `:142`） |
| 连接有效性校验 | `validateMySQL` ✓ | 无（`clients` 只存非 nil） | `conn.conn == nil` 浅判 |

**结论**：`MysqlClient` 是唯一完整实现"锁 + closed + RCU + 锁外 I/O"的数据源；Redis/Mongo 只借用了 `mu/initMu` 的**形状**，没有借用**纪律**。这正是 X1–X4、X7、X11 的共同根因。

---

## 6. 测试覆盖评估（"绿灯不代表正确"的具体证据）

| 包 | 现有测试文件 | 用例数 | 覆盖本轮修复点？ |
|---|---|---|---|
| `pkg/db/mysql` | `mysql_test.go` | 12 | **是**（`closed`/并发 initialize/RCU/pool） |
| `pkg/db/mongo` | **无** | 0 | **否**（X1/X2/X3/X4 全部无保护） |
| `pkg/db/redis` | **无** | 0 | **否**（X1/X7/X8/X11 全部无保护） |
| `pkg/db/sqlite` | **无** | 0 | **否**（X10 无保护） |
| `pkg/db/dao` | `mysql_cache_test.go`、`mysql_lifecycle_test.go` | 3 | 部分（未覆盖 X9 副本语义） |
| `pkg/plugin` | `builtins_test.go` | 1 | 否（X5/X6/X13 无保护） |

> 这就是"`go test ./pkg/db/... ./pkg/plugin/...` 全绿"却没有意义的直接原因：**本轮改动最激进的两个包（redis/mongo）一个测试文件都没有**。建议把本轮 X 系列每条至少落一个 seam（见 §7）。

**⚠️ 入仓约束（工程师提出、我采纳，硬约束而非建议）：X 系列回归用例不得单独入仓，必须与修复同 PR，且能在修复前的 commit（`aaae6a3`）上复现为红。**
理由：当前代码 X5/X10 均未修，`TestPluginMetaPanic` 之类一旦单独进 `pkg/plugin/`，`go test ./pkg/plugin/` **立刻变红**——而本轮验收标准恰是"基线三绿"，红用例单独落盘等于自己把验收标准做废。最现实的失败路径是：测试先入仓 → CI 长期红 → 被 `t.Skip` 注释掉 → 半年后没人记得为什么 skip。正确姿势：`fix + test` 一次提交，PR 描述里贴"该用例在 `aaae6a3` 上必红"的实测输出（`[recovered, repanicked]` / `FAIL`）作为证据。

---

## 7. 交给工程师的复现/回归清单（含我本轮已跑过的结果）

> 下列探针我已实际执行过（输出见正文），临时文件已删除。工程师可原样重建，务必作为**修复前的失败用例 + 修复后的回归用例**。

| # | 目标 | 最小构造 | 我实测结果 |
|---|---|---|---|
| G1 | X1(a) Redis 锁内 I/O | 本地起一个 `net.Listen` 只 accept 不回复的 TCP 服务；`r.Init([]byte("go:\n  data:\n    redis:\n      host: <h>\n      port: <p>\n"))` 于 goroutine，500ms 后主 goroutine `r.GetConnection()` 计时 | 阻塞 **2.50s** |
| G2 | X1(a) Redis Check 持读锁 | 注入 `clients["0"] = goredis.NewClient(&Options{Addr: hung, ReadTimeout: 2s, MaxRetries: -1})`；goroutine `r.Check()`，300ms 后 `r.Close()` 计时 | Close 被阻塞 **1.70s** |
| G3 | X1(b) Mongo Init 持写锁 | `m.confData = mongodb://127.0.0.1:1/testdb`；goroutine `m.Init(cfg)`，300ms 后 `m.ListConnNames()` 计时 | 阻塞 **9.70s** |
| G4 | X2 Mongo 关闭后复活 | `m := &Mongodb{confData: cfg, conns: map[string]connection{}}; m.Close(); m.GetConnection()` 计时；再 `m.Check()` 计时 | **10.001s** / **10.000s** |
| G5 | X4 tags 被擦除 | `m := &Mongodb{multi:true, conns:{a,b}, tags:[a,b]}; m.Init(nil); m.ListConnNames()` | 返回空切片 |
| G6 | X5 Order() 穿透 | 注册 **≥2 个**插件（1 正常 + 1 `Order()` panic），`InitAll` / `CloseAll` / `HealthAll` **三入口**各跑一次；**断言只写「不 panic」+「崩溃插件未污染另外两个正常插件的 Init/Close 状态」**，不要写 `recovered != nil`（断言 bug 存在）、也不要写「返回 error」（绑死修复形态，见 §10.8）；**必须用 `defer recover()` + 立即 `t.Fatalf` 落地**（不是让 panic 穿透——那会杀掉整包，见 §10.10） | `ORDER_PANIC`（三入口全部逃逸） |
| G7 | X5b/X6 Name 缺口 | 两条独立用例：① 直接 `Register` 一个 `Name()` panic 的插件 → 应在 `Register:49` 失败、不进注册表（`Register` 无 error 返回，实际表现为 panic → 说明注册侧也要 `safeName`）；② **先正常 `Register`，再令条目 `Name()` 劣化** → `InitAll` 复现 `NAME_PANIC`（原 `INIT_PANIC` 被 recover 分支里的 `p.Name()` 二次 panic 替换） | `NAME_PANIC` |
| G8 | X7 Redis 关闭后假阳性 | `r.Init(无效配置); r.Close(); r.Close(); r.Check()` | 二次 Close 不 panic；`Check err=<nil>` |
| G9 | X10 sqlite 零值 gorm.DB | `(&Sqlite{sqlite: &gorm.DB{}}).Close()` / `.Check()` / `.GetConnection()` | 前两者 **panic**；第三者返回 `conn!=nil, err=nil` |
| G10 | X9 WithTx nil | `(*MySQLDao[E])(nil).WithTx(nil)` | 预期 panic（`clone := *m`） |

> 建议同时补：`-race` 下并发 `Init/Close/GetConnection`（Redis+Mongo）应**不再**报 map race（这是本轮修复的**正向回归**用例，必须进仓）；以及 `TestMongodbCloseIsTerminal`（G4 的正例断言）、`TestRedisCheckAfterCloseReturnsErrClosed`（G8 的正例断言）。

---

## 8. 范围外但相邻的问题

（不属指派范围，但与 panic/关停直接相关，供主理人决定是否另开单）

1. **`mgin.go:124-126`**：`if err := logs.InitVictoriaLogs(); err != nil { panic(err) }` —— 启动期**显式 panic**，`plugin.InitAll` 的 recover 完全无关。若日志后端不可达即整进程崩溃，建议降级为 error + 本地日志兜底。
2. **`app.go:381-395` 关停顺序**：`SafeExit()`（关 DB）在 `s.Shutdown(ctx)`（排空 HTTP）**之前**。这是 X2 与 R1/R2 关闭侧窗口的放大器。**建议调换顺序**：先 `Shutdown`（排空在途请求，此时 DB 仍可用）→ 再 `SafeExit`（关组件）。这是本轮**性价比最高的一行改动**（同时消除 X1/X2 的关停拖尾）。
3. **`cmd/templates.go:489/509`**：脚手架生成的示例依赖 `dao.WithContext(&ctx).All(...)` 的链式写法，与 X9 的 BREAKING 变更耦合；若后续回退副本语义，这里也要同步。
4. **`pkg/db/dao/CACHE_README.md:42`**：示例未检查 `GetConnection` 的 err。

---

## 9. 假设与未决问题

1. **`mgo.Session.Ping()` 的实际超时**：依赖库 `session.go:89` 用 `context.Background()`，`SetSocketTimeout` 是空实现（`session.go:230-235`），因此 Ping 的超时取决于 mongo-driver 的 `serverSelectionTimeout`（默认 30s）。**我的探针用的是"不可达地址 + Dial"路径（10s，来自 `DialWithTimeout`），未能直接测到 `Ping` 的 30s 上界**（需真实 Mongo 半开连接）。请工程师用容器化 Mongo + `iptables DROP` 或 toxiproxy 验证 `mgoCheck` 在写锁内的真实阻塞上界。这不影响 X1/X3 的结论（`Lock` + 锁内 I/O 已由 G3 证实），只影响"最坏阻塞时长"的数值。
2. **X2 的泄漏在生产的可见度**：若进程在 `Shutdown` 后立即退出，OS 会回收 FD，"泄漏"表现为"关停拖尾"而非长期泄漏；只有在库被复用/热重载/测试进程存活时才是真泄漏。报告中已按此区分。
3. **`go.config.used` 的线上实际取值**：X13 的"漏启用"风险取决于是否存在依赖子串匹配的配置（如 `redis-cluster`、`mysql_backup`）。我无线上配置样本，请运维确认；该风险**不会导致 panic**（未启用 → `GetConnection` 返回 error），只导致功能静默失效。
4. **`Redis PSubscribe` 的生产用法**：全仓 grep 无外部调用方（仅 `redis.go` 自身），故 X7/X8 的实际暴露面取决于业务侧是否使用（`PSubscribe` 是导出方法）。若无使用方，X7/X8 可降为 P3。
5. 本轮严格限定在 panic/健壮性视角；性能压测（X3 的 QPS 上限）结论为静态推导，需工程师用真实 Mongo 跑基准确认。

---

## 10. 交叉验证与修正（工程师独立复现后回传，我逐条复核源码）

工程师（寇豆码）独立复现了我全部动态结论，数值几乎一致（Redis Init 阻塞 **2.5001s**、Check→Close **1.7009s**、Mongo Init **9.7010s**、Close 后复活 **10.0004s / 10.0011s**）。以下三条为对我报告的修正，**我已逐条对照源码验证，全部成立，正文已同步更新**。

### 10.1 X10 根因修正（**结论不变，根因更精确**）

- 我的原表述：「gorm.go:413 对零值 `ConnPool` 解引用」。
- 精确根因：`gorm.DB` **嵌入** `*Config`（`gorm.go:104-110`），`ConnPool` 是 `Config` 的字段（`gorm.go:66-67`）；`gorm.go:413 connPool := db.ConnPool` 实际是 `db.Config.ConnPool`，因此 **`&gorm.DB{}` 的 `Config == nil` 才触发 SIGSEGV（addr=0x80）**，与 `ConnPool` 是否为 nil 无关。
- 决定性对照（工程师实测）：`&gorm.DB{Config: &gorm.Config{}}`（Config 非 nil、ConnPool 仍 nil）**不 panic**，`DB()` 返回 `(nil, ErrInvalidDB)`（`gorm.go:431`）——此时原有的 `err == nil && db != nil` 守护是有效的。
- **对修复的影响**：`validateSqlite` 的三重判断 `conn == nil || conn.Config == nil || conn.Statement == nil` 中，**`Config == nil` 是决定性的那一项**；只判 `conn == nil` 必然漏。我 §2 X10 的修复建议已满足该要求，仅补注这一点。

### 10.2 X5 补一条入口（**性质升级，正文已改**）

- 我原文只列 `InitAll` / `HealthAll` 两个逃逸点。
- 工程师补齐：**`CloseAll` 同样在 `plugin.go:149 ps := sortedByOrder()` → `:109 ps[i].Order()` 处被打穿**，实测 `ORDER_PANIC` 从三个入口全部逃逸。
- **性质变化**：这不是"启动期 + 运行期定时器"两点风险，而是 **`InitAll` / `CloseAll` / `HealthAll` 三条主链路全裸**——连**关停**都能被自定义插件打穿（而 `CloseAll` 是专门为"防漏关/防 panic 中断关闭链"设计并加了 `defer+recover` 的，结果在排序阶段就失守）。§2 正文已补。

### 10.3 X1(a) 阻塞上界比我写的更坏（**量级修正**）

- 我原文：`Init` 锁内阻塞 ≈ 库数 ×（DialTimeout 5s + ReadTimeout 3s）。
- 源码复核（`redis.go:162-171`）：`if r.conf.Int("go.data.redis_pool.max") > 1 { connectTimeout := r.conf.Int("go.data.redis_pool.timeout"); if connectTimeout == 0 { connectTimeout = 60 }; ... rds.DialTimeout = time.Duration(connectTimeout)*time.Second }` —— **缺省 60s**，且 `:174-184` 是**逐库串行** Ping/建连。
- 正确上界：**库数 × 60s**（黑洞地址/半开连接场景）。比我原估计（库数 × 8s）差一个数量级。§1 X1(a) 正文已改。**这意味着「关停卡死」在默认配置下就可能达到分钟级，P1 定级无需调整，但修复紧迫性更高。**

### 10.4 X7 补充：`Receive()` 无超时（**新增事实，非我原假设**）

工程师原本假设"订阅确认失败 = 1s 忙轮询"，实测**自我推翻**：3.5s 窗口内只发出 1 次 `PSUBSCRIBE`，`pubsub.Receive()`（`redis.go:367`）**直接不返回**。即：无超时、无退避、无失败感知——既不重连也不报错，比"1s 重试"更隐蔽（后者至少会打日志）。§2 X7 修复建议已补「`Receive()` 要带超时」。

### 10.6 X5 拆分为 X5 / X5b（**定级修正，我采纳**）+ 测试写法要求

工程师第二轮反馈两点，我复核源码后全部接受，已把原 X5 拆成两条（正文 §2 已改、§0.3 表已拆、§6 G6/G7 已重写）：

**(1) `Order()` 与 `Name()` 的暴露面不同，不应合并定级。**
- **X5（`Order()`，P1）**：`sortedByOrder()` 在**任何 recover 之前**执行（`plugin.go:106-112`，`Order()` 在 `:109`），`InitAll`/`CloseAll`/`HealthAll` 三个入口共同依赖 → **无条件可复现、三链路全裸**。
- **X5b（`Name()`，P2）**：`Register:49 name := p.Name()` 在注册表锁**外**先调用一次 `Name()` → **"无条件 panic 的 `Name()`"根本进不了注册表**（在 `Register` 就炸，属调用方问题）。因此 X5b 的真实暴露面是**"注册之后 `Name()` 才变坏"**——典型场景：插件 `Init()`/`Close()` 失败后内部状态被清空（配置指针置 nil、懒加载的 name 被释放），此后 `sortedByOrder` / `InitAll:131` / `safeInit` 的 recover 分支（`X6`）再调 `Name()` 就地二次 panic。工程师是「先正常 `Register`、再替换注册表条目模拟状态劣化」才复现的；我探针8 走的也是等价路径（直接写注册表绕过 `Register`）。
- 两条**修复手段相同**（注册时把 name/order 缓存进注册表条目，运行时不再调接口方法），但**风险等级不同，终稿请分开列**。

**(2) 测试写法与断言要求（这条比结论更容易被后人踩坑；断言部分**已被工程师的穷举验证修正**，见 §10.8）。**
- 必须 **≥2 个插件**：`sort.Slice` 对 n=1 不调用 `less`，`Order()` 根本不被执行 → 单插件用例会**假绿**（我第一次探针就是 `recovered=nil`，工程师独立踩了同一个坑）。**n≥2 则恒被比较——工程师穷举 n=2/3/4 的全部排列 × 全部 panic 落位共 118 组，无一假绿**，故 §10.6 原先"n 更大时不该赌 pdqsort"的谨慎表述**可以删除**：唯一陷阱是 n=1。
- **断言写法见 §10.8（我上一版写的"三入口均返回 error"是错的，会反向失败）。**

### 10.8 断言形态修正（**我上一版建议有误，工程师反驳成立，已改**）

我上一轮给 team-lead 和工程师的断言建议是「三入口均返回 error 且未 panic」。工程师给出两个反例，我复核后**全部成立**，这是本报告唯一一处**我自己的建议被推翻**，必须显式更正：

**反例 A（最可能要落的修复）**：`safeOrder` 在 recover 后返回 `math.MaxInt` → `sortedByOrder` 正常返回 → `safeEnabled`/`Name()`/`safeInit` 全部走完 → **`InitAll` 返回 `nil`**。此时"断言返回 error"**当场反向失败**。
**反例 B（我的修复建议本身有洞）**：`func Register(p Plugin)` **无 error 返回值**（`plugin.go:45` 已复核）。若按我原话"在 `Register` 时缓存 name/order"，panic 只是从 `InitAll` **搬到注册期**，而 `Register` 无法表达失败。→ 修复必须**两端都守护**（注册侧 `safeName/safeOrder` + 缓存；运行期读缓存），我已把这条补丁写进 §2 X5 的修复建议。

**我上一版的逻辑错误**：我先批评 `recovered != nil` 是"断言 bug 存在、绑死 bug 侧"，随后又把断言换成"返回 error"——那只是**换了一种绑死**（绑死修复的具体形态）。两类断言都不满足"修复前后都成立"。

**正确形态（三条，均不依赖修复形态）**：
1. **永真契约**：三个入口**不得 panic**。这是 X5 的契约本身，且修复前必然真 panic（`testing` 直接 FAIL）、修复后 PASS → **唯一可长期入仓的断言**。
2. **永真契约**：崩溃插件**不得污染别的正常插件**（断言另外两个正常插件的 Init/Close 状态、`HealthAll` 里它们的键存在且为 nil）。
3. **可选（按修复策略二选一）**：若修复选择"跳过坏插件"→ 可断言坏插件未被 Init/Close；若选择"`MaxInt` 兜底"→ **不能**断言它未被 Init。第 3 条属于测试修复策略，不属契约，**不要写进回归基线**。
4. **实现细节（**我上一版本条写错了，被工程师精化，见 §10.10**）**：**必须**用 `defer recover()` 包住调用，但 recover 之后要**立即 `t.Fatalf`**，**绝不能**"recover 之后断言 `recovered == nil` 为通过"。我上一版写的"不要用 `defer recover()` 包住调用"把两件事混为一谈了：**吞掉 panic 才是假绿**，把 panic 转成干净 FAIL 才是正确形态。

> 结论：**「不 panic」是修复前后都成立、且修复前必然失败的断言；`recovered != nil` 与「返回 error」都不合格**。§6 的 G6/G7 已按此重写。

### 10.9 未验证项（承接 10.5/10.7，不作结论）

以下两类我在 §9 已标注为假设，工程师同样无法在本机构造，**明确留待真实依赖环境验证**：
1. 真实 `*mgo.Session` 的 `Copy()/DB()` 并发路径与 `Ping()` 的 30s 上界（`maczh/mgo` 的 Session 字段全不导出、`DialWithTimeout` 必须 Ping 成功才返回，本机无 mongod）。
2. X3 的 QPS 线性、X8/X11/X12/X15/X16（均为静态确认，未动态验证）。

### 10.10 `assertNoPanic` 形态（**工程师精化，我上一版表述过粗，已采纳**）

**事实（工程师实测，符合 Go `testing` 语义）**：`t.Run` 里未被 recover 的 panic **不会**"标记失败后继续"——`testing.tRunner` 的 defer 先标记失败再 **re-panic**，**整个测试二进制直接退出，同包后续用例一个都不跑**。他的证据：本轮 sqlite 探针中一次未 recover 的 `Check()` panic，输出 `[recovered, repanicked]` + `FAIL ... 0.015s`，而同包第二个用例 `TestZZVerifySqliteRealLifecycle` **连 `=== RUN` 都没打印**——根本没执行。
→ 所以"让 panic 自然上抛"虽然能让用例变红，**代价是杀掉整包**：看不到"其他用例是好是坏"，QA 全场扫描时容易误判成"测试框架炸了"。

**正确形态：recover + 立即 `t.Fatalf`**（推荐的落盘形态）
```go
// 修复前 → 该子用例 FAIL（干净红，不是 crash），同包其他用例照常跑完
// 修复后 → PASS，可长期入仓
func assertNoPanic(t *testing.T, label string, fn func()) {
    t.Helper()
    defer func() {
        if r := recover(); r != nil {
            t.Fatalf("%s panic: %v", label, r)
        }
    }()
    fn()
}
```

**必须区分清楚的一对概念（这正是我上一版没讲清的地方）**：
| 形态 | 代码 | 性质 |
|---|---|---|
| ❌ 假绿 | `defer func(){ recovered = recover() }()`，然后断言 **无 panic 为通过 / 或断言 `recovered != nil` 为"复现成功"** | **吞掉 bug**，修复前后都"绿"，完全不可入仓 |
| ✅ 干净红 | `defer func(){ if r:=recover(); r!=nil { t.Fatalf(...) } }()` | **转换失败形态**，修复前 FAIL、修复后 PASS |

两者都用 `recover`，但**一个是假绿、一个是干净红**。我 §10.8 第 4 条原话"不要用 `defer recover()` 包住调用"容易被实施者理解成"一行 recover 都不能写"，**必须按上表纠正**：recover 要用，但只能用来 `t.Fatalf`，不能用来"判定通过"。

**合成一句（工程师建议，我采纳）**：**recover 只能转 `t.Fatalf`，且必须在测试自身 goroutine 内同步调用 `fn()`**。
这两件事是同一约束的两面：`t.Fatalf` 在 defer 中合法（内部 `runtime.Goexit()`，不会二次 panic），前提正是"处于本测试 goroutine"；而把 `fn()` 丢进 `go func(){}` 后，子 goroutine 的 panic **无法**被父 goroutine 的 recover 捕获，且 `t.Fatalf` 在非测试 goroutine 上调用是未定义行为（挂在没有 `tRunner` 的栈上）。

**附带好处**：把三入口写进子测试名（`InitAll/CloseAll/HealthAll` × `order/name-after-register`），失败时一眼看出**哪个入口失守**。

### 10.11 落盘策略约束（**工程师更正我的理由，我采纳：是硬约束，不是"更稳"**）

我在上一轮答复里说"骨架不落盘，等 QA 收尾后一次性入仓**更稳**"。工程师更正：这不是"稳不稳"的偏好问题，而是**硬约束**——现在落盘会**直接破基线**。理由链：
1. 当前代码 **X5/X10 均未修**，`TestPluginMetaPanic` 一旦进 `pkg/plugin/`，`go test ./pkg/plugin/` **立刻变红**；
2. 本轮验收标准是"基线三绿"，**红用例单独落盘等于自己把验收标准做废**；
3. 最现实的失败路径：测试先入仓 → CI 长期红 → 被 `t.Skip` 注释掉 → 半年后没人记得为什么 skip（**假绿变成了永久的静默**，比不写测试更糟）。

**正确姿势**：`fix + test` 同 PR 一次提交，PR 描述里贴"该用例在修复前 commit（`aaae6a3`）上必红"的实测输出（`[recovered, repanicked]` + `FAIL`）作为证据。此约束已写入 §6 入仓约束。

**复验状态（工程师在 QA 清理后独立复跑，我同步骤一致）**：`git status --short` 只剩 ` M .gitignore`；`find pkg -name 'zz_*_test.go'` = **0**；`go test ./pkg/db/... ./pkg/plugin/...` 与 `go vet` 均 exit 0。→ **终稿可写"工作树完全干净、基线三绿"，不必带"QA 临时文件"的尾巴。**

> **§10 结论**：四轮交叉验证后——**X1 / X10 的定级与修复方向不变；X5 拆分，`Order()` 上调 P1、`Name()` 保持 P2**；实施级修正共五条：**X5 修复必须两端守护**（§2）、**断言只能写「不 panic + 不污染正常插件」**（§10.8）、**n≥2 恒被比较（唯一陷阱 n=1）**（§10.6）、**recover 只能转 `t.Fatalf` 且必须在测试自身 goroutine 内同步调用**（§10.10）、**回归用例须与修复同 PR、不得单独入仓**（§10.11 / §6）。累计经复核的修正共 9 条：① X10 根因 `Config == nil`；② X5 含 `CloseAll`→三链路全裸；③ X1(a) 上界库数×60s；④ X7 `Receive()` 无超时；⑤ X5/X5b 拆分定级；⑥ n≥2 恒被比较（118 组穷举，唯一陷阱 n=1）；⑦ 断言形态改为「不 panic」；⑧ recover 必须转 `t.Fatalf`（澄清"假绿 vs 干净红"）；⑨ 落盘须与修复同 PR（硬约束，非偏好）。工程师全文见 `deliverables/software-company/panic-audit-2026-11-25/05-engineer-repro.md`。

（报告完）
