# MGin `jh` 分支独立复审报告（v3）

- 复审日期：2026-11-26
- 仓库 / 分支 / HEAD：`github.com/maczh/mgin`，`jh`，`eab05ed`（"fix: code review by codex"）
- 复审对象：`eab05ed` 的当前源码（非工作区未提交改动）；同时独立复核 `deliverables/panic-audit-jh-2026-11-26.md`（v2）的结论
- 范围：panic / OOM / 内存与 goroutine 泄漏 / 空指针 / 数据竞争 / 生命周期
- 方法：全量静态复核 + 5 个最小动态探针 + `build/vet/test/-race` 基线
- 纪律：**未修改生产代码、未 commit**；探针跑完即删；未触碰正在被并行编辑的 v2 报告
- 说明：v2 报告在本次复审期间仍被并行修改（`git status` 可见），因此本报告独立成文，不复用其结论作为证据

---

## 0. 结论摘要

**Redis 的进程级 fatal 已被真正修复；但 v2 的 4 个 P1 全部依然成立，并且我新增了 1 个 P1 生命周期缺口。**

| 编号 | 等级 | 问题 | 本轮复核 |
|---|---|---|---|
| R-01 | P1 | 热重载后 MySQL / SQLite 永久不可重开 | **确认**：SQLite 探针稳定 FAIL；MySQL 控制流唯一确定；新增测试是假绿 |
| R-02 | P1 | `config.Config` 导出字段在 Reload 与日志热路径间无同步 | **确认**：`-race` 稳定报 4~5 条 DATA RACE |
| R-03 | P1 | Redis `Init` 持写锁做网络 I/O，读者全停 | **确认**：纯读 `IsMultiDB()` 实测阻塞 **2.801s**（Init 总耗时 3.002s） |
| R-04 | P1 | postlog 缓冲放大；chunked 请求体仍无界 | **确认**：chunked 5 MiB 全部被中间件读入；**并修正 v2 的"截断标记会输出"结论（实际永不输出）** |
| N-01 | P1 | 多注册中心模式下 Reload 后不再执行 `RegisterConfigured`，服务从注册中心消失 | **新增**（静态确定） |
| N-02 | P2 | 单注册中心 `polaris` 在 `initResources` 中无注册分支 | **新增**（配置相关） |
| N-03 | P2 | XSS 中间件把空 JSON 对象 `{}` 改写成非法 JSON `}` | **新增**（动态复现） |
| N-04 | P2 | Mongo `Init` 持写锁做最长 10s/库的拨号，读者全停 | **新增**（与 R-03 同类，v2 漏列） |
| N-05 | P3 | postlog 的 `total/truncated` 因值接收者永不更新，截断标记是死代码 | **新增**（动态复现） |
| N-06 | P3 | postlog `GetRawData` 失败后不回填 `c.Request.Body`，handler 读到空体 | **新增**（静态） |
| N-07 | P3 | 同一进程第二次调用 `NewApp` 必 panic：`flag redefined: v` | **新增**（动态复现） |

已确认**真修复**：Redis 共享 map（20 万次 `Close`/`GetConnection` 并发 `-race` 通过）、AES 非整块密文、`cache.diskPath`、`registry` 访问器、i18n 测试契约、config 活体测试门禁、`go vet` 干净。

---

## 1. 基线

```text
$ go build ./...
BUILD_OK

$ go vet ./...
（无输出，退出码 0）

$ go test ./... -count=1 -timeout 5m
TEST_EXIT=0
# config 的活体用例已通过 MGIN_LIVE_CONFIG_TESTS 门禁跳过；
# i18n 已改用真实路由，全部 26 个含测试的包 ok。
```

与上一轮相比，仓库自带测试从"config 挂死 600s + i18n 稳定 404"变为**全绿**。但要注意：

- 全绿不代表本次复审的 P1 不存在——现有测试没有覆盖 `Reload` 与请求路径并发；
- v2 R-20.4 指出的 `registry/{etcd,consul,polaris}` 活体用例仍依赖硬编码外部地址，只是本轮 5 分钟超时内未触发；作为 CI 门仍不可靠。

### 1.1 动态探针结果（全部已删除）

