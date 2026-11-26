# 架构师静态评审报告：pkg/db/ 与 pkg/plugin/builtins.go 的 panic 风险

- 评审人：高见远（Gao）· 架构师
- 日期：2026-11-25
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，工作树干净（未修改任何生产代码）
- 范围：`pkg/plugin/builtins.go`、`pkg/db/db.go`、`transaction.go`、`pkg/db/{mysql,redis,mongo,sqlite}/*`、`pkg/db/dao/*`（含测试与 CACHE_README）
- 方法：逐文件精读（非 grep）；对 `config` / `registry` / `job` / `cache` 关键依赖做定点确认；对照 `9825b9b`「修复mysql健壮性」、`a07e077`「代码审核…事务处理和Where缓存」、`fe75373`「指针安全性与健壮性修复」三个提交的实际 diff。

---

## 0. 结论摘要

| 编号 | 文件:行 | 风险 | 严重度 | 定性 |
|---|---|---|---|---|
| R1 | `redis/redis.go:17-26,40,66-69,103-104,163-173,177-189,234-262` | 客户端无锁，多 map 并发读写 → `fatal error: concurrent map read and map write`（不可 recover） | **P0** | 高概率（并发时序） |
| R2 | `mongo/mgo.go:16-24,34-118,120-137,139-166,168-188,190-223` | 同上；且 `Check/mgoCheck` 内回调 `Init` 写 map，`GetConnection:211` 链式解引用 `.conn.Copy()` | **P0** | 高概率（并发时序） |
| R3 | `plugin/plugin.go:101-117`（对比 `123-147`） | `InitAll` 无 defer/recover，任一 enabled 插件 Init panic 穿透 `mgin.Init:141` | **P1** | 确认的结构性缺口 |
| R4 | `sqlite/sqlite.go:39-45`（重点 `41-42`） | `db, _ := m.sqlite.DB(); db.Close()` 忽略 error，db=nil 时解引用 panic | **P1** | 高概率（特定输入必现） |
| R5 | `dao/mysql.go:70-72,84,90-93,96-105` 及全部写 `Tag` 的 CRUD | 共享 DAO 实例的链式配置字段无锁读改写 → data race，极端下 `*m.ctx` 解引用 panic | **P1** | 高概率（共享用法） |
| R6 | `redis/redis.go:273-304`（重点 `286-288,289-293`） | 修改调用方 `channels` slice + 失败重连不 `pubsub.Close()` → race + 连接泄漏 | **P1** | 确认（缺陷明确） |
| R7 | `plugin/plugin.go:42-53`（重点 `43,46`） | `Register` 只判 `p==nil`，未拦 typed-nil → `p.Name()` 解引用 panic | **P2** | 确认（给定输入必现） |
| R8 | `dao/mysql.go:292,317,342,368,393,418,466,501,531,611,624` | nil receiver 时 `receiver.Tag = notag` 在 nil 守护之前 → panic | **P2** | 确认（误用必现） |
| R9 | `dao/mgo.go:100-119`（重点 `112-113`） | `Update` 缺 `conn==nil` 检查（其他方法都有） | **P2** | 理论（并发窗口） |
| R10 | `builtins.go:129-146` | Registry 插件对 nil/typed-nil `registry.Registry` 解引用 | **P2** | 确认（非标准启动必现） |
| R11 | `plugin/plugin.go:150-159` | `HealthAll` 无 recover，health 插件 panic 穿透 `checkAll` | **P2** | 理论（内置 healthFn 已有保护） |
| R12 | `mysql/mysql.go:85-101` | 发布新连接后锁外关闭旧连接的 use-after-close 窗口 | **P2** | 理论（sql.DB.Close 安全，非 panic） |
| R13 | `redis/redis.go:191-232`、`sqlite/sqlite.go:47-49` | `Check` 全空实现，健康检查形同虚设（掩盖故障，非直接 panic） | **P2** | 确认（逻辑缺口） |

**一句话总览**：三个健壮性提交只系统性加固了 MySQL（锁 + RCU + nil 守护），**Redis / Mongo / SQLite / DAO 层未被同等加固**；其中 Redis/Mongo 的「无锁 + 可写共享 map」是唯一能被 Go runtime 以不可恢复的 fatal error 直接打挂进程的问题，应列首要。

