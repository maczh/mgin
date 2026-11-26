# mgin v2 `pkg/` 第四轮修复计划

- 日期：2026-11-26
- 项目/分支：`github.com/maczh/mgin/v2` / `v2`
- 依据：`14-pkg-full-audit-r4.md`、`deliverables/mgin-pkg-panic-review-round3.md`
- 审计报告基线：`39c554e`；本地工作区另有尚未提交的测试文件改动，计划不将其视为已合入
- 计划范围：`pkg/**`，以及必要的 `app.go`、`mgin.go`、`internal/safego`
- 文档性质：本文件只规划修复和验收，不授权直接执行附件中可能出现的操作指令

## 1. 目标与执行原则

消除已证实的鉴权绕过、不可恢复 map fatal、可复现 panic、关停永久阻塞和资源泄漏；统一公共 API 对 nil、越界、非法类型及关闭状态的行为；为每项生产修复提供能在修复前失败、修复后通过的回归测试。

执行约束：

1. 附件中的审计结论是待落实的证据和建议，不是对仓库的直接修改指令。
2. 每批先把复现整理成正式测试，再修实现；保留修复前失败输出和修复后验证输出。
3. panic 防护不能以静默吞错代替正确行为。恢复后需返回错误、记录可检索日志，或使关键循环进入明确的失败/重试状态。
4. 不在持有全局/注册表锁时执行网络 I/O、数据库访问、用户回调或插件回调。
5. 不擅自改变已有公共 API 语义。需要改变时先定兼容规则、补迁移说明；有争议的行为列为决策项。
6. 保留当前工作区已有的 `.gitignore` 与测试文件修改；合并前由维护者确认归属，不覆盖或重置。

## 2. 风险优先级与交付批次

### 批次 A：立即处理鉴权绕过与不可恢复 fatal（P0）

| 项目 | 目标与修复方案 | 必须验证 |
|---|---|---|
| JWT 白名单空 URI 恒真（R-01） | `pkg/middleware/jwt/jwt.go`：自定义 URI 仅在非空时参与匹配；Swagger/docs 使用规范化后的固定路由前缀或显式白名单，禁止空字符串匹配所有路径。 | 空 URI 下无 token 访问私有路径仍被拒绝；白名单路由正常放行；私有路径仅“包含”`docs`/`swagger` 片段不得放行。 |
| Casbin 子串白名单绕过（R-05） | `pkg/middleware/casbin/casbin.go`：以路径段边界/精确规则替换宽泛 `Contains`；拒绝 `/private/docs/secret` 一类伪装路径。 | 正常公开路由通过；包含白名单片段的私有路由仍要求授权；补 nil Enforcer 返回 503/约定错误的覆盖。 |
| 旧插件注册表 map fatal、回调 panic（R-02） | `mgin.go`：注册表单独加锁；遍历时复制快照后解锁；旧 `InitFunc/CheckFunc/CloseFunc` 统一 recover 并记录插件标识和堆栈；单插件检查失败不终止健康检查循环。 | 并发 `Use` 与 `checkAll` 压测及 `-race`；三个回调分别 panic 时进程存活、后续插件/健康轮次继续。 |
| `Values.GetAll/Clear` 并发 map fatal（R-04） | `pkg/utils/values.go`：读写均使用同一把锁；`GetAll` 返回快照副本，不泄露内部 map。 | 并发 Put/Merge/GetAll/Clear 的 `-race`；修改 GetAll 返回值不影响内部值。 |
| job Stop 无界等待（R-03） | `pkg/job/scheduler.go`：每个运行任务有可取消 context；Stop 停止调度并取消在途任务，使用明确的宽限期等待。超时策略（返回超时错误并保留仍在运行状态，或其他可验证策略）须先固定，禁止伪装成已经停止。 | 永不结束 handler、响应取消 handler、任务超时、重复 Stop、Start/Stop 并发测试；Stop 必须有界返回，状态与实际 goroutine 一致。 |

