# MGin `jh` 分支系统异常风险审计报告（v2）

- 审计日期：2026-11-26
- 审计范围：`github.com/maczh/mgin` 框架源码（分支 `jh`），聚焦 panic / OOM / 内存与协程泄漏 / 空指针 / 数据竞争
- 提交基线：`92ceac3`（"修复文档"）
- 工作区快照（初版）：HEAD 之上 10 个已跟踪文件被修改，`git diff` 指纹 `b08e1f9c9a9edfab5d29b72136cfcfda1c4618bc`
- 工作区快照（**本报告结论所依据的最新版**，同日 11:5x 复核）：17 个已跟踪文件被修改 + 2 个未跟踪测试新增，`git diff` 指纹 `e66e1127789b73e84b92cc2e56a62cc6b836f339`

```text
 M app.go   M cache/cache.go   M client/client.go   M config/configure.go   M config/configure_test.go
 M db/mysql/mysql.go   M db/mysql/mysql_lifecycle_test.go   M db/redis/redis.go   M db/sqlite/sqlite.go
 M i18n/api_test.go   M i18n/store_test.go   M mgin.go   M middleware/postlog/logger.go
 M middleware/xss/xss.go   M registry/registry.go   M utils/aesutil.go   M utils/ziputil.go
?? cache/cache_path_test.go   ?? utils/aes_boundary_test.go
```

> ⚠️ **审计期间工作区持续被并行修改**：注册中心访问器改造、AES/diskPath/zip 边界修复、postlog 体积上限、xss 共享写消除、i18n 与 config 用例契约修正，都是在本报告读取与验证过程中分批落地的。结论以指纹 `e66e1127…` 为准；上一版指纹 `b08e1f9c…` 的差异见 §2.1。复现或动手修复前，请先重新采集 `git status` / `git diff`。

## 0. 结论摘要

**框架层面已不存在"稳定可复现的 P0"**：v1 报告中的 Redis 无锁共享状态（进程级 fatal）已在当前工作区修复并通过 `-race` 压力验证。剩余的稳定性风险集中在 **生命周期与热重载** 两条主线：**配置热重载会把数据面永久打死**，以及 **默认中间件把请求/响应全量缓冲进内存**。

| 编号 | 严重度 | 位置 | 类型 | 触发面 | 工作区状态 |
|---|---|---|---|---|---|
| R-01 | P1 | `db/mysql/mysql.go:59`、`db/sqlite/sqlite.go:63` | 热重载后连接池永久失效 | 运维（`PUT /cfg/reload`） | 补丁未生效：SQLite 实测仍 `FAIL`，MySQL 守卫未变 |
| R-02 | P1 | `config/configure.go:310-314` | 导出字段数据竞争（含日志热路径） | 运维 + 并发流量 | 未修复，新快照重跑探针仍报 DATA RACE |
| R-03 | P1 | `db/redis/redis.go:41-43` | Init 持写锁做网络 I/O，读者全停 | 运维 + Redis 慢/不可达 | **工作区新引入**，实测阻塞 2.70s |
| R-04 | P1 | `middleware/postlog/logger.go:181-198`、`221-234` | 请求/响应体缓冲放大（OOM 面） | 远程（默认中间件） | **已部分修复**：响应与定长请求体封顶 1 MiB；chunked 请求体与 gzip 解压输出仍无界 |
| R-05 | P2 | `mgin.go:285-312` | 先关 DB 后停 job，关闭期写已关闭库 | 每次进程退出 | 未修复 |
| R-06 | P2 | `registry/cache.go:218-242` | 真实后端旁路 singleflight，缓存击穿 | 远程（TTL 到期/注册中心抖动） | 未修复 |
| R-07 | P2 | `client/client.go:428-440`、`client/client.go:93-96` | 发现调用不可取消 → 协程滞留；熔断表无上限 | 依赖故障 | 未修复 |
| R-08 | P2 | `i18n/i18n.go:18-30`、`mgin.go:146` | 每响应同步查库；重载后字典库连接失效 | 启用 i18n 的全部接口 | 未修复 |
| R-09 | P2 | `cache/memcache.go:132-146`、`mgin.go:323` | 无容量、无 TTL 条目永久驻留；缓存不随进程关闭 | 远程（动态 key） | 未修复 |
| R-10 | P2 | `db/dao/mysql.go:614-616` | nil `*gorm.DB` 链式调用 panic；`Alias` 拼接注入面 | DB 故障期 + 业务调用 | 未修复 |
| R-11 | P2 | `db/mongo/mgo.go:144-202` | `Close()` 后 `Check()`/`GetConnection()` 自动复活 | 每次自检 | 未修复 |
| R-12 | P2 | `job/scheduler.go:306`、`job/scheduler.go:930` | Stop 无界等待；handler 不协作则关不掉 | 每次进程退出 | 未修复 |
| R-13 | P2 | `middleware/ratelimit/limiter.go:116-125`、`config.go:151-186` | 限流参数按配置预分配内存；Header 维度键含凭据 | 配置 | 未修复 |
| R-14 | P3 | `middleware/xss/xss.go:94-98`、`xss.go:118-124` | multipart 双份全量缓冲；`fmt.Println/Printf` 直写 stdout | 远程（可选中间件） | **共享写已修复**，残余为缓冲与输出卫生 |
| R-15 | P3 | `middleware/session/session.go:30-65` | 包级变量被构造过程改写 | 多实例中间件 | 未修复 |
| R-16 | P3 | `logsink/sink.go:383-396`、`448-465` | 关闭等待协程滞留 | handler 阻塞 | 未修复 |
| R-17 | P3 | `app.go:110-124`、`app.go:517-529` | 健康检查协程 1s 超时后仍在访问已关闭资源 | 每次进程退出 | 未修复 |
| R-18 | P3 | `middleware/xlang/xlang.go:15-36` | 以 goroutine ID 传递请求语言 | 启用 i18n | 未修复 |
| R-19 | P3 | `logs/output.go:84-129` | 文件目标每行 open/write/close 且持全局锁 | 高 QPS | 未修复 |
| R-20 | P3 | `db/mysql/mysql_lifecycle_test.go:8`、`registry/cache_test.go`、`client/client_test.go` | 用例给出错误信心 / flaky / 测试绕过 `SetRegistry()` | CI | **多数已修**，余 3 项 |

### 已在工作区修复（未提交，需在合并前保留）

| 原结论 | 现状 | 验证 |
|---|---|---|
| Redis 共享状态无锁 → `concurrent map read and map write` fatal | `RedisClient` 增加 `sync.RWMutex`，`Init/Close/Check/GetConnection/IsMultiDB/ListConnNames` 全部持锁或快照 | 并发探针（Close/Get/Check/List 交织）`-race -count=5` 通过 |
| `AESBase64Decrypt` 非整块密文 panic | 新增 `len%blockSize` 校验 + `utils/aes_boundary_test.go` | 修复前我用探针稳定复现 panic，修复后用例通过 |
| `cache.diskPath` 单字符路径切片越界 panic | 新增 `len(cachePath) >= 2` 守卫 + `cache/cache_path_test.go` | 同上 |
| `registry.Registry` 全局接口裸读写 | 新增 `GetRegistry()/SetRegistry()`，`app.go`/`mgin.go`/`client.go` 改用访问器 | 生产代码已无裸访问；**仅测试文件仍在直接赋值 `registry.Registry`**（见 R-20） |
| `MySQLDao.All` 连接失败时空指针 | `connection()` 错误即返回 error，不再向下链式调用 | 代码走查 + `go test -race ./db/dao` 通过 |

### 已剔除 / 降级（v1 中的误报）