---

## 1. P0 风险（线上可触发的进程崩溃）

### R1 — Redis 客户端完全无锁 + 多个可写共享 map（确认的缺陷，高概率触发）

**精确位置**（`pkg/db/redis/redis.go`）：
- 结构体字段 `clients map[string]redis.UniversalClient` / `cfgs` / `conns` / `multi`：`L17-26`，无任何 mutex。
- `Init` 写 map：`L66-69`（`r.clients=...` / `r.cfgs=...` / `r.conns=...` / `r.multi=...`）、`L103-104`（`r.cfgs[dbName]`、`r.conns=append`）、`L163-173`（`r.clients[dbName]=rc`）。
- `Close` 删 map：`L177-189`，`L181 delete(r.clients, dbName)`、`L186 delete(r.clients,"0")`。
- `GetConnection` 读 map：`L234-262`，`L235` 读 `r.multi`、`L239/L246` 读 `r.clients[...]`、`L256` 读 `r.clients["0"]`。
- `PSubscribe` 后台 goroutine 读：`L273-304`，`L276` 调 `GetConnection`。

**触发条件**：任意两个上述路径并发。真实链路：
1. 优雅关闭：`mgin.SafeExit` → `plugin.CloseAll`（逆序）→ `Redis()` 的 `closeFn`（`builtins.go:110`，`db.Redis.Close()`）与在途请求的 `db.Redis.GetConnection()` 并发 → `delete` 与 `mapaccess` 同时发生。
2. 启动期：`plugin.InitAll` → `db.Redis.Init()` 写 map，与已启动的后台任务/健康检查读 map 并发（`Job` order=90 虽在 Redis order=15 之后，但自定义插件或预热 goroutine 可在 Init 期访问）。
3. 若按 `CACHE_README.md:42` 用 Redis 作 DAO 缓存，则每个 DB 查询都经 `RedisCache` → client 持有，关闭时同样竞态。

**后果**：`fatal error: concurrent map read and map write`（Go runtime `throw`，**不是 panic，`recover` 无法捕获**）→ 整个进程崩溃。`plugin.CloseAll` 的 `defer recover`（`plugin.go:133-138`）对此无效。

**调用链**：`mgin.SafeExit` → `plugin.CloseAll` → `builtins.Redis().closeFn` → `db.Redis.Close()` ⇄ 业务 goroutine → `dao.RedisCache.Get/Set` → `db.Redis.GetConnection()`。

**修复方案（代码级）**：
- 参照 `mysql.MysqlClient` 的成熟范式：为 `RedisClient` 增加 `mu sync.RWMutex`，所有 `clients/cfgs/conns/multi` 读写入锁；`Init` 采用「锁外构建新 map、锁内一次性发布、锁外关闭旧 client」的 RCU 流程；`Close` 取写锁后置 `closed=true` 并摘出旧 clients，锁外 `Close()`。
- `GetConnection` 用 RLock 取出 client 引用后立即解锁，再做 `redisCheck`/返回，禁止持锁执行网络 I/O。
- 增加 `closed bool` + `ErrClosed`，`Close` 幂等，关闭后 `GetConnection` 返回 error 而非操作已关闭 client。
- 不要只在 `Close`/`GetConnection` 加锁而漏掉 `Init`/`PSubscribe`——四处必须成对。

**可证伪预测**：如果「Redis 无锁 map」是根因，那么用 `go test -race` 让 N 个 goroutine 持续 `GetConnection()` 同时一个 goroutine 循环 `Close()+Init()` 就会报 `DATA race on r.clients` 并在高并发下直接 `fatal error: concurrent map read and map write`；给四个字段访问统一加 `RWMutex` 后该 race 与 fatal 均消失。

---

### R2 — MongoDB 客户端无锁 + 重连写 map + `GetConnection` 链式解引用（确认的缺陷，高概率触发）

