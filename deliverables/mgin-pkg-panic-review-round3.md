# mgin v2 `pkg/` Panic、并发与生命周期风险第三轮复审

- 审核日期：2026-11-26
- 分支：`v2`
- 当前 HEAD：`39c554ef9e7b503a8febb13b841af302509bb05f`
- 固定点：`4f1d9de0fa932af58605b10ec7b3ee5a21d77ecc`
- 基准以来提交：
  - `625f83c fix: harden pkg panic and lifecycle risks`
  - `feb0481 refactor: route goroutines through safe launcher`
  - `39c554e test: close remaining panic remediation gaps`
- 范围：全量 `pkg/**`，以及与健康检查、插件、数据库关闭直接关联的 `app.go`、`mgin.go`
- 方式：静态审查、仓库外最小复现探针、仓库内临时回归测试、`go build`、`go vet`、目标包测试与安全包集 `-race`
- 源码修改：无。用户原有工作树改动未纳入本次已提交差异审查，也未修改
- 外部服务：未连接真实 MySQL、MongoDB、Redis、etcd、Consul、Nacos 或配置中心

## 1. 结论摘要

上轮 23 项修复中，**14 项已闭环、7 项部分修复、1 项未闭环、1 项基本修复但残留并发关闭候选**。OTel 类型切换、SafeMap 并发 fatal、Mongo 无数据库 URI、client 未知协议、XSS 标量数组、postlog 非法 gzip、RSA 非 RSA 密钥、vo/models 错类型转换等已确认修复有效。

本轮新确认 **18 项问题：P0 5 项、P1 7 项、P2 6 项**。其中 17 项通过最小测试或 HTTP 探针复现，1 项为注释与实现契约不一致。最严重的新发现是：

1. JWT 中间件在 `Swagger.Uri == ""` 时对所有路径免鉴权；
2. 旧插件注册表仍存在并发 map fatal，旧插件 `CheckFunc` panic 仍可终止后台健康检查协程；
3. 无超时 job handler 可使 `Manager.Stop()` 永久阻塞；
4. `Values.GetAll/Clear` 未使用已有锁，可触发不可恢复的并发 map fatal；
5. Casbin 仍用子串匹配放行 Swagger 路径，`/private/docs/secret` 可绕过私有路由鉴权。

建议先处理 P0 鉴权与 fatal 路径，再修复关停阻塞、连接/插件泄漏和可复现 panic，最后统一公共工具边界契约与文档。

## 2. 严重度定义

- **P0**：鉴权绕过、进程 panic、运行时 fatal 或永久退出阻塞，应立即处理。
- **P1**：可复现 panic、关键资源泄漏、关闭语义错误或可导致关停受阻。
- **P2**：条件性 panic、公共 API 边界缺陷、并发隐患、文档/契约不一致。

## 3. 上轮 F-01～F-23 修复核对