- **registry singleflight 永久 `WaitGroup` 泄漏**：`safeGetServiceURL` 已 `recover` 委托 panic，`c.active` 的 Add/Done 配对成立，未复现泄漏。真实缺陷是**去重被旁路**（R-06）与**跟随者误删他人 inflight 条目**（`registry/cache.go:289`），表现为多打一次后端。
- **`DiskCache` 读写锁误用会触发 fatal 并发写**：外层确实用了 `RLock`（`cache/diskcache.go:16-136`），但 bitcask 自身带内部同步，未观察到 fatal；列为并发卫生问题而非崩溃点。
- **job 自检会重建已关闭的数据源**：`Manager.Check()` 在未运行时直接返回 nil（`job/scheduler.go:317-323`），不会复活 DB；Mongo 的复活来自其自身 `Check()`（R-11）。
- **`client` 响应缓存无界**：`cachePut` 走 LRU 且有容量上限（`client/client.go:545-573`），不构成泄漏；无界的是熔断表（R-07）。

---

## 1. 审计方法与命令

### 1.1 静态定位

```bash
git status --short && git diff --stat          # 确认基线与在途改动
go build ./... && gofmt -l .                   # 编译与格式（均通过）
```

全仓 `config.Config.*` 导出字段直读：**141 处 / 25 个非测试文件**（`Grep 'config\.Config\.(App|Config|Log|Discovery|Jwt|Sys|Casbin|WorkDir)'`），这是 R-02 的规模量化依据。

`registry.Registry` 裸访问、`return nil` 后继续链式调用、`make(\.` 按配置分配、无 `Delete` 的全局 map，均按模式扫描定位。

### 1.2 测试与竞态

```bash
go build ./...                                                   # OK
gofmt -l .                                                       # 空
go test -race ./cache ./utils ./db/... ./registry ./client ./logsink ./job -count=1   # 全绿
go test -race ./registry -run TestCachedRegistry -count=20       # 2/20 失败（见 R-06）
go test ./i18n -run TestI18nAPI -count=1                        # 稳定 404（见 R-20）
```

`go test ./...` 与 `go vet ./...` 未作为验收门：前者包含硬编码内网 etcd/Consul 地址且无超时的用例（历史单次运行挂起 600s 后超时），后者在 `i18n/store_test.go:148-149` 报 `assignment copies lock value`（复制含 `sync.RWMutex` 的 `config.config`），属既有用例问题。

### 1.3 动态探针（跑完即删，全部只触达 127.0.0.1 或临时文件，未接触共享基础设施）

| 探针 | 目标 | 结果 |
|---|---|---|
| `db/sqlite/zz_audit_reopen_test.go` | 关闭后能否重开 | **FAIL**：`reopen after Close error = SQLite client is closed`，`GetConnection` 同样失败 |
| `config/zz_audit_race_test.go` | 重载期间直读导出字段 | **5 次 `WARNING: DATA RACE`**（410ms 探针窗口内） |
| `db/redis/zz_audit_lock_test.go` | Init 持锁期间的读阻塞（黑洞 TCP 监听） | `IsMultiDB()` 在 Init 的 3.00s 中被阻塞 **2.70s** |
| `db/redis` 并发探针（v1 已跑） | 复现 Redis fatal | `-race -count=5` 通过 → Redis P0 已修 |
| `utils`、`cache` 探针（v1 已跑） | AES / diskPath panic | 修复前复现，修复后由正式用例覆盖 |

探针文件已全部删除，`git status` 已回到仅含上述 10 改 + 3 未跟踪的状态。

---

## 2. 基线与工作区的差异（先读这一节，再读风险条目）

| 位置 | 提交基线 `92ceac3` | 当前工作区 |
|---|---|---|
| `db/redis/redis.go` | 共享 `clients/cfgs/conns/multi` 全程无锁 | 加 `sync.RWMutex`；`Check/GetConnection` 改为"锁内快照、锁外 Ping/返回"；`Close` 取消订阅并等待 ≤1s。**代价见 R-03** |
| `db/mysql/mysql.go` | `InitWithError` 入口即检查 `closed` | 删除入口检查，改在发布点检查；新增 `m.closed = false`。**意图（关闭后可重开）未达成，见 R-01** |
| `db/sqlite/sqlite.go` | `if closed { return }` | 改为 `if closed && opened`；发布时写入 `closed = false`。**同样未达成，已实测复现，见 R-01** |
| `db/mysql/mysql_lifecycle_test.go` | 断言"关闭即终态" | 断言改为"返回配置/DSN 错误"。**用例在补丁未生效时也会通过，见 R-20** |
| `registry/registry.go` | 仅 `var Registry` | 新增 `registryMu` + `GetRegistry()/SetRegistry()` |
| `mgin.go` / `app.go` / `client/client.go` | 裸读写 `registry.Registry` | 改用访问器 |
| `cache/cache.go` | `cachePath[1:2]` 无长度守卫 | 加 `len(cachePath) >= 2` |
| `utils/aesutil.go` | `CryptBlocks` 前无块长校验 | 加校验并返回错误 |

结论：**工作区修掉了 v1 的 5 项（Redis fatal、AES、diskPath、registry 接口、DAO.All），但 R-01 的修复没生效，且新引入 R-03。**

### 2.1 指纹 `b08e1f9c…` → `e66e1127…` 的增量（复核记录）

在初版报告完成后，工作区又推进了一批修复。逐条核对后的判定：

| 新增改动 | 位置 | 对应条目 | 判定 |
|---|---|---|---|
| `bodyLogWriter` 增加 `limit/total/truncated`，响应体封顶 1 MiB；`ContentLength > 1 MiB` 时不读请求体 | `middleware/postlog/logger.go:26-35`、`121-152`、`181-194` | R-04 | **部分修复**。`ContentLength == -1`（chunked）仍走 `GetRawData()` 全量物化；`shouldSkipLog()` 依然在 `c.Next()` 之后（`logger.go:249`） |
| `clone.BmPolicy` 在构造期归一，`callRemoveXss` 不再写 `mw.BmPolicy` | `middleware/xss/xss.go:94-98`、`118-124` | R-14 | **已修复**（共享写消除）。残余：`fmt.Println/Printf` 直写 stdout、multipart 双份缓冲 |
| i18n 用例改用真实路由 `/i18n/save|get|update|delete` | `i18n/api_test.go:53-95` | R-20-1 | **已修复**，`go test -race ./i18n` 现通过 |
| 三个活体配置用例加 `MGIN_LIVE_CONFIG_TESTS=1` 门禁 | `config/configure_test.go:7-16` | R-20-4 | **已修复**，硬编码内网地址不再是默认测试阻塞点 |
| `SafeExit` 删除 `job.Stop()` 之后的 `job.GetManager().Check()` | `mgin.go:301-311` | R-20-6 | **已修复** |
| `utils.Decompress` 增加 64 MiB 解压上限 | `utils/ziputil.go:81-98` | 解压放大 | **已修复**（zip bomb 类） |
| `config.Init` 收尾改用 `target := c` 局部别名发布 | `config/configure.go:177`、`307-319` | R-02 | **无效改动**。`c` 与 `Config` 是同一对象，锁语义完全没变；直读方也未收口 → R-02 仍开放 |
| `MySQLClient.InitWithError` 删除入口 `closed` 预检、发布点新增 `m.closed = false` | `db/mysql/mysql.go:48-66` | R-01 | **仍未修复**，且新增"关闭态下先真实拨号再丢弃连接池"的开销 |
| `Sqlite.InitWithError` 预检放宽为 `closed && opened`、发布点写 `closed = false` | `db/sqlite/sqlite.go:39`、`72` | R-01 | **仍未修复**（`sqlite.go:63` 的锁内检查照旧拒绝），重跑探针仍 `FAIL` |

---

## 3. P1

### R-01 配置热重载后 MySQL / SQLite 连接永久不可重开（在途补丁未生效）

