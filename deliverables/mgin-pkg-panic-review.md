# mgin v2 `pkg/` Panic、并发与生命周期风险复审

- 审核日期：2026-11-25
- 分支：`v2`
- 当前 HEAD：`4f1d9de0fa932af58605b10ec7b3ee5a21d77ecc`
- 上轮报告基准：`aaae6a3f7c0436bfc881b28e16f529840ad3c82c`
- 范围：当前工作树全量 `pkg/**`，并覆盖与插件、数据库、健康检查直接关联的根包 `app.go`、`mgin.go`
- 方式：静态审查、仓库外最小复现探针、`go build`、`go vet`、目标包测试、安全包集 `-race`
- 源码修改：无；所有复现测试与探针均为临时文件，验证后清理

## 1. 结论摘要

上轮针对插件元数据、nil receiver、Mongo Session、tags、空 Redis 健康检查和请求排空顺序的修复总体有效，但数据库生命周期与优雅关闭改造只完成了部分目标。

本轮新确认 **23 项问题：P0 5 项、P1 8 项、P2 10 项**。其中：

- 5 项可直接导致进程 panic、运行时 fatal、鉴权绕过或永久退出阻塞；
- 8 项涉及可复现 panic、错误初始化被吞、连接泄漏或关闭语义不完整；
- 10 项为条件性 panic、并发缺陷、文档/测试不一致等次级风险；
- `pkg/logsink` 的既有队列 FIFO 测试稳定失败，说明当前测试基线并非全绿。

优先级建议：先修 P0，再处理数据库初始化/关闭与 Redis 取消路径，最后补齐公共工具和中间件的边界校验及回归测试。

## 2. 上轮修复核对

| 上轮问题 | 当前结果 | 证据 |
|---|---|---|
| 插件 `Name()/Order()` panic，recover 中再次 panic | 已修复 | `pkg/plugin/plugin.go:44-73,123-137,243-286` 缓存元数据，安全调用与 recover 不再重新执行插件方法 |
| `MySQLDao.WithTx` nil receiver | 已修复 | `pkg/db/dao/mysql.go` 已增加接收者保护 |
| Mongo `ReturnConnection` 空 Session | 已修复 | `pkg/db/mongo/mgo.go:297-304` |
| Mongo tags 被健康检查清空 | 已修复 | tags 只在真正初始化连接集合时重建 |
| Redis 空 map 健康检查假阳性 | 已修复 | `pkg/db/redis/redis.go:258-261` 返回明确错误 |
| Mongo/Redis `Check` 持锁 Ping | 已修复 | 网络操作已移到锁外 |
| 数据库 Close 后复活 | 部分修复 | Mongo/Redis 增加 `closed`，但 `InitWithError` 关闭后仍返回 nil；SQLite 无终止状态 |
| SQLite 初始化错误传播 | 部分修复 | 新增 `InitWithError`，但 `Init` 丢弃错误，关闭路径仍忽略错误 |
| 健康检查 goroutine 与关闭重叠 | 引入回归 | 增加 `WaitGroup`，但 `checkDone=nil` 与在途 `checkAll()` 组合会死锁 |
| 请求未排空前关闭资源 | 已修复 | 资源关闭已移到 HTTP 请求排空之后 |

## 3. 问题清单

严重度定义：

- **P0**：进程崩溃、运行时 fatal、鉴权绕过或永久退出阻塞，应立即处理。
- **P1**：可复现 panic、初始化错误被吞、关键资源泄漏或关闭语义错误。
- **P2**：条件性 panic、并发隐患、兼容性/文档/测试问题。