| ID | 结果 | 当前证据与残留 |
|---|---|---|
| F-01 | 已修复 | `app.go` 在 goroutine 启动时捕获 ticker/done 局部值，关闭路径增加有界等待，原 select nil channel 死锁已消除 |
| F-02 | 部分修复 | 新插件 `InitAll` 已返回错误且有 recover；旧 `m.plugins` 的 `InitFunc/CheckFunc/CloseFunc` 仍裸调，见 R-02 |
| F-03 | 已修复 | OTel Provider 改为固定具体类型 holder，异构 `atomic.Value.Store` 路径已消除 |
| F-04 | 已修复 | `utils.Map` 写操作统一使用写锁，`Len/Map/Clear` 已加锁，`Map` 返回副本 |
| F-05 | 部分修复 | Casbin 空 URI 恒真已修复，但 `/docs/`、`/swagger/` 和自定义 URI 仍使用子串匹配，见 R-05 |
| F-06 | 已修复 | Mongo 在 Dial 前校验 URI 数据库路径，无数据库 URI 不再进入依赖的越界路径 |
| F-07 | 已修复 | client 未知 Protocol 返回错误，nil response 有保护，resilience goroutine 有 recover |
| F-08 | 部分修复 | 构造 size 与 `Size` 已修复；`Size/Reader/Writer/Buffer` 仍公开，外部破坏索引后越界，见 R-16 |
| F-09 | 已修复 | Mongo 多库全部失败时保留首个错误，零连接健康检查失败 |
| F-10 | 部分修复 | Mongo/Redis closed 与 SQLite 重开终止状态已修复；SQLite `Close` 仍可能跳过底层连接，见 R-09 |
| F-11 | 部分修复 | Dial/Ping 已移出 `mu`，但仍全程持有 `initMu`，并发 Init/Close 仍可能等待网络 IO |
| F-12 | 部分修复 | Redis Pub/Sub 握手与 handler 取消泄漏主体已修复；Close 不等待订阅 goroutine 退出仍为静态候选 |
| F-13 | 已修复 | Casbin claims 使用安全断言与数值转换，不再强制 `float64` |
| F-14 | 已修复 | XSS JSON 数组元素按类型处理，标量元素不再强制转 map |
| F-15 | 已修复 | postlog 仅在 `gzip.NewReader` 成功后 Close/Read |
| F-16 | 已修复 | `AnyToMap` 支持 nil、指针、接口解引用，并安全转换 map 值 |
| F-17 | 基本修复，残留候选 | `count==0` 与全局 `ok` 已处理，业务路径改为单一 mutex；`StopLimitQueueCleaner` 的并发 close 仍非原子，静态可疑但未复现 |
| F-18 | 部分修复 | `UnSplitString` 空输入已修复；`DeleteArray` 越界仍 panic，见 R-12 |
| F-19 | 已修复 | RSA 公钥/私钥解析使用 comma-ok，非 RSA 密钥返回错误 |
| F-20 | 已修复 | vo/models 对外转换增加 nil 与类型不匹配保护 |
| F-21 | 已修复，残留静态候选 | logsink 外部回调与 typed-nil 已有保护；`Emit` 持读锁调用 drop logger 的重入死锁未动态确认 |
| F-22 | 已修复 | VictoriaLogs 不再直接断言 `http.DefaultTransport.(*http.Transport)` |
| F-23 | 未闭环 | 注释仍写“最大 100”，实现常量为 1024，测试只调整输入，未统一契约，见 R-18 |

## 4. 本轮新确认问题清单

