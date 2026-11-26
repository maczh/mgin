# 第三轮独立 QA 复核与最终定级（12-qa-final）

- 复核人：严过关（Yan）· QA 工程师
- 仓库 / 分支 / HEAD：`/home/macro/Work/go/src/github.com/maczh/mgin`，`v2`，`4f1d9de`
- 环境：`go1.25.7 linux/amd64`，`GOPROXY=https://goproxy.cn,direct`
- 方法：**不采信 07/08/10/11 结论**，自行编写 `zz_qa3_*_test.go` 探针独立复跑；未改任何生产代码、未 commit；探针全部跑完即删。
- 交付时工作区：`git status --short` 仅 ` M .gitignore`（与任务要求一致）；四包测试与 vet 全绿（见 §7）。

---

## 0. 三个必答问题（先给结论）

| 问题 | QA 独立裁定 |
|---|---|
| **上一轮 P0 是否继续闭环？** | **是。** 上轮 P0 = Redis/Mongo 无锁共享 map（`fatal: concurrent map read and map write`）。独立核查：`clients/cfgs/conns/confData`（redis）与 `conns/tags/confData`（mgo）全部读写点仍在 `mu` 内；`mgoCheck`（唯一残留 `m.conns[tag]=` 裸写）经 `grep` 全仓仅 1 处定义、**无调用者**，为死代码。无新的无锁 map 读写面。 |
| **本轮是否存在真 P0？** | **存在，且只有 1 条：`pkg/otel/otel.go:31/38/50/58` 的 `atomic.Value` 异构 Store。** 独立探针动态证实 `sync/atomic: store of inconsistently typed value into Value`。触发条件不是"极端边界"，而是**按包注释推荐路径集成任意真实 OTel SDK Provider**（如 `sdktrace.TracerProvider`），100% 必现；发生在初始化/注入点、无 recover，属**进程级不可恢复 panic**。定 **P0，本轮必修**。注意 07 报"无新 P0"是因其范围为 core（未含 otel），08 报 P0-1 与本报告一致，但本报告结论由独立探针重新得出。 |
| **P1 优先级如何排序？** | QA 重排为：**① app.go 关停 nil-channel 死锁（进程级长阻塞） → ② i18n.Format 断言 panic → ③ casbin 中间件裸断言/nil Enforcer → ④ postlog gzip 畸形响应 panic → ⑤ utils.AnyToMap 异构 map panic**。理由见 §5.2。 |

**总览**：本轮 **P0=1（动态证实）｜P1=5（4 动态证实 + 1 静态确认）｜P2=7｜P3=4**。与 07 的核心结论**方向一致但定级不同**：07 判 P0=0，QA 判 P0=1（otel 属外围，不在 07 范围）；07 把关停死锁列为头号 P1，QA 维持其为 P1 首位。

---

## 1. 独立复跑表（全部自建探针，跑完即删）