位置：[db/mysql/mysql.go:58](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mysql/mysql.go#L58)-63 与 [db/sqlite/sqlite.go:62](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/sqlite/sqlite.go#L62)-69，配合 [mgin.go:146](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L146) 与 [mgin.go:201](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L201)-213

触发链：`PUT /cfg/reload` → `mgin.Reload()` → `closeResources()` 调 `db.Mysql.Close()` / `db.Sqlite.Close()`（把 `closed` 置为 true）→ `initResources()` 再调 `Init()`。

根因：两个客户端都在**取得写锁之后、复位 `closed` 之前**再次检查 `closed`，于是关闭状态永远无法被这次 `Init` 解除。

```go
// db/mysql/mysql.go:58-66
m.mu.Lock()
if m.closed {                       // Close() 之后必然为 true → 直接返回 ErrClosed
    m.mu.Unlock()
    closeConnections(candidate.mysql, candidate.mysqls, candidate.multi)
    return ErrClosed
}
oldSingle, oldMulti, oldIsMulti := m.mysql, m.mysqls, m.multi
m.mysql, m.mysqls, m.multi = candidate.mysql, candidate.mysqls, candidate.multi
m.closed = false                    // 永远到不了这里（关闭态）
```

SQLite 同构：`sqlite.go:39` 的 `closed && opened` 判断在 `Close()` 之后（`sqlite == nil`）必然放行，随后 `sqlite.go:63` 的 `if m.closed` 又必然拒绝，`sqlite.go:72` 的 `closed = false` 成为死代码。

影响：一次热重载后，进程内所有 MySQL/SQLite 访问永久返回 `mysql client is closed`，HTTP 层大面积失败，且 **`Check()` 也不会自愈**（`mysql.go:305-307`、`sqlite.go:125-127` 遇到 `closed` 直接返回错误），只能重启进程。原本用于"不停机改配置"的能力反而成为停机开关。

次生问题（新引入）：入口守卫删除后，关闭态下 `InitWithError` 会先执行 `candidate.init()`——**真实打开一套新的 MySQL 连接池**，再在发布点丢弃它。每次失败的重载都产生一轮无谓的连接建立与关闭抖动。

证据：
- SQLite 探针（临时用例，已删除）：
  ```text
  reopen after Close error = SQLite client is closed
  GetConnection after reopen conn=false error = SQLite client is closed
  REOPEN BROKEN: client cannot be reopened after Close
  ```
- MySQL 由控制流唯一确定（同上代码），未在共享数据库上验证：本地无可用 MySQL 实例，且 `gorm.Open` 默认 Ping，无法在离线环境构造"成功打开"的前置状态。

建议修复（择一，注意保持公开 API）：
1. 把"关闭即终态"和"重载换连接"分成两种语义：新增内部 `reopen()`/`reset()` 或在 `InitWithError` 的发布段用 `m.closed = false` 无条件替换，同时保留 `Close()` 之后的显式拒绝给外部调用者；
2. 更小的改动：`Reload()` 不要先 `Close()` 再 `Init()`——`InitWithError` 已经具备"候选成功后原子替换并关闭旧连接"的能力（`mysql.go:64-69`），`closeResources` 里跳过 MySQL/SQLite 即可，既避免关闭空窗，也避免 R-01 与 R-08 的整条链路。

无论采用哪种，都应补一个**能真正证明"重开成功"**的用例（SQLite 最省事，见 R-20）。

---

### R-02 配置对象的导出字段在热重载期间被无同步读取（含日志热路径）

位置：[config/configure.go:22](file:///Users/macro/Work/go/src/github.com/maczh/mgin/config/configure.go#L22)-34、[config/configure.go:303](file:///Users/macro/Work/go/src/github.com/maczh/mgin/config/configure.go#L303)-318，读取方示例 [logs/output.go:155](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/output.go#L155)-156

`config` 结构同时具备**私有 `mu sync.RWMutex`** 和**一批导出字段**（`App / Config / Log / Logger / Discovery / Jwt / Sys / Casbin / Cnf / WorkDir`）。`Init()` 的收尾在锁内整体重发布这些字段；但 accessor（`GetConfigString` 等）之外的**直读遍布全仓：141 处 / 25 个非测试文件**。

代码注释本身已承认这个契约——"direct public-field reads are retained for compatibility and must not race with Init"——但契约并没有被任何机制保证。

实测证据（临时探针，一个 goroutine 反复 `Config.Init()`，其余按业务方式直读导出字段）。**在最新指纹 `e66e1127…` 上重跑，结论不变**（`target := c` 只是别名，锁覆盖范围没变）：

```text
WARNING: DATA RACE
  Write at 0x00c000245950 by goroutine 9:
    config.(*config).Init()  config/configure.go:312      // target.Config = candidate.Config
    zz_audit_race_test.go:66                             // 另一个 goroutine 里的 Config.Init(f1/f2)
  Previous read ...            Config.Config.Used 等直读
WARNING: DATA RACE
  Write at 0x00c000245af0 by goroutine 9:
    config.(*config).Init()  config/configure.go:314      // target.Logger = candidate.Logger
--- FAIL: TestZZAuditConfigFieldRace (0.00s)   // 单次运行 2 条，多次运行 2~5 条，取决于调度
```

危害分级：
1. `configure.go:314` ↔ `logs/output.go:155-156`：**每一条 `logs.*` 调用**都会读 `Config.Logger.Outputs` 这个 slice。写侧在锁内、读侧无锁 → 读方可能拿到**撕裂的 slice 头**（数据指针来自新数组、长度来自旧数组）→ 下标越界 panic 或静默脏读。**panic 发生在日志函数里 = 发生在业务请求协程里**，且 `recover` 之后仍会打出错误日志、再次触发同一路径。
2. `configure.go:312/315` ↔ `config.Config.Config.Used`（`mgin.go:138`、`checkAll`）、`len(config.Config.Discovery.Registries)`（`mgin.go:303` 等处）：同类撕裂；`Used` 是 string，读方还可能拿到"指针新、长度旧"的组合 → 越界或错值。字符串的撕裂读比 slice 更隐蔽，因为它不会每次 panic，只会偶发错判（例如热重载后 `checkAll` 少检/多检组件）。
3. `configure.go:309`（`target.Cnf`）↔ 所有 `config.Config.Cnf.*` 直读（如 `middleware/ratelimit/config.go:118-127`、`middleware/postlog`）：`LoadConfigFrom` 先 `Exists(prefix)` 再 `Unmarshal`，两次调用之间 `Cnf` 可被替换 → 前半读旧实例、后半读新实例，得到"配置存在但解析为空"的假象（限流规则被静默丢弃）。
   注：candidate 阶段对 `c.Logger.Outputs`（`configure.go:249-252`）与 `c.Discovery.Registries`（`configure.go:259`）的原地填充发生在**未发布的私有 candidate** 上（`Init` 每次都新建 candidate，见 `configure.go:182-192`），因此不构成"边填边被读"的竞态——v1 里对该行的判断需要按此修正；真正的暴露面只在发布写与无锁直读之间。

影响面：任何在热重载时仍有流量的服务；`go.cors.headers`、`go.application.port`、`Log.Get`、`VictoriaLogs.Client` 等开关都会读到"半新半旧"的组合，导致同一次重载后行为不一致（例如 CORS 用旧值、限流用新值）。

建议修复：
- 读侧统一走 accessor（新增 `config.AppName()`、`config.LoggerOutputs()` 快照等），或用 `atomic.Pointer[config]` 整体替换，使导出字段仅作为兼容门面存在；
- 若必须保留直读，退而求其次：`Init()` 全程持写锁、并在锁内完成 candidate 构造与发布（当前锁只覆盖最后 10 行发布），使 accessor 读与直读不会撞上"候选已发布但后备数组仍在填"的窗口；
- 无论哪种，日志热路径（`logs/output.go:155`）应先在本地取快照再遍历。

### R-03 Redis `Init` 在写锁内做网络 I/O，慢 Redis 会停住全部读者（工作区新引入）

位置：[db/redis/redis.go:41](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/redis/redis.go#L41)-43、[db/redis/redis.go:170](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/redis/redis.go#L170)-180

为消除 R-级并发 map 问题，工作区给 `Init` 加了 `r.mu.Lock(); defer r.mu.Unlock()`，但 `Init` 内部**逐个 db 串行 `redis.NewUniversalClient` + `Ping()`**，且 `DialTimeout` 默认 60s（`redis.go:158-161`）。所有 `GetConnection()/Check()/IsMultiDB()/ListConnNames()` 都需要 `RLock`，于是全部排队。

实测（黑洞 TCP 监听，`database: 0`，单库）：

```text
IsMultiDB returned after 2.701s while Init was running
Init took 3.002s
0 Redis连接失败: read tcp 127.0.0.1->127.0.0.1: i/o timeout
```

一次纯读操作被完全阻塞至 Redis 超时。多库 + 不可达 + 60s 拨号超时的生产配置下，热重载期间的停顿可达**分钟级**，期间所有依赖 Redis 的请求协程挂起堆积 → 内存增长、健康检查超时、K8s 重启连锁。

影响：把原本"并发不安全"换成了"并发不可用"。这是补丁引入的新风险，基线代码不存在该阻塞（基线是竞态）。

建议修复：把"构造候选客户端"与"发布"分离——在锁外完成解析、`NewUniversalClient` 与 `Ping`，只在替换 `clients/cfgs/conns/multi` 时取写锁（与 `MysqlClient.InitWithError` 的 candidate 模式一致）。同时给 `Init` 内部的 Ping 设定显式短超时，而不是沿用 60s 拨号超时。

### R-04 默认接口日志中间件缓冲请求体与响应体（工作区已封顶 1 MiB，chunked 请求体仍无界）

位置：[middleware/postlog/logger.go:181](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/postlog/logger.go#L181)-194、[logger.go:121](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/postlog/logger.go#L121)-152、[logger.go:249](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/postlog/logger.go#L249)，挂载点 [app.go:183](file:///Users/macro/Work/go/src/github.com/maczh/mgin/app.go#L183)

**当前状态（`e66e1127…`）**：`maxLoggedBodyBytes = 1 << 20` 已引入，响应侧 `Write/WriteString` 只保留前 1 MiB 并打 `<body truncated: N bytes>`，请求侧在 `ContentLength > 1 MiB` 时整体跳过读取。这把"单请求内存与响应体大小成正比"降到"与 1 MiB 成正比"——**主路径已封堵，残余见下**。

残余风险（按危害排序）：

1. **`ContentLength == -1`（chunked 上传）绕过守卫**：判据是 `c.Request.ContentLength >= 0 && > maxLoggedBodyBytes`（`logger.go:188`）。分块请求没有 `Content-Length`，直接落入 `else` 分支 → `c.GetRawData()` 仍把整个请求体读进内存，再 `string(data)` 复制一份、`ioutil.NopCloser(bytes.NewBuffer(data))` 回填第三份（`logger.go:191-196`）。**客户端只要用 `Transfer-Encoding: chunked` 就能重新拿到无上限缓冲**，而这正是大文件上传客户端（含 SDK、代理转发）常用的编码方式。
2. **1 MiB 是"每个在途请求"而非全局**：`limit` 挂在每请求的 `bodyLogWriter` 上。并发 5k 时响应日志缓冲单独就占 ~5 GiB 上限。缺少全局预算或按路径关闭的开关。
3. **`shouldSkipLog()` 仍在 `c.Next()` 之后执行**（`logger.go:249`）：不记日志的路径依然付出了包装 writer、1 MiB 缓冲与 `GetRawData` 的代价。
4. **gzip 响应解压没有输出上限**：`responseBody` 取的是已截断的缓冲（≤1 MiB），但随后 `gzip.NewReader` + `io.ReadAll(r)` 对这段字节做全量解压（`logger.go:221-234`）。1 MiB 的高压缩比 gzip 可膨胀到数百 MB～GB 级，解压结果再 `string(rBody)` 复制一份。上游可控响应体（反向代理、文件内容接口）即构成压缩炸弹放大点。
5. **单请求仍有 2~3 份缓冲**：包装 writer（1 MiB）+ `bodyLogWriter.body.String()` 产生的 string 副本 + gzip 解压副本；`utils.GinParamMap/GinHeaders`（`logger.go:200-207`）在每个请求上再构造两份 map，且**无条件执行**（即使该路径最终被 `shouldSkipLog` 跳过）。

危害分级：修复后 R-04 从"远程可触发的 OOM 等价 P0"降级为 **P1**——常规路径已被 1 MiB 封顶，但 chunked 请求体、gzip 解压输出与并发×单请求预算三个放大面仍在。

建议修复（在已完成的改动之上补齐）：
- 请求侧改为 `io.LimitReader(c.Request.Body, maxLoggedBodyBytes+1)` 读取，读满即截断并回填"已截断"标记，取消对 `ContentLength` 的依赖；
- gzip 分支改用 `io.ReadAll(io.LimitReader(r, maxLoggedBodyBytes+1))`，超限只记录长度；
- 把 `shouldSkipLog()` 与 `logsink.HasHandlers()` 前移到包装 writer 与 `GinParamMap/GinHeaders` 之前，无消费者时零开销；
- 若要严格控制并发内存，`maxLoggedBodyBytes` 应可配置，并对上传/导出类路径默认关闭请求体记录。

与 R-02 的叠加：`getResponseLogMode()` 与 `config.Config.Log.Get` 的读取本身也是无同步直读，热重载时可能读到"半新半旧"的模式开关。

### R-05 `SafeExit` 先关数据库、后停定时任务

位置：[mgin.go:285](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L285)-312（顺序），关联 [job/scheduler.go:1073](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L1073)-1095

关闭顺序是 `postlog.Close()` → `i18n.Close()` → **MySQL/SQLite/Mongo/Redis.Close()** → 注册中心 → **`job.Stop()`** → 插件。也就是说：数据库连接池已经 `Close()`，调度器与其中的任务仍在跑，`asyncUpdateFire/asyncUpdateNext` 还会 `safego.Run` 出**不受 WaitGroup 跟踪**的写库协程（`scheduler.go:1090`、`1110`）。

这正是最初那次生产 panic 的形态（`schedule.preoccupyScan → DAO → GORM`，报 `sql: database is closed`）。DAO 侧的 `All/Where` 已加了连接错误防御，所以现在是**错误与脏日志**而非 panic；但两条残留路径仍在：
- `MySQLDao.JOIN` 仍会把 nil 连接继续链式调用（R-10）；
- 关闭期写调度元数据失败 → 下次启动看到过期的 `next_fire_time`，出现漏跑/重复跑。

建议修复：`SafeExit` 调整为 `postlog.Close()` → `job.Stop()`（等待在跑任务收尾）→ 取消并回收异步落库协程 → 注册中心 → 数据源 → 插件；`asyncUpdate*` 用受管 WaitGroup + 关闭标志跳过。

### R-06 服务发现缓存层对真实后端旁路了 singleflight

位置：[registry/cache.go:218](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L218)-242、[registry/cache.go:243](file:///Users/macro/Work/go/src/github.com/maczh/mgin/registry/cache.go#L243)-294

`GetServiceURL` 先走 `InstanceProvider` 分支（`cache.go:218`）：**只要委托实现了 `GetServiceInstances`，就直接回源，完全不查 `inflight`**。而 nacos / etcd / consul / polaris 四个真实后端**都实现了** `GetServiceInstances`（`registry/nacos/nacos.go:324`、`registry/etcd/etcd.go:212`、`registry/consul/consul.go:183`、`registry/polaris/polaris.go:222`）。

结果：精心写的 singleflight 段（`cache.go:243-294`）在生产路径上是死代码。缓存 TTL 到期或注册中心抖动时，同一服务的 N 个并发请求会各自打一次注册中心；配合 `c.active` 计数，`DeRegister()` 需要等待这批回源全部结束。注册中心变慢时，这直接放大成"每个请求一个阻塞式发现调用"的踩踏。

另一个真实缺陷（与 R-20 的 flaky 用例同源）：跟随者在未取到缓存时会落到 `cache.go:289` 的 `delete(c.inflight, key)`，**无条件删除该 key 当前的 inflight 条目**——此时该条目可能属于别的 goroutine 新登记的 leader，导致后续请求无法再去重，出现 2 次回源。

证据（`-race -count=20`，稳定可复现）：

```text
go test -race ./registry -run TestCachedRegistry -count=20
--- FAIL: TestCachedRegistry_SingleFlightDedup
    并发首查应被 singleflight 合并为1次回源，实际: 2      ×2/20
```

建议修复：把去重上提，使 `InstanceProvider` 分支同样在 `inflight` 保护下执行；`delete(c.inflight, key)` 改为仅由登记该条目的 leader 执行（比对指针/序号）；`registry/cache_test.go` 的断言改为覆盖真实后端（实现 `GetServiceInstances` 的 fake）。

### R-07 跨服务调用：发现不可取消导致协程滞留 + 熔断表无上限增长

位置：[client/client.go:428](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L428)-440、[client/client.go:93](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L93)-96 与 [client/client.go:151](file:///Users/macro/Work/go/src/github.com/maczh/mgin/client/client.go#L151)-153

1. **接口层没有 context**。`RegistryClient.GetServiceURL(service, group...)`（`registry/registry.go:34-38`）无 ctx 参数，`CachedRegistry` 也没有 `GetServiceURLFromContext`，因此 `client.go:293` 的 `NamedRegistryResolver` 断言失败，永远走 `resolveLegacyWithContext`：它把发现调用丢进一个协程，`select` 到 `ctx.Done()` 就返回——**但被丢弃的协程仍在等注册中心返回**（`client.go:432-439`）。注册中心长时间不可达时，每个超时的出站调用留下一个悬挂协程及其缓冲，属真实泄漏路径。
2. **熔断状态表只增不减**。键为 `op.CircuitKey` 或 `discoveryService + "\x00" + uri`（`client.go:151-153`），条目在 `circuitAllow` 中创建（`client.go:195-198`），全仓无删除逻辑。若 `uri` 含动态段或查询串（业务里 `client.Call(svc, "/api/x/"+id, op)` 很常见），该 map 随不同 URI 单调增长，永不回收。

对照组：出站响应缓存 `cachePut` 有 LRU 上限与过期清理（`client.go:545-573`），说明作者知道该模式，只是没用在熔断表上。

建议修复：`RegistryClient` 增加可选的 context 版本接口并让 `CachedRegistry` 实现（`resolveWithContext` 已具备分派逻辑）；熔断表加上容量上限或空闲淘汰，并对 `uri` 做模板归一（与 `ratelimit.routeOf` 同思路）。

### R-08 i18n：每个响应同步查库，且热重载后字典库连接失效

位置：[i18n/i18n.go:18](file:///Users/macro/Work/go/src/github.com/maczh/mgin/i18n/i18n.go#L18)-30、[i18n/store.go:244](file:///Users/macro/Work/go/src/github.com/maczh/mgin/i18n/store.go#L244)-257、[mgin.go:146](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L146)

`GetString()` 每次调用都开一个 **3s 超时**的 ctx 去查库，`Translate()` 未命中还会**再查一次默认语言**（`store.go:250-252`），全程无任何进程内缓存。`String()/Format()/Error()/Success()` 是所有响应封装的入口，于是：

- 一次正常接口响应至少 1～2 次字典 SQL；
- **异常路径更糟**：`app.go:531-533` 的 `recoveryHandler` 与 404 处理都会调 `i18n.Error(...)`。数据库本身故障时，每个 panic 请求还要再打一次必然失败、最长 3s 的查询 → 请求协程成片滞留（OOM 放大器），这正是"故障时更慢"的经典形态。
- 热重载时 `closeResources()` 关闭 MySQL 但**不重建 i18n store**（`Reload()` 中无 `i18n.Close()`/`Init()`，只有 `SafeExit` 才 `i18n.Close()`），`currentStore` 继续持有已关闭连接池 → 之后每个响应走"查询失败 → 回退 messageId 原文"，多语言能力静默丢失且不可自愈。

建议修复：加进程内字典缓存（`sync.Map` 或既有 `cache.OnGetCache("i18n")` + TTL + 写操作失效）；`Translate` 失败与"未命中"分离，异常时熔断降级到默认语言/原文而不是每次都查库；把 i18n store 纳入 `Reload()` 的资源重建清单；`recoveryHandler`/`NoRoute` 使用不触发远程查询的快速回退路径。

### R-09 内存缓存无容量上限、无 TTL 条目永不回收，且进程退出时不关闭缓存

位置：[cache/memcache.go:132](file:///Users/macro/Work/go/src/github.com/maczh/mgin/cache/memcache.go#L132)-146、[memcache.go:87](file:///Users/macro/Work/go/src/github.com/maczh/mgin/cache/memcache.go#L87)-95、[cache/cache.go:75](file:///Users/macro/Work/go/src/github.com/maczh/mgin/cache/cache.go#L75)-92、[mgin.go:323](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L323)

- `Set(key, value, 0)` 使 `expires = 0`，而清理协程只删 `item.expires > 0 && now > item.expires`（`memcache.go:90`）→ **零 TTL 条目永久驻留**，且没有任何容量/LRU 上限；`Add()` 还会 `clone.Clone(value)` 深拷贝一份常驻。
- `OnMemCache(name)` 把实例存进全局 `mc.cache`（`cache.go:83-91`），`New(time.Minute)` 会为每个新名字起一个清理协程。若调用方使用**动态缓存名**（按租户/日期/请求命名），名字与协程同时无界增长。
- `OnDiskCache(path)` 同理：每个不同 path 打开一个 bitcask 目录句柄并常驻 `mc.db`（`cache.go:45-60`）。
- `CloseCache()` 已实现（`cache.go:108-121`）但在 `SafeExit` 中被注释掉（`mgin.go:323`）→ 进程退出既不 flush 也不关闭 bitcask 文件句柄。

影响：长时间运行进程的 RSS 单调上升；bitcask 文件句柄不关闭还可能触发 `ulimit`。

建议修复：`MemCache` 增加容量上限与淘汰策略（或明确文档化"零 TTL = 永久，调用方自负"并禁用该入口）；`OnGetCache/OnDiskCache` 的名字/路径改为白名单登记制；恢复 `cache.CloseCache()` 到 `SafeExit`（放在数据源关闭之前，因为接口日志的 MongoDB handler 可能依赖缓存）。

### R-10 `MySQLDao.JOIN` 对 nil 连接继续链式调用；`Alias` 存在标识符拼接注入面

位置：[db/dao/mysql.go:614](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L614)-616、[mysql.go:594](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L594)-602、[mysql.go:247](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/dao/mysql.go#L247)-255

`Where()/Alias()/DB()` 在连接失败时**返回 nil `*gorm.DB`**，`JOIN()` 直接 `receiver.Alias(alias).Joins(query, args...)`。对 nil `*gorm.DB` 调用方法会进入 `db.getInstance()` 并解引用 `db.clone` → panic。框架内部无 `.JOIN(` 调用方，所以暴露面在业务代码；DB 故障或 R-01 生效后，这条路径就是"错误被转换成 panic"的通道（gin 的 recovery 中间件能兜住请求，但会污染错误语义，且 `recoveryHandler` 还会触发 R-08）。

同类问题：`Alias()` 用 `fmt.Sprintf("%s AS %s", e.TableName(), alias)` 拼接表名与别名并交给 `Table()`。`alias` 若来自请求参数（列表接口"排序别名字段"这类需求很常见），即为 SQL 注入。

建议修复：`JOIN/Alias/DB` 在连接失败时返回 `nil` 的同时，为 `JOIN` 增加 nil 守卫（返回 error 或空 `*gorm.DB` 的显式失败对象），并保持其余方法的错误约定一致；`alias` 走 `^[A-Za-z0-9_]{1,64}$` 白名单校验。另外 `WithContext(*context.Context)`/`WithCache` 直接改接收者字段（`mysql.go:99-120`），若 DAO 作为包级共享变量复用即产生字段竞态与跨请求 ctx 串用——建议改为返回值副本（与 `Debug()/WithTx()` 一致，后者已经是拷贝语义）。

---

## 4. P2 / P3

### R-11 Mongo：`Close()` 之后会被自检自动复活（P2）

位置：[db/mongo/mgo.go:128](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L128)-142、[mgo.go:144](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L144)-191、[mgo.go:193](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L193)-202、[mgo.go:228](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mongo/mgo.go#L228)

`Mongodb` 有 `mu sync.RWMutex` 但**没有终态 `closed` 标志**。`Close()` 只是把 `conns` 换成空 map 并断开连接；随后：
- `Check()` 发现 `len(m.conns) == 0` 就 `m.Init(confData)`（`mgo.go:202`）；
- `mgoCheck(tag)` 同样在缺失时 `Init`，并在 Ping 失败时**替换后在锁外关闭旧 session**（`mgo.go:173-188`）；
- 非多库的 `GetConnection()` 每次调用都走一遍完整 `Check()`（`mgo.go:228` 起）→ 每个请求一次 Ping。

影响：
1. `SafeExit` 关闭 Mongo 之后，`app.go` 的 5 分钟自检协程（见 R-17）仍可能在退出窗口内把 Mongo **重新连回来**，进程带着"已关闭又被重建"的连接池退出；
2. 每请求 Ping 是隐性延迟与注册风暴放大器；
3. Ping 失败即重建 session，与 MySQL 侧"探活不再关闭共享池"的加固方向不一致。

建议修复：与 `MysqlClient` 对齐——引入终态 `closed`（`Close()` 置位，`Check/GetConnection` 遇 `closed` 直接返回错误），`InitWithError` 风格的成功后再发布；`GetConnection` 不再内联全量 `Check`，改由自检协程驱动。

### R-12 定时任务：`Stop()` 无界等待 + 关闭后仍有未跟踪的写库协程（P2）

位置：[job/scheduler.go:306](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L306)、[scheduler.go:925](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L925)-931、[scheduler.go:1073](file:///Users/macro/Work/go/src/github.com/maczh/mgin/job/scheduler.go#L1073)-1095

- `Manager.Stop()` 依次 `close(stopChan)` → `jr.cancelAll()` → `m.wg.Wait()`，**没有任何超时**。而 `runOnce` 在 `baseCtx.Done()` 分支里还要再 `<-done` 等待 handler 协程真正退出（`scheduler.go:925-931`，注释明确说明这是为了保住 active/信号量所有权）。结论：**任何一个不遵守 `ctx` 取消的 handler，都会让进程永远关不掉**，最终被 SIGKILL，正在处理的 HTTP 请求随之硬中断。MGin 无法强制业务协作，但至少要有一个可配置的退出预算。
- `asyncUpdateFire/asyncUpdateNext` 用 `safego.Run` 起协程写库，**不计入 `m.wg`**（`scheduler.go:1090`、`1110`）→ `Stop()` 返回后仍可能执行，叠加 R-05 的关闭顺序，就是"向已关闭的 MySQL 写调度元数据"。
- 次一级的并发卫生：`BlockSerial` 的队列上限是 `LoadInt32(...) >= maxQueue` 之后 `AddInt32(..., 1)`（`scheduler.go:697-703`、`969-974`），非原子，并发提交可短暂超过上限；`BlockDiscard` 先查 `jr.active > 0` 再放行，而 `active` 要到拿到 serial/全局槽位后才自增，两个并发触发可同时通过 → 同任务重复执行。二者都不会崩溃，但会让"串行/丢弃"语义在抖动时失真。

建议修复：`Stop(ctx)` 或 `StopWithTimeout(d)`，超时后记录并放弃等待（保持 `Stop()` 兼容签名）；`asyncUpdate*` 纳入 WaitGroup 并在 `running == false` 时跳过；串行/丢弃准入改为 CAS 或 `mu` 保护下的判定。

### R-13 限流中间件：按配置预分配内存、Header 维度键含凭据、独立管理器不回收（P2）

位置：[middleware/ratelimit/limiter.go:116](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/limiter.go#L116)-125、[limiter.go:268](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/limiter.go#L268)、[limiter.go:206](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/limiter.go#L206)-211、[config.go:151](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/config.go#L151)-186、[ratelimit.go:186](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/ratelimit.go#L186)、[ratelimit.go:215](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/ratelimit/ratelimit.go#L215)-219

1. `newSlidingWindow(int(r.Rate), ...)` 直接 `make([]int64, limit)`，`newConcurrencyLimiter(max)` 直接 `make(chan struct{}, max)`；`Rule.normalize()` 只补默认值、**不设上限**（`config.go:151-186`）。一条 `rate: 100000000` 的滑窗规则 → 单个限流器 800MB，且**按维度键逐个分配**（`DimIP`/`DimIPPath` 下每个 IP 一个）。配置手误即可打爆内存。
2. `DimHeader` 的键是 `r.Name + "|h:" + key + ":" + c.GetHeader(key)`，`HeaderKey` 缺省即 `Authorization`（`ratelimit.go:175-187`）→ **原始 token 被当作 map 键常驻**，并在 `X-RateLimit-*`/调试日志场景下有外泄面；基数取决于 token 轮换频率，回收要等 `idleTimeout`（默认 600s）。
3. `RateLimitWith(rules...)` 每次调用 `newManager(cfg)` → `startGC()` 起一个后台协程，返回的中间件没有任何停止入口（`ratelimit.go:215-219`，对比 `Stop()` 注释"通常无需调用"）。在路由组循环里构造会稳定泄漏协程。

正面结论：路径维度用 `c.FullPath()` 模板而非实际 URL（`ratelimit.go:189-195`），因此**不存在**路径参数导致的限流器爆炸；`gc()` 会清理空闲限流器。

建议修复：`normalize` 给 `Rate/Burst/MaxConcurrent` 设硬上限（或改用计数型滑窗，不预分配 `limit` 大小的环）；Header 维度改用 `sha256(header)[:16]` 之类的摘要作键；`RateLimitWith` 提供 `Stop()` 或在文档中要求进程级单例。

### R-14 XSS 中间件（残余）：multipart 双份全量缓冲；错误路径直写 stdout（P3）

位置：[middleware/xss/xss.go:118](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L118)-131、[xss.go:378](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xss/xss.go#L378) 起

**已修复部分**：工作区把 `FieldsToSkip` 改为构造期拷贝（`xss.go:94-95`），并把 `BmPolicy` 的默认值归一移到 `clone` 上（`xss.go:96-98`），`callRemoveXss` 内不再出现 `mw.BmPolicy = ...` 的写（原 `xss.go:117-118`），闭包改为 `clone.callRemoveXss(c)`（`xss.go:111`）→ **共享字段写-写/写-读竞态已消除**，中间件实例可安全并发复用。

**残余**：
1. `HandleMultiPartFormData` 把每个 part 读进 buffer、再把重建后的表单整体读进第二个 buffer（`xss.go:378` 之后），与 R-04 同类，是上传体积放大器；
2. 策略非法与 XssRemove 错误路径用 `fmt.Println` / `fmt.Printf` 直写 stdout（`xss.go:121`、`129`），每请求一次，绕过 `logs`/logsink，既污染结构化日志又可在错误被刷时放大 I/O。

该中间件为可选挂载，故列 P3。建议：multipart 改为流式重写或加体积上限；两处 stdout 改为 `logs.Error`。

### R-15 session 中间件用包级变量保存 gin 键（P3）

位置：[middleware/session/session.go:30](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/session/session.go#L30)-32、[session.go:53](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/session/session.go#L53)-65、[session.go:88](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/session/session.go#L88)-112

`storeKey/manageKey` 是包级 `string`，`NewWithConfig` 在构造时写它们，`FromContext/Manage` 在读它们。两个不同键配置的同名中间件（或"构造与首请求并发"）会产生竞态，更糟的是**后构造者会全局改变先前中间件的取值语义**——读到 nil 或错误类型时静默返回 `false`，表现为 session 莫名丢失。建议把键收进实例字段，`FromContext(c, key)` 显式传入。

### R-16 logsink 关闭等待协程滞留（P3）

位置：[logsink/sink.go:383](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/sink.go#L383)-396、[sink.go:448](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logsink/sink.go#L448)-465

`waitTimeout` 用 `safego.Run(func(){ wg.Wait(); close(done) })` 实现等待超时：超时返回后，**该等待协程与仍在写下游的消费协程都会继续存活**（直到 handler 自行返回）。`Close()` 会调用 handler 的 `Close()`，因此正常情况下能收敛；只有 handler 阻塞在网络写时才会残留。队列本身有界（1024，满则淘汰最旧并计数，`sink.go:358-380`），handler panic 有 recover，所以**不构成 OOM 或崩溃源**——这也是 v1 把它降级的原因。建议在文档中明确"退出预算 `go.log.sink.shutdown` 用尽后日志可能丢失"，不要试图强杀 handler。

### R-17 健康检查协程与资源关闭的竞争窗口（P3）

位置：[app.go:110](file:///Users/macro/Work/go/src/github.com/maczh/mgin/app.go#L110)-124、[app.go:517](file:///Users/macro/Work/go/src/github.com/maczh/mgin/app.go#L517)-529、[mgin.go:240](file:///Users/macro/Work/go/src/github.com/maczh/mgin/mgin.go#L240)-276

`NewApp` 起一个 5 分钟自检协程，`checkAll()` 会对 MySQL/Mongo/Redis 及各插件做 Ping（阻塞式网络 I/O），并**无同步直读 `config.Config.Config.Used`**（`mgin.go:242`，属 R-02 的一部分）。`stopHealthChecker()` 只等 1 秒就放弃（`app.go:521-528`），随后 `SafeExit()` 立刻关闭连接池 → 自检协程可能正拿着即将关闭的 `*sql.DB` 做 Ping，产生退出噪声；配合 R-11，Mongo 还可能在此窗口被重新初始化。建议把自检协程改为持有各自资源的"快照 + ctx"，并把退出预算与 `SafeExit` 合并，而不是 1 秒后弃之。

### R-18 xlang 以 goroutine ID 传递请求语言（P3）

位置：[middleware/xlang/xlang.go:15](file:///Users/macro/Work/go/src/github.com/maczh/mgin/middleware/xlang/xlang.go#L15)-36

`RequestLanguage()` 用 `getGoroutineID()` 作 key 写入全局缓存，`GetCurrentLanguage()` 再按当前 goroutine ID 读取。后果：业务在 handler 内 `go func()` 起的子协程**取不到请求语言**（退化为默认语言，进而影响 R-08 的字典查询结果）；goroutine ID 会被运行时复用，理论上存在跨请求串读。加上每次调用都要 `runtime.Stack` 解析（`xlang.go:38-49`）且 `Add/Value` 走深拷贝。建议改用 `context` 或 gin 的 `c.Set/c.Get` 传递；至少在文档中明确"子协程需显式传 ctx"。

### R-19 日志文件目标：每行一次 open/write/close 且全程持全局锁（P3）

位置：[logs/output.go:84](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/output.go#L84)-129（尤其 `106-119`）、[output.go:61](file:///Users/macro/Work/go/src/github.com/maczh/mgin/logs/output.go#L61)-82

`writeRecord` 的 `file` 分支在 `outputMu` 保护下 `MkdirAll + OpenFile + Write + Close`，即**每条日志 4 次系统调用 + 一把全局互斥**；console 目标同样持锁。并且调用发生在业务协程里（`emit()` 同步执行），磁盘抖动会直接拖慢请求。另外 `captureRecord` 每行都 `runtime.Callers(2, 16帧)` + `CallersFrames` 逐帧解析定位调用者（`output.go:63-80`），`enabled()` 每行新建一个 `map[string]int`（`output.go:142`）。这是吞吐与 GC 压力问题，不是崩溃风险，但会放大 R-02（热路径读 `Logger.Outputs`）。建议常驻 `*os.File` + 按日轮转、异步化（复用 logsink 总线），并把 rank 表提为包级变量。

### R-20 测试与契约问题（P3，但会误导修复）

1. **`i18n` API 用例稳定失败**：`RouterGroup` 注册的是 `/i18n/list|get|save|update|delete`（[i18n/api.go](file:///Users/macro/Work/go/src/github.com/maczh/mgin/i18n/api.go)），而 [i18n/api_test.go:53](file:///Users/macro/Work/go/src/github.com/maczh/mgin/i18n/api_test.go#L53) 首个操作就打 `POST /i18n` → `HTTP status=404: 404 page not found`，`go test ./i18n` 必红。要么补路由，要么改用例；现在它掩盖的是"契约不一致"。
2. **`TestMysqlClientCanValidateReopenAfterClose` 给出虚假信心**（[db/mysql/mysql_lifecycle_test.go:8](file:///Users/macro/Work/go/src/github.com/maczh/mgin/db/mysql/mysql_lifecycle_test.go#L8)-16）：用例只断言"返回的不是 `ErrClosed`"。因为假 DSN 在 `candidate.init()` 阶段就失败，**根本走不到发布点的 `closed` 检查**，所以 R-01 的实际缺陷（关闭后无法重开）它抓不到。用例名还暗示"可重开"，容易让后续维护者误判已修。建议用 SQLite 或 sqlmock 写一个"关闭后成功重开并可用"的断言。
3. **`TestCachedRegistry_SingleFlightDedup` 是 flaky**：`-race -count=20` 下 2 次失败（`实际: 2`）。它真实暴露了 R-06 的两处缺陷，但作为 CI 用例会被当成噪声忽略。修 R-06 时应把它变成确定性用例（用屏障替代裸并发）。
4. **`config` 包测试硬编码内网地址且无超时**，历史单次 `go test ./config` 挂起 600s 后超时；`go vet ./...` 在 `i18n/store_test.go:148-149` 报 `assignment copies lock value`（复制含 `sync.RWMutex` 的 `config.config`）。这两项使"全仓测试"无法作为回归门，也是本次改用聚焦测试的原因。
5. **测试仍在裸改 `registry.Registry`**（`client/client_test.go:29-50`、`client/cache_regression_test.go:32-39`）：生产代码已迁到 `GetRegistry()/SetRegistry()`，测试却绕过 `registryMu` 直接赋值——`-race` 下若并发跑即报竞态，且无法验证访问器语义。应统一改用 `SetRegistry()`。
6. `mgin.go:311-315` 在 `job.Stop()` 之后又调一次 `job.GetManager().Check()`：对已停止的调度器做健康检查没有意义（`Check()` 在未运行时返回 nil），属遗留噪声，建议删除。

---

## 5. 修复优先级建议

**第 0 步（先做，成本最低、收益最大）**
1. `Reload()` 不再关闭 MySQL/SQLite，改为依赖 `InitWithError` 的"候选成功后原子替换并关闭旧连接"（一步解决 R-01，同时消除 R-08 的字典库失联与 R-05 的一部分窗口）。
2. Redis `Init` 的 I/O 移出写锁（R-03）——这是对基线的**回归**，合并前必须处理。

**第 1 步（对外服务发布前）**
3. postlog 缓冲上限 + 提前跳过（R-04）；恢复 `cache.CloseCache()` 并给 MemCache 容量/淘汰（R-09）。
4. 配置读侧收口（R-02）：先处理日志热路径与 `Sys.Swagger.Uri`、`Log.Get` 等每请求读取点。
5. `SafeExit` 顺序调整 + `job.Stop` 超时（R-05、R-12）。

**第 2 步（迭代内）**
6. `CachedRegistry` 去重上提到 `InstanceProvider` 分支、修正 inflight 删除（R-06）；熔断表上限与 URI 归一、`RegistryClient` 增加 ctx 版本（R-07）。
7. Mongo 终态 `closed` 与 MySQL 对齐（R-11）；i18n 字典缓存与重载重建（R-08）。
8. DAO：`JOIN` nil 守卫、`Alias` 白名单、`WithContext/WithCache` 改副本语义（R-10）。
9. 限流参数上限与 Header 摘要键（R-13）。

**第 3 步（卫生）**
R-14、R-15、R-17、R-18、R-19，以及 R-20 的用例修正（**先修用例再修代码**，否则回归门会给出错误信号）。

### 回归测试的"正确接缝"

- R-01：`db/sqlite` 公开 API 层——`InitWithError → Close → InitWithError → GetConnection → 执行一条语句`，全程临时目录，无需外部依赖（本次审计已用此接缝确定性复现）。
- R-02：`config` + `logs` 接缝——并发 `Config.Init(fileA/fileB)` 与 `logs.Info()`，`go test -race` 即红（本次已复现 5 处竞态）。
- R-03：`db/redis` 接缝——黑洞监听器 + `Init` 与 `IsMultiDB()` 并发，断言读操作等待时长（本次实测 2.70s）。
- R-06：`registry` 接缝——**fake 必须实现 `GetServiceInstances`**，否则测到的是生产不会走的分支（这正是当前用例给出错误结论的原因）。
- R-04：`middleware/postlog` 接缝——`httptest` 回放超大请求/响应体，断言 `Entry` 中内容与内存上界。
- R-12：`job` 接缝——注册一个忽略 `ctx` 的 handler，断言 `Stop()` 在预算内返回。

---

## 附录 A：探针源码（验证后已删除）

### A.1 SQLite 关闭后重开（确定性 FAIL）

```go
func TestAuditReopenAfterClose(t *testing.T) {
    client := &Sqlite{}
    file := filepath.Join(t.TempDir(), "audit.db")
    if err := client.InitWithError(file); err != nil { t.Fatal(err) }
    client.Close()
    err := client.InitWithError(file)               // → SQLite client is closed
    _, err2 := client.GetConnection()               // → SQLite client is closed
    t.Fatalf("reopen error=%v, getConn error=%v", err, err2)
}
```

```text
=== RUN  TestAuditReopenAfterClose
    reopen after Close error = SQLite client is closed
    GetConnection after reopen conn=false error = SQLite client is closed
    REOPEN BROKEN: client cannot be reopened after Close: SQLite client is closed
--- FAIL: TestAuditReopenAfterClose (0.01s)
```

### A.2 config 导出字段竞态（`-race`，FAIL）

```go
go func() { for !stopped { Config.Init(filepath.Join(dir, "b.yml")) } }()   // 写：configure.go:303-318
go func() { for { outs := Config.Logger.Outputs                             // 读：复刻 logs/output.go:155
                 for _, out := range outs { _ = out.Target; _ = out.Fields } } }()
go func() { for { _ = Config.App.Name; _ = Config.Config.Used
                 _ = len(Config.Discovery.Registries) } }()
```

```text
WARNING: DATA RACE  ×5
  write configure.go:310 ↔ read Config.App.Name
  write configure.go:311 ↔ read Config.Config.Used
  write configure.go:313 ↔ read Config.Logger.Outputs        ← 日志热路径
  write configure.go:314 ↔ read len(Config.Discovery.Registries)
  read  Config.Logger.Outputs[i].Fields ↔ write configure.go:249（Unmarshal 就地填充后备数组）
```

### A.3 Redis Init 持锁 I/O（PASS，但记录了阻塞时长）

```go
// 黑洞监听器：accept 后只读不回包，令 go-redis 卡在 handshake
go func() { client.Init(cfg); initDone <- time.Since(start) }()
go func() { _ = client.IsMultiDB(); readDone <- struct{}{} }()   // 纯读，无 I/O
```

```text
IsMultiDB returned after 2.701254792s while Init was running
Init took 3.002344417s
0 Redis连接失败: read tcp 127.0.0.1:58156->127.0.0.1:58155: i/o timeout
```

## 附录 B：可复用的检索命令

```bash
# 无同步直读配置导出字段的规模
grep -rn 'config\.Config\.\(App\|Config\|Log\|Logger\|Discovery\|Jwt\|Sys\|Casbin\|WorkDir\)' --include='*.go' . | grep -v _test | wc -l   # 141

# 全局接口/变量的裸访问
grep -rn 'registry\.Registry' --include='*.go' .

# 返回 nil 后继续链式调用（DAO 类问题）
grep -rn 'return nil$' --include='*.go' db/dao/

# 按配置预分配的容器（内存放大）
grep -rn 'make(\[\]' --include='*.go' middleware/ratelimit/
grep -rn 'make(chan' --include='*.go' middleware/ratelimit/

# 只增不删的全局 map
grep -rn 'func .*Delete(' --include='*.go' client/ | grep -v _test      # circuits 无对应实现

# 生命周期终态标志缺失
grep -rn 'closed' --include='*.go' db/ | grep -v _test

# goroutine 起点与是否可停
grep -rn 'safego.Run\|go func(' --include='*.go' . | grep -v _test | wc -l
```

## 附录 C：验证局限

1. **未在真实 MySQL/Redis/Mongo/etcd 上运行**：遵循"不为了验证编译而连接共享基础设施"。R-01 的 MySQL 分支、R-11、R-06 的 etcd 行为以代码走查 + 可控 fake 为准；SQLite/Redis/AES/diskPath/config 均有本地实测。
2. **`go test ./...` 与 `go vet ./...` 未作为验收门**，原因见 §1.2 与 R-20；已改为受影响包的聚焦 `-race` 运行（全部通过）。
3. **工作区在审计过程中持续变化**（见开头提示）。合并前请以 `git diff` 指纹 `b08e1f9c9a9edfab5d29b72136cfcfda1c4618bc` 对齐，或重跑 §1.2 命令重新确认。
4. **未审计**：`registry/{nacos,etcd,consul,polaris}` 与外部 SDK 的内部行为、`casbin`/`jwt` 中间件的鉴权正确性、以及 `/Users/macro/Work/go/src/github.com/maczh/` 下的伴生插件（mgkafka、elasticsearch 等）——它们各自应单独过一次同流程。