**精确位置**（`pkg/db/mongo/mgo.go`）：
- 字段 `conns map[string]connection` / `tags []string` / `multi`：`L16-24`，无 mutex。
- `Init` 写：`L42-45`、`L90-95`（multi，`m.conns[dbName]=...`、`m.tags=append`）、`L111-115`（单库 `m.conns["0"]=...`）。
- `Close` 删：`L120-137`，`L129 delete(m.conns,k)`、`L135 delete(m.conns,"0")`。
- **`mgoCheck` 读+写+回调 Init**：`L139-166`，`L143-145 if len(m.conns)==0 { m.Init(m.confData) }`、`L146` 读 `m.conns[tag]`、**`L159 m.conns[tag]=...`（重连后替换）**。
- **`Check` 回调 Init**：`L168-188`，`L173-175 if len(m.conns)==0 { m.Init(m.confData) }`、`L177 for dbName := range m.conns`。
- **`GetConnection`**：`L190-223`，multi 分支 `L198-206` 读 `m.tags`/`m.conns`，`L207 mgoCheck(...)`，**`L211 return m.conns[dbName[0]].conn.Copy().DB(...)`（三重解引用，无 `conn==nil` 守护）**；单库分支 `L213 m.Check()`（忽略错误）、`L217-221` 读 `m.conns["0"]` 并在 `L218` 有 nil 检查。

**触发条件**：
1. **并发 map 崩溃**：`checkAll`（`mgin.go:175`，每 5 分钟）→ `HealthAll` → `builtins.Mongodb().healthFn` → `db.Mongo.Check()` → 可能 `m.Init()`/`mgoCheck()` 写 map；同时 DAO 查询（`dao/mgo.go:32/57/82/...`）→ `db.Mongo.GetConnection()` 读 map → `concurrent map read and map write` fatal。优雅关闭时 `Mongo.Close()` 删 map 与在途 DAO 请求并发同理。
2. **nil 解引用**：`GetConnection` 在 `L207 mgoCheck` 之后到 `L211` 之间，若另一 goroutine 执行 `Close()`（`L129/L135` 删除 conn），`L211 m.conns[dbName[0]]` 取到零值 `connection{conn:nil}`，`.conn.Copy()` → nil 指针解引用 panic。（单 goroutine 下 `mgoCheck:L147` 保证 conn 非 nil，故非必现。）

**后果**：同 R1——不可恢复 fatal 或 nil panic，进程崩溃；`CloseAll` 的 recover 无效。

**调用链**：`checkAll`/`SafeExit`/业务 DAO ⇄ `db.Mongo.{Check,Close,GetConnection}` → `mgoCheck` → `Init`/`m.conns[...]`。

**修复方案（代码级）**：
- 与 R1 同：加 `sync.RWMutex`（建议 `initMu + mu` 分离，仿 `mysql.go:19-20`），`conns/tags/multi` 全部入锁；`Init` 锁外建连、锁内发布；`Close` 写锁摘旧 session、锁外关闭。
- `GetConnection`：在 RLock 内一次性取出 `conn` 与 `db` 到本地变量并解锁，随后 `conn.Copy().DB(db)`；取到 `conn.conn == nil` 时返回 error，杜绝 `L211` 链式解引用。
- 删除 `Check`/`mgoCheck` 内的隐式 `Init`（健康探针不应触发重连初始化）；重连逻辑收敛到单一受锁保护的方法，`mgoCheck` 的 `L159` 写必须持写锁。

**可证伪预测**：如果「Mongo 无锁 map + L211 链式解引用」是根因，那么并发执行 `Check()`（强制 `m.conns` 为空以触发 `Init`）与多 goroutine `GetConnection("tag")` 会触发 race/`concurrent map read and map write`，而让 `Close()` 与 `GetConnection("tag")` 交错会命中 `L211` 的 nil panic；加 `RWMutex` 并把 `L211` 改为「取本地 conn + nil 守护」后两者均消失。

---

## 2. P1 风险（特定配置或并发下可触发）

### R3 — `InitAll` 缺 defer/recover，与 `CloseAll` 不对称（结构性缺口，确认）

**位置**：`plugin/plugin.go:101-117`（`InitAll`）对比 `L123-147`（`CloseAll` 已在 `L132-138` 用 defer/recover 兜底每个 Close）。

