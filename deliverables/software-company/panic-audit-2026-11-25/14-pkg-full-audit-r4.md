# 全量 `pkg/` 代码 panic 风险第四轮复审报告（修复后再审核）

- 复审日期：2026-11-25
- 仓库 / 分支 / HEAD：`/home/macro/Work/go/src/github.com/maczh/mgin`，`v2`，`39c554e`
- 上轮复审基线：`4f1d9de`（其后 3 个提交：625f83c / feb0481 / 39c554e，58 文件 +1640/−371）
- 复审范围：`pkg/` 全量（含 20 个子包、约 21,800 行）+ `app.go` / `mgin.go` / `internal/safego` 生命周期装配；全 38+ 既有测试文件
- 复审方法：四路并行静态通读（DB 层 / 中间件+工具 / 生命周期+并发原语 / otel+i18n+规范）→ 主审逐条复核关键代码 → 最小探针动态证实 → `code-review` 技能规范轴 / 规格轴双审查 → 全量 `vet`/`test`/`test -race` 基线
- 纪律：**未修改任何生产代码、未 commit**；临时探针（`zz_r4_*`）跑完即删；工作区进入任务前既有的 4 个测试文件改动（`mongo_lifecycle_test.go` / `redis_lifecycle_test.go` / `plugin_panic_test.go` / `mysql_clone_test.go`）与 ` M .gitignore` 均**原样保留、未触碰**

---

## 0. 结论摘要

### 0.1 三个必答问题

| 问题 | 结论 |
|---|---|
| 上一轮的 P0 / P1 是否真的修好了？ | **是，全部真闭环，无假修复。** 上轮唯一 P0（otel `atomic.Value` 异构 Store）改为 `providerHolder` 统一包装，交叉 Store 回归用例真绿；P1×5（app 关停死锁、i18n 数值断言、casbin 裸断言、postlog gzip、AnyToMap）全部落地且经动态复核。 |
| 本轮是否有新的 P0？ | **没有。** 无启动必崩、无并发 map fatal、无不可恢复 runtime fatal。上轮 P2×7 也基本闭环（errcode/job/memcache 竞态经 `-race` 全绿确认）。 |
| 本轮发现了什么？ | **3 项 P1「遗漏型」panic + 4 项 P2 风险。** 共同特征是「修了同族函数、漏了孪生兄弟」：`rsautil` 4 个加密方法有 nil 检查而 6 个签名/验签方法没有；`structutil` 只修了 `AnyToMap` 而 4 个同类函数仍裸解引用；`sliceutil` 3 个 Delete 加了边界而 `DeleteArray` 没加。**3 项均已用最小探针动态证实**（9/9 触发 panic）。 |

### 0.2 最终风险分布

| 级别 | 数量 | 内容 |
|---|---|---|
| **P0** | 0 | 上轮 P0 闭环，本轮无新增启动必崩/不可恢复 fatal |
| **P1** | 3 | `rsautil` 零值密钥签名（3/3 探针 panic）、`structutil` nil 入参（4/4 panic）、`DeleteArray` 越界（2/2 panic） |
| **P2** | 4 | otel typed-nil Provider → `StartSpan` nil Tracer（动态证实）、app 二次 `Run` close of closed channel（动态证实）、safego 吞 `runtime.Error` 致关键循环静默假死、job `StoreWithError` 锁内 DB I/O 回归 |
| **P3** | 6 | cache `clone.Clone` 理论 panic（动态未复现）、postlog 中间件顺序、logsink 队列测试假绿、app 关停测试假绿、config nil 告警降级、规范/PR 边界问题 |

**总体判断**：本轮是一轮**高质量修复**——上轮全部 P0/P1 与 P2×7 主体落地，`go vet` / `go test` / `go test -race` 三条基线首次同时全绿。但修复面集中在「已登记的风险点」，审计视野边缘的**同族孪生函数**成为新的主要风险来源，且 3 项 P1 修复成本极低（各 1–3 行守卫 + 表驱动测试），应在本轮立即收口。

### 0.3 基线验证（看退出码与完整输出，不 grep 计数）

