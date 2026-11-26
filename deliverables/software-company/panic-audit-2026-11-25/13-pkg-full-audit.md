# 全量 `pkg/` 代码 panic 风险复审报告（第三轮 · 修复后）

- 复审日期：2026-11-25
- 仓库 / 分支 / HEAD：`/home/macro/Work/go/src/github.com/maczh/mgin`，`v2`，`4f1d9de`
- 上轮复审基线：`aaae6a3`（其后 9 个提交，14 文件，+331/−87）
- 复审范围：`pkg/` 全量 148 个源文件（约 21,680 行）+ `app.go` / `mgin.go` 的生命周期装配；含全部 38 个既有测试文件
- 复审方法：两条架构师静态评审线（核心高风险区 / 外围包并行通读）→ 工程师对抗性动态复现 → QA 完全独立的探针复核与定级 → `code-review` 技能规范轴 / 规格轴双审查
- 纪律：**未修改任何生产代码、未 commit**；所有临时探针跑完即删；最终 `git status --short` 仅剩进入任务前既有的 ` M .gitignore`

---

## 0. 结论摘要

### 0.1 三个必答问题

| 问题 | 结论 |
|---|---|
| 上一轮的 P0 是否继续闭环？ | **是。** 上轮 P0（Redis / Mongo 无锁共享 map 触发 `fatal error: concurrent map read and map write`）仍闭环：全部 map 读写点都在 `mu` 内；`mgoCheck` 中唯一残留的裸写已是**无调用者死代码**。 |
| 本轮是否有新的 P0？ | **有，且只有一条：`pkg/otel/otel.go:31,38,50,58` 的 `atomic.Value` 异构 Store。** 按包注释推荐路径注入任意真实 OTel SDK Provider 必现 `sync/atomic: store of inconsistently typed value into Value`，发生在初始化/注入点、无 recover，等价于进程启动即崩。工程师与 QA 各自独立动态证实。 |
| 上轮 P1/P2 是否都修完？ | **大部分闭环，三处未闭环**：① `app.go` 新增关停等待引入**新的 P1 死锁**；② Redis/Mongo `Init` 仍持双锁做网络 I/O（X1 只修了读路径）；③ Redis 订阅泄漏与 `InitWithError` 关闭后假成功仍在。 |

### 0.2 最终风险分布（以 QA 独立定级为准）

| 级别 | 数量 | 内容 |
|---|---|---|
| **P0** | 1 | OTel `atomic.Value` 异构 Store |
| **P1** | 5 | 关停 nil-channel 死锁、i18n 数值断言、casbin 中间件裸断言/nil Enforcer、postlog gzip nil Reader、`AnyToMap` 异构 map |
| **P2** | 7（核心表）＋ 外围若干待排期 | Init 锁内 I/O、Init-after-Close 假成功、订阅泄漏、Wait 无超时、errcode/job/memcache 竞态；另有 client/xss/nacos/vo 等静态确认项 |
| **P3** | 4（核心表）＋ 外围 11 条 | 初始化 panic（**按仓库规范合规**）、Register 静默拒绝语义、mgoCheck 死代码、logsink 测试断言错误等 |

**总体判断**：本轮修复把上轮遗留的 X2/X3/X5/X5b/X6/X10/X12/X13/X14 真正闭环，`plugin` 的「注册期缓存 name/order + 六个 `safe*` 覆盖全部接口方法」是最干净的一处修复；但**审计视野之外的 `pkg/otel` 藏着一个必现 P0**，且新加的关停等待代码引入了一条条件性进程级死锁。两者都是小改动、高收益，应最优先处理。

### 0.3 基线验证（必须看退出码与完整输出，不能 grep 计数）