| # | 探针名（临时文件） | 目标 | 方法 | 结果（原始证据） | 定性 |
|---|---|---|---|---|---|
| 1 | `TestQA3OtelHeterogeneousStorePanic` | P0 候选 otel | 嵌入 `trace.TracerProvider` 构造与 `noop` 动态类型不同的 Provider | **PASS**（复现）`sync/atomic: store of inconsistently typed value into Value` | **动态证实** |
| 2 | `TestQA3OtelSdkProviderCannotBeInjected` | otel Disable 分支 | 同上 + Disable 前后状态检查 | **PASS**：业务 Provider 首次注入即 panic；`init()` 已把 noop 具体类型锁死，`Disable` 的异构分支被前置 panic 遮蔽，实际不可达 | 动态证实 |
| 3 | `TestQA3OtelNoopOnlyIsFalseGreen` | 假绿验证 | 同类型重复 Store | PASS：noop-only 路径不 panic → `otel_test.go:27` 恰好绕开唯一触发路径 | 动态证实假绿 |
| 4 | `TestQA3FormatInt64Panic` | i18n int64 | `Format("{}", int64(7))` | **PASS** `interface conversion: interface {} is int64, not int` | 动态证实 |
| 5 | `TestQA3FormatFloat32Panic` | i18n float32 | `Format("{}", float32(1.5))` | **PASS** `interface conversion: interface {} is float32, not float64` | 动态证实 |
| 6 | `TestQA3FormatOtherIntWidthsPanic` | i18n 覆盖面 | int8/16/32、uint/8/16/32/64、uintptr | PASS：9/9 全部 panic | 动态证实 |
| 7 | `TestQA3AnyToMapStringIntPanic` | utils map[string]int | `AnyToMap(map[string]int{"a":1})` | **PASS** `interface conversion: interface {} is map[string]int, not map[string]string` | 动态证实 |
| 8 | `TestQA3AnyToMapOtherMapTypesPanic` | utils 覆盖面 | map[string]bool / map[int]string / map[string][]string | PASS：3/3 全部 panic | 动态证实 |
| 9 | `TestQA3AnyToMapNilRegression` | 上轮 nil 修复 | `AnyToMap(nil)` | PASS：返回非 nil 空 map，无 panic → **X 系列 nil 修复闭环** | 回归通过 |
| 10 | `TestQA3PostlogMalformedGzipPanic` | postlog gzip | gin+httptest，写畸形 gzip 响应体 | **PASS** `runtime error: invalid memory address or nil pointer dereference`（日志侧先打印 `gzip.NewReader error`） | 动态证实 |
| 11 | `TestQA3CasbinClaimsTypePanic` | casbin claims | `c.Set("claims", map[string]any{...})` | **PASS** `interface conversion: interface {} is map[string]interface {}, not jwt.MapClaims` | 动态证实 |
| 12 | `TestQA3CasbinUserIdAssertionPanic` | casbin 字段断言 | userId 非 float64 / 缺失 / roleId nil | PASS：3/3 panic | 动态证实 |
| 13 | `TestQA3CasbinNilEnforcerPanic` | casbin nil Enforcer | 清空 `SyncedCachedEnforcer` + DB 不可用 | **PASS** nil pointer dereference（`GetEnforcer` 内部日志"获取数据库连接失败!"后返 nil） | 动态证实 |
| 14 | `TestQA3ShutdownDeadlockDuringCheck` | **关停死锁** | 复刻 app.go:118-130 + :384-391，ticker 压缩到 20ms 命中 checkAll 执行期 | **PASS**：`checkWG.Wait()` 250ms 超时；`checkAll` 结束后仍阻塞（`done=nil` 使退出分支失效） | **动态证实** |
| 15 | `TestQA3ShutdownNormalPath` | 关停正常时序 | 后台 goroutine 阻塞在 select 时关停 | PASS：正常路径可唤醒、不阻塞 → 死锁是**条件性**而非必现 | 动态证实（界定性） |
| 16 | `TestQA3ErrcodeRegisterDefRace` | P2 errcode | 8 goroutine × 200 次 `RegisterDef/LookupDef/AllDefs` | **`-race` 报 `WARNING: DATA RACE`**（Read/Write 同时命中 `definitions` map） | 动态证实 |
| 17 | `TestQA3JobStoreUnderRLockRace` | P2 job.Store | 8 goroutine 并发 `Store()`/`IsRunning()` | **`-race` 报 `WARNING: DATA RACE`**（`m.st` 读/写并发） | 动态证实 |
| 18 | `TestQA3MemcacheClearRace` | P2 memcache | 并发 `Add/Value/Clear` | **`-race` 报 `WARNING: DATA RACE`**（`sync.Map` 整体赋值） | 动态证实 |
| 19 | `TestQA3Shutdown*` under `-race` | 关停竞态 | 同 #14/#15 加 `-race` | **`-race` 报 `WARNING: DATA RACE`**：`tc.done = nil` 写 vs goroutine 读（对应 `app.checkDone = nil` vs goroutine `case <-app.checkDone`） | 动态证实 |
| 20 | `TestQA3RedisInitAfterCloseFalseSuccess` / `GetConnectionAfterCloseRegression` | Redis 回归 | 手工构造 `closed=true` | PASS：InitWithError 返 nil 假成功；GetConnection 返 `ErrClosed` | 动态证实 + 回归通过 |
| 21 | `TestQA3MongoInitAfterCloseFalseSuccess` / `GetConnectionAfterCloseRegression` | Mongo 回归 | 同上 | PASS：同上；`GetConnection` 无网络 I/O | 动态证实 + 回归通过 |
| 22 | `TestQA3PluginRegisterRejectsPanickingNameOrder` / `AcceptsLegitPlugin` / `SafeWrappersCoverAllLifecycle` | plugin 反向检查 | panic-on-field 插件遍历 Name/Order/Init/Close/Health/Enabled | PASS：Register 安全拒绝、合法插件不误拒、6 个 `safe*` 覆盖完整、三态 map 一致 | 回归通过 |
| 23 | logsink 单跑 vs 全包跑 | 基线 FAIL 归因 | `go test -run '^TestQueueKeepsNewestAndFIFO$'` 与 `go test ./pkg/logsink` 对照 | **两者一致 FAIL**：`options: {QueueSize:1000 Workers:1}` vs 期望 `{100, 1}` | 测试断言错误，**非顺序污染** |