| ID | 严重度 | 位置 | 问题与影响 | 修复建议 |
|---|---|---|---|---|
| F-01 | P0 | `app.go:117-126,384-391` | 信号到达时若 `checkAll()` 仍在执行，关闭路径先 `close(checkDone)` 再置 nil；在途检查返回后 goroutine 重新 select，ticker 已 Stop、done 为 nil，`checkWG.Wait()` 永久阻塞。已用 1ms ticker overlay 稳定复现。 | goroutine 启动时捕获 `done` 和 `ticker` 的局部值；关闭时只关闭 channel，等 `Wait()` 返回后再置 nil；为 `Wait()` 增加受 `shutdownTimeout` 约束的超时与兜底关闭。 |
| F-02 | P0 | `mgin.go:78,170-193,218-221` | 旧插件 `InitFunc`、后台 `CheckFunc`、`CloseFunc` 直接调用且无 recover；`CheckFunc` panic 会从后台 goroutine 逃逸并终止进程。已复现。 | 为新旧插件生命周期调用统一增加 recover 边界，记录插件名和 panic；Init 错误必须返回/传播，不能继续伪装成功。 |
| F-03 | P0 | `pkg/otel/otel.go:31,38,46-58` | `atomic.Value` 直接保存 `trace.TracerProvider` 接口；默认 noop Provider 与 SDK/包装 Provider 的具体类型不同，切换时触发 `sync/atomic: store of inconsistently typed value into Value`。已复现。 | 将 Provider 包在固定具体类型的结构体中存储，例如 `atomic.Value` 保存 `providerHolder{tp}`；或改用 `atomic.Pointer[providerHolder]`。增加真实 SDK Provider 与 disable 切换测试。 |
| F-04 | P0 | `pkg/utils/safemap.go:50-91` | `LoadAndStore`、`LoadAndDelete` 在 `RLock` 内写 map，`DeleteDirectly`/`Len`/`Map` 无锁，`Clear` 在 `RLock` 内替换 map。8 goroutine 并发写入已触发 `fatal error: concurrent map read and map write`，recover 无法处理。 | 所有写操作使用 `Lock`；`Len`/`Map` 加锁，`Map` 返回副本；`DeleteDirectly` 与公开删除方法统一。补充 `-race` 并发读写测试。 |
| F-05 | P0 | `pkg/middleware/casbin/casbin.go:30` | `strings.Contains(path, Swagger.Uri)` 在 URI 为空时恒为 true，导致 Casbin 中间件直接放行所有请求。已动态确认无 claims 的私有 handler 仍被执行。这是鉴权绕过，不只是 panic。 | 仅当 URI 非空时才做 contains 判断；Swagger/docs 放行使用固定前缀白名单；增加空 URI、普通路径、Swagger 路径的鉴权回归测试。 |
| F-06 | P1 | `pkg/db/mongo/mgo.go:124` | 合法但无数据库路径的 URI（如 `mongodb://127.0.0.1:1`）进入 `mgo.DialWithTimeout` 后，由依赖 `extractDBName` 触发切片越界 panic。已复现完整依赖栈。 | 在 Dial 前用 `net/url` 校验 scheme、host 和数据库路径；无路径时明确使用配置 `go.data.mongodb.db` 或返回配置错误；为无数据库 URI 增加回归测试。 |
| F-07 | P1 | `pkg/client/client.go:191-227`；`pkg/client/resilience.go:184-193` | 未知 `Protocol` 时 switch 不进入任何分支，`resp==nil`、`err==nil`，随后 `resp.String()` panic；`CallResilient` 的 goroutine 无 recover，直接进程崩溃。两种路径均已复现。 | switch 增加 `default` 返回明确错误；调用 `resp.String()` 前检查 nil；后台调用 goroutine 增加 recover，并将 panic 转为 `callResult.err`。 |
| F-08 | P1 | `pkg/utils/ringbuffer.go:12-22,37-38` | 构造函数未设置 `Size`，`NewRingBuffer(1).Latest()` 也会整数除零；`Write` 还会先因索引越界失败。已复现。 | 校验 size 大于 0，并在构造时设置 `Size=int32(size)`；明确空环形缓冲区的读行为；覆盖 size 0、1、满/空读写。 |
| F-09 | P1 | `pkg/db/mongo/mgo.go:91-115,201-238` | 多库所有 Dial 失败时只 `continue`，不设置 `initErr`；结果 `InitWithError=nil`、`connections=[]`、`Check=nil`，健康检查假阳性。已复现。 | 记录首次 Dial 错误；配置中声明了数据库但最终零连接时返回明确错误；`Check` 在零连接时直接失败。 |
| F-10 | P1 | `pkg/db/mongo/mgo.go:48-50,139-146,241-294`；`pkg/db/redis/redis.go:47-50,198-205`；`pkg/db/sqlite/sqlite.go:39-79` | 关闭语义不完整：Mongo/Redis `InitWithError` 在 Close 后返回 nil；`InitWithError` 先调用 `Init` 再单独读 `initErr`，并发 Init 可覆盖结果；SQLite Close 后可重新 Init，且 `conn.Error!=nil` 时可能跳过底层 `DB.Close()`。均已动态确认。 | `InitWithError` 与 `Init` 共用同一把初始化锁并直接返回本次错误；closed 检查放在错误返回路径；SQLite 增加终止状态，关闭时无论 GORM Error 如何都尝试关闭底层 `sql.DB` 并记录错误。 |
| F-11 | P1 | `pkg/db/mongo/mgo.go:44-47,103,124`；`pkg/db/redis/redis.go:43-46,183-194` | Mongo/Redis `Init` 仍持 `initMu` 和 `mu` 执行 Dial/Ping，违反“锁内不做 IO”；Close、健康检查和读取会被最长 10 秒网络等待阻塞。Redis Ping 失败的 client 还未 Close。 | 只在锁内校验 closed、解析配置并发布状态；锁外建立连接；提交结果时再次检查 closed，失败则关闭新连接；Ping 失败分支关闭 `rc`。 |
| F-12 | P1 | `pkg/db/redis/redis.go:367-439` | Pub/Sub 有两条取消泄漏路径：初始 `pubsub.Receive()` 不监听 `ctx.Done()`；handler 执行期间取消 context 后，循环尾部直接 return，却不关闭 pubsub。已用假 RESP Redis 服务确认取消后连接仍打开。 | 握手阶段用 goroutine/独立接收路径响应取消，并统一 `defer pubsub.Close()`；handler 后检查取消时走同一关闭路径；增加握手取消、消息处理中取消、服务端断连三类测试。 |
| F-13 | P1 | `pkg/middleware/casbin/casbin.go:39-40` | claims 直接断言为 `jwt.MapClaims`，并进一步将 `userId`、`roleId` 断言为 `float64`；字段缺失、JSON 解码为 `json.Number` 或 claims 类型不符都会 panic。已复现。 | 使用类型断言 + 数值转换辅助函数，失败返回 401/403；兼容 decoder 配置产生的 `json.Number`；不要将身份解析 panic交给 Gin recovery。 |
| F-14 | P2 | `pkg/middleware/xss/xss.go:452-505` | JSON 顶层数组假定每个元素都是对象，标量元素触发 `n.(map[string]interface{})` panic。请求 `[1]`、`Content-Type: application/json`、显式 `Content-Length: 3` 已稳定复现。 | 数组元素按类型分支处理，非对象元素保留或报 400；不要用强制断言处理外部 JSON。 |
| F-15 | P2 | `pkg/middleware/postlog/logger.go:184-190` | `gzip.NewReader` 失败后仍 `defer r.Close()` 和 `io.ReadAll(r)`，nil reader panic。非法 gzip body 已复现。 | `err != nil` 时记录错误并跳过解压；Close 错误按策略处理；覆盖非法 gzip、空 body、正常 gzip。 |
| F-16 | P2 | `pkg/utils/structutil.go:102-126` | `AnyToMap(nil)` 在 `reflect.ValueOf(nil).Type()` panic；`map[string]int` 落入默认分支后强制转 `map[string]string` panic。均已复现。client 的 Header/Query/Data 调用该函数，属于公共调用链风险。 | 先判 nil；仅支持明确列出的 map 类型，其他类型返回 error/空 map；将反射 Kind 分支与接口断言分支统一。 |
| F-17 | P2 | `pkg/utils/limit.go:30-67` | `count==0` 时第二次调用访问 `q[0]` 越界，已复现。清理 goroutine 无退出路径且无锁写 `LimitQueue.LimitQueue=nil`；业务路径的 nil 检查、初始化、读写不是完整临界区，全局 `ok` 也有竞争。 | count 必须大于 0；用单一 mutex 原子完成查找/更新/清理；为清理 goroutine 增加 context/停止函数，或改为惰性清理；删除全局 `ok`。 |
| F-18 | P2 | `pkg/utils/sliceutil.go:293-298` | `UnSplitString(nil, ",")` 返回空字符串后执行 `dst[:-1]` panic，已复现。其他按索引删除函数同样依赖调用者保证边界。 | 空输入直接返回空字符串；索引删除函数对越界返回原切片或 error，并统一约定。 |
| F-19 | P2 | `pkg/utils/rsautil.go:271,288` | 公钥/私钥解析成功后直接断言为 RSA 类型；合法 EC PKIX/PKCS8 密钥会 panic。两种路径均已复现。 | 使用 `pub.(*rsa.PublicKey)` 的 comma-ok 形式并返回“密钥类型不是 RSA”错误；私钥同理；增加 EC/错误 PEM 用例。 |
| F-20 | P2 | `pkg/vo/response.go:39-52,77-82,163-174,194-195`；`pkg/models/result.go:41-45` | nil Data、nil Page、Data 与目标泛型不一致时，`ToAny`/`ResponseToT`/`models.ToAny` 会 nil deref 或类型断言 panic。四类输入均已复现。 | 对外转换 API 使用 comma-ok 断言，无法转换时返回明确错误或安全零值；禁止将内部断言失败升级为进程 panic；补充 nil 与错类型表驱动测试。 |
| F-21 | P2 | `pkg/logsink/sink.go:119-139,329-338,369-381,403-422` | worker recover 后调用 `notifyError`，但 error/drop logger 自身 panic 无保护，会从 worker 穿透；`Closer.Close` panic 也无外层 recover。已复现 error logger panic 导致进程崩溃。`Register` 还对 typed-nil handler 调 `Name()`，存在条件性 panic。 | 所有外部回调（Handler Name、Write、Closer、drop/error logger）统一使用安全调用包装；recover 分支本身也不得调用未保护回调；typed-nil 使用反射检测。 |
| F-22 | P2 | `pkg/logs/victorialogs.go:71` | 直接 `http.DefaultTransport.(*http.Transport)`，调用方替换为任意 `http.RoundTripper` 时 panic。已复现。 | 类型断言使用 comma-ok；非 `*http.Transport` 时使用 `http.DefaultTransport` 或新建默认 Transport，不修改全局对象。 |
| F-23 | P2 | `pkg/logsink/sink.go:63-77`；`pkg/logsink/sink_test.go:233-237` | `WithQueueSize` 注释声称最大 100，实现允许最大 1024；测试期望 1000 被归一化为 100，当前实现保留 1000，导致 `TestQueueKeepsNewestAndFIFO` 稳定失败。普通、race、`-count=5` 均失败。 | 明确唯一上限并同步注释、常量、实现和测试；不要通过删除断言掩盖行为差异。 |