**触发条件/调用链**：`mgin.Init:141 plugin.InitAll(context.Background())` → `p.Init(ctx)` 无 recover。任一 `Enabled()==true` 的插件 Init panic 会穿透整个 `mgin.Init`。高风险来源：
- 自定义插件（`mgin.Use`/`plugin.Register`）的 Init 未自检；
- `builtins.Registry().initFn`（`builtins.go:129-145`）中 `registry.Registry.Register(...)`：若 `registry.Registry` 为 nil 接口（未走 `mgin.Init:127` 赋值）或 typed-nil，直接 panic。虽然正常 `NewRegistry()`（`registry.go:44-69`）总返回非 nil（含 `noopRegistryClient`），但脱离标准启动顺序即失效。

**后果**：启动期进程崩溃；且 `InitAll` 的「首个错误继续」语义（`L99-100`）对 panic 无效。

**修复方案**：在 `InitAll` 循环内仿 `CloseAll` 加 `func(){ defer func(){ if r:=recover(); r!=nil { /* 记录为 error，继续 */ } }(); ... p.Init(ctx) ... }()`，把 panic 转成 error 计入 `firstErr` 并 `logs.Error`；`HealthAll`（R11）同理。

**可证伪预测**：如果「InitAll 无 recover」是根因，那么注册一个 `Init` 中 `panic("x")` 的 enabled 插件后调用 `plugin.InitAll` 会直接崩溃；在循环内加 defer/recover 后 `InitAll` 会返回 error 且后续插件仍被初始化。

---

### R4 — `sqlite.Close` 忽略 `DB()` error 导致 nil 解引用（高概率，特定输入必现）

**位置**：`pkg/db/sqlite/sqlite.go:39-45`，核心 `L41 db, _ := m.sqlite.DB()`、`L42 db.Close()`。

**触发条件**：`m.sqlite != nil` 但其 `ConnPool` 不是 `*sql.DB` 时，`gorm.DB.DB()` 返回 `(nil, ErrInvalidConnPool)`，代码丢弃 error 后 `db.Close()` 对 nil 解引用。可达路径：
- `Init`（`L30-36`）失败时仅 `logger.Error`，未把 `m.sqlite` 复位为 nil；若 gorm.Open 在某些 dialector/路径错误下留下非 nil 但无效的 `*gorm.DB`，后续 `Close()` 命中；
- 测试或注入场景构造 `&Sqlite{sqlite: &gorm.DB{}}`。

**后果**：`Close` 阶段 panic；因 `builtins.Sqlite().closeFn`（`builtins.go:84`）在 `CloseAll` 内，通常被 recover 兜住，但会中断后续插件关闭链中 sqlite 自身的资源释放。

**对照证据**：同仓库 `mysql/mysql.go:262-269 closeMySQL` 正确写成 `if sqldb, err := conn.DB(); err == nil && sqldb != nil { _ = sqldb.Close() }`——sqlite 明显遗漏同类守护。

**修复方案**：
```go
func (m *Sqlite) Close() {
    if m == nil { return }
    if m.sqlite != nil {
        if sqldb, err := m.sqlite.DB(); err == nil && sqldb != nil {
            _ = sqldb.Close()
        }
        m.sqlite = nil
    }
}
```
并在 `Init` 失败分支显式 `m.sqlite = nil`；`Init`/`GetConnection` 加 `if m == nil` 守护。

**可证伪预测**：如果「`DB()` error 被忽略」是根因，那么 `(&sqlite.Sqlite{sqlite: &gorm.DB{}}).Close()` 会在 `L42` 以 nil 指针 panic；按上式加 `err==nil && sqldb!=nil` 守护后同一输入不再 panic。

---

### R5 — 共享 DAO 实例的链式配置字段无锁读改写（高概率，共享用法）

**位置**（`pkg/db/dao/mysql.go`）：
- 写 `m.Tag`：`L70-72`（`connection()` 内）、以及 `L292/317/342/368/393/418/466/501/531/611/624`（各 CRUD/Alias/DB）；
- 写 `m.ctx`：`L90-93 WithContext`；写 `m.cache`：`L96-105 WithCache`；
- 读并解引用 `m.ctx`：`L84 if m.ctx != nil && *m.ctx != nil`，`L303-305` 等重复出现。

