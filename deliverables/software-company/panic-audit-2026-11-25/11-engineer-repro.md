# 第三轮对抗性验证：工程师真实运行复现报告

- 验证人：寇豆码（Kou）· 工程师
- 日期：2026-11-25（第三轮）
- 仓库：`/home/macro/Work/go/src/github.com/maczh/mgin`，分支 `v2`，`HEAD = 4f1d9de0fa932af58605b10ec7b3ee5a21d77ecc`
- 环境：`export PATH=/usr/local/go/bin:$PATH`，`GOPROXY=https://goproxy.cn,direct`，go1.25.7 linux/amd64
- 方法：全部结论来自**真实运行**。共写 11 个 `zz_r3_*_test.go` + 1 个 `/tmp/mgin-r3-probes/zz_r3_shutdown.go` 独立程序，**跑完已全部删除**；未改任何生产代码、未 commit、未改 01–10 报告。
- 口径：判定分「成立 / 不成立 / 部分成立 / 未验证」。所有输出无敏感信息，地址均为 127.0.0.1 本地环回。

---

## 0. 结论速览

| 编号 | 指控 | 实测判定 | 关键数字 |
|---|---|---|---|
| A-1 | P0-1 OTel atomic.Value 异构 Store | **成立** | `sync/atomic: store of inconsistently typed value into Value` 必现 |
| A-2 | R-01 app.go 关停 nil-channel 死锁 | **成立** | nil 赋值版 1s 超时未返回；对照版 544ms 返回；-race 捕获字段竞争 |
| A-3 | P1-1 i18n.Format 数值断言 | **成立（且比报告更广）** | 11 种数值类型 panic，仅 int/float64 OK |
| A-4 | P1-2 utils.AnyToMap | **成立** | nil 值 / 异构 map 两种 panic 文本逐字复现 |
| A-5 | P1-3 postlog gzip 畸形响应 | **成立** | `invalid memory address or nil pointer dereference` |
| A-6 | P1-4 casbin 中间件 | **成立（有条件）** | 三种输入三种 panic；但 Swagger.Uri 为空时中间件提前 return，探针设计需注意 |
| A-7 | R-02 Redis Init 锁内 Ping | **成立** | `GetConnection` 阻塞 **2.802189817s**（架构师 2.8023s，吻合） |
| A-8 | R-03 Close 后 InitWithError 假成功 | **成立** | Redis/Mongo 均 `err=<nil>` |
| A-9 | R-04 PSubscribe 泄漏 | **成立** | Close 后 1.6s 订阅 goroutine 栈仍 1；PSubscribeContext cancel 后归零 |
| A-10 | logsink TestQueueKeepsNewestAndFIFO FAIL | **成立（测试自身缺陷，非生产 bug）** | 单跑/全包跑同错：`{QueueSize:1000 Workers:1}`，非状态污染 |
| A-11 | 规格轴「Register 会杀掉整包」 | **不成立** | `go test -count=1 -v ./pkg/plugin/` exit=0 |
| B-3 | P2-3 memcache sync.Map 整体赋值 | **成立** | -race 多条 DATA RACE + `fatal error: sync: unlock of unlocked mutex` |
| B-4 | P2-4 job Store RLock 内写 | **成立** | -race 读/写、写/写冲突各命中 |
| B-10 | P2-10 errcode RegisterDef 无锁 map | **成立** | -race map read/write 冲突 |
| B-1 | P2-1 client 未知 Protocol | **成立** | nil `*grequests.Response` 解引用 panic |

**与上游差异**：① A-3 外围报告只点名 int64/float32，实测 11/13 种数值类型全 panic；② A-6 存在「默认空 Swagger.Uri 导致中间件提前放行」的假绿陷阱，报告未提；③ A-10 规格/基线只知 FAIL，实测证明是测试期望值过期，非全局状态污染；④ A-11 规格轴结论被实测推翻；⑤ 收尾时 `pkg/otel/zz_qa3_otel_test.go`（QA 并发探针，非本人文件）导致 otel 包暂时编译失败，详见 §C。