| ID | 严重度 | 位置 | 问题与影响 | 修复建议 |
|---|---|---|---|---|
| R-01 | P0 | `pkg/middleware/jwt/jwt.go:14` | `strings.Contains(path, Swagger.Uri)` 在 URI 为空时恒真，所有私有路径无需 token 直接放行。已用无 Authorization 的 `/private/secret` 确认业务 handler 被执行 | 自定义 URI 仅在非空时判断；docs/swagger 使用固定前缀或显式路由白名单；增加空 URI 和包含 docs 片段的私有路径测试 |
| R-02 | P0 | `mgin.go:56-80,170-193,198-221` | 旧插件 map 无锁；并发 `Use` 与 `checkAll` 可触发 `fatal error: concurrent map iteration and map write`。`CheckFunc` panic 也会从后台健康检查协程逃逸 | 为旧插件注册表增加独立 mutex；生命周期回调统一 safe call；健康检查循环不能因单插件 panic 退出 |
| R-03 | P0 | `pkg/job/scheduler.go:261-273,747-800` | handler context 来自 `context.Background()`，`Stop` 不取消它；无超时 handler 永久阻塞时 `wg.Wait()` 永久阻塞 | 为运行中任务保存 cancel；Stop 先取消并设置有界宽限期；明确超时后是等待还是强制返回，不允许无界关停 |
| R-04 | P0 | `pkg/utils/values.go:27-45` | `GetAll/Clear` 绕过已有 `lock`，与 `Put/Merge` 并发可触发 map 数据竞争和运行时 fatal；`GetAll` 还暴露内部 map | `GetAll` 返回加锁副本，`Clear` 使用写锁；固定公开返回类型，避免调用方修改内部状态 |
| R-05 | P0 | `pkg/middleware/casbin/casbin.go:35-40` | Swagger/docs 使用 `Contains`，`/private/docs/secret` 等私有路径被直接放行；自定义 URI 同样会放行包含该片段的任意路径 | 改为固定前缀或路由白名单；对自定义 URI 做规范化并精确匹配边界；增加私有路径反例 |
| R-06 | P1 | `pkg/utils/safego.go:57-65` | `goBeforeF` 在 `safego.Run` 的 recover 边界外同步执行，before hook panic 会直接逃逸 | 将 before hook 放入同一 recover 边界，或先安全执行并转换/记录错误；覆盖 before hook panic 测试 |
| R-07 | P1 | `pkg/middleware/limit/limit.go:8-19` | 获取信号量不响应请求 context；满载时已取消请求仍等待槽位，HTTP Shutdown 可能被拖住 | `select` 同时等待 semaphore 与 `c.Request.Context().Done()`；取消时返回 429/503，且不得 defer 释放未获得的槽位 |
| R-08 | P1 | `pkg/plugin/plugin.go:174-205` | `CloseAll` 仅关闭当前 `Enabled()==true` 的插件；Init 成功后运行期被禁用的插件永远不关闭 | 记录成功初始化的插件集合，CloseAll 按该集合关闭；Enabled 只控制 Init，不控制资源回收 |
| R-09 | P1 | `pkg/db/sqlite/sqlite.go:27-35,75-88` | `validateSqlite` 将 `conn.Error!=nil` 视为不可用，`Close` 因此跳过底层 `sql.DB.Close()`；已确认 Close 后 Ping 仍成功 | 关闭时直接取 `sql.DB` 并尝试 Close，不依赖 GORM Error；保留并返回/记录 Close 错误 |
| R-10 | P1 | `pkg/utils/desutil.go:25-36`；`pkg/utils/3desutil.go:29-44` | 合法 key 配非块长整数倍密文时，`CryptBlocks` 直接 panic；DES 与 3DES 均复现 | 解密前校验非空及 `len(crypted)%block.BlockSize()==0`，非法输入返回 error；同时校验 CBC IV 长度 |
| R-11 | P1 | `pkg/registry/consul/consul.go:149-151,181-183,302-314` | 检查 `err` 前调用 `filterCluster`，且未跳过 nil `ServiceEntry/Service`；httptest 返回 `[null]` 可稳定触发 nil deref | 先处理 err；过滤 nil 条目与 nil Service；单个异常实例不应使整次发现 panic |
| R-12 | P2 | `pkg/utils/sliceutil.go:215-219` | `DeleteArray([]any{1}, 1)` 触发 slice bounds panic；负数同样危险 | 越界返回原切片或 error，与 `StringArrayDelete/IntArrayDelete` 的既有安全语义统一 |
| R-13 | P2 | `pkg/cache/memcache.go:133-149` | `Set` 接受 `any` key 并直接交给 `sync.Map.Store`，slice/map/function key 可触发 `hash of unhashable type` panic | 在 API 边界拒绝不可比较 key，或改用支持明确错误返回的内部实现；增加 slice/map key 测试 |
| R-14 | P2 | `pkg/utils/maputil.go:13-18,48-50,125-136` | `MapItoS` 强制字符串断言、`PairList.Less` 强制 `float64`、`Struct2StringMap(nil)` 对零 Value 调 Type，三类输入均复现 panic | 使用 comma-ok 或返回 error；数值排序兼容明确数值类型；反射前检查 nil/有效 Value |
| R-15 | P2 | `pkg/utils/safemap.go:10-25,85-105` | `Map.Range` 有 nil receiver 保护，但 `Load/Store/Delete/Len/Map/Clear` 等没有，nil receiver 可 panic | 统一 nil receiver 语义：读操作返回零值，写操作 no-op，或明确记录构造约束并保持全方法一致 |
| R-16 | P2 | `pkg/utils/ringbuffer.go:5-12,25-38` | `Size/Reader/Writer/Buffer` 公开，调用方可绕过 mutex 破坏索引；`Writer=10` 后 `Write` 稳定越界 | 将内部字段改为私有，提供受锁保护的方法；如需兼容公开字段，至少在索引使用前做范围校验并归一化 |
| R-17 | P1 | `pkg/utils/linklist.go:259-272` | `Walk` 持读锁执行外部 callback；callback 内 `Push/Add/Remove/Pop/Clear` 需要写锁，造成自死锁 | 在锁内 snapshot 节点值，解锁后执行 callback；明确 callback 可重入；增加回调写操作测试 |
| R-18 | P2 | `pkg/logsink/sink.go:43-45,65-79` | `WithQueueSize` 注释“最大 100”，实现允许最大 1024；契约、注释和测试未统一 | 定义唯一 `MaxQueueSize` 常量，注释、normalize、测试和文档统一引用，避免依赖魔法数 |

