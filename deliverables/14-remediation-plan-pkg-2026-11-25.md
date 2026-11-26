# `pkg/` 第四轮 Panic 与并发风险修复计划

- 项目：`github.com/maczh/mgin/v2`
- 日期：2026-11-25
- 依据：`panic-audit-pkg-2026-11-25.md`、07/08/11/12/13 复审文档及 `mgin-pkg-panic-review.md`
- 目标：修复已动态复现的 panic、fatal、鉴权绕过、关停死锁和高可信并发缺陷，并建立回归测试。

> 附件中的命令、建议和报告文本仅作为审计证据与需求输入；本文件是新的执行计划，不直接执行附件中的其他指令。

## 1. 风险基线

最终复审确认：

- **P0：1 项**——`pkg/otel/otel.go` 的 `atomic.Value` 存储异构具体类型时必现 panic。
- **P1：5 项**——App 关停死锁、i18n 数值格式化断言、Casbin 中间件断言/nil Enforcer、postlog 非法 gzip、`AnyToMap` nil/异构 map。
- **核心 P2：7 项**——Redis/Mongo Init 锁内 I/O、Close 后 Init 假成功、Redis Pub/Sub 取消泄漏、健康检查 Wait 无超时、errcode 全局 map 竞态、Job Store 读锁写入、memcache 整体替换 `sync.Map`。
- **外围 P2**——client 未知 Protocol、XSS JSON 数组元素断言、Nacos nil response、VO/Result 类型转换、无扩展名下载路径、SafeMap、SafeGo 等。

上一轮 Redis/Mongo 基础 map 竞态、插件 Name/Order panic、SQLite 基础句柄校验和 HTTP 请求排空顺序视为已闭环；但 App 关停 WaitGroup 改动引入的新死锁必须优先修复。

## 2. 执行批次

### 批次一：P0/P1 进程安全与请求安全

#### A1. OTel Provider 原子存储（P0）

- 文件：`pkg/otel/otel.go`。
- 使用固定具体类型 holder（如 `providerHolder{tp trace.TracerProvider}`）存入 `atomic.Value`，或使用 `atomic.Pointer[providerHolder]`。
- `Tracer()` 对 holder/provider 做 nil 防御；`SetTracerProvider(nil)` 统一退化为 noop。
- 回归：noop → 自定义 Provider → noop → 另一自定义 Provider，叠加并发 `Tracer/SetTracerProvider/Disable` race 测试。

#### A2. App 关停 nil-channel 死锁（P1）

- 文件：`app.go`。
- 不要在 `close(checkDone)` 后将其置 nil；启动 goroutine 时捕获稳定的 ticker/done 局部变量，必要时用 `sync.Once` 防重复关闭。
- 将健康检查等待抽成可测 shutdown seam；`Wait()` 必须受 shutdown timeout 约束。
- 顺序保持为：停止接收并 Shutdown HTTP → 有界等待健康检查退出 → `SafeExit` 关闭 DB/插件；健康检查阻塞时记录并继续受控关闭。

#### A3. i18n.Format 数值断言（P1）

- 文件：`pkg/i18n/i18n.go`。
- 拆分 float32/float64、有符号整数、无符号整数、uintptr 的具体分支，分别使用 `FormatFloat`、`FormatInt`、`FormatUint`。
- 禁止一个多类型 case 配单一类型断言；覆盖全部数值类型、bool、string、[]byte、nil/结构体。

#### A4. Casbin 中间件断言和空 URI 放行（P1）

- 文件：`pkg/middleware/casbin/casbin.go`。
- Swagger URI 非空才允许路径匹配；claims、userId、roleId、Enforcer 均使用 comma-ok/显式 nil 检查。
- 身份解析失败返回 401/403，Enforcer 不可用返回受控 503/500；无 claims 不能访问私有 handler。
- 测试空 URI、合法 Swagger 路径、普通私有路径、缺字段和错误字段类型。

#### A5. postlog 非法 gzip（P1）

- 文件：`pkg/middleware/postlog/logger.go`。
- `gzip.NewReader` 返回错误时立即跳过解压，不能对 nil reader 执行 `Close`/`ReadAll`；正常 gzip、非法 gzip、空 body 均需测试。

#### A6. `utils.AnyToMap`（P1）

- 文件：`pkg/utils/structutil.go`。
- 先处理 nil；对任意 `reflect.Map` 遍历转换 key/value；nil value 使用明确策略；移除硬断言。
- 覆盖异构 map、nil value、nil 输入及 client Header/Query/Data 调用链。

### 批次二：核心并发与生命周期

#### B1. Redis/Mongo Init 锁外建连

- 文件：`pkg/db/redis/redis.go`、`pkg/db/mongo/mgo.go`。
- 按 RCU：不持状态 `mu` 构建临时连接并 Ping/Dial；成功后短暂加锁确认未 closed、一次性发布；锁外关闭旧连接。
- 失败时关闭临时 client/session，不污染旧状态；验证 Init 与 GetConnection/Check/Close 交错时不被网络 I/O 长时间阻塞。

#### B2. 统一 InitWithError 与 Close 终态