**探针自身 2 处编译错误（QA 自修，Round 1 内）**：① `trace.TracerProvider` 含非导出方法，外部类型无法实现 → 改为嵌入接口；② shutdown 探针 `inCheck` 误声明为 receive-only。二者均为测试代码 bug，按"Send To: QA(self)"自行修复，不涉及生产代码。

---

## 2. 与 07/08/10/11 的差异（独立结论对照）

| 项 | 07（架构师核心） | 08（架构师外围） | 10（规格轴） | 11（工程师） | **QA 独立结论与差异** |
|---|---|---|---|---|---|
| 本轮 P0 | 0 | P0-1 otel | 未评 | （未落盘） | **QA = 1（otel）**。与 08 同，与 07 不同——07 范围未含 otel，非事实冲突。 |
| otel 异构 Store | 未评 | P0-1，静态 + 探针 | 未评 | — | **动态证实**，并补充 08 未强调的一点：`Disable` 的异构分支因 `init()` 类型锁定而**实际不可达**，真正的必现点是首次注入。 |
| 关停死锁 | R-01 P1，动态证实 | 未评 | 指出 Wait 顺序错 | — | **独立复刻证实**，且补充：即使 `checkAll` 结束仍永久阻塞（这是 07 未明确的 second-order 结论）。严重度 QA 判 P1 首位，**与 07 同级**。 |
| 关停竞态 | 未测 | 未评 | 未评 | — | **QA 新增动态证据**：`-race` 报 `checkDone=nil` 读写竞争。07 只报逻辑死锁，未报竞态。 |
| i18n / utils / postlog / casbin | 未评 | P1-1~4 | 未评 | 工程师已复现 | **QA 全部独立动态证实**，与 08 一致，但复现路径不依赖其探针。 |
| Init 锁内 I/O | R-02 P2，实测 2.8s | 未评 | 判为"部分实现/错误" | — | **QA 静态确认**（未在本次独立跑网络探针）：`redis.go:43-46/185`、`mgo.go:44-47/103/124` 仍在 `initMu→mu` 双锁内 I/O；读路径 `GetConnection` 首段 `RLock` 会被阻塞。维持 P2。 |
| InitWithError 假成功 | R-03 P2 | 未评 | 指出 multi 模式 initErr 漏报 | — | **QA 动态证实**（Redis/Mongo 均返 nil 假成功）；与 07 一致。 |
| PSubscribe 泄漏 | R-04 P2 | 未评 | 指出 `:432` 提前 return 漏关 | — | **QA 静态确认**：`pubsub.Channel()` 关闭路径末尾有 `pubsub.Close()`，但 handler 段 `if ctx.Err() != nil { return }`（`redis.go:432` 附近）直接 return 未 Close → 泄漏仍在。维持 P2。 |
| errcode / job / memcache 竞态 | 未评 | P2-3/P2-4/P2-10 | 未评 | — | **QA 对 08 的三条 P2 全部独立跑 `-race` 证实**。 |
| logsink FAIL | 未归因 | 提到测试覆盖 | 未评 | — | **QA 新增结论**：单跑与全包跑一致 FAIL，排除顺序污染；根因是测试断言与 `normalize()` 实现不一致（上限 1024 而非 100），属**测试缺陷**，非生产 panic 面。 |
| plugin 反向检查 | R-09 P3（一致性隐患） | — | 质疑三态退化 | — | **QA 动态证实**：Name/Order panic 被安全拒绝、合法插件不误拒、`safe*` 六方法全覆盖、三态 map 当前一致。R-09 降为 P3 观察项。 |
| `4f1d9de` 删不可达 return | 未评 | §5 澄清非本轮引入 | 判为 scope creep | — | **QA 同意 08**：仅删除不可达语句，不引入新 panic；`dao/*` 新增 nil 守护是改善，非回归。 |