---

## A. 必验项逐条

### A-1 P0-1 OTel `atomic.Value` 异构 Store —— **成立**

**临时探针**：`pkg/otel/zz_r3_otel_test.go`（`type zzR3CustomProvider struct{ noop.TracerProvider }`，即 `*zzR3CustomProvider` 模拟业务 SDK Provider）。

**命令**（耗时 648ms，exit=0；探针内 recover 后断言输出）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3' ./pkg/otel/
```
**关键输出**：
```
concrete type before: noop.TracerProvider
PANIC CAUGHT: sync/atomic: store of inconsistently typed value into Value
```
**判定**：**成立**。init 中存的 `noop.TracerProvider`（值类型）与 `SetTracerProvider(&customProvider{})`（指针类型）具体类型不一致，`Store` 必 panic，与架构师/外围轴描述逐字一致。

**现有 `otel_test.go:27` 为何假绿**：`TestSetThenDisable` 传的是 `noop.NewTracerProvider()`，与 `init()` 存的具体类型完全相同；`Disable()` 也存同类型，所以整条测试路径从不触发异构 Store。这是**测了 API 调用形式、没测类型-changing 的真实路径**。

### A-2 R-01 app.go 关停 nil-channel 死锁 —— **成立（含 data race）**

**临时探针**：`/tmp/mgin-r3-probes/zz_r3_shutdown.go`，struct 精确复刻 `app.go:117-130 + :384-391` 的 `select` / `close(checkDone)` / `checkDone=nil` / `checkWG.Wait()`；`checkAll()` 用 300ms 睡眠模拟（不使用 channel 同步，否则会人为制造 happens-before 掩盖 race）。参数 `nil` = 当前代码，`keep` = 删除 `checkDone=nil` 的对照。

**命令与输出**：
```bash
go run zz_r3_shutdown.go nil       # 耗时 1347ms，exit=1（探针 1s 超时后 os.Exit(2)）
DEADLOCK: checkWG.Wait() did not return within 1s

go run zz_r3_shutdown.go keep      # 耗时 544ms，exit=0
worker: observed closed done channel, exiting
RESULT: checkWG.Wait() returned

go run -race zz_r3_shutdown.go nil # exit=1
WARNING: DATA RACE
Read at 0x00c000016068 by goroutine 8:
  main.main.func1()  zz_r3_shutdown.go:36
Previous write at 0x00c000016068 by main goroutine:
  main.main()  zz_r3_shutdown.go:47   // checkDone = nil
DEADLOCK: checkWG.Wait() did not return within 1s
```
**判定**：**成立**。nil 赋值版 `Wait()` 永久阻塞；删除该行后 <600ms 正常返回；字段读/写构成 data race（探针的无 happens-before 版本被 -race 捕获）。

**差异/边界**：这是**精确模式探针**，不是对真实 `App.Run()` 发 SIGTERM 的端到端压测——真实进程还需要配置、信号与插件环境；但 select 语义与字段赋值序列与生产代码逐行一致。第一次探针用 channel 放行 checkAll，意外建立 happens-before，-race 未报竞争；改为定时睡眠后即报，说明race 是否可观测取决于关停窗口有无同步点，而死锁与同步点无关、恒现。

### A-3 P1-1 `i18n.Format` 数值断言 —— **成立，且范围大于报告**

**命令**（耗时 3304ms，exit=0）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3' ./pkg/i18n/
```
**关键输出**（表驱动，16 种类型逐个 recover）：
```
int       -> result="42"
int8      -> PANIC: interface conversion: interface {} is int8, not int
int16     -> PANIC: interface conversion: interface {} is int16, not int
int32     -> PANIC: interface conversion: interface {} is int32, not int
int64     -> PANIC: interface conversion: interface {} is int64, not int
uint      -> PANIC: interface conversion: interface {} is uint, not int
uint8/16/32/64/uintptr -> PANIC: ... not int
float32   -> PANIC: interface conversion: interface {} is float32, not float64
float64   -> result="2.500000"
bool/string/[]byte -> OK
```
**判定**：**成立**。`i18n.go:107-110` 的多类型 case + 单一断言导致 **11 种数值类型 panic**（int8/16/32/64、uint/8/16/32/64/uintptr、float32），仅 `int`、`float64` 正常。外围报告只举了 int64/float32 两例，实测覆盖面更大。