| 命令 | 结果 |
|---|---|
| `go vet ./pkg/...` | **exit=0**，无告警 |
| `go test -count=1 -timeout 150s $PKGS`（排除 `pkg/config`、`pkg/registry/etcd` 两个 gRPC 代理拨号环境超时包） | **exit=0**，全部 ok（上轮 `pkg/logsink` FAIL 已修） |
| `go test -race -count=1 -timeout 150s $PKGS` | **exit=0**，无 DATA RACE（上轮 errcode/job/memcache 三处竞态确认闭环） |
| 探针清理后 `find . -name 'zz_r4_*' -o -name 'zz_qa3_*' -o -name 'zz_r3_*'` | 空 |

---

## 1. 上轮风险逐条回归

来源：第三轮报告 `13-pkg-full-audit.md` §2/§3/§4；本轮复核基于当前 HEAD 代码与 diff。

| 编号 | 上轮问题 | 本轮判定 | 当前证据 |
|---|---|---|---|
| **P0-1** | otel `atomic.Value` 异构 Store 必崩 | ✅ **真闭环** | `otel.go:36,40,52,60` 三处 Store 统一写 `providerHolder{provider: tp}`，`:66` 用 `holder, ok := Load().(providerHolder)` + nil 兜底；`otel_test.go:42-53` 交叉 Store 自定义 Provider 与 noop，旧实现必 panic，真绿 |
| **P1-1** | app 关停 `checkDone=nil` 死锁 + race | ✅ **真闭环** | `app.go:121-122` 快照局部 `checkTicker/checkDone`，`:390-392` 只 `close(checkDone)` 不再置 nil，`:406` `waitForHealthChecks` 有界超时且排在 `Shutdown` **之后**；`app_shutdown_test.go` 覆盖超时与立即返回 |
| **P1-2** | i18n `Format` 11 种数值类型 panic | ✅ **真闭环** | `i18n.go:103-143` 改 `switch v := value.(type)` 逐分支格式化，`float32` 显式转 `float64`；`format_panic_test.go` 覆盖 12 种数值 + 5 种其他类型 |
| **P1-3** | casbin claims 裸断言 / nil Enforcer | ✅ **真闭环** | `middleware/casbin/casbin.go:35,50,58` comma-ok + `claimID` type switch + nil 时 503；`casbin.go:186` `if s==nil return nil`；空 Uri 加守卫；`casbin_panic_test.go` 覆盖 |
| **P1-4** | postlog gzip 失败后 nil Reader 解引用 | ✅ **真闭环** | `logger.go:185-197` err 分支 else 包裹，`r` 为 nil 时不进入解压；`logger_test.go:75` 覆盖畸形 gzip |
| **P1-5** | `AnyToMap` nil 值 / 异构 map panic | ✅ **真闭环** | `structutil.go:103-153` reflect 统一遍历，nil → `"<nil>"`，非 string 键返空 map；`structutil_panic_test.go` 覆盖 |
| **P2-1** | Redis/Mongo Init 锁内网络 I/O | ✅ 闭环 | `redis.go:48-94`、`mgo.go:45-90` builder 在锁外构建、锁内仅 RCU 发布 + 二次 `closed` 检查；`InitNetworkIODoesNotHoldClientStateLock` 覆盖 |
| **P2-2** | `InitWithError` 关闭后假成功 | ✅ 闭环 | 两源 `Close` 清 `confData` + `Init` 查 `closed` 返回 `ErrClosed` |
| **P2-3** | PSubscribe 泄漏 | ✅ 闭环 | `PSubscribeContext` 内部 ctx + handler 可 cancel + 断线重连 + 统一收口；工作区 `redis_lifecycle_test.go` 新增两条订阅生命周期用例 |
| **P2-4** | `checkWG.Wait()` 无超时/顺序错 | ✅ 闭环 | 见 P1-1；有界等待且移到 `Shutdown` 后 |
| **P2-5/6/7** | errcode / job / memcache 竞态 | ✅ 闭环 | errcode 加 `RWMutex`、job `Store()` 改 `Lock()`、memcache 用 `Range+Delete` 替代 `sync.Map` 整体赋值；`-race` 全绿 |

**统计**：上轮 P0×1 + P1×5 + P2×7 共 13 项，**12 项完全闭环、1 项（P2-1 Mongo 静态部分）主体闭环**；无一项假修复。这是本轮最值得肯定的成果。

---

## 2. P1：本轮新发现（均已动态证实）

### P1-1 `pkg/utils/rsautil.go:110-182` — 全局零值 `RSA` 调用签名/验签必 panic