| 命令 | 结果 |
|---|---|
| `go test -count=1 ./pkg/...` | 卡住：`pkg/config` 120s 超时、`pkg/registry/etcd` 45s 超时，均为 gRPC 经代理拨号的环境问题，非生产代码 panic |
| 排除上述两包后 `go test -count=1 -timeout 120s $PKGS` | **exit=1**：`pkg/logsink/TestQueueKeepsNewestAndFIFO` FAIL；QA 单独跑与全包跑同错，根因是测试期望 `{100,1}` 与 `normalize()` 实际上限 `1024` 不一致，属**测试断言错误**，非顺序污染、非生产缺陷 |
| `go vet ./pkg/...` | exit=0 |
| 排除上述两包后 `go test -race -count=1 $PKGS` | **exit=1**：仅上述 config 超时与 logsink FAIL，**无 DATA RACE 告警** |
| 目标包收尾复验 `go test -count=1 ./pkg/db/... ./pkg/plugin/... ./pkg/otel/... ./pkg/i18n/... ./pkg/utils/...` | 全 ok |

---

## 1. 上轮风险逐条回归

来源：核心架构师报告 `07-architect-core-review.md` §1，QA 独立复核后未推翻。

| 编号 | 上轮问题 | 本轮判定 | 当前证据 |
|---|---|---|---|
| 上轮 P0 | Redis/Mongo 无锁共享 map → fatal | ✅ 闭环 | `redis.go:19-32`、`mgo.go:17-29` 的 map 读写全部入 `mu`；`mgoCheck`（含 `m.conns[tag]=`）无调用者 |
| X1 | 锁内网络 I/O | 🟡 部分闭环 | 读路径已修（Check/GetConnection 快照后解锁再 Ping）；**写路径未修**：`redis.go:43-46/185`、`mgo.go:44-47/103/124` 仍在双锁内 Ping/Dial |
| X2 | Mongo 关闭后复活 | ✅ 闭环 | `mgo.go:149-156` 置 `closed` + 清 `confData`；三处 `ErrClosed` 早退；`needsInit` 恒 false |
| X3 | Mongo 写锁 + 每次 Ping | ✅ 闭环 | `mgo.go:256` 改 `RLock`，`:276/:289` 仅 `Copy()`，`mgoCheck` 移出调用链 |
| X4 | 冗余 Init 擦除 tags | ✅ 闭环 | `mgo.go:62-63` 移入 `len(conns)==0` 块内 |
| X5 / X5b / X6 | `Order()`/`Name()` 元信息 panic 与 recover 内二次 panic | ✅ 闭环 | `plugin.go:56-72` 注册期 `safeName/safeOrder` + 缓存；运行期只读缓存；`safeInit/safeClose/safeHealth` recover 用形参 name |
| X7 | Redis 无 closed / 假阳性 / 订阅泄漏 | 🟡 部分闭环 | closed 与 Check 假阳性已修；**PSubscribe 仍用 `Background()`，Close 无 cancel，泄漏仍在** |
| X9-1 | `WithTx` nil receiver | ✅ 闭环 | `dao/mysql.go:61-63` |
| X10 | SQLite 零值 `*gorm.DB` | ✅ 闭环 | `sqlite.go:24-33` `validateSqlite` 判 `Config`/`Statement`/`Error`，三入口接入 |
| X12 | MgoDao 改写调用方 slice | ✅ 闭环 | `mgo.go:265-270` 用局部 `tag` |
| X13 | `Enabled()` panic 静默漏关 | ✅ 闭环 | `plugin.go:180-189` 出错仍 Close |
| X14 | `RedisCache.Range(nil)` | ✅ 闭环 | `dao/redis_cache.go:80` 判 `f==nil`，并补齐 nil receiver 守护 |
| X8 / X11 / X15 | `{db}` 模板、注释残骸、ctx 装饰性 | ❌ 维持 P3 | 当前不可达或仅行为问题 |
| X16 | 三源生命周期契约不一致 | 🟡 部分 | 三源均有 `InitWithError`，但 Redis/Mongo 关闭后返回 nil 假成功 |

**统计**：完全闭环 13 项，部分闭环 3 项（X1/X7/X16），维持 P3 3 项。

---

## 2. P0：本轮必修

### P0-1 `pkg/otel/otel.go:31,38,50,58` — `atomic.Value` 存储异构具体类型