### A-4 P1-2 `utils.AnyToMap` —— **成立**

**命令**（耗时 2891ms，exit=0）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3' ./pkg/utils/
```
**关键输出**：
```
nil-map-value       -> PANIC: reflect: call of reflect.Value.Type on zero Value
string-any-map      -> result=map[string]string{"a":"b"}
string-string-map   -> result=map[string]string{"a":"b"}
string-int-map      -> PANIC: interface conversion: interface {} is map[string]int, not map[string]string
nil                 -> result=map[string]string{}
```
**判定**：**成立**。两条 panic 文本与外围轴报告逐字一致；`obj==nil` 有守护，`map[string]any` 内含 nil 值没有。

### A-5 P1-3 postlog 畸形 gzip —— **成立**

**临时探针**：`pkg/middleware/postlog/zz_r3_gzip_test.go`，gin 路由挂 `RequestLogger()`，handler 返回 `Content-Encoding: gzip` + `Content-Type: application/json` + body `not-gzip`（用 application/json 是为了避开 `fileResponseSummary` 对 text/plain 的文件识别短路）。

**命令**（耗时 3564ms，exit=0）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3' ./pkg/middleware/postlog/
```
**关键输出**：
```
[ERR] ... RequestLogger.func2 ... [187] gzip.NewReader error:
PANIC CAUGHT: runtime error: invalid memory address or nil pointer dereference
```
**判定**：**成立**。`gzip.NewReader` 失败后仅记录错误继续执行，`defer r.Close()` / `io.ReadAll(r)` 对 nil Reader 解引用，全局中间件下每个畸形响应都会 panic（有 gin.Recovery 时 500）。

### A-6 P1-4 casbin 中间件 —— **成立（但探针设计有假绿陷阱）**

**临时探针**：`pkg/middleware/casbin/zz_r3_casbin_test.go`；`config.Config.Casbin.Enabled=true`、`config.Config.Sys.Swagger.Uri="/zz-r3-no-such-swagger-prefix"`（关键）、`mginCasbin.Casbin.UnAuthPath=nil`、`db.Mysql.Close()` 使 `GetEnforcer()` 走 DB 失败分支返回 nil。

**命令**（exit=0）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3' ./pkg/middleware/casbin/
```
**关键输出**：
```
claims-not-map         -> PANIC CAUGHT: interface conversion: interface {} is struct { UserID int }, not jwt.MapClaims
claims-missing-userId  -> PANIC CAUGHT: interface conversion: interface {} is nil, not float64
enforcer-nil           -> [ERR] 获取数据库连接失败! / PANIC CAUGHT: runtime error: invalid memory address or nil pointer dereference
```
**判定**：**成立**。三种入口均真实可达。

**重要差异**：第一次探针（未设 Swagger.Uri）三种输入全部「NO PANIC」假绿，原因是 `casbin.go:30` 的 `strings.Contains(path, config.Config.Sys.Swagger.Uri)` 在 Uri 为默认空串时恒 true，中间件在类型断言**之前**就 `c.Next(); return`。外围报告的「必现」需在非空 Uri 配置下成立；默认零配置反而是提前放行。这既不是缺陷不成立，也不是探针错——而是**可达性有配置前提**，建议修复时同时关注这个空串放行行为。

### A-7 R-02 Redis Init 锁内 Ping —— **成立**

**临时探针**：`pkg/db/redis/zz_r3_lifecycle_test.go`。`net.Listen("127.0.0.1:0")` + accept 后不回包（hung Redis），YAML 配置指向该地址；goroutine 调 `Init(cfg)`，200ms 后主 goroutine 计时 `GetConnection("0")`。

**命令**（测试二进制 6.517s，整批含 mongo 9.121s，exit=0）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3' ./pkg/db/redis/ ./pkg/db/mongo/
```
**关键输出**：
```
[ERR] ... redis.go:186 0 Redis连接失败: read tcp 127.0.0.1:55924->127.0.0.1:37351: i/o timeout
GetConnection elapsed=2.802189817s err=Redis connection unavailable
```
**判定**：**成立**。阻塞时长 2.8022s 与架构师实测 2.8023s 吻合，证明 `Ping` 在 `initMu→mu` 写锁内。Mongo 侧 `DialWithTimeout(10s)` 在锁内为确定代码事实，本轮未构造 hung Mongo 端点，**Mongo 部分标记「静态确认、未动态」**（不影响本条 Redis 成立判定）。

