# 工程师最小复现验证报告：panic 风险动态确认

- 验证人：寇豆码（Kou）· 工程师
- 日期：2026-11-25
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，module `github.com/maczh/mgin/v2`
- 环境：`go version go1.25.7 linux/amd64`（PATH 需前置 `export PATH=/usr/local/go/bin:$PATH`）；依赖已就绪，命令统一加 `GOPROXY=https://goproxy.cn,direct`
- 方法：在各自包目录新建临时 `package <pkg>` 测试文件（可访问私有字段），逐个 `go test` 真跑，**跑完逐一删除**。全程未修改任何生产代码、未 git commit。
- 收尾状态：`git status --short` 仅 `?? deliverables/`；删除临时文件后 `go test ./pkg/db/... ./pkg/plugin/...` 退出码 0（全绿），与基线一致。

> 证据说明：以下输出中的内存地址、goroutine 编号每次运行不同，属正常；本地测试地址 `127.0.0.1:1` 为不可达端口，用于模拟连接失败。无外部敏感信息，故未使用 `<REDACTED>`。

---

## 0. 判定总表

| 编号 | 风险 | 判定 | 与架构师定性的差异 |
|---|---|---|---|
| R1 | Redis 无锁 map | **可复现**（DATA race + `fatal error`） | 完全一致，P0 成立 |
| R2 | Mongo 无锁 map | **可复现**（DATA race + `fatal error`） | map 崩溃一致；但 L211 链式 nil panic **动态未复现**（需真实 Mongo） |
| R4 | SQLite Close 忽略 DB() error | **可复现**（panic 确凿） | panic 实际在 **L41 的 `m.sqlite.DB()` 内部（gorm.go:413）**，非架构师所述 L42 `db.Close()`；且 gorm.Open 失败**非必现**此 panic，触发概率应下调 |
| R3 | InitAll 无 recover | **可复现** | 完全一致 |
| R7 | Register typed-nil | **可复现** | 完全一致（L46） |
| R8 | DAO nil receiver | **可复现** | 完全一致（L292） |
| R6 | PSubscribe 改写调用方 slice | **可复现**（内容污染 + race） | 完全一致 |

未决问题 #1（关停是否排空在途请求）：**否，代码顺序是先关 DB 后 Shutdown HTTP**，详见第 9 节（加强了 R1/R2 的关闭侧窗口结论）。
未决问题 #2（gorm.Open 失败后 m.sqlite 实际值）：**非 nil 且 ConnPool 有效**，架构师担忧的"生产 Init 失败路径触发 R4"不成立，详见第 3 节。

---

## 1. R1 — Redis 无锁 map（可复现，P0 成立）

**临时文件**：`pkg/db/redis/zz_repro_r1_test.go`（`package redis`，8 个读 goroutine 循环 `GetConnection()` + 1 个写 goroutine 循环 `Close()`/`Init()`/`r.clients["0"]=fake`）

**确切命令**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test -race -count=3 ./pkg/db/redis/ -run 'TestReproR1ConcurrentMap' -v
```

**关键输出（节选）**：
```
WARNING: DATA RACE
Write at 0x00c00045e9c0 by goroutine 19:
  runtime.mapdelete_faststr()
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).Close()
      redis.go:186 +0x144
Previous read at 0x00c00045e9c0 by goroutine 11:
  runtime.mapaccess2_faststr()
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).GetConnection()
      redis.go:256 +0x2a5

WARNING: DATA RACE
Write at 0x00c000478690 by goroutine 19:
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).Init()
      redis.go:66 +0x444          // r.multi = ...
Previous read at ... by goroutine 16:
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).GetConnection()
      redis.go:235 +0x3c          // if r.multi

WARNING: DATA RACE
Write at 0x00c0004786a8 by goroutine 19:
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).Init()
      redis.go:67 +0x46d          // r.clients = make(...)
Previous read at ... by goroutine 15:
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).GetConnection()
      redis.go:256 +0x284         // r.clients["0"]

fatal error: concurrent map read and map write
goroutine 25 [running]:
github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).GetConnection(...)
      redis.go:256 +0x2a6