## 4. 动态复现记录

以下探针均位于仓库外 `/tmp/mgin-pkg-audit/`，通过 `replace` 指向当前工作树，未向仓库写入临时测试。输出中的连接地址均为本地回环或保留测试地址，无真实凭据。

### 4.1 Mongo 无数据库 URI

```text
$ /tmp/mgin-pkg-audit/audit mongo-no-db
mongo URI without database: PANIC: runtime error: slice bounds out of range [1:0]
github.com/maczh/mgo.extractDBName(...)
github.com/maczh/mgo.DialWithTimeout(...)
github.com/maczh/mgin/v2/pkg/db/mongo.(*Mongodb).Init(...)
```

最小输入：

```yaml
go:
  data:
    mongodb:
      uri: mongodb://127.0.0.1:1
      db: test
```

### 4.2 OTel Provider 切换

```text
$ /tmp/mgin-pkg-audit/audit otel
otel provider switch: PANIC: sync/atomic: store of inconsistently typed value into Value
```

探针使用包装后的非 noop 具体类型，只改变 Provider 的具体类型，不改变接口语义。

### 4.3 XSS 顶层数组标量

```text
$ /tmp/mgin-pkg-audit/audit xss
xss JSON array scalar: PANIC: interface conversion: interface {} is json.Number, not map[string]interface{}
```