- **触发链**：包 `init()` 存入 `noop.TracerProvider`（值类型）→ 业务按包注释引入 OTel SDK 后调用 `SetTracerProvider(sdktraceProvider)`（指针类型）→ `atomic.Value.Store` 立即 panic。
- **证据**：工程师探针输出 `concrete type before: noop.TracerProvider` + `PANIC CAUGHT: sync/atomic: store of inconsistently typed value into Value`；QA 用嵌入 `trace.TracerProvider` 的异构类型独立复现同一文本。
- **为何测试假绿**：`pkg/otel/otel_test.go:27` 的 `TestSetThenDisable` 传入的正是 `noop.NewTracerProvider()`，与 init 中存储的动态类型**完全相同**，恰好绕开唯一会出问题的路径。
- **后果**：初始化/注入点无 recover，进程启动即崩；`Disable()` 的异构分支被前置 panic 遮蔽，实际不可达。
- **修复**：改 `atomic.Pointer[trace.TracerProvider]`（Go 1.19+），或所有 Store 统一存固定包装结构体 `struct{ tp trace.TracerProvider }`；`Tracer()` 的断言同步改为安全读取；补「异构 Provider 交叉 Store」回归用例。

---

## 3. P1：本轮必修/强烈建议

| # | 位置 | 触发与证据 | 修复要点 |
|---|---|---|---|
| **P1-1** | `app.go:118-130,384-391` | `close(checkDone); checkDone=nil; checkWG.Wait()`：SIGTERM 撞上 `checkAll` 执行窗口时，goroutine 下一轮 select 读到 nil channel，分支永久禁用，`Wait()` 永不返回 → `Shutdown` 与 `SafeExit` 全跳过。工程师复刻探针：nil 版 1s 超时未返回、对照版 544ms 返回；QA 独立探针确认 `checkAll` 结束后仍阻塞，`-race` 还捕获字段读写竞争。**条件性但后果不可逆**。 | 删除 `checkDone = nil`（或 `sync.Once` + 局部 channel 快照）；`Wait()` 加超时兜底，并移到 `Shutdown` **之后**（先排空 HTTP 再等健康检查），避免坏 `Health()` 阻塞排空。建议把关停序列抽成可测的 `shutdown(ctx)` seam。 |
| **P1-2** | `pkg/i18n/i18n.go:107-110` | `Format` 的多类型 case 配单一断言。工程师穷举 16 种类型：**11 种 panic**（int8/16/32/64、uint/8/16/32/64/uintptr、float32），仅 `int` 与 `float64` 正常。它是 `ErrorDef`/`NoRoute`/`Recovery` 的底层，业务传 `int64` ID 极常见。 | 拆分 case 各自 `FormatInt/FormatUint/FormatFloat`；禁止一个 case 组配单一断言；补 13 种数值类型表驱动测试。 |
| **P1-3** | `pkg/middleware/casbin/casbin.go:34-45` | `claims.(jwt.MapClaims)["userId"].(float64)` 两重裸断言 + `GetEnforcer()` nil 链式调用；工程师与 QA 均实测三种输入三种 panic。**假绿陷阱**：默认 `Swagger.Uri` 为空串时 `strings.Contains(path,"")` 恒真，中间件提前放行，探针必须显式设非空 Uri。 | 逐层 comma-ok；`GetEnforcer()` nil 时返回 503/500；空 Uri 加守卫。 |
| **P1-4** | `pkg/middleware/postlog/logger.go:184-190` | `gzip.NewReader` 失败仅记日志不 return，随后 `defer r.Close()` / `io.ReadAll(r)` 对 nil Reader 解引用；后端返回畸形 gzip（网关错误页、空 body）即触发。QA 指出：panic 位于 `c.Next()` 之后，`nice.Recovery` 已出栈，**框架层兜不住**。 | err 分支立即 return/跳过解压，绝不继续使用 `r`；补 gin+httptest 畸形响应测试。 |
| **P1-5** | `pkg/utils/structutil.go:102-127` | `AnyToMap` 中 `reflect.ValueOf(v).Type()` 对 nil 值 panic、`obj.(map[string]string)` 对异构 map panic；被 `pkg/client/client.go` 的 Header/Data/Query 六处调用。工程师实测 nil 值、`map[string]int` 两种文本；QA 补测 `map[string]bool`/`map[int]string` 等 3/3 panic。 | 用反射统一遍历任意 `reflect.Map`；nil value 显式映射；去掉硬断言。补表驱动测试。 |