### A-8 R-03 Close 后 InitWithError 假成功 —— **成立（Redis + Mongo 均动态证实）**

**命令**：同 A-7 批次。
```
redis InitWithError after Close: err=<nil>
mongo InitWithError after Close: err=<nil>
```
**判定**：**成立**。架构师仅动态证实 Redis、Mongo 标静态；本次两端均实测 `err=<nil>`。与 MySQL `ErrClosed` 语义不一致。

### A-9 R-04 PSubscribe 泄漏 —— **成立**

**探针改进**：第一版用 `runtime.NumGoroutine()`，被前序测试 goroutine 退出抵消（base=4、afterPSubscribe=4，看似无泄漏）；改为 `runtime.Stack(buf,true)` 统计栈帧中 `PSubscribeContext.func1` 出现次数，按 goroutine 身份计数。

**命令**（测试二进制 2.314s，exit=0）：
```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3PSubscribeLeak' ./pkg/db/redis/
```
**关键输出**：
```
[ERR] redis.go:385 Redis connection failed: Redis connection unavailable
[ERR] redis.go:385 Redis connection failed: redis client is closed
PSubscribe goroutine stacks: before=0 afterSubscribe=1 afterClose=1   （期望 afterClose=0）
[ERR] redis.go:385 ... （Close 后错误日志仍继续）
PSubscribeContext goroutine stacks: beforeCancel=2 afterCancel=1      （减去上面泄漏的 1，本例已归零）
```
**判定**：**成立**。`PSubscribe` 用 `context.Background()`，Close 无 cancel 入口 → 订阅 goroutine 在 Close 后 1.6s 仍存活并每秒刷错误日志；`PSubscribeContext` 的 cancel 版本可正常退出（afterCancel 的 1 是第一条泄漏协程，非第二条）。与架构师、外围轴结论一致。

### A-10 logsink `TestQueueKeepsNewestAndFIFO` FAIL 根因 —— **FAIL 成立，但根因是测试期望过期（非全局状态污染）**

**命令与耗时**：
```bash
go test -count=1 -v -run '^TestQueueKeepsNewestAndFIFO$' ./pkg/logsink/   # 708ms，exit=1
go test -count=1 -v ./pkg/logsink/                                       # 634ms，exit=1
```
**关键输出**（两次完全一致）：
```
--- FAIL: TestQueueKeepsNewestAndFIFO (0.00s)
    sink_test.go:237: options: {QueueSize:1000 Workers:1}
FAIL
```
全包运行中其余 9 个用例（RecordAndEmit / DropWhenQueueFull / PanicAndErrorAreIsolated / CloseDrainsAndCallsCloser / ConcurrentEmit 等）全 PASS，只有该用例 FAIL。