最小请求：`POST /`，`Content-Type: application/json`，`Content-Length: 3`，body 为 `[1]`。

### 4.4 postlog 非法 gzip

```text
$ /tmp/mgin-pkg-audit/audit postlog
postlog invalid gzip: PANIC: runtime error: invalid memory address or nil pointer dereference
```

最小响应：`Content-Encoding: gzip`、`Content-Type: application/json`、body 为 `not-gzip`。

### 4.5 client 未知协议与 goroutine panic

```text
$ /tmp/mgin-pkg-audit/audit client
client unknown protocol: PANIC: runtime error: invalid memory address or nil pointer dereference
```

`CallResilient` 使用相同未知协议时，panic 从 `resilience.go` 启动的 goroutine 逃逸，栈顶为 `grequests.(*Response).String` → `client.callInternal`。

### 4.6 SafeMap 并发 fatal

```text
$ /tmp/mgin-pkg-audit/audit safemap
fatal error: concurrent map read and map write
github.com/maczh/mgin/v2/pkg/utils.(*Map[...]).LoadAndStore(...)
```

探针使用 8 个 goroutine，每个执行 100,000 次 `LoadAndStore`。该错误不是可 recover 的 panic。

### 4.7 工具函数边界

```text
$ /tmp/mgin-pkg-audit/audit utils
RingBuffer size one Latest: PANIC: runtime error: integer divide by zero
UnSplitString empty: PANIC: runtime error: slice bounds out of range [:-1]
```