---

## 4. P2：待排期（核心 7 条）

| # | 位置 | 问题 | 可信度 |
|---|---|---|---|
| P2-1 | `redis.go:43-46,185`、`mgo.go:44-47,103/124` | Init 仍持 `initMu→mu` 双锁 Ping/Dial；工程师实测 `GetConnection` 被阻塞 **2.8022s**（与架构师 2.8023s 吻合）。Redis `redis_pool.max>1` 时 DialTimeout 缺省 60s 且逐库串行。 | 工程师动态证实（Redis）/ 静态确认（Mongo） |
| P2-2 | `redis.go`/`mgo.go` 的 `InitWithError` | `Close()` 后再 Init 静默 return、`initErr` 保持 nil → 返回假成功；Redis/Mongo 均动态证实 `err=<nil>`，与 MySQL `ErrClosed` 不一致。 | 动态证实 |
| P2-3 | `redis.go:363-365,432` | `PSubscribe` 恒用 `Background()`，`Close` 无 cancel；`ctx.Err()!=nil` 分支直接 return 漏关 pubsub；`Receive()` 无超时。Close 后 goroutine 不回落并每秒刷错误日志。 | 静态确认 + 上轮动态证据 |
| P2-4 | `app.go:391` | `checkWG.Wait()` 无超时且排在 `Shutdown` 之前；坏 `Health()`（mgo Ping 无框架超时）可拖死关停。与 P1-1 同批修。 | 静态确认 |
| P2-5 | `pkg/errcode/definition.go:79-81` | `RegisterDef` 无锁写全局 map；QA 用 8 goroutine × 200 次并发 `-race` 报 DATA RACE。运行期调用可升级为不可恢复 `fatal: concurrent map read and map write`，当前文档限定 init 期调用故保持 P2。 | 动态证实 |
| P2-6 | `pkg/job/scheduler.go:301-308` | `Manager.Store()` 在 RLock 内写 `m.st`；并发 `-race` 证实读/写、写/写冲突。 | 动态证实 |
| P2-7 | `pkg/cache/memcache.go:63` | `Clear()`/`Close()` 对 `sync.Map` 整体赋值（复制内含锁）；并发 `-race` 报 DATA RACE，工程师还观测到 `fatal error: sync: unlock of unlocked mutex`。 | 动态证实 |

**外围审计另登记但 QA 最终表未列为核心的 P2**（均为静态确认，建议下轮排期）：`client` 未知 Protocol 时 `resp.String()` nil 解引用（工程师动态证实）、`xss.go:494-505` JSON 数组元素裸断言、`nacos.go:178-188` Register 未检查 `resp==nil`、`vo/response.go` 与 `models/result.go` 多处未检查断言、`downloadutil.go:23-30` 无扩展名时 slice 越界、`utils/safemap.go` 无锁/读锁写 map（无调用点，修复后须 `-race` 回归）、`utils/safego.go` nil `goBeforeF` 同步 panic 等。完整清单见 `08-architect-periphery-review.md` §3 与 §4。

---

## 5. P3：观察项