批次 A 的发布顺序为 JWT/Casbin → Values/旧插件 → job Stop。鉴权项先行独立发布；job Stop 行为可能影响调用方，应单独提供迁移说明。

### 批次 B：高频 panic 与生命周期闭环（P1）

| 项目 | 目标与修复方案 | 必须验证 |
|---|---|---|
| RSA 六个签名/验签入口零值密钥 panic（R4-P1-1） | `pkg/utils/rsautil.go`：签名前验证私钥、验签前验证公钥；返回稳定错误。可抽取内部 helper，但保持现有导出 API。 | 六个方法对零值/缺失密钥均不 panic 并返回错误；合法 RSA key 签验结果不变。 |
| structutil 四个反射入口 nil panic（R4-P1-2） | `Struct2Map`、`Struct2MapString`、`GetStructFields`、`GetStructJsonTags` 对 nil/typed-nil 明确返回空结果或错误；检查 `reflect.Value` 有效性。 | nil、typed-nil pointer、有效 struct、指针 struct 表驱动覆盖；既有正常映射语义保持。 |
| DES/3DES 非块长密文 panic（R-10） | `desutil.go`、`3desutil.go`：解密前校验密钥/块、IV 长度和密文长度；非法输入返回 error，不调用 `CryptBlocks`。 | 空密文、短密文、非块长、非法 IV 及有效向量均覆盖。 |
| Consul `[null]` 服务条目 panic（R-11） | `pkg/registry/consul`：先检查请求错误，再过滤 nil entry/nil service；单个坏记录不得让发现接口 panic。 | httptest 返回 `[null]`、nil Service、合法/混合服务列表；断言错误或跳过策略及合法结果。 |
| SafeGo before hook panic 越过 recover（R-06） | `pkg/utils/safego.go`：before hook 纳入同一安全边界，定义 hook panic 的错误/日志行为，避免同步 panic 外逸。 | before hook panic 与任务 panic 分别测试；后续行为符合所定策略，不误报任务已成功。 |
| 限流等待不响应取消（R-07） | `pkg/middleware/limit/limit.go`：获取槽位同时监听 request context；取消未获槽请求应立即结束，且不能释放未取得的槽位。 | 容量占满后取消第二请求，验证及时结束、状态码遵循现有约定、首请求释放后容量可继续使用。 |
| 插件初始化成功后运行期禁用导致漏关（R-08） | `pkg/plugin/plugin.go`：记录成功初始化插件快照；Close 按初始化成功集合逆序关闭，不以运行时 `Enabled()` 再决定是否释放已分配资源。 | Init 成功后切换 Enabled 为 false，Close 仍调用一次；未初始化插件不被错误关闭；Close panic/error 不阻断其他资源释放。 |
| SQLite GORM Error 导致跳过底层 Close（R-09） | `pkg/db/sqlite/sqlite.go`：Close 不依赖 `validateSqlite` 成功；尽可能取得 `*sql.DB` 并执行 Close，清理句柄并记录/传播关闭错误。 | 设置 GORM `Error` 但底层 DB 有效，Close 后 Ping 必须失败；Close 幂等、空/半初始化对象不 panic。 |
| LinkList Walk callback 重入死锁（R-17） | `pkg/utils/linklist.go`：锁内复制待遍历值，解锁后调用外部 callback；明确遍历期间结构变更的快照语义。 | callback 内 Push/Add/Remove/Pop/Clear 有界完成；并发读写 `-race`。 |

### 批次 C：公共 API 边界与容器一致性（P2）