---

## 3. 反向检查结论（重点：修复是否引入新问题）

### 3.1 plugin 三态一致性 / `safe*` 覆盖 / `Register` 语义
- **三态**：`plugins`、`meta`、`order` 的写入全部集中在 `Register/Unregister` 的 `mu.Lock()` 内（`plugin.go:66-72, 89-103`）；读路径 `All()`/`sortedByOrder()` 持 `RLock`。探针遍历 `order` 快照未发现"有 order 无 plugin"的不一致。
- **`safe*` 覆盖**：`safeEnabled/safeInit/safeClose/safeHealth/safeName/safeOrder` 六个方法均有 `defer recover`（`plugin.go:234-286`），覆盖 `Plugin` 接口全部方法。动态探针逐一注入 panic，全部被兜住。
- **`Register` 拒绝语义变化**：旧行为是 `p.Name()` panic 直接崩进程；新行为是记日志后静默跳过注册。**不构成合法插件误拒**（探针动态证实），但引入一个低概率行为变化：若合法插件的 `Name()` 依赖尚未初始化的外部状态而 panic，将被静默跳过，业务侧仅从日志可见 → **P3 观察项**，建议 `Register` 返回 error 或在日志中带插件类型信息。
- **结论**：本轮 plugin 修复**未引入新 panic/死锁**。

### 3.2 app.go 关停顺序与新增窗口
- **关停顺序**：`app.go:397-406` 已是 `Shutdown( servers )` → `SafeExit()`，DB 在 HTTP 排空后才关，**上轮 P1 项闭环**，未引入"排空期使用已关闭 DB"的窗口。
- **排空期健康检查窗口**：架构师/规格担心的"排空期健康检查继续跑"**实际不存在，反而相反**——`app.go:384-391` 的 `ticker.Stop() + close(checkDone) + checkWG.Wait()` 位于 `Shutdown` **之前**，即健康检查在 HTTP 排空前就被停掉。这不是新窗口，而是把 `checkAll` 的阻塞风险**前置并放大**：`checkWG.Wait()` 无超时，若 `HealthAll()` 中某个 `Health()` 卡住（如 mongo `conn.Ping()` 无框架侧超时、driver 默认 30s serverSelectionTimeout），HTTP `Shutdown` 根本不会开始 → 在途请求被 SIGKILL 强断。**与 nil-channel 死锁叠加，是 QA 认为其严重度不低于 07 判定的原因。**
- **结论**：引入的是**更严重的阻塞面**（P1 首位 + P2 无超时项），不是"健康检查继续跑"的窗口。