```text
$ /tmp/mgin-pkg-audit/audit limit
LimitFreqSingle count zero second call: PANIC: runtime error: index out of range [0] with length 0
```

### 4.8 Casbin 放行与 claims panic

```text
$ /tmp/mgin-pkg-audit/audit casbin-bypass
handler reached without claims: true
```

```text
$ /tmp/mgin-pkg-audit/audit casbin-fields
casbin claims missing fields: PANIC: interface conversion: interface {} is nil, not float64
```

### 4.9 RSA 非 RSA 密钥

```text
$ /tmp/mgin-pkg-audit/audit rsa
RSA public key with EC key: PANIC: interface conversion: interface {} is *ecdsa.PublicKey, not *rsa.PublicKey
RSA private key with EC key: PANIC: interface conversion: interface {} is *ecdsa.PrivateKey, not *rsa.PrivateKey
```

### 4.10 vo/models 条件性转换

```text
$ /tmp/mgin-pkg-audit/audit vo
ResponsePage nil Data ToAny: PANIC: runtime error: invalid memory address or nil pointer dereference
SuccessPage nil page: PANIC: runtime error: invalid memory address or nil pointer dereference
ResponseToT nil Data: PANIC: interface conversion: interface {} is nil, not int
models.ToAny mismatched Data: PANIC: interface conversion: interface {} is string, not int
```

### 4.11 logsink 回调二次 panic

```text
$ /tmp/mgin-pkg-audit/audit logsink
panic: write panic [recovered]
    panic: error logger panic
```

`Write` 的 panic 已被 worker recover，但 recover 分支调用 error logger 后再次 panic，最终逃逸出 goroutine。

### 4.12 VictoriaLogs 自定义 Transport

```text
$ /tmp/mgin-pkg-audit/audit victorialogs
VictoriaLogs custom DefaultTransport: PANIC: interface conversion: http.RoundTripper is main.roundTripperFunc, not *http.Transport
```

### 4.13 Redis Pub/Sub 取消泄漏

```text
$ /tmp/mgin-pkg-audit/audit redis-pubsub
subscription still open 300ms after handler cancellation
```

```text
$ /tmp/mgin-pkg-audit/audit redis-handshake
subscription handshake still open 300ms after cancellation
```

探针使用本地假 RESP Redis 服务，不连接真实 Redis。

### 4.14 数据库生命周期

```text
$ /tmp/mgin-pkg-audit/audit mongo-multi-fail
InitWithError: <nil>
Check: <nil>
connections: []
```

```text
$ /tmp/mgin-pkg-audit/audit mongo-closed-init
InitWithError after Close: <nil>
```

```text
$ /tmp/mgin-pkg-audit/audit redis-closed-init
InitWithError after Close: <nil>
```

```text
$ /tmp/mgin-pkg-audit/audit sqlite-reopen
first InitWithError: <nil>
InitWithError after Close: <nil>
Check after reopen: <nil>
```

### 4.15 健康检查关闭死锁

通过 Go overlay 仅将 `app.go` 的 ticker 间隔从 5 分钟缩短为 1ms，使在途健康检查与信号到达的竞态稳定发生；业务代码未修改。

```text
$ /usr/local/go/bin/go test -overlay=/tmp/mgin-pkg-audit/overlay.json . \
    -run '^TestAuditShutdownWaitDeadlock$' -count=1 -timeout=5s
Get Signal:terminated
Shutdown Server ...
--- FAIL: TestAuditShutdownWaitDeadlock (1.30s)
    zz_shutdown_audit_test.go:50: shutdown deadlocked after in-flight health check returned
FAIL github.com/maczh/mgin/v2 1.314s
```

### 4.16 旧插件后台 panic