| 探针 | 结果 |
|---|---|
| Redis `Close` 与 `GetConnection` 并发 20 万次 | **PASS（-race）**，P0 已修复 |
| SQLite `Init → Close → Init → GetConnection` | **FAIL**：`reopen: SQLite client is closed` |
| `config.Init` 循环 + 直读 `App.Name/Config.Used/Logger.Outputs` | **FAIL**：4~5 条 `WARNING: DATA RACE`，写点 `configure.go:311-314` |
| Redis `Init` 运行中调用 `IsMultiDB()` | 纯读阻塞 **2.801s**；Init 总耗时 3.002s |
| postlog 接收 5 MiB chunked 请求体 | 中间件读入 **5,242,880 字节**（`ContentLength=-1` 绕过 1 MiB 守卫） |
| postlog `bodyLogWriter` 写 10 字节、limit=4 | buffer 正确截到 4，但 `total=0`、`truncated=false`（值接收者） |
| XSS `ConstructJson({})` | 输出 `"}"`，不是 `"{}"` |
| `NewApp` 连续调用两次 | 第二次 panic：`flag redefined: v` |

---

## 2. v2 P1 的独立复核

### R-01 热重载后 MySQL / SQLite 永久不可重开（确认，且测试假绿）

**代码事实**（与 v2 一致，我重新逐行核对）：

- `db/mysql/mysql.go:49-68`：`InitWithError` 在 `m.mu.Lock()` 之后仍有
  ```go
  if m.closed { ... return ErrClosed }
  ...
  m.closed = false
  ```
  `Close()`（`mysql.go:203-220`）把 `closed` 置 true，**没有任何路径在守卫前复位**。因此 `Close → Init` 必然在发布点返回 `ErrClosed`。
- `db/sqlite/sqlite.go:31-80`：入口的 `closed && opened` 在 `Close` 后（`sqlite == nil`）会放行，但发布点的 `if m.closed` 仍拒绝；`m.closed = false`（`sqlite.go:72`）不可达。
- `mgin.go:131-148`：`Reload` = `closeResources`（关 MySQL/SQLite）→ `initResources`（再 Init）。`Init` 只打日志不返回错误，`Reload` 仍返回 nil，**热重载接口表面成功、数据面永久死亡**。

**动态证据**（SQLite 最省事，无需外部服务）：

```text
=== RUN   TestZZR3ReopenAfterClose
    zz_r3_probe_test.go:16: reopen: SQLite client is closed
--- FAIL: TestZZR3ReopenAfterClose (0.00s)
```

**MySQL 用例假绿**：`db/mysql/mysql_lifecycle_test.go:8-16` 断言"返回非 `ErrClosed`"，但传入的 DSN 是 `mysql: test`（无斜杠），`candidate.init` 在 `closed` 守卫**之前**就以 `invalid DSN` 失败，用例因此通过。日志可证：

```text
[ERR] ... mysql connection error:invalid DSN: missing the slash separating the database name
--- PASS: TestMysqlClientCanValidateReopenAfterClose
```

该用例没有、也无法证明"关闭后可以重开"。

**修复建议**（二选一）：

1. 把"关闭即终态"限定给外部直接调用，`Reload` 走单独的内部 `reopen()`；或
2. 更小：`Reload` 不再先 `Close` —— `InitWithError` 本身已经是"候选成功后原子替换并关闭旧连接"，让 `closeResources` 跳过 MySQL/SQLite 即可，既避免关闭空窗，也顺带消除 R-08 的 i18n 连接失效链路。

并补一个**真正证明重开成功**的 SQLite 用例（Init → Close → Init → 查询成功）。

### R-02 config 导出字段数据竞争（确认）

`config/configure.go:303-318` 在 `target.mu` 内整体重发布 `App/Config/Log/Logger/Discovery/...`，但全仓 141 处直接读导出字段（日志热路径 `logs/output.go:155` 读 `Config.Logger.Outputs`）不持锁。探针在 HEAD 上稳定复现：

```text
WARNING: DATA RACE
Write at ... config/configure.go:311   // target.App = ...
Previous read ... Config.App.Name
WARNING: DATA RACE
Write at ... config/configure.go:313   // target.Logger = ...
Previous read ... Config.Logger.Outputs
```

危害与 v2 相同：slice/string 撕裂读可能越界 panic 或静默错配。若短期不改所有读点，至少应把日志热路径改为先取本地快照；长期用 `atomic.Pointer[config]` 整体替换。

### R-03 Redis `Init` 写锁内网络 I/O（确认，补丁新引入）