- **触发链**：`var RSA = &RSASecurity{}`（`:21`，零值 `prikey/pubkey` 均为 nil）→ `RSA.SignSha256WithRsa(data)` → `rsa.SignPKCS1v15(rand.Reader, nil, ...)` → 标准库 fips 检查解引用 nil。
- **根因**：同文件 `PubKeyENCTYPT/PubKeyDECRYPT/PriKeyENCTYPT/PriKeyDECRYPT`（`:56/:69/:82/:95`）本轮都补了 `if rsas.pubkey/prikey == nil { return err }`，但**六个签名/验签方法漏改**：`SignSha1WithRsa`、`SignSha256WithRsa`、`SignSha256WithRsaHex`、`SignSha256WithRsaUrlSafe`、`VerifySignSha1WithRsa`、`VerifySignSha256WithRsa` 直接把可能为 nil 的密钥传给 `crypto/rsa`。典型「修了同族、漏了孪生」。
- **动态证据**（探针 `pkg/utils/zz_r4_probe_test.go`，已删）：
  ```
  [R4-PROBE] rsautil.SignSha256WithRsa zero-value PANIC: runtime error: invalid memory address or nil pointer dereference
  [R4-PROBE] rsautil.SignSha1WithRsa zero-value PANIC: runtime error: invalid memory address or nil pointer dereference
  [R4-PROBE] rsautil.VerifySignSha256WithRsa zero-value PANIC: runtime error: invalid memory address or nil pointer dereference
  ```
  3/3 探针 panic；另外 3 个同构方法经代码复核（同一 rsa 调用、无守卫）静态确认。
- **影响面**：`RSA` 是导出全局单例，业务常见「先签名、配置后加载」或「配置加载失败仍继续」路径即可触发；HTTP handler 内可被 `nice.Recovery` 兜住，但启动期/定时任务/中间件内调用为进程级崩溃。
- **假绿**：`panic_edges_test.go:104-125` 只测 `getPubKey/getPriKey` 对非 RSA key 的返回，零值 `RSASecurity` 签名路径完全未覆盖。
- **修复**：六个方法开头统一加 `if rsas.prikey == nil`（验签为 `pubkey`）返回明确错误；抽 `signPKCS1`/`verifyPKCS1` 内部 helper 消除六份重复；补零值表驱动测试。**与修复同 PR**。

### P1-2 `pkg/utils/structutil.go:16-89` — 四个反射函数对 nil 入参裸解引用

- **触发链**：`Struct2Map(nil)` → `reflect.TypeOf(nil)` 返回 nil → `objT.Kind()` / `t.NumField()` nil 指针解引用。同族四个函数：`Struct2Map`、`Struct2MapString`、`GetStructFields`、`GetStructJsonTags`。
- **根因**：本轮只修了同文件的 `AnyToMap`（`:103` 有 `obj==nil` 守卫），这四个「孪生」函数漏改。
- **动态证据**：
  ```
  [R4-PROBE] structutil.Struct2Map(nil) PANIC: runtime error: invalid memory address or nil pointer dereference
  [R4-PROBE] structutil.Struct2MapString(nil) PANIC: ...
  [R4-PROBE] structutil.GetStructFields(nil) PANIC: ...
  [R4-PROBE] structutil.GetStructJsonTags(nil) PANIC: ...
  ```
  4/4 panic。
- **影响面**：通用工具函数，调用方传入接口零值/`map` 查询未命中结果时易出现 nil；非 HTTP 路径为进程级风险。
- **假绿**：`structutil_panic_test.go` 只测 `AnyToMap`，四个函数裸奔。
- **修复**：每个函数开头 `if obj == nil { return 空值 }`；补 nil 入参表驱动测试。

### P1-3 `pkg/utils/sliceutil.go:216-219` — `DeleteArray` 无边界检查

- **触发链**：`DeleteArray([]any{1}, 5)` → `src[:5]` 越界；`index=-1` → `src[:-1]` 越界。
- **根因**：同文件 `StringArrayDelete`（`:21`）、`IntArrayDelete`（`:41`）、`Float64ArrayDelete`（`:61`）本轮都加了边界守卫，唯 `DeleteArray` 漏改。
- **动态证据**：
  ```
  [R4-PROBE] sliceutil.DeleteArray index=5 len=1 PANIC: runtime error: slice bounds out of range [:5] with capacity 1
  [R4-PROBE] sliceutil.DeleteArray index=-1 len=1 PANIC: runtime error: slice bounds out of range [:-1]
  ```
  2/2 panic。