FAIL    github.com/maczh/mgin/v2/pkg/db/redis  0.050s
```

**判定**：**可复现**。多组 DATA race 命中架构师标注的全部热点（L66/67 写 ⇄ L235/256 读，`Close` L186 delete ⇄ `GetConnection` L256 mapaccess），并最终触发 `fatal error: concurrent map read and map write`——Go runtime 的 `throw`，`recover` 无法捕获，进程级崩溃。架构师的可证伪预测完全成立，**P0 定性与严重度无需下调**。

---

## 2. R2 — Mongo 无锁 map（map 崩溃可复现；L211 nil panic 动态未复现）

**临时文件**：`pkg/db/mongo/zz_repro_r2_test.go`（`package mongo`，8 个读 goroutine 循环 `GetConnection("tag")` + 1 个写 goroutine 循环 `Close()`/`Check()`（conns 清空后隐式 Init）/`m.conns["tag"]=connection{...}`）

**确切命令**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test -race -count=3 ./pkg/db/mongo/ -run 'TestReproR2MapRace' -v
```

**关键输出（节选）**：
```
WARNING: DATA RACE
Write at 0x00c0002573b0 by goroutine 18:
  runtime.mapdelete_faststr()
  github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).Close()
      mgo.go:129 +0x215
Previous read at 0x00c0002573b0 by goroutine 14:
  runtime.mapaccess2_faststr()
  github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).GetConnection()
      mgo.go:204 +0x2ab

WARNING: DATA RACE
Read at 0x00c000116588 by goroutine 13:
  github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).mgoCheck()
      mgo.go:144 +0xad
Previous write at ... by goroutine 18:
  github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).Init()
      mgo.go:36  (m.confData)  /  mgo.go:72 (m.multi)
  github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).Check()
      mgo.go:174

fatal error: concurrent map read and map write
goroutine 25 [running]:
github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).GetConnection(...)
      mgo.go:204 +0x2ac
FAIL    github.com/maczh/mgin/v2/pkg/db/mongo
```

**判定**：
- **无锁 map 部分完全可复现**：`Close` L129 delete ⇄ `GetConnection` L204 mapaccess 的 DATA race、`Check()->Init()` L36/72 写 ⇄ `mgoCheck` L144 读、以及最终 `fatal error: concurrent map read and map write`，均命中。P0 的"不可恢复 fatal"定性成立。
- **L211 链式解引用 nil panic（`m.conns[dbName[0]].conn.Copy()`）动态未复现**。原因：无真实 MongoDB 时 `mgo.DialWithTimeout`（mgo.go:85/106/154）经 10 秒超时返回 error，`Init`/`mgoCheck` 建不出有效 session，`m.conns[tag].conn` 恒为 nil；执行流在 `mgoCheck` L147-148 的 `!ok || conn.conn == nil` 守卫处提前返回 error，到不了 L211。该单点属**静态确认、需真实外部依赖**，建议 QA 用真实/可 mock 的 Mongo 复核，或接受代码审查结论。

**踩坑记录（测试夹具问题，非生产风险）**：首次用 URI `mongodb://127.0.0.1:1` 触发 mgo 库 `extractDBName`（session.go:54）的 `panic: runtime error: slice bounds out of range [1:0]`；改为带库名 `mongodb://127.0.0.1:1/testdb` 后消失。此 panic 源于 mgo 依赖库对无库名 URI 的解析，与本次审计目标无关。

---

## 3. R4 — SQLite Close 忽略 DB() error（可复现；panic 位置需修正，触发概率应下调）

**临时文件**：`pkg/db/sqlite/zz_repro_r4_test.go`（`package sqlite`，3 个用例：直接 Close、精确探针、gorm.Open 失败状态）

**确切命令**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test ./pkg/db/sqlite/ -run 'TestReproR4' -count=1 -v
```

**关键输出（节选）**：
```
=== RUN   TestReproR4CloseNilDB
panic: runtime error: invalid memory address or nil pointer dereference
    gorm.io/gorm.(*DB).DB(0x191c530?)
        /home/macro/go/pkg/mod/gorm.io/gorm@v1.31.0/gorm.go:413 +0x19
    github.com/maczh/mgin/v2/pkg/db/sqlite.(*Sqlite).Close(0xc000095730)
        /home/macro/Work/go/src/github.com/maczh/mgin/pkg/db/sqlite/sqlite.go:41 +0x25