## 5. 动态复现记录

复现文件均为临时审核探针，验证后清理；仓库外模块通过 `replace` 指向当前工作树。以下输出不包含真实地址、token 或客户数据。

### 5.1 JWT 空 Swagger URI 全路径放行

最小请求：`GET /private/secret`，无 `Authorization`，`Swagger.Uri=""`。

```text
handler reached without token: true
```

根因是 `Contains(path, "") == true`，不是 Gin recovery 或 token 解析问题。

### 5.2 Casbin 私有 docs 路径放行

```text
GET /private/docs/secret
handler reached without claims: true
```

普通私有路径仍要求 claims；只有包含 `/docs/`、`/swagger/` 或自定义 URI 片段的路径被放行。

### 5.3 旧插件 panic 与 map fatal

```text
legacy CheckFunc panic: PANIC: legacy check panic
```

并发 `Use()` 与 `checkAll()` 在 race 报告中出现 map 读写竞争，继续加压可触发：

```text
fatal error: concurrent map iteration and map write
github.com/maczh/mgin/v2/pkg/mgin.(*mgin).checkAll(...)
```

该 fatal 不能被 recover。生产路径中 `GoSafe` 可 recover 普通 panic，但健康检查 goroutine 会退出。

### 5.4 job Stop 永久阻塞

探针使用 SQLite 内存库，注册一个阻塞 handler，不设置任务超时：

```text
job.Stop returned: false
Stop still blocked after 500ms
```

`Stop` 只关闭 `stopChan`，没有取消 handler context，也没有等待上限。

### 5.5 Values 并发 fatal

```text
fatal error: concurrent map iteration and map write
github.com/maczh/mgin/v2/pkg/utils.(*Values).GetAll(...)
github.com/maczh/mgin/v2/pkg/utils.(*Values).Clear(...)
```

`Put/Merge` 的锁无法保护未加锁的 `GetAll/Clear`。

### 5.6 SafeGo before hook panic

```text
SafeGo before hook: PANIC: before hook panic
```

panic 发生在进入 `safego.Run` 之前，因此现有 recover 不生效。

### 5.7 限流请求取消无效

容量为 1，第一个请求占满槽位，第二个请求 context 取消：

```text
second request still waiting after cancellation: true
second request completed only after first released slot
```

### 5.8 插件运行期禁用后漏关

```text
Close called: false
```

插件 Init 成功后 `Enabled()` 改为 false，`CloseAll` 直接跳过，已分配资源无法回收。

### 5.9 SQLite Close 跳过底层连接

```text
Close returned
underlying sql.DB Ping after Close: <nil>
```

探针仅给 GORM DB 设置 `Error`，底层 `sql.DB` 仍健康；当前 Close 因 `validateSqlite` 失败而没有调用 `db.Close()`。

