# MGin `jh` 分支开放风险报告（v4）

- 审查日期：2026-11-26
- 分支：`jh`
- 审查基线：`97f159bfa67886d1104275804181aa5ea5e68609`（tag `v1.25.39-jh`）
- 对比范围：`eab05ed..HEAD`
- 范围 diff SHA-256：`50ded53e6964f780ac6b2f39cc9aeaa5d09017d1373e9bbceefa8056b5748fa6`
- 审查方式：静态审查、离线确定性探针、聚焦单元测试、`-race`、`go vet`
- 审查边界：只读生产代码；未修改业务实现；未创建提交；未主动连接共享数据库、Redis、MongoDB 或注册中心。全量测试意外触发 etcd 集成用例的限制见文末。

本报告只列出当前源码中仍然存在的开放风险，不包含已修复项。

## 结论

共确认 **21 个开放风险**：P1 8 个、P2 12 个、P3 1 个。

发布前优先处理：

1. 配置重载失败仍销毁旧资源并返回成功。
2. 出站 HTTP 响应 `Content-Length` 可触发 panic 或巨量预分配。
3. multipart 请求在 XSS 与访问日志链路中被全量缓存并成倍放大。
4. 不响应取消的 Job Handler 可让进程关闭无限等待。
5. 真实注册中心的 `InstanceProvider` 路径绕过 singleflight。
6. DAO `JOIN` 在连接失败时触发空指针 panic。
7. logsink 超时后仍关闭正在使用的 Handler，且关闭后可重新注册。
8. VictoriaLogs OTLP 仅限制消息长度，不限制结构化字段和最终编码大小。

---

## P1：可导致 panic、OOM、级联故障或关闭挂死

### R-01 配置重载失败仍销毁旧资源并返回成功

**位置**