### 3.3 本轮 9 提交涉及文件的新 panic 面
- `pkg/middleware/xss/xss.go:510`、`pkg/utils/rsautil.go:366/402`：仅删除编译器不可达的 `return`，**不引入**新 panic；`xss.go:494-505` 的数组元素断言 panic 改动前就存在（同意 08 的澄清），非本轮引入。
- `pkg/db/dao/mysql.go`、`pkg/db/dao/redis_cache.go`：新增 nil receiver 与 `f==nil` 守护，**改善**。
- `app.go`、`plugin/*`、`db/redis`、`db/mongo`、`db/sqlite`：见 §3.1/§3.2 与 §4。
- **结论**：本轮修复**未新增可归因的生产 panic**。

### 3.4 上轮修复回归结果

| 项 | 状态 | 证据 |
|---|---|---|
| X2/X3 关旧连接 + mongo 不再 Ping | ✅闭环 | `GetConnection` 已无 Ping；`Copy()/DB()` 仍在 RLock 内取值但无 I/O；探针确认 closed 后无网络调用 |
| X5/X5b/X6 plugin 元信息缓存 + 双端守护 | ✅闭环 | 动态探针 3/3 通过 |
| X10 sqlite validate | ✅闭环 | `validateSqlite` 判到 `conn.Config`/`conn.Statement`/`conn.Error`，`Close/Check/GetConnection` 三入口均调用 |
| X13/X14 | ✅闭环 | 静态确认 + 现有测试通过 |
| **X1 Init 锁内 I/O** | 🟡**部分闭环** | 读路径已修（快照后解锁再 Ping），**写路径 Init 原地未动** |
| **Redis/Mongo InitWithError Close 后假成功** | ❌未闭环 | 动态证实返 nil |
| **Redis PSubscribe 泄漏** | ❌未闭环 | 静态确认 `ctx.Err()` 提前 return 漏关 pubsub |

---

## 4. 最终定级表

> 后果分类：`fatal`=进程级不可恢复 / `panic-init`=初始化或定时器无 recover 的进程级 panic / `panic-req`=请求级可恢复 panic / `deadlock`=死锁长阻塞 / `race`=竞态泄漏 / `behavior`=行为回归。
> 可信度：**动态证实**（本人探针实际触发）/ **静态确认**（代码事实明确但未跑最小复现）/ **未验证**。