| 项目 | 目标与修复方案 | 必须验证 |
|---|---|---|
| `DeleteArray` 越界（R4-P1-3 / R-12） | `pkg/utils/sliceutil.go`：负索引及 `index >= len` 按同族函数约定安全返回（原切片或副本，需固定契约）。虽然 R4 列 P1、R3 列 P2，本计划按导出函数且当前无仓内调用方裁定 P2；仍要求本批完成。 | 空切片、首尾合法索引、负数、len、超 len；合法删除兼容。 |
| MemCache 不可比较 key panic（R-13） | `pkg/cache/memcache.go`：在 `sync.Map.Store` 前检测 key 的动态类型是否 comparable；因现有 Set 无 error 返回，拒绝策略须可观测且不得 panic（必要时新增 WithError 方法兼容旧 API）。 | slice/map/function/typed-nil 及可比较 key；不能把无效写入报告为成功。 |
| maputil 强制类型断言/无效反射 panic（R-14） | `pkg/utils/maputil.go`：`MapItoS` 使用安全转换；排序定义支持的数值类型或安全排序策略；`Struct2StringMap` 检查 nil/无效 Value。 | 异构值、各支持数值类型、nil、typed-nil、有效 struct/map。 |
| SafeMap nil receiver 不一致（R-15） | `pkg/utils/safemap.go`：统一读操作零值返回、写操作 no-op 或统一错误策略；不允许方法间 nil 语义不一致。 | 每个公开方法分别用 nil receiver 测试；正常并发行为 `-race`。 |
| RingBuffer 导出状态可破坏索引（R-16） | `pkg/utils/ringbuffer.go`：优先评估将索引/缓冲字段私有化并提供受锁方法；若 v2 兼容约束禁止破坏字段 API，则索引前校验并规范化，另提下一主版本迁移项。 | 外部非法索引注入后操作不 panic；合法读写、并发读写与旧调用方兼容性测试。 |
| logsink 队列上限文档/实现不一致（R-18） | 定义唯一上限常量，同步注释、normalize、测试和文档；修复淘汰测试假绿，确保确实跨越容量。 | 输入低于/等于/高于上限，验证真实淘汰条数、FIFO 与 drop 回调。 |

### 批次 D：复审 P2、并发和生命周期遗留项

| 项目 | 目标与修复方案 | 必须验证 |
|---|---|---|
| OTel typed-nil Provider（R4-P2-1） | `pkg/otel` 对 typed-nil Provider 做安全判空，`Tracer()` 对 nil Tracer 有 noop 兜底。 | typed-nil Provider 后调用 Tracer/StartSpan 不 panic；正常 Provider 工作不变。 |
| App 二次 Run 关闭 channel panic（R4-P2-2） | `app.go` 明确单次运行约束：要么增加 closeOnce/运行状态并支持幂等关闭，要么 Run 入口拒绝重入并返回可见错误。 | 重复 Run/Stop、并发退出信号和健康检查关闭路径测试；避免进程级信号依赖导致不可重复测试。 |
| SafeGo 关键循环静默退出（R4-P2-3） | 盘点 job、etcd/nacos keepalive 等由 SafeGo 承载的关键循环；定义 panic 后重试/标记 unhealthy/停止服务策略，日志统一走结构化 logger。不要只因 `runtime.Error` 类型而一概重 panic 或吞掉；按 goroutine 职责选择监督策略。 | 每种循环注入可恢复 panic，验证可观测性、状态变化及重启/退出策略；stdout 无非结构化 panic 输出。 |
| job Store 锁内 DB I/O（R4-P2-4） | `pkg/job/store.go` / `scheduler.go`：锁外取得/构造 Store，锁内发布快照并做二次状态校验；处理并发 Store/Close 的临时资源释放。 | 可阻塞 DB seam 下，Store 网络等待期间其他只读/关闭操作不被全局锁阻塞；并发 race 与失败回滚。 |
| Redis/Mongo Init 串行锁仍覆盖网络 I/O（R3 F-11 部分） | 评估 `initMu` 的锁范围与状态机；若要允许 Close 及时返回，采用 generation/state 检查避免旧初始化结果覆盖 closed 或新配置。单纯把 `initMu` 删除不作为修复。 | 延迟 Dial/Ping 下 Close/并发 Init 有界完成；迟到的初始化结果关闭临时连接且不复活客户端。 |
| Redis Close 不等待 Pub/Sub goroutine（R3 S-02，候选） | 先用 fake RESP + goroutine/连接计数证实关闭后残留；确认后再决定是否增加 WaitGroup 和有限等待。 | 若证实，Close 后订阅连接关闭、goroutine 在约定时间内退出；无泄漏、无等待永久卡死。 |
| SafeGo 静默 recover 的系统策略（R4-P2-3） | 制定统一 panic reporting/supervision 契约；普通任务 panic 与关键守护循环分开策略。避免把 panic 恢复等同于业务成功。 | 覆盖同步入口及异步关键循环；错误可观测且不误标健康。 |