### 5.10 DES/3DES 短密文 panic

```text
DES decrypt short ciphertext: PANIC: crypto/cipher: input not full blocks
3DES decrypt short ciphertext: PANIC: crypto/cipher: input not full blocks
```

### 5.11 其他公共工具边界

```text
DeleteArray index == len: PANIC: runtime error: slice bounds out of range [2:1]
MemCache slice key: PANIC: runtime error: hash of unhashable type []int
MapItoS non-string: PANIC: interface conversion: interface {} is int, not string
SortMapByValue non-number: PANIC: interface conversion: interface {} is string, not float64
Struct2StringMap nil: PANIC: reflect: call of reflect.Value.Type on zero Value
SafeMap nil receiver: PANIC: runtime error: invalid memory address or nil pointer dereference
RingBuffer Writer corruption: PANIC: runtime error: index out of range [10] with length 1
LinkList Walk callback Push: DEADLOCK
```

`RingBuffer` nil receiver 的 `Write` 已有保护，探针返回 OK，因此未把 nil receiver 列为该类型缺陷。

### 5.12 Consul nil 实例条目

httptest 服务返回 `[null]`：

```text
$ /usr/local/go/bin/go test ./pkg/registry/consul \
  -run '^TestRound3NilConsulServiceEntryReturnsError$' -count=1
panic: runtime error: invalid memory address or nil pointer dereference
github.com/maczh/mgin/v2/pkg/registry/consul.filterCluster(...)
github.com/maczh/mgin/v2/pkg/registry/consul.(*ConsulClient).GetServices(...)
```

## 6. 未动态确认的静态候选

以下项目只有源码路径证据或复现尝试未成功，**不应视为已确认缺陷**：

| ID | 位置 | 候选问题 | 建议验证方式 |
|---|---|---|---|
| S-01 | `app.go:406-430` | 健康检查自身永久阻塞时，超时后等待 goroutine 仍泄漏，随后可能在使用中的资源上执行 `SafeExit` | 注入一个永不返回的 `CheckFunc`，检查关停超时后的 goroutine 与资源关闭顺序 |
| S-02 | `pkg/db/redis/redis.go` | Close 不等待 Pub/Sub goroutine 退出，关闭后仍可能访问已释放 client | goleak + 假 RESP 服务，Close 后检查订阅连接与 goroutine |
| S-03 | `pkg/registry/etcd/etcd.go` | DeRegister 不等待心跳协程，注销后心跳可能复活；Delete 使用 Background 可能无界阻塞 | 可控假 etcd 服务与取消测试 |
| S-04 | `pkg/registry/consul/consul.go:204-255` | TTL 注销与心跳协程存在竞态，最后一次 UpdateTTL 可能晚于反注册 | 用可阻塞 UpdateTTL 的 HTTP server 验证顺序 |
| S-05 | `pkg/job/scheduler.go:232-272` | `running=true` 后才 `wg.Add()`，与并发 `Stop()` 的 `Wait()` 存在 WaitGroup Add/Wait 竞态 | `go test -race` 高并发 Start/Stop；用状态机消除窗口 |
| S-06 | `pkg/logsink/sink.go:194-211` | `Emit` 持全局读锁同步调用 drop logger；logger 重入 Register/Close 需要写锁，可死锁 | 在 drop logger 中调用 Register，设置有界超时 |
| S-07 | `pkg/middleware/ratelimit/ratelimit.go` | 每次调用可能创建不可停止的 GC goroutine | 检查 `runtime.SetFinalizer` 路径并做重复创建/回收测试 |
| S-08 | `app.go:347-365` | HTTP/HTTPS Listen 失败只记录日志，进程继续等待信号；端口占用或证书错误不会退出 | 占用测试端口启动最小 App，断言进程进入失败状态 |
| S-09 | 配置与插件初始化路径 | 配置读取或部分插件初始化失败后仍继续启动 | 构造失败配置，断言 fail-fast 与明确降级策略 |
| S-10 | `pkg/utils/sysutil.go:18-38` | `CmdRunWithTimeout` 的 done 为无缓冲 channel；超时后依赖额外 goroutine 排空，panic/Process nil 路径可能阻塞 | 覆盖未 Start、已退出、Wait panic 和超时场景 |
| S-11 | `pkg/utils/limit.go:65-71` | `StopLimitQueueCleaner` 的 check-close 非原子，理论可 double-close | 10,000 goroutine 高压未复现；仍建议 `sync.Once` 或 mutex |
| S-12 | `pkg/db/mongo/mgo.go` | `GetConnection` 自愈语义被删除/改变，可能不符合既有调用方预期 | 对照上轮 Spec 与真实业务调用，确认是否属于范围扩张 |
| S-13 | `pkg/middleware/casbin/casbin.go` | 全局 Casbin 对象可被外部置 nil，后续请求可能 deref | 明确全局对象生命周期，增加 nil/替换竞态测试 |