| ID | 级别 | 问题（file:line） | 后果分类 | 可信度 | 本轮必修 | 修复要点 |
|---|---|---|---|---|---|---|
| **QA-P0-1** | **P0** | `pkg/otel/otel.go:31,38,50,58` `atomic.Value` 异构 Store | `panic-init` | **动态证实** | **是** | 保存**接口类型**而非具体类型：`atomic.Value` 改为 `var tracerProvider atomic.Pointer[trace.TracerProvider]`（Go 1.19+）或包裹为统一 struct `{tp trace.TracerProvider}` 后 Store；`Disable` 同理。`Tracer()` 的 `.(trace.TracerProvider)` 改为安全读取。补异构 Provider 交叉 Store 回归用例。 |
| **QA-P1-1** | P1 | `app.go:118-130,384-391` `close(checkDone); checkDone=nil; checkWG.Wait()` | `deadlock`+`race` | **动态证实** | **是** | 去掉 `checkDone = nil`（`close` 后无需置 nil）；或改用 `sync.Once` + 独立局部 channel 快照；`Wait()` 必须带超时（`select { case <-done: case <-time.After(X): logs.Error(...) }`），且放在 `Shutdown` **之后**（先排空 HTTP，再等健康检查），避免坏 Health 阻塞排空。 |
| QA-P1-2 | P1 | `pkg/i18n/i18n.go:107-110` `Format` 断言 | `panic-req`（若在初始化/定时任务调用则 `panic-init`） | **动态证实** | 是 | 拆分 type switch 为具体类型 case 各自格式化：`case float32: FormatFloat(float64(v),'f',6,32)`；`case int64/uint64/...: strconv.FormatInt/FormatUint`；`int8..uintptr` 先转 `int64`。禁止一个 case 组配单一断言。 |
| QA-P1-3 | P1 | `pkg/middleware/casbin/casbin.go:34-45` 裸断言 + nil Enforcer | `panic-req` | **动态证实** | 是 | claims 用 comma-ok 取 `jwt.MapClaims`，`userId/roleId` 用 comma-ok 取 float64 并校验缺失；`GetEnforcer()` 结果判 nil，nil 时 `AbortWithStatusJSON(503/500)`；`LoadPolicy()` 错误记录。另建议给 `strings.Contains(path, config.Config.Sys.Swagger.Uri)` 加空串守卫（生产由 config.Init 填默认值，未走 Init 时会全放行）。 |
| QA-P1-4 | P1 | `pkg/middleware/postlog/logger.go:184-190` gzip nil Reader | `panic-req`（且**不在** `nice.Recovery` 保护范围内——Recovery 注册在 postlog 之后，postlog 的 `c.Next()` 后代码 panic 时 Recovery 已出栈） | **动态证实** | 是 | `if err != nil { logs.Error(...); responseBody = ""（或原样） } else { defer r.Close(); ... }`；不得在 err 分支继续用 `r`。 |
| QA-P1-5 | P1 | `pkg/utils/structutil.go:102-127` `AnyToMap` | `panic-req` | **动态证实** | 是 | 用反射遍历 map：`for _, k := range reflect.ValueOf(obj).MapKeys() { v := ...MapIndex(k).Interface(); ... }`，任何 `reflect.Map` 都能安全转 `map[string]string`；去掉 `else { return obj.(map[string]string) }` 硬断言；nil value 用 `"<nil>"` 分支。 |
| QA-P2-1 | P2 | `redis.go:43-46,185`、`mgo.go:44-47,103/124` Init 持双锁 I/O | `deadlock`（可用性退化） | 静态确认 | 否（下轮） | 按 RCU：锁外解析配置 + 建连/Ping 到临时变量，锁内一次性发布 + 关闭旧连接。 |
| QA-P2-2 | P2 | `redis.go:InitWithError` / `mgo.go:InitWithError` Close 后返 nil | `behavior` | **动态证实** | 否（建议同批） | `Init` 内 `if r.closed { r.initErr = ErrClosed; return }`，使 `InitWithError` 与 Mysql 语义一致。 |
| QA-P2-3 | P2 | `redis.go:PSubscribeContext` `if ctx.Err() != nil { return }` 漏关 pubsub | `race`（goroutine/连接泄漏） | 静态确认 | 否 | 统一 `defer pubsub.Close()` 或该分支补 Close；`Receive()` 加超时/context 感知。 |
| QA-P2-4 | P2 | `app.go:391` `checkWG.Wait()` 无超时且先于 Shutdown | `deadlock` | 静态确认 | 是（与 P1-1 同批） | 见 QA-P1-1。 |
| QA-P2-5 | P2 | `pkg/errcode/definition.go:79-81` `RegisterDef` 无锁写全局 map | `race` → **运行时调用即升级为 `fatal`** | **动态证实** | 否（下轮） | 加 `sync.RWMutex` 或改 `atomic.Value` 快照；`LookupDef/AllDefs` 同步加读锁。当前文档限定"init() 中调用"故保持 P2。 |
| QA-P2-6 | P2 | `pkg/job/scheduler.go:301-308` `Store()` 在 RLock 内写 `m.st` | `race` | **动态证实** | 否 | 改为 `Lock()` 或在锁外 `LoadOrStore`。 |
| QA-P2-7 | P2 | `pkg/cache/memcache.go:63` `c.items = sync.Map{}` | `race` | **动态证实** | 否 | 用 `Range+Delete` 逐项清理，禁止整体赋值 `sync.Map`（复制内含锁）。 |