=== RUN   TestReproR4Probe
(&gorm.DB{}).DB() PANIC: runtime error: invalid memory address or nil pointer dereference

=== RUN   TestReproR4GormOpenFailureState
R4-2 gorm.Open 无效路径: db==nil=false, err=unable to open database file: no such file or directory
R4-2 db.DB(): sqldb==nil=false, dberr=<nil>
```

**判定与两处修正**：
1. **panic 位置修正**：架构师预测"L41 `db,_:=m.sqlite.DB()` 返回 (nil, ErrInvalidDB)，L42 `db.Close()` 对 nil 解引用"。实测 panic **在 L41 调用 `m.sqlite.DB()` 时即发生，位于 gorm v1.31.0 的 `gorm.(*DB).DB()` 内部 gorm.go:413（`connPool := db.ConnPool` 对零值 gorm.DB 解引用），根本到不了 L42**。修复时的 nil 守护需覆盖 L41 的调用本身（例如判断 `m.sqlite.Config == nil || m.sqlite.Statement == nil` 就直接跳过），而不只是判断 `db != nil`。
2. **未决问题 #2 回答（推翻"生产 Init 失败必触发"假设）**：当前 gorm v1.31.0 + gorm.io/driver/sqlite v1.6.0 下，`gorm.Open` 用无效路径失败时返回**非 nil 的 `*gorm.DB`**，且其内部 `ConnPool` 是有效的 `*sql.DB`（`db.DB()` 返回 `sqldb != nil, err == nil`）——因为 `sql.Open` 惰性建连，路径错误要到首次 Ping/查询才暴露。因此 `Init` 失败留非 nil `m.sqlite` 属实，但**其 `DB()` 不会返回 nil，生产 Init 失败路径不会触发 L42 的 nil panic**。R4 的 panic 在**人为/注入构造 `&gorm.DB{}`（ConnPool 完全未初始化）时必现**，生产中需 `ConnPool` 被替换为非 `*sql.DB` 的非标准类型才可能。
   - **建议修正定性**：从"P1 / 高概率（特定输入必现）"下调触发概率描述为"低概率，仅异常构造或非标准 ConnPool 时必现"；代码缺陷（丢弃 DB() error、缺 nil 守护、Init 失败未复位 m.sqlite）依然真实，值得修，但不宜作为高优先级线上崩溃项。

---

## 4. R3 — InitAll 无 recover（可复现，P1 成立）

**临时文件**：`pkg/plugin/zz_repro_r3_test.go`（`package plugin`，自定义 `reproR3Plugin` 实现 Plugin，Init 内 `panic("REPRO_R3_PANIC")`；注册 order=1 的 panic 插件与 order=2 的正常插件；临时替换全局注册表并在 defer 中还原）

**确切命令**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test ./pkg/plugin/ -run 'TestReproR3InitAllPanic' -count=1 -v
```

**关键输出（节选）**：
```
=== RUN   TestReproR3InitAllPanic
    zz_repro_r3_test.go:53: R3 panic 穿透 InitAll: recovered=REPRO_R3_PANIC
    zz_repro_r3_test.go:54: R3 后续插件是否初始化: laterInited=false
    zz_repro_r3_test.go:56: R3 复现：panic 穿透 InitAll，且后续插件未初始化
--- PASS
```

**判定**：**可复现**。panic 穿透 `InitAll`（进而穿透 `mgin.Init:141`），且后续已注册插件未初始化——架构师"首个错误继续语义对 panic 无效"的判断成立。P1 结构性缺口确认，与 `CloseAll` 的 recover 不对称属实。

---

## 5. R7 — Register typed-nil（可复现，P2 成立）

**临时文件**：`pkg/plugin/zz_repro_r7_test.go`（`package plugin`，`var p *funcPlugin = nil; Register(p)`）