```text
$ /usr/local/go/bin/go test . -run '^TestAuditLegacyCheckPanic$' -count=1
panic: legacy check panic
github.com/maczh/mgin/v2.(*mgin).checkAll(...)
    /home/macro/Work/go/src/github.com/maczh/mgin/mgin.go:188
```

## 5. Standards

以下为相对 `docs/coding-standards.md` 的独立符合性审查，不与 Spec 结论合并排序。

1. **锁内网络 IO（硬违规）**：`pkg/db/mongo/mgo.go:44-47,103,124` 与 `pkg/db/redis/redis.go:43-46,183-194` 在持有 `initMu`、`mu` 时执行 Dial/Ping，违反 `docs/coding-standards.md:160`“锁内不做 IO”。这会放大关闭与健康检查阻塞，也是 F-11 的规范根因。
2. **忽略错误（硬违规）**：`pkg/db/sqlite/sqlite.go:35-36,73-75,93` 使用 `_ =` 丢弃 `InitWithError`、`DB()`、`Close()` 错误；Mongo/Redis Close 同样忽略底层关闭错误。违反 `docs/coding-standards.md:95`“不忽略错误”，并使关闭成功无法判断。
3. **后台 goroutine 退出与 panic 边界不足（硬违规）**：`app.go:117-126,384-391` 的关闭 channel 可变为 nil；`pkg/db/redis/redis.go:405,432-433` 的取消路径不释放 pubsub；`pkg/client/resilience.go:189-191`、`pkg/logsink/sink.go:329-333` 的 goroutine/callback panic 可逃逸。违反 `docs/coding-standards.md:155-158`“协程必须有退出路径”“不裸起 goroutine”“资源释放”。
4. **公开 API 注释与行为文档不完整（硬违规）**：新增 `ErrClosed`、`InitWithError` 等公开符号缺少符合 `docs/coding-standards.md:33-36` 的注释；Close 终止语义、Mongo `GetConnection` 不再执行原 Ping/自愈流程等行为变化未同步 CHANGELOG，违反 `docs/coding-standards.md:80,220-229`。
5. **新增边界测试不足（硬违规）**：OTel 仅测试 noop→noop，未覆盖真实 Provider 类型切换；数据库未覆盖 Close 后 Init、多库全失败；健康检查未覆盖信号与在途检查交错；Redis 未覆盖握手/消息处理取消。违反 `docs/coding-standards.md:212-216` 与 `docs/development/03-development/04-testing.md:267-270`。
6. **文档与实现不一致（判断性异味：Mysterious Name/Divergent Change）**：`logsink.WithQueueSize` 注释、测试和实现对“最大队列长度”的定义不同，说明该选项的契约没有单一事实来源；当前稳定测试失败即证据。

**Standards 最严重问题**：Mongo/Redis 初始化仍持锁执行网络 IO，直接违反明确规范并放大关闭阻塞。

## 6. Spec

Spec 来源为上一轮报告 `deliverables/mgin-db-plugin-panic-review.md` 中列出的修复目标。以下保持独立维度，不与其他发现重排。

### 已实现

- 插件元数据缓存、安全 Name/Order 调用，以及 recover 分支不再二次调用插件方法：已实现。
- `WithTx` nil receiver、Mongo `ReturnConnection` nil Session：已实现。
- Mongo tags 保留、Redis 空连接健康检查失败：已实现。
- Mongo/Redis `Check` 的网络 IO 移出锁：已实现。
- 健康检查增加 `WaitGroup`，资源关闭移到 HTTP 请求排空之后：方向正确，但关闭实现引入 F-01。

### 缺失或部分实现

- **Redis Pub/Sub 取消泄漏未修复**：上轮要求的“取消后关闭订阅、握手可取消”仍缺失，F-12 两条路径均可复现。
- **Mongo/Redis Init 锁内 IO 未完成整改**：只修复了 `Check`，`Init` 仍持两把锁执行网络调用，F-11。
- **Close 终止状态不完整**：Mongo/Redis 的 `InitWithError` 关闭后仍返回 nil；SQLite 无 closed 状态且可重新初始化，F-10。
- **错误传播不完整**：Mongo 多库全失败仍返回 nil，SQLite `Init` 仍丢弃 `InitWithError`，F-09/F-10。

### 实现错误或行为回归