**P3（观察项，非必修）**：

| ID | 问题 | 说明 |
|---|---|---|
| QA-P3-1 | `app.go:144` `panic(err)`（i18n.Init 失败） | **按 `docs/coding-standards.md §3.1 第4条**「不 panic 作为控制流：**仅初始化不可恢复**（如 `logs.InitVictoriaLogs` 失败）才 panic」——此 panic 在初始化阶段且不可恢复，**合规**，不列为问题。若评审意见与此冲突，以仓库成文规范为准。 |
| QA-P3-2 | `plugin.Register` 静默拒绝 Name/Order panic 插件 | 行为变化（见 §3.1），建议日志带插件类型或改返回 error。 |
| QA-P3-3 | `mgoCheck` 死代码（唯一 unlocked map 写） | 当前无调用者，不可达；下次触碰 mgo.go 时删除。 |
| QA-P3-4 | `pkg/logsink/sink_test.go:236` 测试断言错误 | `normalize()` 上限为 `DefaultQueueSize=1024`，测试却期望 100；单跑/全包跑一致 FAIL，非顺序污染、非生产缺陷。修测试断言即可（QA 未改既有测试，以免污染交付工作区）。 |

### 5.1 严重度口径说明
- `otel` 的 panic 是 `sync/atomic` 抛出、`recover` 可捕获，但**按推荐路径发生在服务初始化/注入点，框架与业务均无 recover** → 进程启动即崩，归为 `panic-init`/P0。
- `i18n.Format`/`utils.AnyToMap` 在 HTTP handler 链内可被 `nice.Recovery` 兜住 → `panic-req`/P1；但一旦在初始化、定时任务等非 HTTP 上下文调用即升级为进程级。
- `postlog gzip` 的 panic 位于 `c.Next()` **之后**，此时链中后面的 `nice.Recovery` 已出栈，框架层兜不住，只能靠 `net/http` 的 per-conn recover（连接被断开、请求无响应）→ 仍归 `panic-req`，但可恢复性弱于 i18n/casbin。
- 关停死锁非 fatal，但导致 `Shutdown`/`SafeExit` 全部不执行、需 SIGKILL，等价于进程级服务不可用 → P1 首位。

### 5.2 P1 重排理由（QA 可重排）
1. **关停死锁（P1-1）**：唯一影响**整个进程生命周期**的问题；一旦命中，所有在途请求、DB 连接、日志 handler、订阅均无法优雅收尾，K8s 会在宽限期后 SIGKILL，可能造成数据丢失。虽条件触发，但后果不可逆且无法自恢复。
2. **i18n.Format（P1-2）**：框架级 API（`ErrorDef`/`NoRoute`/`Recovery` 均经它），`int64` 是 Go 业务最常见整型，触发概率最高；修复成本极低（拆分 case）。
3. **casbin（P1-3）**：鉴权中间件裸断言，panic 会在启动阶段暴露为 500 而非 403，可能掩盖权限问题；影响面限启用 casbin 的项目。
4. **postlog gzip（P1-4）**：需特定响应头 + 畸形 body，触发面较窄；且 `net/http` 可兜底进程不崩。
5. **utils.AnyToMap（P1-5）**：`map[string]int` 等异构 map 在业务中常见，但该函数在多处调用点已有前置类型约束，实际触发面略窄于 i18n。

---

## 6. 测试 seam 说明（哪些风险缺正确 seam / 哪些测试是假绿）

**假绿 / 覆盖缺口**：
- `pkg/otel/otel_test.go:27`：`SetTracerProvider(noop.NewTracerProvider())` 与 `init()` 动态类型完全相同，**恰好绕开 P0-1 唯一触发路径**。必须补"异构 Provider 交叉 Store"用例（可用嵌入 `trace.TracerProvider` 的自定义类型）。
- `pkg/i18n/store_test.go:178`：仅 `Format("welcome","Sam")`（string），未覆盖 11 种数值类型。
- `pkg/casbin/`：无任何中间件测试，P1-3 完全裸露；探针必须用 `gin.CreateTestContext` + 显式设置非空 `Swagger.Uri`（否则 `strings.Contains(path, "")` 恒真提前放行，制造新的假绿）。
- `pkg/middleware/postlog/logger_test.go`：只测正常 handler 投递，未测 gzip 畸形响应。

**缺正确 seam（难以直接单测）**：
- **真实 `mgo.Session`**：`Mongodb.GetConnection()` 返回具体类型 `*mgo.Database`，无法注入 fake；本轮只能靠 `closed=true` 构造无 I/O 路径（已验证 `ErrClosed`），**"Init 锁内 Dial 阻塞读路径"无法在不引入接口抽象的情况下写最小探针** → 该项只能静态确认。
- **OTel SDK Provider**：`trace.TracerProvider` 含非导出方法，外部包不能实现；只能靠嵌入接口构造异构类型。真实 `sdktrace.TracerProvider` 未进 go.mod，无法在包内测真实集成路径。
- **gzip 中间件路径**：需通过完整 `gin.Engine`+`httptest` 才能命中（响应头 `Content-Encoding` 由 `bodyLogWriter.Header()` 透传），无独立纯函数 seam。
- **`app.Run()` 关停逻辑**：`Run()` 内阻塞等 `signalChan`，无法直接注入信号；本轮探针采用**复刻时序模式**（同一段 `close/nil/Wait` 代码）而非调用 `Run()`。建议把关停序列抽成 `func (app *App) shutdown(ctx)` 可测 seam。
- **`-race` 三项**：`errcode`/`job`/`memcache` 均可直接构造并发探针；`job.Store` 需临时清空 `config.Config.Config.Used` 以避免 `newStore` 走真实 DB。

---

## 7. 交付前状态确认

```
$ git status --short
 M .gitignore