`db/redis/redis.go:41-43` 给 `Init` 加了整段 `mu.Lock()`，而 `Init` 内部逐个 DB 执行 `NewUniversalClient` + `Ping()`，`DialTimeout` 默认 60s（`redis.go:158-161`）。所有读者（`GetConnection/Check/IsMultiDB/ListConnNames`）都要 `RLock`。

**实测**（黑洞监听，`timeout: 3`，单库）：

```text
IsMultiDB blocked for 2.801113667s while Init was running
Init took 3.002182292s
0 Redis连接失败: read tcp 127.0.0.1:...: i/o timeout
```

Go `RWMutex` 是写者优先：一次慢 Redis 重载会让后续所有 Redis 调用排队，生产 60s 拨号超时下可放大到分钟级，造成请求堆积与健康检查失败。

**修复**：与 `MysqlClient.InitWithError` 的 candidate 模式一致——锁外完成解析、建客户端、Ping；只在替换 `clients/cfgs/conns/multi` 时取写锁；Ping 用独立的短超时。

### R-04 postlog 缓冲（确认 chunked 无界，并修正 v2 的一处错误结论）

**请求侧**：`middleware/postlog/logger.go:187-196` 仅当 `ContentLength >= 0 && > 1 MiB` 时跳过；`ContentLength == -1`（chunked）落入 `GetRawData()`（Gin 实现为 `io.ReadAll`，已核对 gin v1.11.0 `context.go:1029-1034`）。探针证明 5 MiB chunked 请求体被完整读入并复制：

```text
chunked request: middleware read 5242880 bytes
known Content-Length request: middleware read 0 bytes
```

**响应侧**：1 MiB 上限生效（buffer 确实截到 4/limit=4），但：

```text
truncated = false, want true (value receiver drops update)
total = 0, want 10
```

原因：`Write/WriteString` 是**值接收者**（`logger.go:120/137`），`w.total += ...` 和 `w.truncated = true` 只改了栈上副本；`c.Writer` 持有的是 `*bodyLogWriter`，原对象的字段永远为零。因此 `logger.go:213` 的 `<body truncated: N bytes>` **永远不会追加**。v2 中"会打截断标记"的表述需要修正为"标记逻辑存在但无效"。

**残余风险**：

1. chunked 请求体仍可无上限物化（远程可触发，最严重）；
2. gzip 分支 `io.ReadAll(r)` 对 1 MiB 高压缩比响应可放大到数百 MB；
3. 1 MiB 是每请求预算，并发 × 1 MiB 无全局上限；
4. `shouldSkipLog` 仍在 `c.Next()` 之后，跳过路径照样付缓冲成本；
5. `bodyLogWriter.body.String()` 每请求再复制一份最多 1 MiB。

**修复**：请求侧改用 `io.LimitReader(c.Request.Body, max+1)`，读满即截断并回填，不依赖 `ContentLength`；gzip 解压同样 `LimitReader`；把 `Write/WriteString` 改为指针接收者；`shouldSkipLog` 前移。

---

## 3. 新增发现

### N-01（P1）多注册中心模式下 Reload 后不再注册服务

**位置**：`mgin.go:151-177`、`app.go:412-417`、`registry/registry.go:59-71,163-180`

`initResources` 在 Reload 时执行 `registry.SetRegistry(registry.NewRegistry())`，得到一个**全新的** `RegistryCollection`；但随后只有非 multi 分支调用 `Register`：

```go
if _, multi := currentRegistry.(registry.ConfiguredRegistrar); !multi && strings.Contains(configs, "nacos") {
    currentRegistry.Register(...)
}
// etcd / consul 同理，没有 multi 分支
```

multi 模式的注册只发生在 `app.RunContext` 的 `registrar.RegisterConfigured()`（`app.go:416`），而 `RunContext` 有 `runStarted` 保护，**整个进程只执行一次**。与此同时 `closeResources` 已经把旧集合 `DeRegister()`。

结果：一次 `PUT /cfg/reload` 后，旧实例从注册中心注销，新实例从未注册——**服务从服务发现中彻底消失**，直到进程重启。单注册中心（Nacos/Etcd/Consul）不受影响，因为走 `Register` 分支。

**修复**：Reload 在 `initResources` 之后（或其中）对 `ConfiguredRegistrar` 也调用一次 `RegisterConfigured()`；注意此时 listener 已就绪，与启动时的前置条件一致。

### N-02（P2）单注册中心 polaris 无注册分支