- **WaitGroup 关闭顺序错误**：关闭 `checkDone` 后立即置 nil，而在途 `checkAll()` 返回后会再次读取 nil channel，造成永久等待，F-01。
- **Mongo `GetConnection` 自愈语义被删除**：原实现会通过 `mgoCheck` 在 Ping 失败时重建连接；当前 `GetConnection` 只复制现有 Session，不再执行 Ping/自愈。该变化超出“避免持锁 IO”的必要范围，也未在迁移说明中标注。
- **`InitWithError` 读取共享 `initErr` 不是本次初始化结果**：并发 Init 可在返回前覆盖错误，导致调用方观察到另一次初始化的结果，F-10。

### 范围扩张

- SQLite 新增 `InitWithError`/`validateSqlite` 与 RedisCache nil 防护有助于健壮性，但不属于原始 panic 清单；后续应作为独立变更补充契约与测试，避免和数据库生命周期修复混在一个提交中。
- xss/rsautil 的不可达 return 清理属于代码整理，不应掩盖本轮新发现的类型断言与 JSON 数组边界问题。

**Spec 最严重问题**：Redis Pub/Sub 取消路径未修复，且健康检查 `WaitGroup` 改造引入确定性关闭死锁。

## 7. 验证结果

### 已通过

```text
/usr/local/go/bin/go build ./...
/usr/local/go/bin/go vet ./pkg/...
```

目标包测试与 race：

```text
/usr/local/go/bin/go test ./pkg/db/... ./pkg/plugin ./pkg/otel ./pkg/client ./pkg/utils -count=1
/usr/local/go/bin/go test -race ./pkg/db/... ./pkg/plugin ./pkg/otel ./pkg/client ./pkg/utils -count=1
```

安全包集普通测试：除 `pkg/logsink` 外全部通过。

安全包集 race（排除已知失败的 `pkg/logsink`）：全部通过，包括 `pkg/db/...`、`pkg/plugin`、`pkg/otel`、`pkg/client`、`pkg/utils`、中间件、日志、健康检查、任务与注册中心基础包。

### 稳定失败

```text
$ /usr/local/go/bin/go test ./pkg/logsink -run '^TestQueueKeepsNewestAndFIFO$' -count=5 -v
--- FAIL: TestQueueKeepsNewestAndFIFO
    sink_test.go:237: options: {QueueSize:1000 Workers:1}
FAIL
```

连续 5 次均在同一断言失败；race 模式同样失败。原因是测试期望队列最大 100，而实现允许最大 `DefaultQueueSize=1024`。

### 未执行

未运行 `go test ./pkg/...` 全量测试。只读检查确认以下测试会访问真实外部服务，且没有 `testing.Short`/`TestMain` 跳过保护：

- `pkg/config/configure_test.go:41-50`：向内网 etcd 地址执行 Put；
- `pkg/registry/etcd/etcd_test.go`：真实 etcd 注册/查询/注销，并可能启动续约 goroutine；
- `pkg/registry/consul/consul_test.go`：真实 Consul；
- `pkg/registry/polaris/polaris_test.go`：真实 Polaris。

因此本轮没有执行可能修改共享注册中心状态的操作，也没有将“全量测试通过”作为结论。

## 8. 建议修复顺序

1. **立即**：F-01 退出死锁、F-02 旧插件后台 panic、F-03 OTel 类型切换、F-04 SafeMap fatal、F-05 Casbin 空 URI 鉴权绕过。
2. **本轮修复闭环**：F-06 Mongo URI 校验、F-07 client 未知协议、F-08 RingBuffer、F-09/F-10 数据库初始化与关闭语义、F-11 锁内 IO、F-12 Redis 取消、F-13 claims 校验。
3. **边界加固**：F-14 至 F-23，优先覆盖 HTTP 中间件、公共转换 API 与外部回调。
4. **防复发**：为每个 P0/P1 增加最小回归测试；涉及 goroutine、ticker、Pub/Sub、map 的测试必须跑 `-race`；关闭/取消测试使用可控 fake server 或 context，不依赖真实中间件。

## 9. 清理状态

- 仓库内临时测试 `zz_panic_audit_test.go`、`zz_shutdown_audit_test.go` 已删除。
- 仓库外探针 `/tmp/mgin-pkg-audit/` 在报告生成后清理。
- 本轮未修改任何 Go 源码；工作区原有 `.gitignore` 修改不属于本轮变更。