$ find . -name 'zz_qa3_*' -o -name 'zz_r3_*'
(空)

$ go test ./pkg/otel/... ./pkg/plugin/... ./pkg/i18n/... ./pkg/utils/... -count=1
ok  github.com/maczh/mgin/v2/pkg/otel     0.012s
ok  github.com/maczh/mgin/v2/pkg/plugin   0.021s
ok  github.com/maczh/mgin/v2/pkg/i18n     0.150s
ok  github.com/maczh/mgin/v2/pkg/utils    0.013s

$ go vet ./pkg/otel/... ./pkg/plugin/... ./pkg/i18n/... ./pkg/utils/...
(无输出)
```

**未修改任何生产代码，未 commit，未删除 01–11 报告。**

---

## 8. 修复优先级建议（给主理人）

1. **本轮必修（P0+P1-1）**：`otel` 异构 Store（4 行改动）+ `app.go` 关停 nil-channel 与无超时 Wait（同样 4~6 行）。二者都是小改动、高收益。
2. **本轮强烈建议（P1-2~5）**：i18n.Format / casbin / postlog / AnyToMap 四处断言与 nil 检查，均为局部补丁，建议同批合入并补对应回归用例（修复与用例同 PR，遵循 04§6/§10.11）。
3. **下轮排期（P2）**：Init 锁内 I/O 的 RCU 重构、InitWithError 语义统一、PSubscribe 泄漏、errcode/job/memcache 三处竞态。
4. **规范裁定**：`app.go:144` 的初始化 panic 与 `docs/coding-standards.md §3.1 第4条`一致，**不要求整改**。
5. **测试债**：修 `pkg/logsink/sink_test.go:236` 的断言（上限 1024），使 `go test ./pkg/...` 在排除 config/etcd 网络超时包后恢复全绿。

---

*报告完 · QA 严过关 · 第三轮独立复核*