**判定**：**测试缺陷，非生产缺陷**。`sink.go:42` 的 `DefaultQueueSize=1024`，`normalize()`（`:69-77`）只在 `<=0 || >1024` 时钳到 1024，因此 `QueueSize:1000` 原样保留；测试 `sink_test.go:236` 却断言 `==100`。**单独跑同样失败 → 排除测试间全局状态污染/顺序依赖**；这是断言期望值与实现常量脱节（疑似早期上限 100 的残留）。基线 `-race` 日志中也是同一 FAIL，无 DATA RACE 报告。建议修测试断言或明确把上限改回 100，不应据此判生产代码有 bug。

### A-11 规格轴「Register 会杀掉整包」 —— **不成立**

**命令**（耗时 3405ms，exit=0）：
```bash
go test -count=1 -v ./pkg/plugin/
```
**关键输出**：
```
=== RUN   TestRegisterRejectsPanickingMetadata
[ERR] plugin.go:58 注册插件失败:{ }
[ERR] plugin.go:63 注册插件panic-metadata失败:{ }
--- PASS: TestRegisterRejectsPanickingMetadata (0.00s)
...
ok  github.com/maczh/mgin/v2/pkg/plugin  0.017s
```
**判定**：**不成立**。`plugin.go` 的 `safeName` / `safeOrder` 在注册期 recover，`panicName:true` 的插件被拒但不进表；第二个插件因 `Order()` panic 同样被拒；`GetByName("panic-metadata")==nil`，测试通过，整包 exit=0。基线普通跑与 -race 跑中 pkg/plugin 也都 ok。规格轴 (c)「会杀掉整包」为**误判**：规范层面「recover 只能转 t.Fatalf」的指责可另议，但运行层面没有 package kill。

---

## B. 并发类补充验证（全部动态证实）

### B-3 P2-3 `memcache.go:63,192` sync.Map 整体赋值 —— **成立**

**命令**（耗时 1397ms，exit=1）：
```bash
go test -race -count=1 -timeout 90s -run 'TestZZR3' ./pkg/cache/
```
**关键输出（节选）**：
```
WARNING: DATA RACE
Write at ... memcache.go:63 (*MemCache).Clear
Previous write at ... sync.(*Map).Store -> memcache.go:144 (*MemCache).Set
WARNING: DATA RACE
Write at ... memcache.go:63 (*MemCache).Clear
Previous read at ... sync.(*Map).Load -> memcache.go:113 (*MemCache).Get
fatal error: sync: unlock of unlocked mutex
  internal/sync.(*HashTrieMap...).initSlow -> ... (*MemCache).Set ... memcache.go:144
```
**判定**：**成立且后果严重**：不仅 DATA RACE，还在 Go 1.25 的 swiss-map 实现下触发不可恢复的 `fatal error: sync: unlock of unlocked mutex`（并发 Set 与 Clear 撕裂内部锁状态）。

### B-4 P2-4 `job/scheduler.go:301-308` RLock 内写 `m.st` —— **成立**

```bash
go test -race -count=1 -timeout 90s -run 'TestZZR3' ./pkg/job/    # 3818ms，exit=1
```
```
WARNING: DATA RACE
Read at ... scheduler.go:304 (*Manager).Store
Previous write at ... scheduler.go:305 (*Manager).Store
WARNING: DATA RACE
Write at ... scheduler.go:305 / Previous write at ... scheduler.go:305
--- FAIL: TestZZR3ManagerStoreUnderRLock: race detected during execution of test
```
**判定**：**成立**（读/写与写/写均被 -race 捕获）。

### B-10 P2-10 `errcode/definition.go:79-81` 无锁写全局 map —— **成立**

```bash
go test -race -count=1 -timeout 90s -run 'TestZZR3' ./pkg/errcode/    # 747ms，exit=1
```
```
WARNING: DATA RACE
Write at ... runtime.mapassign_fast64 -> errcode.RegisterDef definition.go:80
Previous read at ... runtime.mapaccess2_fast64 -> errcode.LookupDef definition.go:71
--- FAIL: TestZZR3RegisterDefConcurrent: race detected during execution of test
```
**判定**：**成立**。本轮 2000 次并发注册/查询被 -race 捕获；更高并发下可升级为 runtime fatal。