- **假绿**：`panic_edges_test.go:55-69` 只测 `IntArrayDelete`，`DeleteArray` 未覆盖；仓库内暂无调用方（导出 API），故排在 P1 末位。
- **修复**：补 `if index < 0 || index >= len(src) { return append([]any(nil), src...) }`（或原样返回）；补表驱动测试。

---

## 3. P2：本轮新发现（待排期）

| # | 位置 | 触发与证据 | 修复要点 |
|---|---|---|---|
| **P2-1** | `pkg/otel/otel.go:48-52,65-70,78-79` | P0 修复统一了 atomic 类型，但 `SetTracerProvider` 仅判接口 `tp == nil`，**typed-nil 指针**（如 `var p *customProvider = nil; SetTracerProvider(p)`）越过检查，`Tracer()` 调 `holder.provider.Tracer()` 返回 nil Tracer，`StartSpan` 对 nil 调 `.Start()` panic。探针证实：`SetTracerProvider(typed-nil)` NO-PANIC、`Tracer()` NO-PANIC、**`StartSpan(nil Tracer)` PANIC**。 | 用反射判 typed-nil，或 `Tracer()` 内对返回 nil 兜底 noop；补 typed-nil Provider 回归用例。 |
| **P2-2** | `app.go:390-392` | `close(app.checkDone)` 无 `sync.Once` / 重入保护，二次 `Run()`（测试重启、嵌入式复用场景）触发 `close of closed channel`。探针证实 PANIC。正常单进程只 Run 一次，故降 P2。 | 用 `closeOnce.Do` 或 `select { case <-done: default: close(done) }`；若明确不支持重入，在 `Run()` 入口加守卫并返回错误。 |
| **P2-3** | `internal/safego/safego.go:16-25`、job `scheduler.go:247,603`、etcd/nacos keepalive loop | `recover()` 无差别吞掉所有 panic（含 `runtime.Error`：nil deref、slice OOB）后仅 `fmt.Printf` 即返回，goroutine 永久退出，但 `m.running` / 注册状态仍为 true → 定时任务/服务发现静默假死，健康检查可能仍报健康。非崩溃但可用性风险，且 stdout 污染结构化输出。 | 区分业务 panic 与 `runtime.Error`：后者记录后应重启循环/触发健康检查失败/进程退出；关键循环加 supervision；日志走 `logs` 而非 `fmt.Printf`。 |
| **P2-4** | `pkg/job/scheduler.go:312-323` → `pkg/job/store.go:36-79` | `StoreWithError` 持 `m.mu.Lock()` 调 `newStore` → `db.Mysql.GetConnection()`（DB 网络 I/O）。本轮将原 `RLock` 升级为 `Lock`，放大阻塞面：DB 抖动时 `tick`/`refreshLoop`/`Trigger`/`Check` 全线阻塞。与上轮 7fb2617「锁外 I/O」意图相悖。 | 锁外建连/取 store，锁内只做发布 + 二次检查（RCU 范式，参照 `redis.go:48-94`）；补 DB 阻塞不持锁的时序测试。 |

---

## 4. P3：观察项、假绿与降级