**触发条件**：业务共享一个 `*MySQLDao[T]` 实例（常见做法），一个 goroutine 调用 `WithContext(&ctx)` / `WithCache(...)`，另一个 goroutine 调用 `All()/One()/...`。`m.Tag/m.ctx/m.cache` 的读改写无同步 → data race；若在 `connection()` 读到 `m.ctx != nil` 后另一 goroutine 将其置 nil，`L84 *m.ctx` 存在 nil 解引用窗口。

**后果**：`go test -race` 必报 race；特定时序下 nil 解引用 panic；缓存配置被并发覆盖导致行为异常。

**修复方案**：把 `WithContext`/`WithCache`/`Debug` 改为**返回新副本**（仿 `WithTx` 的 `L57-61 clone := *m`），不再原地写 receiver；`connection()` 内在入口把 `m.Tag/m.ctx` 读到局部变量后使用，避免重复解引用 `m.ctx`；若必须原地配置，则在文档中明确「配置完成后不可再共享修改」并对字段加 `RWMutex`。

**可证伪预测**：如果「共享 DAO 字段无锁」是根因，那么一个 goroutine 循环 `dao.WithContext(&ctx)`、另一个循环 `dao.All(e)` 用 `-race` 运行会报 `m.ctx`/`m.Tag` 的 DATA race；把 `WithContext` 改为返回副本后 race 消失。

---

### R6 — `PSubscribe` 修改调用方 slice + 失败重连泄漏 pubsub（确认缺陷）

**位置**：`pkg/db/redis/redis.go:273-304`。
- `L273 channels ...string`，`L286-288 for i := range channels { channels[i] = strings.ReplaceAll(channels[i], "{db}", ...) }`——**原地改写入参 slice 的底层数组**。若调用方以 `sub.PSubscribe("0", handler, chans...)` 传入共享 slice，则与其读者 race，并污染调用方数据。
- `L289 pubsub := conn.PSubscribe(...)`；`L291-293` `Receive()` 失败直接 `continue`，**未 `pubsub.Close()`**——重连循环每次泄漏一个订阅连接。
- `L298 handler(msg, dbName)` 未检查 `handler != nil`。

**后果**：data race；长期重连导致连接/FD 泄漏，极端 OOM；handler=nil 时后台 goroutine panic（无 recover，直接打挂进程）。

**修复方案**：函数内 `local := append([]string(nil), channels...)`，只对 local 做 `ReplaceAll`；`Receive()` 失败分支先 `pubsub.Close()` 再 continue；入口 `if handler == nil { return }`；提供停止信号（context）以终止 `for` 循环，避免 goroutine 永久泄漏。

**可证伪预测**：如果「原地改写 channels」是根因，那么把同一 `[]string{"ch-{db}"}` 传给 `PSubscribe` 并在主 goroutine 读它，`-race` 会报该 slice 的 DATA race 且主 goroutine 观察到内容被改写；改为复制局部 slice 后 race 与内容污染均消失。

---

## 3. P2 风险（理论风险 / 防御性加固）

### R7 — `plugin.Register` 未拦截 typed-nil（确认，给定输入必现）
`plugin/plugin.go:42-53`：`if p == nil`（`L43`）挡不住 typed-nil，例如 `var p *funcPlugin=nil; plugin.Register(p)`；`L46 name := p.Name()` → `(*funcPlugin).Name():24 return p.name` → nil 解引用。
- 反证框架作者已知此问题：`mgin.go:103-114 isNilPlugin` 用反射专门识别 typed-nil，但 v2 的 `plugin.Register` 未复用。
- 修复：`Register` 入口复用反射判空（或把 `isNilPlugin` 上移到 plugin 包），`p==nil` 或反射 IsNil 时直接 return。
- 可证伪预测：如果此因成立，`var p *funcPlugin=nil; plugin.Register(p)` 会在 `L46` panic；加入反射判空后不 panic 且注册表不变。