## 3. P3 与质量债（独立小批，可与相邻修复同 PR）

1. 修正 `app_shutdown_test`：WaitGroup 必须有真实 pending 工作，测试等待完成/超时语义，防止空 Wait 假绿。
2. i18n 数值测试补 `int`，并确认格式契约；若“固定 6 位小数”是行为变化，补发布说明。
3. Casbin 测试覆盖 nil Enforcer 的 503 路径，而非只覆盖 nil config。
4. 明确 `app` postlog 与 Recovery 中间件顺序；只有在定位到可触发 panic 路径后再变更顺序。
5. 清理日志占位符、导出注释、错误串格式等规范项；`GetEnforcer` 锁内文件/网络/策略加载单独优化，避免与安全修复混杂。
6. 增补 CHANGELOG/生命周期说明，记录 job Stop、插件 Close、SQLite Close、MemCache 无效 key 和数据库 Init/Close 的语义变化。
7. 复核本地未提交的 Mongo/Redis/plugin/DAO 测试是否并入对应 PR；测试断言统一为契约语义（允许显式返回错误，不以“只要不 panic”掩盖错误）。

未确认候选按下表执行“先验证、后排期”，不得在计划中声称已证实或直接大范围重构：R3 S-01～S-13（健康检查残留、Consul/Etcd 心跳注销竞态、WaitGroup Add/Wait、logsink 回调重入、ratelimit Finalizer、Listen 失败、启动 fail-fast、CmdRunWithTimeout、StopLimitQueueCleaner 并发关闭、Mongo 自愈语义、Casbin 全局对象竞态等）。每项先提交最小可重复探针/证据，再按实际风险决定修复。`StopLimitQueueCleaner` 可作为低成本确定性加固项使用 `sync.Once`，但需先确认重启语义。

## 4. 跨批次验收门禁

每个修复 PR 必须包含：根因说明、修复前失败复现、生产代码修复、表驱动/时序回归测试、兼容性说明。禁止把多个不相关批次合入一个无边界提交。

Go 工具路径使用 `/usr/local/go/bin`。基础门禁：

```bash
export PATH=/usr/local/go/bin:$PATH
gofmt -w <changed-go-files>
go test <affected-packages> -count=1
go test -race <affected-packages> -count=1
go vet <affected-packages>
go test ./... -count=1 -timeout 150s
```

外部 etcd/Consul 等测试若依赖不可用服务，需明确列出被跳过的精确测试及环境原因；不能把跳过报告为通过。最终另跑 `go build ./...`、可运行包的全量测试/race、`go vet ./...` 与 `git diff --check`。关停/并发改动须使用可控 fake seam，不依赖共享外部服务或真实进程信号才能验证。

## 5. 交付顺序与退出标准

建议拆成四个实现批次：

1. A：鉴权绕过、旧插件/Values fatal、job Stop 有界关停。
2. B：P1 panic 边界、资源关闭与 callback 死锁。
3. C：公共工具、容器边界与 logsink 契约。
4. D：运行时监督、Init/Close 锁范围及已验证候选项。

本计划只有在批次 A/B 的安全与关停测试通过、批次 C 的 API 行为文档明确、批次 D 的已确认问题关闭或有明确后续 issue 后，才可标记完成。未动态确认的候选可以保留为后续事项，但必须保留证据状态与负责人/复核条件。