| # | 项 | 裁定 |
|---|---|---|
| P3-1 | `pkg/cache/memcache.go:25,39` `clone.Clone(v)` 理论 panic（go-clone 对私有字段/嵌套类型有 panic 分支） | **降级**：扩展探针 5 种类型（私有字段 map+Mutex、time.Time、嵌套 interface、func、uintptr）全部 NO-PANIC；go-clone v1.4.1 对 chan/func/私有字段均有处理，实际触发面极窄，且调用发生在调用者 goroutine 无 recover。列为观察，不作本轮必修。 |
| P3-2 | `app.go:191,207` postlog `RequestLogger()` 注册在 `nice.Recovery()` 之前 | 中间件链中 `c.Next()` 之后的 postlog 逻辑执行时 Recovery 已出栈，若该段 panic 框架兜不住。当前未定位必然触发行（Config 启动已初始化，非 nil），列结构性风险。 |
| P3-3 | `pkg/logsink/sink_test.go:207,221` 假绿 | `WithQueueSize(4096)` 被 `normalize()` 钳到 1024，1000 条永不触发淘汰，`h.len()+dropped==1000` 恒真。FIFO 用例（`:234-259`）已修为 100/1024，属部分闭环。 |
| P3-4 | `app_shutdown_test.go:22-27` 假绿 | `TestWaitForHealthChecksReturnsWhenDone` 中 `wg` 未 `Add(1)` 即断言立即返回 true，未测「有 pending 任务、最终完成」的等待路径。 |
| P3-5 | `pkg/casbin/casbin.go:201`、`pkg/i18n/store.go:310,320` config nil 告警 | **降级**：`config.Config = &config{}`（`configure.go:202`）为启动即初始化的非 nil 全局指针，正常路径不会为 nil；中间件层已有 nil 守卫。仅测试显式置 nil 时可达，非生产风险。 |
| P3-6 | 工作区 4 个未提交测试文件 | 均为「修复后补测」性质（mongo tags 保留、redis 订阅取消/重连、plugin 缓存元信息、DAO fluent 副本），未随 3 个提交入仓，违反上轮 §7「回归用例与修复同 PR」约束；且 `plugin_panic_test.go:65` 断言 `InitAll`「返回 error」与上轮 §7「只许不 panic」口径冲突。属流程债，非生产 panic。 |

---

## 5. 代码审查双轴（`code-review` 技能）

固定点 `4f1d9de...HEAD`，3 个提交 + 工作区未提交测试改动。规范轴以 `docs/coding-standards.md` 为准，规格轴以 `13-pkg-full-audit.md` 为规格来源；两轴独立执行、不合并排序。

### Standards（规范轴）

- **成文规范违反（硬，格式级）**：§1.2 中文注释——`internal/safego/safego.go:1,11`、`app.go:416` 为英文注释，`pkg/utils/limit.go:65` `StopLimitQueueCleaner()` 导出无注释；§4.1 日志占位符——`logger.go:192` `logs.Error("io.ReadAll error:", err)` 无 `{}` 却传参，`redis.go:491` `"订阅错误:"+err.Error()` 字符串拼接；§3.1.3 新增英文错误串大写开头（`mgo.go:47,193,198,203,258`、`redis.go:50,243`、`sqlite.go:44,52`、`mysql.go:432`、`xss.go:497`）。
- **Smell（判断性）**：`claimID` 六个数值分支重复、`safeXxx` 同形 defer/recover（Duplicated Code）；`casbinService.Once` 死字段（Speculative Generality）；`GetEnforcer`（`casbin.go:185-219`）锁内做 DB 连接 + 模型文件 IO + `LoadPolicy`（违反 §5 锁内 I/O）；`waitForHealthChecks` 超时后 `wg.Wait()` 协程悬挂（Middle Man / 资源残留）。
- **合规项**：未新增非测试 `panic`；裸 `go` 全仓已改 `GoSafe`（Grep 确认唯一 `go func` 在 `safego.go:16`）；日志占位符主体正确；`-race` 全绿。按规范抑制对应 smell。

### Spec（规格轴）

- **缺失/部分**：§8 要求分四个 PR，实际 625f83c 单提交混 P0/P1/P2/外围、feb0481 全仓协程重构、39c554e 再混修复+测试；§7 回归用例需与修复同 PR，「修复前必红」输出未见；P1-1 未抽 `shutdown(ctx)` seam（`app.go:406` 仅 helper）；P1-5 `structutil.go:116` 非 string 键直接返空 map，未完全实现「统一遍历任意 reflect.Map」；i18n 测试 12 类型缺 `int`。
- **范围外行为**：上轮 §4 标「下轮排期」的外围项被提前修（`client.go:214`、`xss.go:497`、`rsautil.go:271`、`downloadutil.go:25`、`vo/response.go:47`、`models/result.go:42`、`safemap.go:29`、`registry/cache.go:184`）；feb0481 全仓 `go` → `GoSafe` 改变 panic 语义，不在 §2-4；`mgo.go:198` 新增 URI 改写语义（上轮 X8 维持 P3 未要求）。
- **看似实现实际错误**：`app_shutdown_test.go` 不触发 SIGTERM×checkAll 唯一窗口，删回 `checkDone=nil` 或前移 `Wait()` 仍绿；`plugin_panic_test.go:65` 断言「返回 error」与上轮 §7 口径冲突；`casbin_panic_test.go:53` 只覆盖 nil config，未覆盖 P1-3 的 nil Enforcer 503 路径。
- **被动态复核推翻**：规格轴曾隐示 P1-1 仍有重入风险——确实存在（见 P2-2），但正常单 Run 路径已闭环，定级由 P1 降为 P2。