**位置**：`mgin.go:169-177`、`registry/registry.go:88-105`、`registry/polaris/polaris.go:40`

`NewRegistry` 的 switch 支持 `case "polaris"`，`closeResources` 也会对 polaris 做 `DeRegister`，但 `initResources` 只有 nacos/etcd/consul 三个 `Register` 分支，**没有 polaris**。若 `go.discovery.registry=polaris` 且未使用 multi 列表，则 `PolarisClient.Register` 永远不会被调用（其自身也不会在构造时注册）。

该路径依赖"polaris 是否出现在 `go.config.used`"这一前提，建议结合部署配置确认；代码层面是明确的覆盖缺口。

### N-03（P2）XSS 把空 JSON 对象改写成非法 JSON

**位置**：`middleware/xss/xss.go:564-600`（`ConstructJson` 末尾）

```go
buff.WriteByte('{')
for k, v := range m { ... }
buff.Truncate(buff.Len() - 1) // 空 map 时把 '{' 也截掉
buff.WriteByte('}')
```

当输入是 `{}` 时，循环不执行，`buff.Len()==1`，`Truncate(0)` 删掉唯一的 `{`，最终输出 `}`。业务 handler 随后 JSON 解析失败或读到错误内容。

**动态证据**：

```text
ConstructJson({}) = "}", want "{}"
```

**修复**：只有写过至少一个字段时才截掉最后一个逗号，例如记录 `fields := 0`，`if fields > 0 { Truncate(...) }`；`unravelSlice` 已用 `Len()>1` 正确处理空数组，可对齐。

### N-04（P2）Mongo `Init` 同样在写锁内拨号

**位置**：`db/mongo/mgo.go:34-35`（`m.mu.Lock(); defer Unlock()`）、`mgo.go:93/114`（`DialWithTimeout(..., 10s, ...)`）

这是 R-03 的同族问题：`Init` 持写锁，内部对每个 DB 串行拨号，单库最长 10s；multi 模式 N 个库叠加。期间 `GetConnection/Check/IsMultiDB/ListConnNames` 全部阻塞。启动时阻塞可接受，**Reload 时不可接受**。

**修复**：与 R-03 相同，先锁外构造候选连接，再锁内发布。

### N-05（P3）postlog 截断标记因值接收者永不生效

见 R-04 的响应侧证据。影响：日志中的大响应被静默截到 1 MiB 且没有"已截断"提示，排障时会误以为响应本身只有 1 MiB；同时 `fileResponseSummary` 拿到的是截断后的长度，文件大小日志失真。

**修复**：`func (w *bodyLogWriter) Write` / `WriteString` 改为指针接收者（两处），并补一个直接单测（本轮探针可直接复用）。

### N-06（P3）postlog 读取失败后不回填请求体

**位置**：`middleware/postlog/logger.go:191-196`

```go
data, err := c.GetRawData()
if err != nil {
    logs.Error(...)          // 不回填
} else {
    body = string(data)
    c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
}
```

`GetRawData` 内部是 `io.ReadAll(c.Request.Body)`，失败时 body 已被消费。旧代码无条件回填（哪怕为空），新代码在错误路径直接跳过，后续 handler 读到 EOF。概率低（客户端中断/读超时），但会导致"日志中间件改变业务可见请求体"。

**修复**：错误路径也回填已读到的 `data`（`bytes.NewBuffer(data)`），保持 `Body` 可重读。

### N-07（P3）`NewApp` 第二次调用必 panic

**位置**：`app.go:79-85`

`flag.BoolVar(&getVersion, "v", ...)` 在每次 `NewApp` 都执行；`flag` 包对重复注册直接 panic。探针：

```text
second NewApp panicked: ... flag redefined: v
```

单进程单应用不受影响；但库使用者做多应用/测试内多次构造、或进程内"软重启"时会直接崩溃。**修复**：用 `sync.Once` 注册 flag，或改用 `flag.CommandLine.Lookup` 先查后注册；更彻底的是不在库构造函数里调用 `flag.Parse()`。

---

## 4. 已确认的真修复