### R8 — DAO nil receiver 调用 CRUD（确认，误用必现）
`dao/mysql.go` 的 `Create:292`、`MultiCreate:317`、`Delete:342`、`Updates:368`、`Save:393`、`All:418`、`One:466`、`Exists:501`、`Count:531`、`Alias:611`、`DB:624` 均在方法开头执行 `if receiver.Tag == nil { receiver.Tag = notag }`，而唯一的 nil 守护在 `connection():64`，晚于这些写入。
- `var d *MySQLDao[User]; d.All(User{})` → `L418` nil 解引用。（对比 `WhereAll:249` 经 `whereCacheKey:240` 的 nil 守护 + `ok==false` 短路，反而是安全的——不一致。）
- 修复：每个导出方法入口统一 `if m == nil { return zero, error }`，或把 `Tag` 惰性初始化收拢到 `connection()`。
- 可证伪预测：如果此因成立，`(*MySQLDao[User])(nil).Create(&u)` 在 `L292` panic；前置 nil 守护后返回 error 不 panic。

### R9 — `MgoDao.Update` 缺 `conn==nil` 守护（理论，并发窗口）
`dao/mgo.go:100-119`：`L107 GetConnection`、`L108` 仅检查 err，`L112 defer ReturnConnection(conn)`（ReturnConnection 内部 `L226` 有 nil 守护，OK），**`L113 conn.C(...)` 无 nil 检查**；而 `Insert:37/Delete:62/Updates:87/All:134/One:161/Pager:190` 都有 `if conn==nil`。并发 `Close`（R2）下 `GetConnection` 可能返回 nil conn（R2 的 L211 场景）。修复：与其它方法一致补 `if conn == nil { return error }`。

### R10 — Registry 插件对 nil / typed-nil `registry.Registry` 解引用（确认，非标准启动必现）
`builtins.go:129-145`：
- `L130 if registrar, ok := registry.Registry.(registry.ConfiguredRegistrar); ok` 对 **nil interface** 逗号-ok 断言安全（ok=false），但对 **typed-nil**（`(*RegistryCollection)(nil)`）ok=true，`L131 registrar.RegisterConfigured()` 解引用 nil panic；
- `L136/139/142 registry.Registry.Register(...)` 对 nil interface 直接方法调用 panic。
- 正常路径 `mgin.Init:127` 先赋值 `NewRegistry()`（永非 nil，`registry.go:63-65` 兜 noop），故此风险仅在绕过标准启动时出现。`closeFn:146 DeRegister()` 同理，但被 `CloseAll` recover 兜住（仅注销失效，不崩）。
- 修复：init/close 前 `if registry.Registry == nil { return error/nil }`，并对类型断言结果再做反射判空。

### R11 — `HealthAll` 无 recover（理论）
`plugin/plugin.go:150-159`：直接 `p.Health()`。其调用者 `mgin.checkAll:175`（每 5 分钟）也无 recover。内置 healthFn 已各有守护（Mysql.Check:404 有 nil 检查、Sqlite.Check 返回 nil、Mongo.Check:168 有 nil 检查、Redis.Check 返回 nil、Job:165 有 nil 检查、Registry 返回 nil），故内置路径安全；自定义插件 health panic 会穿透定时器。建议与 R3 一并加 recover。

### R12 — MySQL 发布新连接后锁外关闭旧连接的 use-after-close 窗口（理论）
`mysql/mysql.go:85-101`：`L91` 保存 `oldSingle/oldMulti`，`L98` 解锁，`L100-101` 关闭旧连接。解锁到关闭之间，另一 goroutine 可能经 `GetConnection`（`L434-466`，RLock）取到即将被关闭的旧 `*gorm.DB`。`sql.DB.Close()` 后操作返回 error 而非 panic，故为逻辑风险（查询偶发失败），非 panic。若要消除：可给连接加代际编号/引用计数，或对“发布+关闭旧”整体持写锁（代价是 Init 会阻塞读）。

### R13 — `Check` 空实现掩盖故障（确认的逻辑缺口，非直接 panic）
`redis/redis.go:191-232`（`redisCheck`/`Check` 全为注释）、`sqlite/sqlite.go:47-49`（`Check` 直接 return nil）。健康检查恒 nil，插件 `Health` 无法发现 Redis/SQLite 故障，导致 R1/R2 类问题更难被提前发现；不属于 panic，但与本次健壮性目标相关，建议实现真实 Ping。