---

## 6. 测试 seam 与假绿清单

| 测试 | 状态 |
|---|---|
| `pkg/otel/otel_test.go:42-53` | **真绿**：交叉 Store 异构 Provider，旧实现必 panic；但 typed-nil Provider 未覆盖（P2-1） |
| `pkg/i18n/format_panic_test.go:9` | 覆盖 12 种数值 + 5 种其他类型，仅缺 `int`；核心路径真绿 |
| `pkg/middleware/casbin/casbin_panic_test.go` | 空 Uri / claimID / nil config 已覆盖；**未覆盖 nil Enforcer 503**（P1-3 部分路径） |
| `pkg/middleware/postlog/logger_test.go:75` | 畸形 gzip 已覆盖，真绿 |
| `pkg/utils/structutil_panic_test.go` | 只测 `AnyToMap`，同文件 4 个函数裸奔（P1-2 假绿） |
| `pkg/utils/panic_edges_test.go:55,104` | 只测 `IntArrayDelete` 与 `getPubKey/getPriKey`，`DeleteArray` 与 RSA 签名路径裸奔（P1-1/P1-3 假绿） |
| `pkg/logsink/sink_test.go:207,221` | 队列钳位导致淘汰用例恒真（P3-3） |
| `app_shutdown_test.go:22-27` | wg 未 Add，未测等待路径（P3-4） |
| 缺正确 seam | typed-nil Provider 需嵌入 noop.TracerProvider 构造；`app.Run()` 二次调用需进程级 or 抽 `shutdown(ctx)`；safego 吞 `runtime.Error` 需可观测 supervision 探针 |

**硬性约束（沿用上轮 §7）**：X 系列回归用例必须与修复进同一 PR，附「用例在修复前 commit 上必红」的实测输出；断言只能写「不 panic + 不污染正常行为」，`recover` 只能转 `t.Fatalf`，不能断言 `recovered != nil` 或「返回 error」（当前 `plugin_panic_test.go:65` 违此约束）。

---

## 7. 修复顺序建议

1. **第一个 PR（3 项 P1，各 1–3 行 + 测试，小改动高收益）**：`rsautil` 六个签名/验签方法补 nil 密钥守卫 + 抽公共 helper；`structutil` 四个函数补 nil 入参；`sliceutil.DeleteArray` 补边界。每个补表驱动回归测试，与修复同 PR，附「修复前必红」输出。**三个探针已证明修复前必红**。
2. **第二个 PR（P2-1/P2-2）**：otel typed-nil 检测 + `Tracer()` nil 兜底；app `closeOnce` 或 `Run()` 重入守卫。附 typed-nil 与二次 Run 回归用例。
3. **第三个 PR（P2-3/P2-4）**：safego 区分 `runtime.Error` 与业务 panic，关键循环加 supervision + 日志改走 `logs`；job `StoreWithError` 改锁外建连 + 锁内发布的 RCU 范式。
4. **测试债清理（随 PR）**：修 `logsink` 队列钳位假绿、`app_shutdown_test` wg.Add、i18n 补 `int`、casbin 补 nil Enforcer 503、把工作区 4 个未提交测试归入对应修复 PR 并统一断言口径。
5. **规范清理（可随 PR）**：补 4 处英文注释、2 处日志占位符、10+ 处错误串小写开头；删除 `casbinService.Once` 死字段。

---

## 8. 交付物与最终状态

| 文件 | 内容 |
|---|---|
| `14-pkg-full-audit-r4.md` | 本报告（第四轮终稿汇总） |
| `13-pkg-full-audit.md` 及 `01`–`12` | 前三轮审计报告（未改动） |

**最终状态**：`HEAD=39c554e`；`git status --short` 为 ` M .gitignore` + 4 个进入任务前既有的测试文件改动（未触碰）；`find . -name 'zz_r4_*'` 为空；`go vet` / `go test` / `go test -race`（排除两个环境超时包）三条基线均 exit=0。全程未修改生产代码、未 commit。

（报告完）