| 项 | 验证 |
|---|---|
| Redis 共享 map → concurrent map fatal | 20 万次 `Close`/`GetConnection` 并发，`-race` 通过；代码中 `clients/multi/conns` 的读写均已进锁或快照 |
| `AESBase64Decrypt` 非整块密文 panic | 新增 `len%blockSize` 守卫；探针输入 5 字节密文返回 error，不 panic |
| `cache.diskPath` 单字符切片越界 | `len(cachePath) >= 2` 守卫；`OnDiskCache("x")` 不再 panic |
| `registry.Registry` 生产代码裸读写 | 已改用 `GetRegistry/SetRegistry`；生产代码无裸访问（测试仍有，v2 R-20.5） |
| `go vet` 干净 | `go vet ./...` 无输出 |
| i18n/config 测试红灯 | `go test ./...` 现在退出码 0 |

> 注：v2 的 R-05～R-19（先关 DB 后停 job、singleflight 被旁路、发现调用不可取消、i18n 每响应查库、memcache 无容量、DAO JOIN nil、Mongo 复活、job 无界等待、限流 Header、session 包级变量、logsink 关闭等待、health 1s 窗口、xlang goroutine ID、日志文件每行 open/close）我逐条抽查了代码路径，**未发现 v2 误报**，本轮不重复展开。

---

## 5. 与 v2 的差异 / 需要修正的结论

1. **R-04 的"响应侧会打 `<body truncated: N bytes>`"不成立**：值接收者导致 `truncated/total` 永不更新（N-05）。上限本身有效，标记无效。
2. **v2 未覆盖 N-01（多注册中心 Reload 后不注册）**、**N-02（polaris 注册缺口）**、**N-03（XSS 空对象）**、**N-04（Mongo Init 锁内 I/O）**、**N-07（NewApp flag panic）**。
3. **v2 R-01 的 SQLite 结论正确**（我独立复现 FAIL），且其"MySQL 用例假绿"的判断也正确（R-20.2）；本轮补充了"假绿的具体原因：invalid DSN 在 closed 守卫之前失败"。

---

## 6. 修复优先级

### 发布前必须完成

1. **R-01**：让 Reload 真正能重开 MySQL/SQLite（或在 Reload 中跳过 Close），并补可执行的 SQLite 重开用例；
2. **N-01**：Reload 后对 multi 注册中心调用 `RegisterConfigured()`；
3. **R-02**：日志热路径改快照；长期用 `atomic.Pointer`；
4. **R-03 / N-04**：Redis/Mongo 的 Init 改为"锁外建连、锁内发布"；
5. **R-04**：请求侧 `LimitReader`，gzip 解压限流，writer 改指针接收者。

### 迭代

- N-02 polaris 注册分支；N-03 XSS 空对象；N-06 postlog 回填；N-07 NewApp flag；
- R-05 关闭顺序（job 先于 DB）；R-11 Mongo closed 标记；R-10 JOIN 判空；R-13 限流 Header 桶上限；
- R-20 的 registry 活体测试门禁与 `SetRegistry` 统一。

---

## 7. 修复落地（2026-11-26，已提交）

本轮按 §6 修复了 5 项发布前 P1（并把同族的 N-04 一并处理）。生产代码改动随本次提交进入 `jh` 分支；以下状态与提交内容一致。

| 项 | 改动 | 回归验证 |
|---|---|---|
| R-01 | 删除 MySQL 发布点的 `closed` 短路；删除 SQLite 入口与发布点的 `closed` 拒绝。成功的候选连接现在可以复活已关闭的客户端，Reload 的 Close→Init 链路恢复。 | 新增 `db/sqlite/sqlite_reopen_test.go`：Init→Close→Init→建表/查询；`go test -race ./db/sqlite ./db/mysql` 通过。MySQL 成功重开离线不可构造，但发布点已无 `closed` 拒绝，路径唯一确定。 |
| N-01 | `Reload` 在 `initResources` 之后对 `ConfiguredRegistrar` 调用 `RegisterConfigured()`。 | 编译 + 全仓测试通过；未构造真实多注册中心环境。 |
| R-02 | 新增 `config.Snapshot` / `config.Current()`：在 RLock 内复制不可变快照；生产代码 176 处直接字段读改为 `Current()` 快照读；`GetConfigUrl/GetConfigData/getNacosConfigData/Validate` 改为在快照上执行。 | 新增 `config/snapshot_race_test.go`：100 轮 reload × 4 个读者，`-race` 通过。 |
| R-03 | Redis `Init` 改为锁外解析/建连/Ping、锁内发布；Ping 使用独立 5s ctx；顺带修复重载时复用旧 `conf` 导致新配置不生效的问题。 | 新增 `db/redis/redis_lock_test.go`：黑洞监听下 `IsMultiDB/ListConnNames` 在 200ms 内返回；`-race` 通过。 |
| N-04 | Mongo `Init` 同样改为锁外拨号、锁内发布；竞争失败时关闭候选连接。 | `go test -race ./db/mongo` 通过（无真实 Mongo）。 |
| R-04 | 请求体改 `io.LimitReader(max+1)`，超限部分用 `MultiReader` 回填；gzip 解压加 `LimitReader`；`Write/WriteString` 改指针接收者；`shouldSkipLog` 前移到包装 writer 之前；错误路径回填已读字节（同时修复 N-06）。 | 新增 `middleware/postlog/body_limit_test.go`：2MiB chunked 只读 max+1 字节、handler 仍能读到完整 2MiB；截断计数正确；小请求体回填。 |