**确切命令**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test ./pkg/plugin/ -run 'TestReproR7RegisterTypedNil' -count=1 -v
```

**关键输出（节选）**：
```
panic: runtime error: invalid memory address or nil pointer dereference
    github.com/maczh/mgin/v2/pkg/plugin.(*funcPlugin).Name(0x55f140?)
        pkg/plugin/builtins.go:24
    github.com/maczh/mgin/v2/pkg/plugin.Register({0x17e25f0, 0x0})
        pkg/plugin/plugin.go:46 +0x50
```

**判定**：**可复现**。`p == nil`（L43）挡不住 typed-nil，L46 `p.Name()` 经 `(*funcPlugin).Name()`（builtins.go:24 `return p.name`）解引用 nil。架构师定性完全一致。

---

## 6. R8 — DAO nil receiver（可复现，P2 成立）

**临时文件**：`pkg/db/dao/zz_repro_r8_test.go`（`package dao`，最小 `schema.Tabler` 实体 `reproR8Entity`，`(*MySQLDao[reproR8Entity])(nil).Create(&e)`）

**确切命令**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test ./pkg/db/dao/ -run 'TestReproR8DAONilReceiver' -count=1 -v
```

**关键输出（节选）**：
```
panic: runtime error: invalid memory address or nil pointer dereference
    github.com/maczh/mgin/v2/pkg/db/dao.(*MySQLDao[...]).Create(0x0?, 0x1fa5880?)
        pkg/db/dao/mysql.go:292 +0x22
```

**判定**：**可复现**。nil receiver 在 L292 `receiver.Tag == nil` 处即 panic，早于 `connection()` L64 的 nil 守护。架构师定性完全一致。

---

## 7. R6 — PSubscribe 改写调用方 slice（可复现，P1 成立）

**临时文件**：`pkg/db/redis/zz_repro_r6_test.go`（`package redis`，向 `r.clients["0"]` 注入不可达的 fake client 使 `GetConnection` 成功返回，传入共享 `[]string{"ch-{db}"}` 给 `PSubscribe`，主 goroutine 轮询读取同一 slice）

**确切命令 1（内容污染）**：
```
export PATH=/usr/local/go/bin:$PATH
cd /home/macro/Work/go/src/github.com/maczh/mgin
GOPROXY=https://goproxy.cn,direct go test ./pkg/db/redis/ -run 'TestReproR6PSubscribeMutatesSlice' -count=1 -v
```
```
R6 channels[0]="ch-0" original="ch-{db}" mutated=true
R6 复现：调用方 slice 被 PSubscribe 后台 goroutine 原地改写
--- PASS
```

**确切命令 2（DATA race）**：
```
GOPROXY=https://goproxy.cn,direct go test -race ./pkg/db/redis/ -run 'TestReproR6PSubscribeMutatesSlice' -count=1 -v
```
```
WARNING: DATA RACE
Write at 0x00c000384010 by goroutine 23:
  github.com/maczh/mgin/v2/pkg/db/redis.(*RedisClient).PSubscribe.func1()
      redis.go:287 +0x416
Previous read at 0x00c000384010 by goroutine 21:
  github.com/maczh/mgin/v2/pkg/db/redis.TestReproR6PSubscribeMutatesSlice()
      zz_repro_r6_test.go:23
```

**判定**：**可复现**。无需真实 Redis 即可确认：调用方 slice 被后台 goroutine 原地改写为 `ch-0`，`-race` 命中 L287 的写 ⇄ 调用方读的 DATA race。架构师对"修改入参 slice + race"的定性完全一致。（重连不 `pubsub.Close()` 的连接泄漏属资源问题，本次未做长跑验证，静态确认。）

---

## 8. 基线核查（临时文件删除前后）

- **带临时文件时** `go test ./pkg/db/... ./pkg/plugin/...`（无 `-race`）退出码 1：`pkg/db/mongo`、`pkg/db/redis` FAIL——系 R1/R2 的并发测试在无 race 模式下同样触发 `fatal error`，属临时测试预期表现。
- **删除全部临时文件后**：`go test ./pkg/db/... ./pkg/plugin/...` 退出码 0，全部 ok。
- 工作树：`git status --short` → 仅 `?? deliverables/`，生产代码零改动。