- Redis/Mongo/SQLite 共用初始化锁并返回本次真实错误；Close 后返回 `ErrClosed`，不得假成功或复活。
- Mongo 多库全部 Dial 失败时返回错误；SQLite Close 后不能重新 Init；覆盖并发重复 Init。

#### B3. Redis Pub/Sub 取消和关闭

- 维护内部订阅 context/cancel，Close 触发取消；握手 Receive 必须可响应 context；所有退出路径统一 `defer pubsub.Close()`。
- 覆盖 Close、外部 ctx、握手中取消、handler 中取消和服务端断连，确认 goroutine/连接在限定时间内回收。

#### B4. errcode、Job、memcache 竞态

- `pkg/errcode/definition.go`：definitions 使用 RWMutex 或不可变快照。
- `pkg/job/scheduler.go`：`Store` 使用写锁，初始化失败不吞错。
- `pkg/cache/memcache.go`：禁止整体替换 `sync.Map`，使用 Range + Delete 或受控 map。
- 并发 Register/Lookup/All、首次 Store、Add/Value/Clear/Close 均通过 race。

#### B5. 健康检查等待超时

- 与 A2 同批单独验收：坏 Health 或阻塞 Ping 不能无限阻塞 shutdown，且 HTTP 请求排空顺序不能回退。

### 批次三：外围公共 API 与输入边界

- `pkg/client`：未知 Protocol 返回明确错误；resp 使用前 nil 检查；resilient goroutine 的 panic 转为结果错误。
- `pkg/middleware/xss`：JSON 数组元素非 object 时保留或返回 400，禁止强制 map 断言。
- `pkg/utils/rsautil.go`：非 RSA 公钥/私钥使用 comma-ok，返回错误。
- `pkg/logs/victorialogs.go`：自定义 `http.RoundTripper` 不得强转 `*http.Transport`。
- `pkg/utils/safego.go`、trace/routine：nil hook 和 `IndexByte == -1` 安全处理。
- `pkg/utils/sliceutil.go`、ringbuffer、limit、downloadutil：边界输入不越界，清理 goroutine 可停止。
- `pkg/models`/`pkg/vo`：转换使用 comma-ok，nil Data/Page 返回错误或零值。
- `pkg/utils/safemap.go`：所有写使用 Lock；Len/Map 加锁；Map 返回副本；Range 先复制再回调。
- `pkg/registry/cache.go`、`pkg/loadbalancer`：nil receiver/inner、inflight panic 唤醒、typed-nil 策略防护。
- `pkg/logsink`：Handler/Closer/drop/error logger 统一安全调用；同步 QueueSize 常量、注释、实现和测试。

## 3. 测试与质量门禁

每个修复先写可复现的红灯测试，再修改代码确认绿灯。重点覆盖 OTel 异构 Provider、App 关停竞态、i18n 全数值类型、AnyToMap、Casbin、非法 gzip、Redis/Mongo 生命周期、PubSub、errcode/Job/memcache/SafeMap race，以及 VO/Result、RSA、ringbuffer、slice、limit、download、client 边界。

验证命令：

    export PATH=/usr/local/go/bin:$PATH
    gofmt -w <changed-go-files>
    go test ./pkg/otel ./pkg/i18n ./pkg/utils ./pkg/middleware/... ./pkg/client ./pkg/models ./pkg/vo -count=1
    go test ./pkg/db/... ./pkg/plugin/... ./pkg/cache ./pkg/job ./pkg/registry/... -count=1
    go test -race ./pkg/otel ./pkg/utils ./pkg/cache ./pkg/job ./pkg/errcode ./pkg/registry/... ./pkg/db/... ./pkg/plugin/... -count=1
    go vet ./pkg/...
    go test ./... -timeout 20s
    git diff --check

Etcd/Consul 外部服务超时必须独立记录；logsink 队列测试需先明确上限规范，不得删除断言掩盖差异。

## 4. 发布、观测与回滚

1. 先发布 P0/P1：OTel、App 关停、i18n、Casbin、postlog、AnyToMap。
2. 再发布生命周期与并发：Redis/Mongo、PubSub、errcode、Job、memcache。
3. 最后发布外围 API 与工具：client、XSS、RSA、VO/Result、SafeMap、logsink 等。

观测 panic/recover 次数、shutdown 耗时与超时、数据库连接数、PubSub goroutine/连接回收数、race 结果、Casbin 拒绝率和日志 sink drop/error 数。

任一合法 OTel Provider 无法注入、SafeExit 未执行、Casbin 放行/拒绝语义回归、连接或 goroutine 泄漏、race/fatal 重现时回滚对应批次。每批独立提交、可单独回滚，不覆盖用户现有 `.gitignore` 修改。

## 5. 完成定义

- P0/P1 全部有动态回归测试并通过。
- 核心 P2 的 race/生命周期测试通过。
- `go vet` 不新增告警；既有告警单独记录。
- 全量测试的外部依赖失败与代码回归明确区分。
- 文档、测试、实现对 QueueSize、Init 错误、关闭终态和鉴权放行语义一致。