---

## 4. 逐文件精读记录（无风险项也注明）

| 文件 | 精读结论 |
|---|---|
| `pkg/plugin/builtins.go` | `funcPlugin` 各方法对函数字段均有 nil 检查（`L26-49`），安全。`enabled/getCfg` 依赖的 `config.Config` 为包级 `&config{}`（`configure.go:202`，非 nil），`GetConfigData` 有 nil 守护（`configure.go:450`）。风险集中在 Registry 插件（R10）及依赖的 `InitAll` 缺口（R3）。 |
| `pkg/plugin/plugin.go` | 注册表本体（`Register/Unregister/All/sortedByOrder`，`L42-96`）用 RWMutex，切片副本安全；`CloseAll` 有 recover（`L132-138`）。缺口：`InitAll`（R3）、`HealthAll`（R11）、`Register` typed-nil（R7）。 |
| `pkg/db/db.go` | `L10-13` 全局单例均为 `&T{}`，非 nil，无风险。 |
| `pkg/db/transaction.go` | `MySQLTx:15-18`、`TransactionOn:24-34`：fn/conn/ctx 均有守护，ctx=nil 回退 Background；gorm `Transaction` 内部处理 panic 回滚。**未发现 panic 风险**；事务回调自身 panic 会按 Go 语义穿透，属预期。 |
| `pkg/db/mysql/mysql.go` | 三个提交中 `9825b9b` 的重点重写对象。`initialize` 用 `initMu+mu` 分离、`closed` 幂等、锁外建连锁内发布、全链路 nil 守卫（`closeMySQL`、`validateMySQL`、`mySqlCheck`、`GetConnection` 等）。`mysqlsCheck:354-402` 的 RUnlock→initialize→RLock 序列正确，无自锁。**仅 R12（use-after-close 窗口）为理论残余**；`mysql_test.go` 的 12 个用例正是这些健壮性的回归保障。 |
| `pkg/db/mysql/mysql_test.go` | 测试代码；`newScriptedGormDB:407-420` 用 atomic 唯一 driver 名，`scriptedPingDriver.ping:380-389` 有 mutex。无生产风险。 |
| `pkg/db/redis/redis.go` | R1、R6、R10(间接)、R13。`fe75373` 仅修了 `L82/L116` 的 `i>=len(ports)` 越界（已确认在现文件中），**未触及并发与生命周期**。 |
| `pkg/db/mongo/mgo.go` | R2、R9(被调用侧)。`GetConnection:202 dbName[0]=m.tags[0]` 会改写入参 slice 首元素（与 R6 类似的副作用，低危，并入防御建议）。 |
| `pkg/db/sqlite/sqlite.go` | R4；`Init:22-37`/`GetConnection:51-56` 缺 nil receiver 守护（低危）；`Init` 失败未复位 `m.sqlite`（R4 诱因）；`Check` 空实现（R13）。 |
| `pkg/db/dao/define.go` | 纯接口定义，无实现，无风险。 |
| `pkg/db/dao/mysql.go` | R5、R8。`cacheQueryString/cacheKey/loadCached/storeCached`（`L127-213`）用 comma-ok 断言 + cacheEnabled 守护，安全；`clearCache:215-226` 在 Range 回调内 Delete（sync.Map/SCAN 均安全）；`Pager:598` 除 size 已被 `L576-578` 保证 >0，无除零；`WithCache:100-102` 负数 TTL 归零，安全。 |
| `pkg/db/dao/mgo.go` | R9；其余方法 `CollectionName`/`conn==nil`/`Tag` 守护齐全；`Pager:198-221` 分页 size 有下界，无除零。 |
| `pkg/db/dao/redis_cache.go` | 每个方法均有 `c.Client==nil` 守护（`L32/39/50/60/83`），`var _ ICache`（`L109`）接口匹配。**未发现 panic 风险**。 |
| `pkg/db/dao/mysql_cache_test.go` | 测试；`OnMemCache` + comma-ok，无风险。 |
| `pkg/db/dao/mysql_lifecycle_test.go` | 测试；用 `&gorm.DB{}` 验证 `connection()` 的 `Config/Statement` 守护（`mysql.go:78`），无风险。 |
| `pkg/db/dao/CACHE_README.md` | 文档；`L42 GetConnection()` 未检查 err 就传给 `NewRedisCache`（示例缺陷，nil client 已被 RedisCache 守护，不会 panic，仅缓存静默失效）。 |