---

## 9. 额外核实：未决问题 #1（关停是否在关 DB 前排空在途请求）

**结论：否。代码顺序是先关 DB、后 Shutdown HTTP 服务器；在途请求未被排空即遭遇 DB 关闭。**

证据链（`app.go` 的信号处理块 + `mgin.go` 的 `SafeExit`）：
```
app.go:374  sig := <-signalChan                          // 收到退出信号
app.go:380  app.checkTicker.Stop()                       // 先停健康检查定时器
app.go:383  close(app.checkDone)                         // 关健康检查信号
app.go:387  app.MGin.SafeExit()
    └─ mgin.go:207  postlog.Close()                      // 排空的是【日志队列】，不是 HTTP 请求
    └─ mgin.go:208  i18n.Close()
    └─ mgin.go:211  plugin.CloseAll(ctx)                 // 【关闭全部 DB 连接】
app.go:393  for _, s := range servers { s.Shutdown(ctx) } // 【最后才】优雅关闭 HTTP server、排空在途请求
```

分析：
- `SafeExit` 内 `postlog.Close()`（`mgin.go:207`）排空的是日志 sink 队列（`pkg/middleware/postlog/logger.go:135-138` → `logsink.Close`），与 HTTP 在途请求无关。
- 真正的在途请求排空发生在 `s.Shutdown(ctx)`（`app.go:393-400`），而它在 `app.MGin.SafeExit()`（含 `plugin.CloseAll` 关 DB）**之后**才执行。
- 因此在 `CloseAll` 关闭 Redis/Mongo 期间，HTTP server 仍在线并可能处理请求，这些请求经 `dao.RedisCache` / `dao/mgo` 调用 `GetConnection()`，与 `Close()` 的 map delete 并发——**R1/R2 的关闭侧竞态窗口是代码顺序保证的真实窗口，不是理论推测**。这加强了架构师对 R1/R2 触发链路 #1 的判断。
- **平衡修正**：`app.checkTicker.Stop()` / `close(app.checkDone)`（`app.go:380-385`）在 `SafeExit` 之前，健康检查定时器（`checkAll`，每 5 分钟）驱动的 `HealthAll` → `Check()`/`Init()` 侧窗口在关停时**已被主动收口**。故关停期主要风险来自**在途业务请求**侧，而非健康检查侧；启动期/运行期的健康检查并发窗口（架构师触发链路 #2）仍存在。

---

## 10. 对架构师报告定性修正建议汇总

1. **R1、R2（map 无锁/fatal）**：定性、严重度、行号、可证伪预测全部成立，无需修改。
2. **R2 的 L211 链式 nil panic**：标注为"静态确认，动态未复现（需真实 MongoDB 或可 mock 的 session）"，避免与已复现的 map fatal 混为一谈——两者修复手法不同（前者加 RWMutex + 本地变量，后者加 conn==nil 守护）。
3. **R4**：(a) panic 位置由 L42 修正为 L41 调用 `m.sqlite.DB()` 时、gorm 内部 gorm.go:413；(b) 触发概率由"高概率（特定输入必现）"下调——当前 gorm 版本下 `gorm.Open` 失败返回非 nil 但 ConnPool 有效的 db，生产 Init 失败路径不触发该 panic，仅异常构造/`&gorm.DB{}` 注入必现；代码缺陷仍在，但优先级可下调。
4. **未决问题 #1**：从"未知"更新为"已确认未排空，且关 DB 先于 Shutdown HTTP"——R1/R2 关闭侧窗口由理论升级为代码事实；同时注明健康检查侧在关停时已被 `checkTicker.Stop()` 收口。
5. **未决问题 #2**：从"未知"更新为"非 nil 且 DB() 有效"——推翻"Init 失败必触发 R4"假设。
6. **R3/R6/R7/R8**：全部动态确认，架构师定性可信，可直接进入 QA 定级。

（报告完）