提交前审阅补充：同一逻辑操作内的多次 `Current()` 已收敛为单次快照（`initResources`、`RunContext`、四个注册中心的 `Register`、`i18n.configuredStore`、`ratelimit.LoadConfigFrom`、JWT 中间件、postlog Mongo handler、VictoriaLogs 初始化），避免重载窗口内读到不同批次的字段；日志热路径从每行 2 次快照降为 1 次。

验证基线：

```text
go build ./...          # OK
gofmt -l .              # 空
go vet ./...            # 空
go test -count=1 ./...                     # 全绿（含 registry 活体用例，本轮 200s 内未超时）
go test -count=1 -race -timeout 240s ./...  # 全绿（含新增回归用例）
```

仍未处理（下一批）：**N-02** polaris 单注册中心分支、**N-03** XSS 空对象、**N-07** NewApp flag 重复注册、**R-05** 关闭顺序、**R-06** singleflight 旁路，以及 R-10/R-13/R-20 的剩余项。N-05/N-06 已随 R-04 顺带修复。

> 兼容性说明：`config.Config` 的导出字段仍保留，外部插件若继续直接读 `config.Config.<字段>`，在热重载并发下仍会竞态；框架内部已全部改用 `config.Current()`，伴生插件应跟进。

---

## 附录 A：本轮探针（均已删除）

### A.1 SQLite 关闭后重开

```go
func TestZZR3ReopenAfterClose(t *testing.T) {
    c := &Sqlite{}
    file := filepath.Join(t.TempDir(), "audit.db")
    if err := c.InitWithError(file); err != nil { t.Fatal(err) }
    c.Close()
    if err := c.InitWithError(file); err != nil { t.Fatalf("reopen: %v", err) }
    if _, err := c.GetConnection(); err != nil { t.Fatalf("get: %v", err) }
}
```

### A.2 config 导出字段竞态

一个 goroutine 循环 `Config.Init(a)`/`Config.Init(b)`，另一个循环直读
`Config.App.Name / Config.Config.Used / len(Config.Discovery.Registries) / Config.Logger.Outputs`；
`go test -race` 报 4~5 条 DATA RACE，写点 `configure.go:311-314`。

### A.3 Redis Init 持锁 I/O

黑洞 TCP 监听（accept 后不回包），配置 `go.data.redis.uri=127.0.0.1:PORT`、
`go.data.redis_pool.timeout=3`；Init 在 goroutine 中执行，200ms 后主协程调用
`IsMultiDB()`，实测阻塞 2.801s。

### A.4 postlog chunked 与截断标记

见正文 R-04：5 MiB chunked 被读入 5,242,880 字节；`bodyLogWriter` 写 10 字节/limit=4 后
`total==0 && truncated==false`。

### A.5 XSS 空对象

`mw.ConstructJson(XssMwJson{}, bytes.Buffer{})` 输出 `"}"`。

### A.6 NewApp 二次调用

同一进程连续调用 `NewApp(tempConfig, "audit", "1", false)` 两次，第二次 panic
`flag redefined: v`。

---

## 附录 B：验证局限

- 未连接真实 MySQL/Mongo/Redis/注册中心；MySQL 的"不可重开"由控制流唯一确定，SQLite 已用真实文件系统验证；
- 多注册中心 Reload 与 polaris 注册缺口为静态确定，未构造真实注册中心环境；
- v2 报告中 R-15～R-19 的 P3 项未逐条动态复现，仅做代码路径抽查；
- 本报告不覆盖 `registry/*` SDK 内部行为、casbin/jwt 鉴权正确性与外部插件。