---

## 5. 近期提交对照分析

- **`9825b9b`「修复mysql健壮性」**：新增 `mu/initMu/closed/openConn`，重写 `initialize/loadMySQLConnections/closeMySQL`，配套 12 个回归测试。**MySQL 的 panic 面被系统性收口**；但同批改动**没有覆盖 redis/mongo/sqlite**——这正是 R1/R2/R4 的根因：加固不均衡。
- **`a07e077`「代码审核…事务处理和Where缓存」**：新增 `transaction.go`（守护完整，无风险）、DAO 缓存链路（`L127-226`）。引入 `m.cache/m.ctx/m.Tag` 可变字段，为 R5（共享实例 race）与 R8（nil receiver 前置写 Tag）埋下伏笔。
- **`fe75373`「指针安全性与健壮性修复」**：对 `redis.go` 只加了 `hosts/ports` 的 `i>=len(ports)` 越界检查（`L82/L116`，已生效）；对 `mgin.go` 加 typed-nil 插件守护（`isNilPlugin`）。**未处理 Redis/Mongo 的锁与生命周期**，也未把 typed-nil 守护下沉到 `plugin.Register`（R7）——属于"修了入口、漏了注册表"。

结论：三个提交呈"按文件修补"模式，缺少跨数据源的统一并发/生命周期契约。建议以 `MysqlClient` 为模板抽象出统一的数据源生命周期基类（锁 + closed + RCU 发布 + nil 守护），让 Redis/Mongo/SQLite 对齐，从根本上关闭 R1/R2/R4。

---

## 6. 交给工程师的最小复现清单（按优先级）

1. **R1**：`go test -race` + 并发 `GetConnection()` vs `Close()/Init()`（Redis）→ 期望命中 race / fatal。
2. **R2**：并发 `Mongo.Check()`（清空 conns 触发 Init）vs `GetConnection("tag")`，及 `Close()` vs `GetConnection("tag")` → 期望 race 或 `mgo.go:211` nil panic。
3. **R4**：`(&sqlite.Sqlite{sqlite: &gorm.DB{}}).Close()` → 期望 `sqlite.go:42` panic。
4. **R3**：注册 `Init` 内 `panic` 的 enabled 插件后 `plugin.InitAll` → 期望穿透；加 recover 后返回 error。
5. **R7**：`var p *funcPlugin=nil; plugin.Register(p)` → 期望 `plugin.go:46` panic。
6. **R8**：`(*MySQLDao[User])(nil).Create(&u)` → 期望 `mysql.go:292` panic。
7. **R6**：共享 `[]string{"ch-{db}"}` 传入 `PSubscribe` 并并发读取 → `-race` 命中 slice race。

---

## 7. 假设与未决问题（Anything UNCLEAR）

1. **调用并发度未知**：未获得线上 `SafeExit` 是否先排空在途请求的代码证据（`mgin.go:206+` 仅注释说明"先排空日志队列再关 DB"）。若确实排空，R1/R2 的关闭侧窗口缩小，但 `checkAll` 与业务并发的启动侧窗口仍在。请工程师确认 `app.go` 的关停顺序。
2. **`gorm.Open` 失败时 `m.sqlite` 的实际值**：R4 的"非 nil 无效 gorm.DB"触发路径依赖 gorm 版本行为；直接构造 `&gorm.DB{}` 必现，但生产 Init 失败是否留下非 nil 值需工程师用当前 gorm 版本验证（见复现 3）。
3. **Redis/Mongo 是否被用作 DAO 缓存/主库的线上比例**：影响 R1/R2 的现网暴露面；代码层面（`CACHE_README.md`）已推荐 Redis 作缓存。
4. 本报告严格限定在指定范围；`mgin.go:124-126 logs.InitVictoriaLogs()` 的显式 `panic(err)` 属范围外，未列入。

（报告完）