### B-1 P2-1 client 未知 Protocol 的 `resp.String()` —— **成立**

**探针**：`pkg/client/zz_r3_protocol_test.go`，注入最小 fake `registry.RegistryClient`（`GetServices` 返回一个本地地址），使 `callInternal` 走到 switch；`Protocol:"grpc"` 不匹配任何 case。

```bash
go test -count=1 -timeout 90s -v -run 'TestZZR3UnknownProtocol' ./pkg/client/    # 2704ms，exit=0
```
```
PANIC CAUGHT: runtime error: invalid memory address or nil pointer dereference
```
**判定**：**成立**，nil `*grequests.Response` 解引用。

---

## C. 收尾状态与清理说明

**我的临时文件（11 个测试 + 1 个独立程序）全部删除**：
```
pkg/otel/zz_r3_otel_test.go          pkg/i18n/zz_r3_format_test.go
pkg/utils/zz_r3_anytomap_test.go     pkg/middleware/postlog/zz_r3_gzip_test.go
pkg/middleware/casbin/zz_r3_casbin_test.go
pkg/db/redis/zz_r3_lifecycle_test.go pkg/db/mongo/zz_r3_lifecycle_test.go
pkg/cache/zz_r3_race_test.go         pkg/job/zz_r3_race_test.go
pkg/errcode/zz_r3_race_test.go       pkg/client/zz_r3_protocol_test.go
/tmp/mgin-r3-probes/zz_r3_shutdown.go（含目录）
```
中途曾出现**非本人创建**的 9 个 QA 并发探针（`zz_qa3_*`），其 otel 探针一度编译失败（`:4 could not import fmt`、`:38/54/55 *qa3Provider does not implement trace.TracerProvider`），我已通知 QA；QA 已修复、实跑 PASS，并在任务结束后**全部删除**。

**最终状态（QA 清理后由我独立复核）**：
```
git status --short  →   M .gitignore   （唯一，进入任务前既有）
Glob **/zz_*        →   No files found（本人与 QA 探针均无残留）
```
```bash
go test -count=1 -timeout 180s ./pkg/db/... ./pkg/plugin/... ./pkg/otel/... ./pkg/i18n/... ./pkg/utils/...
# exit=0，耗时 5012ms
ok  pkg/db/dao 0.019s / pkg/db/mongo 0.011s / pkg/db/mysql 0.021s / pkg/db/redis 0.018s / pkg/db/sqlite 0.018s
ok  pkg/plugin 0.020s / pkg/otel 0.004s / pkg/i18n 0.108s / pkg/utils 0.013s
```
我未删除他人文件、未改生产代码、未 commit、未删除 01–10 报告。

---

## D. 给主理人的三句话

1. **外围轴 P0/P1 五条全部实测成立**：otel 异构 Store 必现 panic、i18n 实际 11 种数值类型 panic、AnyToMap/postlog/casbin 三种 panic 均能真实触发；其中 casbin 有「Swagger.Uri 空串提前放行」的配置前提，i18n 覆盖面比报告更广。
2. **架构轴 R-01/R-02/R-03/R-04 全部成立**：nil-channel 死锁 + 字段 race 已双探针证实，Redis Init 锁内 Ping 实测阻塞 2.8022s，Redis/Mongo Init-after-Close 均返回 nil，PSubscribe 订阅 Close 后持续泄漏。
3. **两条需要纠正上游结论**：logsink FAIL 是测试期望值过期（单跑同样失败，非状态污染），规格轴「plugin 测试杀掉整包」被 exit=0 实测推翻。另 P2-3/P2-4/P2-10/P2-1 均动态证实，P2-3 还额外触发 `fatal error: sync: unlock of unlocked mutex`。

（报告完）