## 7. Standards 轴审查

审查对象：`4f1d9de...HEAD` 已提交差异。标准来源：`docs/coding-standards.md`、`docs/development/03-development/04-testing.md`、`docs/development/02-concepts/03-design-decisions.md`、`docs/testing-strategy.md`。

### 硬性规范问题

1. **非原子关闭与导出注释缺失**  
   `StopLimitQueueCleaner` 使用 check-close，多个调用者可同时进入 default 并 double-close，违反 `coding-standards.md` §5“全局状态加锁”和 §1.2“导出符号必须有注释”。该问题本轮未动态复现，但代码形态明确。

2. **错误仍被丢弃**  
   Mongo/Redis 的兼容 `Init` 仍吞掉 `InitWithError` 错误；`job.Manager.Store()` 丢弃 `StoreWithError` 错误，违反 §3.1“不忽略错误”。公共 API 应返回错误或提供明确的 WithError 路径并记录原因。

3. **行为与生命周期变化缺少发布说明**  
   差异修改了数据库 closed 语义、插件关闭选择、下载文件命名、i18n 数值格式和 job Store 初始化语义，但变更文件列表中没有 CHANGELOG 或架构文档更新，违反 §2.2.4“行为变化必须在 CHANGELOG 的 Changed 中标注”和 §9 生命周期/架构文档要求。

### 判断项

- `vo` 切片转换、reflect 判空等存在重复代码形状，属于“Duplicated Code”判断项；当前规模不足以强制抽象，可在下一次相关改动时统一。

**Standards 汇总：3 项硬性问题、1 项判断项；最严重的是生命周期行为变化未记录，以及错误仍被公共入口丢弃。**

## 8. Spec 轴审查

Spec 来源：`deliverables/mgin-pkg-panic-review.md` 的 F-01～F-23、验收标准、修复建议和验证边界。

### 缺失或部分实现

1. **F-02 部分**：Spec 要求“为新旧插件生命周期调用统一增加 recover 边界，记录插件名和 panic；Init 错误必须返回/传播”。新插件已实现，旧 `m.plugins` 的 Init/Check/Close 仍裸调，且 `InitAll` 的错误在启动链中仍可能只记录。

2. **F-05 部分**：Spec 要求“仅当 URI 非空时才做 contains 判断；Swagger/docs 放行使用固定前缀白名单”。空 URI 已修，但 Casbin 仍使用子串匹配，私有路径可绕过。

3. **F-10 部分**：Spec 要求“SQLite 增加终止状态，关闭时无论 GORM Error 如何都尝试关闭底层 sql.DB 并记录错误”。closed 状态已加，但 GORM Error 仍跳过 `db.Close()`。

4. **F-11 部分**：Spec 要求“只在锁内校验 closed、解析配置并发布状态；锁外建立连接”。Dial/Ping 已移出 `mu`，但仍全程持有 `initMu`，未达到锁外 IO 的完整目标。