| # | 问题 | 裁定 |
|---|---|---|
| P3-1 | `app.go:144` i18n 初始化失败 `panic(err)` | **按 `docs/coding-standards.md §3.1 第 4 条**，初始化不可恢复时 panic 是被明确允许的（示例即 VictoriaLogs），**合规，不要求整改**。 |
| P3-2 | `plugin.Register` 静默拒绝 `Name()`/`Order()` panic 的插件 | 合法插件未被误拒（动态证实）；但只记日志不返回 error，业务侧可见性弱。 |
| P3-3 | `mgoCheck` 死代码 | 唯一残留 unlocked map 写，当前无调用者不可达；下次触碰 mgo.go 时删除。 |
| P3-4 | `pkg/logsink/sink_test.go:236` 测试断言错误 | 期望 `{100,1}` 与 `normalize()` 上限 `1024` 不符，单跑/全包跑同错；修测试断言即可，不属于生产 panic。 |

外围报告另列 11 条防御性/无调用点项（trace 栈头切片、`http.DefaultTransport` 断言、RingBuffer 负长度、sliceutil 边界等），见 `08-architect-periphery-review.md` §4，本轮不排期。

---

## 6. 代码审查双轴（`code-review` 技能）

固定点 `aaae6a3...HEAD`，提交 9 个。规范轴以 `docs/coding-standards.md` 为准，规格轴以 `06-qa-reaudit.md` / `04-architect-reaudit.md` / `07-remediation-plan-reaudit.md` 为规格来源；两轴独立执行、不合并排序。

### Standards（规范轴）

- **成文规范违反（硬，格式级）**：§1.2 导出符号注释缺失——`mgo.go:38 ErrClosed`、`:139 InitWithError`，`redis.go:37 ErrClosed`、`:198 InitWithError`，`sqlite.go:39 InitWithError`；§3.1.1 `sqlite.go:36` 的 `_ = m.InitWithError(...)` 未注明忽略原因；§3.1.3 新增英文错误串首字母大写（`"Mongodb is nil"` 等）。
- **Smell（判断性）**：五个 `safeXxx` 同形 defer/recover、`firstErr==nil` 赋值重复约 6 处；`mgo.go`/`redis.go` 的 `closed→ErrClosed` 与 needsInit/confData 拷贝模板重复 4 处；`redis_cache.go` 8 处 nil 守卫。`sqlite.Init` 的 Middle Man 属兼容旧签名的可接受取舍。
- **合规项**：Ping 移出读路径锁、`ticker.Stop+done+WaitGroup` 齐备、日志占位符 `{}` 正确、删不可达 return 清理 vet 告警——按规范抑制对应 smell，`gofmt -l` 改动文件无输出。

### Spec（规格轴）

- **缺失/部分实现**：X1 的 RCU 重构只完成读路径；X2 的「锁外关旧 session」未做；X7 订阅取消与 `Receive` 超时未做；`mgin.go:124-125` VictoriaLogs `panic(err)` 未改（但 QA 按仓库规范裁定合规）；X8/X11 未修；`initErr` 在 multi 模式 Dial 失败仅 `continue` 导致假成功；07 回归清单 16 项仅落地 4 项。
- **范围外行为**：`4f1d9de` 修改 xss/rsautil 与 panic 审计无关（经确认仅删不可达语句，无语义变化）；`29e2a28` 混入了 DAO/缓存/SQLite 修复，批次边界不清。
- **看似实现实际错误**：`5c17214` 是纯 test 提交，违反「X 系列回归用例必须与修复同 PR」的硬约束；X3 以删除 `mgoCheck` 调用代替「移出锁 + 节流」，失效连接失去自愈；`app.go:391 checkWG.Wait()` 排在 `Shutdown` 之前，顺序与规格相反。
- **被实测推翻的一条**：规格轴称 `plugin_panic_test.go:23` 的 `Register` 调用会「杀掉整包」——工程师与 QA 均实测 `go test -count=1 -v ./pkg/plugin/` exit=0；`Register` 内部 `safeName` 已 recover，结论不成立。

---

## 7. 测试 seam 与假绿

| 测试 | 状态 |
|---|---|
| `pkg/otel/otel_test.go:27` | **假绿**：只存同类型 noop，绕开 P0-1 唯一触发路径；必须补异构 Provider 交叉 Store。 |
| `pkg/i18n/store_test.go:178` | 只测 string 参数，11 种数值 panic 全裸。 |
| `pkg/casbin/` | 无中间件测试；探针必须显式设非空 `Swagger.Uri`，否则空串放行制造新假绿。 |
| `pkg/middleware/postlog/logger_test.go` | 只测正常投递，未测畸形 gzip；需完整 gin+httptest seam。 |
| `pkg/db/sqlite/sqlite_lifecycle_test.go` | 未覆盖 `&gorm.DB{}` 半初始化关键判据；若 `validateSqlite` 退化成只判 `conn==nil` 仍会绿。 |
| `pkg/plugin/builtins_test.go` + `plugin_panic_test.go` | 覆盖注册期拒绝，未覆盖 X6 recover 分支与 X13。 |
| 缺正确 seam | 真实 `mgo.Session`（字段不导出、唯一构造入口需握手成功）：Init 锁内 Dial 阻塞读路径只能静态确认；真实 OTel SDK Provider：需嵌入接口构造异构类型；`app.Run()` 关停逻辑：建议抽 `shutdown(ctx)` 可测 seam。 |

**硬性约束**：X 系列回归用例不得单独入仓，必须与修复进同一 PR，并在 PR 描述附「用例在修复前 commit 上必红」的实测输出；断言只能写「不 panic + 不污染正常插件」，`recover` 只能转 `t.Fatalf`，不能断言 `recovered != nil` 或「返回 error」。

---

## 8. 修复顺序建议

1. **第一个 PR（P0 + P1-1，小改动）**：`otel` 改 `atomic.Pointer[trace.TracerProvider]` 或统一 holder；`app.go` 删除 `checkDone=nil`、给 `Wait()` 加超时并移到 `Shutdown` 之后。两处合计约 10 行，消除唯一的启动必崩与唯一的条件性关停死锁。
2. **第二个 PR（P1-2~5）**：i18n 数值 case 拆分、casbin 逐层 comma-ok + Enforcer nil、postlog gzip 错误分支 return、`AnyToMap` 反射遍历。每处都补对应回归用例，与修复同 PR。
3. **第三个 PR（P2-1~4）**：Redis/Mongo `Init` 按 `mysql.go:55-103` 的 RCU 范式重构（锁外建连 → 锁内发布 → 锁外关旧）；`InitWithError` 关闭后返回 `ErrClosed`；PSubscribe 内部 ctx + `ReceiveTimeout` + 退出路径统一 `pubsub.Close()`。
4. **第四个 PR（P2-5~7 + 测试债）**：errcode 读写加锁、job `Store()` 改 `Lock()`、memcache 用 `Range+Delete` 清理；修 `logsink/sink_test.go:236` 断言使排除网络超时包后的 `go test ./pkg/...` 恢复全绿。
5. **规范清理（可随 PR）**：补 5 个导出符号注释、`_ = m.InitWithError(...)` 加说明、统一新增错误串小写开头。

---

## 9. 交付物与最终状态

| 文件 | 内容 |
|---|---|
| `13-pkg-full-audit.md` | 本报告（终稿汇总） |
| `12-qa-final.md` | QA 独立复核、最终定级、测试 seam（23 个自建探针） |
| `11-engineer-repro.md` | 工程师 11 个探针的逐条命令、输出与判定 |
| `10-review-spec.md` / `09-review-standards.md` | 代码审查规格轴 / 规范轴 |
| `08-architect-periphery-review.md` | 外围包逐文件通读（P0-1、P1-1~4、P2-1~11、P3-1~11） |
| `07-architect-core-review.md` | 核心高风险区通读 + 上轮 X 系列逐条回归 |
| `01`–`06` | 前两轮审计报告（未改动） |

**最终状态**：`HEAD=4f1d9de`；`git status --short` 仅 ` M .gitignore`（任务前既有）；`find . -name 'zz_r3_*' -o -name 'zz_qa3_*'` 为空；目标包 `go test` / `go vet` 全绿。全程未修改生产代码、未 commit。

（报告完）