- [mgin.go:131](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L131)
- [config/configure.go:217](file:///Users/macro/Work/go/src/github.com/maczh/mgin/config/configure.go#L217)
- [mgin.go:157](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L157)

**触发链**

`Reload` 调用无错误返回的 `config.Config.Init`，随后用 `Current().Cnf == nil` 判断是否加载失败。`Init` 在文件读取失败时直接返回并保留旧的非空 `Cnf`，因此该判断永远无法识别“首次成功初始化后的重载失败”。

失败后流程仍会执行 `closeResources(oldUsed, oldRegistry)` 和 `initResources(m)`。后者对数据库、Job、插件初始化错误只记录日志，不向 `Reload` 返回错误。

**影响**

配置文件路径写错、文件被临时替换或 YAML 损坏时，旧数据库、注册中心和插件连接会被关闭，进程却向 `/cfg/reload` 返回成功；新资源初始化失败时也没有回滚，形成实际服务中断。

**证据**

确定性探针加载一个不存在的重载文件：

```text
Reload missing file: err=<nil> pluginInits=2 pluginCloses=1
```

**最小修复方向**

- 增加返回错误的配置加载 API，在候选配置完整解析并校验成功后才发布。
- `Reload` 仅在加载成功后关闭旧资源。
- 让资源初始化返回错误，并尽可能先准备新资源、再切换旧资源；切换失败时回滚。

---

### R-02 出站响应 `Content-Length` 可触发 panic 或 OOM

**位置**

- [client/client.go:382](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L382)
- [go.mod:25](file:///Users/macro/Work/go/src/github.com/maczh/mgin/go.mod#L25)

**触发链**

成功路径直接调用 `resp.String()`。当前依赖 `github.com/levigross/grequests v0.0.0-20250606031859-3f3c12e4e704` 在读取响应体前执行：

```go
r.internalByteBuffer.Grow(int(r.RawResponse.ContentLength))
```

`Content-Length` 来自下游服务，没有最大值校验。恶意或异常下游可返回超大正数，即使实际响应体很短，也会先触发预分配。

**影响**

- 超大值可触发 `bytes.Buffer: too large` panic。
- 较大的合法值可造成单请求巨量内存分配，并发时形成 OOM。

**证据**

本地 HTTP 服务返回 `Content-Length: 1152921504606846976` 和短响应体：

```text
client huge Content-Length: panic=bytes.Buffer: too large
```

**最小修复方向**

不要用 `resp.String()` 读取不受信任响应。直接读取 `RawResponse.Body`，使用 `io.LimitReader(maxBytes+1)` 并拒绝超过预算的响应；若保留 `Content-Length` 快速失败，也必须在任何预分配前校验。

---

### R-03 multipart 请求被全量缓冲并多次复制，可造成 OOM

**位置**

- [middleware/xss/xss.go:367](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L367)
- [middleware/xss/xss.go:386](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L386)
- [middleware/xss/xss.go:419](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L419)
- [utils/ginutil.go:38](file:///Users/macro/Work/go/src/github.com/maczh/mgin/utils/ginutil.go#L38)
- [middleware/postlog/logger.go:220](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/postlog/logger.go#L220)

**触发链**

XSS multipart 处理对每个 part 执行 `io.Copy(&buf, part)`，随后把文件内容也写入第二个完整 `multiPrtFrm` 缓冲区，最后通过 `String()` 和 `[]byte(...)` 再复制一遍构建请求体。

与此同时，访问日志中间件对所有 POST multipart 请求调用 `GinParamMap`，后者使用 `ParseMultipartForm(100 << 20)`。即使访问日志自身已限制正文日志为 1 MiB，multipart 解析仍可能占用约 100 MiB 内存。

**影响**

并发上传或较大的表单字段可同时放大 XSS 缓冲区、multipart 解析内存和日志对象，形成进程级 OOM。文件内容不需要 XSS 过滤，却被完整读入内存。

**最小修复方向**

- 在服务器或中间件入口限制 multipart 总字节数、字段数和文件大小。
- 文件 part 流式写入临时文件，不重组完整内存请求体。
- 访问日志不要为了记录参数而强制 `ParseMultipartForm`；仅记录有界的元数据或抽样字段。

---

### R-04 不响应取消的 Job Handler 可让关闭无限等待

**位置**

- [job/scheduler.go:280](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L280)
- [job/scheduler.go:306](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L306)
- [job/scheduler.go:920](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L920)
- [job/scheduler.go:930](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L930)

**触发链**

`runOnce` 在 context 超时或取消后仍执行 `<-done`，等待实际 Handler goroutine 退出；`Manager.Stop` 又无超时地执行 `m.wg.Wait()`。

任意 Handler 忽略 `ctx.Done()`、阻塞在第三方调用或死循环时，`Stop` 永远不返回。

**影响**

SIGTERM、配置重载和 `SafeExit` 都可能永久挂起，容器无法在 Kubernetes 终止宽限期内退出，只能被强制杀死。

**最小修复方向**

为关闭等待设置明确 deadline；超时后记录失控 Handler 并返回，由进程退出策略处理残余 goroutine。将“Handler 必须响应取消”作为公开契约，并增加不响应取消的关闭回归测试。

---

### R-05 真实注册中心路径绕过缓存 singleflight，形成发现风暴

**位置**

- [registry/cache.go:144](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L144)
- [registry/cache.go:218](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L218)
- [registry/cache.go:243](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L243)

**触发链**

`GetServiceURL` 在进入 `inflight` 合并逻辑前先处理 `InstanceProvider`。Nacos、etcd、Consul、Polaris 都实现该接口，因此真实后端不会合并并发回源。`GetServiceInstances` 也直接调用 provider，没有缓存或 singleflight。

**影响**

缓存失效或注册中心抖动时，同一服务的所有并发调用同时回源，放大注册中心压力并延长故障恢复时间。现有 singleflight 单测即使通过，也不能覆盖真实 provider 路径。

**证据**

阻塞型 provider 探针同时启动 20 个调用：

```text
registry singleflight: backend calls=20 for 20 concurrent callers
```

**最小修复方向**

让 provider 和 legacy 路径共用同一缓存、singleflight 与失效机制；实例列表也应有 TTL 缓存。删除 inflight 时增加指针身份检查，避免旧请求误删新 flight。

---

### R-06 `MySQLDao.JOIN` 在连接失败时触发空指针 panic

**位置**

- [db/dao/mysql.go:594](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L594)
- [db/dao/mysql.go:614](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L614)

**触发链**

`Alias` 在 `connection()` 失败时返回 `nil`；`JOIN` 立即执行 `receiver.Alias(alias).Joins(...)`，没有判空。

**影响**

数据库未初始化、连接关闭或获取连接失败时，调用 `JOIN` 直接 panic。在 HTTP Handler 中可能被 recovery 转成 500，在后台任务或初始化流程中可能终止进程。

**证据**

不连接数据库，直接调用无连接 DAO 的 `JOIN`：

```text
MySQLDao.JOIN without connection: panic=runtime error: invalid memory address or nil pointer dereference
```

**最小修复方向**

将 `Alias`/`JOIN` 改为返回 `(*gorm.DB, error)`，或返回携带错误的非空 GORM 对象；调用方不得对可能为 nil 的 `*gorm.DB` 继续链式调用。

---

### R-07 logsink 可在 Handler 仍写入时关闭资源，且关闭后允许重新注册

**位置**

- [logsink/sink.go:223](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/sink.go#L223)
- [logsink/sink.go:383](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/sink.go#L383)
- [logsink/sink.go:448](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/sink.go#L448)
- [logsink/record.go:55](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/record.go#L55)
- [logsink/record.go:150](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/record.go#L150)

**触发链**

`sink.stop` 等待 worker 超时后仍调用 Handler 的 `Close()`，即使 worker 阻塞在 `Write` 中。`waitTimeout` 通过额外 goroutine 执行 `wg.Wait()`，worker 不退出时该 waiter 也永久泄漏。

全局 `Close` 只清空 registry，没有永久 closed 状态；随后 `Register`/`RegisterRecord` 仍可创建新 worker。普通 sink 和 Record sink 都有相同模式。

**影响**

外部 Handler 若在 `Close` 中关闭连接、文件或客户端，而 worker 仍在写入，会产生 use-after-close、panic 或数据竞争。关闭后重新注册还会绕过初始化顺序，形成难以解释的写入目标切换。

**证据**

```text
logsink close: handler Close called while Write blocked=true
logsink close: register-after-close accepted=true writes=1
```

**最小修复方向**

增加终态 closed 状态并拒绝关闭后注册；worker 未退出时不要关闭其依赖资源，或要求 `Close` 与 `Write` 明确并发安全。用 worker 自身的 done channel 替代额外 `wg.Wait` goroutine。

---

### R-08 VictoriaLogs OTLP 结构化字段没有字节预算，可放大为 OOM

**位置**

- [logs/victorialogs.go:141](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/victorialogs.go#L141)
- [logs/victorialogs.go:215](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/victorialogs.go#L215)
- [logs/victorialogs.go:230](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/victorialogs.go#L230)
- [logs/victorialogs.go:261](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/victorialogs.go#L261)
- [logsink/sink.go:44](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/sink.go#L44)

**触发链**

`write` 只限制 `Message` 为 1 MiB，不限制 `Fields`。`otlpAttributes` 将整个 `PostLog` 或 `Record` 先 `json.Marshal`，再 `json.Unmarshal` 成 map，随后递归编码所有属性和嵌套值。

`PostLog` 可包含较大的 `RequestBody`、`RequestParam`、`ResponseMap` 和 `ResponseStr`。logsink 队列只限制条目数量，不限制字节数。

**影响**

单条日志会同时持有原始对象、JSON 字节、解析后的 map 和 protobuf 字节；大队列可成倍放大内存。大表单或大响应日志可能直接推动进程 OOM。

**最小修复方向**

在序列化前限制字段数量、嵌套深度和总字节数；丢弃或截断大字段；对最终 OTLP payload 设置硬上限。logsink 增加按字节计算的队列预算，而不仅是条目数。

---

## P2：可导致数据竞争、资源复活、内存增长或功能失效

### R-09 注册中心关闭与 legacy 发现取消存在挂死/goroutine 泄漏

**位置**

- [registry/cache.go:346](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L346)
- [registry/cache.go:362](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L362)
- [client/client.go:428](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L428)

**触发链**

`CachedRegistry.DeRegister` 无超时执行 `c.active.Wait()`。legacy 客户端发现通过新 goroutine 调用不支持 context 的 `GetServiceURL`，调用方 context 取消只让外层 select 返回，不会停止后台 goroutine。

**影响**

后端调用长期阻塞时关闭流程挂死；频繁超时会积累永久阻塞的发现 goroutine。

**最小修复方向**

为注册中心接口增加 context 感知方法；关闭等待设置 deadline；确保 delegate 的取消能传播到实际网络调用。

---

### R-10 XSS 手工 JSON 序列化会破坏合法输入

**位置**

- [middleware/xss/xss.go:564](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L564)
- [middleware/xss/xss.go:596](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L596)

**触发链**

`ConstructJson` 无条件删除缓冲区最后一个字节来移除逗号。空对象写入 `{` 后也会删除该字节，再写入 `}`，得到非法 JSON `}`。手工拼接还不能可靠处理所有键名转义和嵌套组合。

**影响**

合法请求 `{}` 被改写成非法 JSON，Handler 解析失败；复杂对象可能出现静默数据损坏。

**证据**

```text
xss empty object: body="}" validJSON=false readErr=<nil>
xss over HTTP: body="{\"x\":\"\"}"
```

**最小修复方向**

使用 `json.Decoder` 解析、递归清洗、`json.Encoder` 重新编码，不手工删除末尾字节。

---

### R-11 i18n Store 在配置重载后仍持有旧数据库连接

**位置**

- [i18n/store.go:333](file:///Users/macro/Work/go/src/github.com/maczh/mgin/i18n/store.go#L333)
- [i18n/store.go:336](file:///Users/macro/Work/go/src/github.com/maczh/mgin/i18n/store.go#L336)
- [mgin.go:131](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L131)

**触发链**

`i18n.Init` 在 `currentStore` 非空时直接返回。`mgin.Reload` 关闭并重建数据库，却没有关闭或重建 i18n Store。使用 MySQL 字典时，Store 继续引用已关闭的旧连接。

**影响**

重载后国际化查询、错误码翻译和字典 API 使用失效连接，出现持续 500 或旧数据。

**最小修复方向**

把 i18n Store 纳入资源重载生命周期：先停止使用旧 Store，再基于新数据库创建；或让 Store 每次从当前数据库客户端解析连接。

---

### R-12 MongoDB `Close` 不是终态，后续访问会自动重连

**位置**

- [db/mongo/mgo.go:122](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L122)
- [db/mongo/mgo.go:138](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L138)
- [db/mongo/mgo.go:187](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L187)
- [db/mongo/mgo.go:222](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L222)

**触发链**

`Close` 清空 `conns` 但保留 `confData`。`mgoCheck`、`Check`、`GetConnection` 发现连接为空时调用 `Init(confData)` 自动恢复。

**影响**

关闭后的迟到调用会重新建立连接；重载失败或关闭流程中可能意外复活旧客户端，破坏资源生命周期和关闭语义。

**最小修复方向**

增加显式 closed 状态；`Close` 清除配置数据并禁止自动重连；只有重新调用 `Init` 才能恢复。

---

### R-13 Job 关闭顺序错误，异步调度更新不受生命周期管理

**位置**

- [mgin.go:208](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L208)
- [mgin.go:285](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L285)
- [job/scheduler.go:1074](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L1074)
- [job/scheduler.go:1090](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L1090)
- [job/scheduler.go:1098](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L1098)

**触发链**

`closeResources` 和 `SafeExit` 都先关闭 MySQL/SQLite/MongoDB/Redis，再调用 `job.Stop`。`asyncUpdateFire` 和 `asyncUpdateNext` 还启动未纳入 `m.wg` 的 goroutine，直接访问存储。

**影响**

运行中的 Job 和调度更新可能在数据库关闭后执行，产生失败日志、状态丢失或 goroutine 越过关闭边界。

**最小修复方向**

先停止调度器和异步更新，再关闭数据库；把所有后台更新纳入可等待、可取消的生命周期。

---

### R-14 `MemCache` 与请求 ID 幂等缓存可被高基数键撑大

**位置**

- [cache/memcache.go:132](file:///Users/macro/Work/go/src/github.com/maczh/mgin/cache/memcache.go#L132)
- [middleware/requestid/requestid.go:27](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/requestid/requestid.go#L27)
- [middleware/requestid/requestid.go:42](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/requestid/requestid.go#L42)

**触发链**

`MemCache` 没有条目数或字节上限。幂等中间件把客户端完全可控的 `X-Request-ID` 直接作为 key，每个新 ID 都插入一条记录。TTL 为一分钟，但高请求速率下仍可积累大量条目；清理器每分钟全量扫描 map，也会放大 CPU 开销。

**影响**

攻击者或异常客户端持续发送唯一请求 ID，可造成持续内存增长和周期性 CPU 峰值。

**最小修复方向**

为缓存增加最大条目数和字节预算、LRU 淘汰；限制请求 ID 长度和格式；对幂等键使用有界结构。

---

### R-15 客户端熔断 map 无限增长，响应缓存只有条数上限

**位置**

- [client/client.go:93](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L93)
- [client/client.go:151](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L151)
- [client/client.go:192](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L192)
- [client/client.go:76](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L76)
- [client/client.go:574](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L574)

**触发链**

熔断 key 默认包含服务名和完整 URI，`circuits.values` 没有容量或淘汰机制。响应缓存限制最多 1024 条，但不限制单条响应字节数。

**影响**

高基数 URI 可持续增加熔断状态；少数超大响应即可占用大量缓存内存，与 R-02 叠加时风险更高。

**最小修复方向**

对熔断 map 增加容量和过期淘汰；响应缓存增加总字节预算和单值上限。

---

### R-16 限流配置缺少资源上限，Header 维度和独立 Manager 可泄漏资源

**位置**

- [middleware/ratelimit/limiter.go:123](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/limiter.go#L123)
- [middleware/ratelimit/limiter.go:211](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/limiter.go#L211)
- [middleware/ratelimit/ratelimit.go:181](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/ratelimit.go#L181)
- [middleware/ratelimit/ratelimit.go:215](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/ratelimit.go#L215)
- [middleware/ratelimit/ratelimit.go:43](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/ratelimit.go#L43)

**触发链**

滑动窗口直接 `make([]int64, limit)`，并发限流直接 `make(chan struct{}, max)`，配置值没有上限。Header 维度把完整请求头值放入 key，攻击者可构造唯一值。`RateLimitWith` 每次创建新 Manager 和 GC goroutine，但只返回 Handler，没有停止句柄。

**影响**

错误配置可在启动时 OOM；唯一 Header 可制造高基数 limiter；动态创建或丢弃中间件会泄漏 Manager 和 goroutine。

**最小修复方向**

校验并 clamp 所有容量配置；限制 key 长度、哈希或归一化 Header 值；限制 limiter 总数；为独立 Manager 提供停止机制或复用单例。

---

### R-17 Session 中间件包级 key 会跨实例串用并产生竞态

**位置**

- [middleware/session/session.go:30](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/session/session.go#L30)
- [middleware/session/session.go:53](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/session/session.go#L53)
- [middleware/session/session.go:87](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/session/session.go#L87)

**触发链**

`storeKey` 和 `manageKey` 是包级变量，每个 `NewWithConfig` 都会覆盖它们；`FromContext`、`Destroy`、`Refresh` 又读取全局值。多个中间件实例或并发初始化时会相互覆盖。

**影响**

请求可能读取到错误实例的 key，表现为 Session 丢失、`invalid session manager`，并存在数据竞争。

**证据**

顺序创建两个不同 key 的中间件后，使用第一实例写入的 context：

```text
session cross-instance: destroyErr=invalid session manager
```

**最小修复方向**

删除包级可变 key；把 key 捕获到中间件闭包中，或使用固定 context key/类型化访问器。

---

### R-18 共享 `MySQLDao` 实例会被 `WithContext`/`WithCache` 原地修改

**位置**

- [db/dao/mysql.go:71](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L71)
- [db/dao/mysql.go:99](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L99)
- [db/dao/mysql.go:108](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L108)

**触发链**

`connection` 在 `Tag == nil` 时写入 `m.Tag`；`WithContext` 和 `WithCache` 直接修改 receiver。若业务把同一个 DAO 指针共享给多个请求，并发调用会竞态，并可能把一个请求的 context 或缓存配置泄漏给另一个请求。

**影响**

出现非确定性查询上下文、缓存串用和 `-race` 告警；严重时可能跨请求复用已取消 context。

**证据**

两个 goroutine 并发调用同一个 DAO 的 `WithContext`：

```text
WARNING: DATA RACE
Write ... db/dao/mysql.go:103
Found 1 data race(s)
```

**最小修复方向**

参照现有 `Debug`/`WithTx` 返回浅拷贝，不在共享 receiver 上修改字段。

---

### R-19 同一进程重复调用 `NewApp` 会因 flag 重定义 panic

**位置**

- [app.go:75](file:///Users/macro/Work/go/src/github.com/maczh/mgin/app.go#L75)
- [app.go:79](file:///Users/macro/Work/go/src/github.com/maczh/mgin/app.go#L79)
- [app.go:82](file:///Users/macro/Work/go/src/github.com/maczh/mgin/app.go#L82)

**触发链**

`NewApp` 每次向全局 `flag.CommandLine` 注册 `-v`，在空配置路径时还注册 `-f`。第二次调用直接触发 `flag redefined`。

**影响**

测试、嵌入式宿主或需要创建多个 App 实例的场景启动即 panic。

**证据**

```text
NewApp twice: panic=... flag redefined: v
```

**最小修复方向**

使用包级 `sync.Once` 注册 flag，或改为局部 `flag.FlagSet`，避免修改全局命令行状态。

---

### R-20 单 Polaris 模式不会注册客户端

**位置**

- [mgin.go:177](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L177)
- [registry/registry.go:78](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/registry.go#L78)
- [registry/registry.go:94](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/registry.go#L94)

**触发链**

`NewRegistry` 支持创建 Polaris 客户端，但 `initResources` 只为 Nacos、etcd、Consul 执行单客户端 `Register`，没有 Polaris 分支。多注册中心集合路径不受此问题影响。

**影响**

使用单 Polaris 配置时客户端对象存在但从未注册，服务发现与注册不可用。

**最小修复方向**

补充 Polaris 注册分支，或把单注册中心初始化统一为通用 registrar。

---

## P3：特定错误路径可造成请求数据损坏

### R-21 postlog 读取请求体失败时用部分数据替换原始流

**位置**

- [middleware/postlog/logger.go:121](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/postlog/logger.go#L121)
- [middleware/postlog/logger.go:124](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/postlog/logger.go#L124)

**触发链**

`readRequestBody` 读取失败时把 `Request.Body` 替换为已读到的部分数据，原始读取错误和未读部分都丢失。

**影响**

客户端断连或流读取异常时，Handler 可能把不完整请求当作正常请求处理，造成静默数据损坏。

**最小修复方向**

读取失败时保留原始错误语义并中止请求，或恢复原始流；不要用部分数据伪装成完整请求体。

---

## 验证记录

### 确定性探针

探针位于仓库外，不修改生产代码。当前基线输出：

```text
registry singleflight: backend calls=20 for 20 concurrent callers
xss empty object: body="}" validJSON=false readErr=<nil>
xss over HTTP: body="{\"x\":\"\"}"
MySQLDao.JOIN without connection: panic=runtime error: invalid memory address or nil pointer dereference
client huge Content-Length: panic=bytes.Buffer: too large
logsink close: handler Close called while Write blocked=true
logsink close: register-after-close accepted=true writes=1
Reload missing file: err=<nil> pluginInits=2 pluginCloses=1
NewApp twice: panic=... flag redefined: v
session cross-instance: destroyErr=invalid session manager
dao shared WithContext: DATA RACE at db/dao/mysql.go:103
```

### 测试与静态检查

以下离线聚焦测试通过：

```text
go test ./config ./db/mysql ./db/sqlite ./db/redis ./db/mongo ./db/dao \
  ./middleware/postlog ./middleware/ratelimit ./middleware/requestid \
  ./cache ./client ./logs ./logsink ./job ./i18n ./registry ./utils
```

以下聚焦竞态测试通过：

```text
go test -race ./registry ./logsink ./middleware/ratelimit ./config \
  ./db/mysql ./db/dao ./job
```

`go vet ./...` 通过。

现有测试通过不代表上述路径安全：例如 registry 单测使用的 delegate 未覆盖真实 `InstanceProvider` 分支，logsink 测试也未覆盖“关闭超时后 Handler 仍阻塞”的竞态。

### 全量测试限制

`go test ./...` 不能作为本环境的离线验证命令。[registry/etcd/etcd_test.go:11](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/etcd/etcd_test.go#L11) 硬编码了共享 etcd 地址。本次全量测试进入该集成用例后，在注册前的 `Get` 阶段超时 10 分钟，未执行到后续 `Put` 注册写操作。后续验证已改为离线包列表。

建议为该集成测试增加显式环境开关或 build tag，默认 `go test ./...` 不访问共享基础设施。