5. **F-18 部分**：Spec 要求“索引删除函数对越界返回原切片或 error，并统一约定”。`UnSplitString` 已修，`DeleteArray` 未修。

6. **F-23 未闭环**：Spec 要求“明确唯一上限并同步注释、常量、实现和测试；不要通过删除断言掩盖行为差异”。当前注释 100、实现 1024，仅测试输入变化。

### 实现看起来不正确

- R-01 JWT 空 URI 绕过不属于上轮 F-05 的 Casbin 修复，但说明同类放行规则在另一个中间件中仍存在。
- R-03 job Stop 无界等待与框架优雅关停目标冲突；Spec 的验证边界明确要求覆盖“超时、关闭/重载”。
- R-08 CloseAll 以运行期 Enabled 状态决定是否关闭，破坏“初始化成功即需要回收”的生命周期契约。

### 范围扩张

以下行为变化不是 F-01～F-23 要求的修复：

- `pkg/utils/downloadutil.go:31-32` 增加响应头文件名提取与 `path.Base` 规则；
- `pkg/i18n/i18n.go:100-110` 将数值格式统一为固定 6 位小数；
- `pkg/job/scheduler.go:302-330` 改变 `Store()` 的首次初始化与错误语义；
- Mongo `GetConnection` 自愈语义变化需另行确认兼容性。

**Spec 汇总：6 项缺失/部分闭环、3 项实现风险、4 项范围扩张；最严重的是 F-05 的鉴权绕过只修了一部分，且 JWT 同类问题仍可导致全路径免鉴权。**

## 9. 已执行验证

以下命令使用 `/usr/local/go/bin/go`，因为当前环境 Go 不在 PATH：

```text
/usr/local/go/bin/go build ./...
/usr/local/go/bin/go vet ./pkg/...
```

通过的安全包集普通测试（至少包括）：

```text
/usr/local/go/bin/go test \
  ./pkg/db/mongo ./pkg/db/redis ./pkg/logsink \
  ./pkg/middleware/casbin ./pkg/utils ./pkg/vo \
  -count=1
```

同一安全包集 `-race` 通过。仓库内临时复现测试验证后均已删除。

未执行：

- `go test ./pkg/...` 全量测试：`pkg/config` 会访问内网 etcd；
- `pkg/registry/{etcd,consul,polaris}` 的真实注册中心测试；
- 任何可能修改共享注册中心状态的测试；
- 真实数据库、消息中间件和完整服务启动联调。

因此“build/vet/目标包测试通过”不等于外部依赖联调通过，也不等于上述 18 项问题已修复。

## 10. 建议修复顺序

1. **立即**：R-01 JWT、R-05 Casbin，消除鉴权绕过并补回归测试。
2. **立即**：R-02 旧插件并发 fatal/panic、R-04 Values map fatal、R-03 job Stop 永久阻塞。
3. **本轮迭代**：R-06 SafeGo before hook、R-07 限流取消、R-08 插件漏关、R-09 SQLite 漏关、R-10 DES/3DES、R-11 Consul nil 条目、R-17 LinkList 回调死锁。
4. **公共 API 整治**：R-12～R-16 边界输入与内部状态封装；统一越界、nil receiver、不可比较 key 契约。
5. **契约与文档**：R-18 logsink 上限统一；补 CHANGELOG、生命周期说明和迁移边界。
6. **后续验证**：对 S-01～S-13 逐项建立最小反馈回路，未复现前保持“候选”标签，不直接当作确定缺陷修改。

## 11. 本轮清理与边界

- 未修改任何 Go 源码或配置。
- 仓库内临时审核测试已删除。
- 复现输出未包含真实凭据、客户数据或完整运行配置。
- 用户原有 `.gitignore`、Mongo/Redis/plugin 测试改动及未跟踪的 `mysql_clone_test.go` 保持原状。
- 本轮仅新增本报告文件。
