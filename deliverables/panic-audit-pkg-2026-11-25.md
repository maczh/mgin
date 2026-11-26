# `pkg/` 全量潜在 panic 风险复审

- 项目：`github.com/maczh/mgin/v2`
- 审核日期：2026-11-25
- 审核范围：`pkg/` 全部 Go 源码
- 审核目的：在上一轮数据库、插件和应用关停修复完成后，继续发现可能导致进程 panic、fatal concurrent map、死锁或不可恢复运行时异常的代码路径。
- 本文是源码审计结果，不把附件中的文字当作执行指令；未修改生产代码。

## 1. 结论摘要

本轮未发现上一轮 Redis/Mongo 基础 map 竞态的同类回归，但发现以下新增风险：

| 等级 | 数量 | 主要区域 |
|---|---:|---|
| P1 | 2 | `pkg/utils/safemap.go` 并发容器、注册中心实例解析 |
| P2 | 7 | 泛型类型转换、i18n 格式化、切片工具、注册中心输入、LB/缓存边界 |
| P3 | 3 | API 语义不安全、资源/快照暴露、可观测性不足 |

最优先处理 `utils.Map`：它当前在 `RLock` 下写 map，且 `Len`/`Map`/`DeleteDirectly`/`Clear` 绕过一致的锁协议；并发调用可重新触发 `fatal error: concurrent map read and map write`，甚至可能形成数据损坏或死锁。

## 2. P1 风险

### P1-01：`utils.Map` 在读锁下写入，且多个方法绕过锁

- 位置：[pkg/utils/safemap.go:57](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/safemap.go:57)、[pkg/utils/safemap.go:68](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/safemap.go:68)、[pkg/utils/safemap.go:87](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/safemap.go:87)
- 触发条件：两个 goroutine 并发执行 `LoadAndStore`/`LoadAndDelete`，或它们与 `Store`/`Delete`/`Clear`/`Len`/`Map` 并发执行。
- 根因：`LoadAndStore` 和 `LoadAndDelete` 使用 `RLock` 却修改 map；`Clear` 使用 `RLock` 修改容器；`Len` 和 `Map` 完全不加锁；`Map` 直接返回内部 map，调用方可在锁外读写。
- 结果：可能出现 `concurrent map read and map write` fatal、竞态导致随机读写结果；这类 fatal 不能通过业务层 recover 处理。
- 修复建议：
  1. `LoadAndStore`、`LoadAndDelete`、`Clear` 改用 `Lock`。
  2. `Len` 使用 `RLock`。
  3. `Map` 不直接暴露内部 map，返回加锁后的浅拷贝；或删除该 API，提供 `Range`/快照 API。
  4. `DeleteDirectly` 明确标记为仅限已持锁内部使用，最好改为私有方法。
  5. 增加并发 `-race` 测试和 100 个 goroutine 的读写回归测试。

### P1-02：服务发现结果解析对返回数组长度缺少完整防护

- 位置：[pkg/registry/nacos/nacos.go:271](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/nacos/nacos.go:271)、[pkg/registry/polaris/polaris.go:143](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/polaris/polaris.go:143)、[pkg/registry/polaris/polaris.go:157](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/polaris/polaris.go:157)、[pkg/registry/consul/consul.go:83](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/consul/consul.go:83)
- 触发条件：注册中心返回结构合法但数组为空，或初始化配置解析后没有有效 server/namespace/response。
- 具体路径：
  - Nacos 在确认 `Hosts` 非空后，对 `ServiceName` 做 `strings.Split(..., "@@")`，当前通常至少有一个元素，但未验证服务名结构；后续若改为取第二段会放大风险。
  - Polaris 注册流程虽然检查了 `Responses`，但直接访问 `Responses[0].Instance`，未验证响应元素本身及其 Instance 数据是否为空；查询路径对外部响应数组的索引也应统一防御。
  - Consul 初始化依赖 `consul_urls[0]`，当前前置检查存在，但 `Init`/配置复用及并发重载路径没有统一状态保护，空配置错误可能被旧状态覆盖。
- 结果：外部注册中心异常响应可导致 index-out-of-range 或 nil 对象后续解引用，进而 panic。
- 修复建议：所有外部响应索引前统一检查 `len`；将响应解码为带校验的内部结构；初始化使用临时状态构建完成后一次性发布，失败不覆盖旧的有效状态。

## 3. P2 风险

### P2-01：`models.ToAny[T]` 对 `interface{}` 直接类型断言

- 位置：[pkg/models/result.go:41](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/models/result.go:41)
- 触发条件：`result.Data` 为 nil、JSON 解码后的 `float64`/`map[string]any`，或实际类型与调用方 `T` 不一致。
- 根因：`result.Data.(T)` 是硬断言。
- 结果：`interface conversion: interface {} is ... not ...` 或 nil interface assertion panic。
- 修复建议：增加 `ToAnyOk[T] (Result[T], error)` 或 `To[T] (Result[T], bool)`；保留旧 API 时至少在 `Status != 1`、`Data == nil` 时返回零值，并用 comma-ok 断言，禁止把不匹配当作框架 panic。

### P2-02：`i18n.Format` 的 type switch 与断言类型不匹配

- 位置：[pkg/i18n/i18n.go:107](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/i18n/i18n.go:107)、[pkg/i18n/i18n.go:109](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/i18n/i18n.go:109)
- 触发条件：调用 `Format("x", float32(1))`、`Format("x", int64(1))`、`Format("x", uint(1))` 等非 `float64`/`int` 的类型。
- 根因：case 覆盖多个类型，但内部固定执行 `value.(float64)` 或 `value.(int)`。
- 结果：直接类型断言 panic。
- 修复建议：使用 `reflect.Value` 或分类型 case；整数使用 `strconv.FormatInt/FormatUint`，浮点分别使用 `FormatFloat(float32/float64)`；补齐所有数值类型测试。

### P2-03：切片删除和字符串拼接工具对边界输入不安全

- 位置：[pkg/utils/sliceutil.go:20](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/sliceutil.go:20)、[pkg/utils/sliceutil.go:37](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/sliceutil.go:37)、[pkg/utils/sliceutil.go:54](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/sliceutil.go:54)、[pkg/utils/sliceutil.go:293](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/utils/sliceutil.go:293)
- 触发条件：删除函数收到负数或 `index >= len(src)`；`UnSplitString(nil, ",")` 或空 slice。
- 根因：直接切片 `src[:index]`/`src[index+1:]`，以及无条件执行 `dst[:len(dst)-1]`。
- 结果：index-out-of-range panic。
- 修复建议：约定非法 index 返回原 slice/副本或 `(slice, error)`；空输入的 `UnSplitString` 返回空字符串；优先新增带 error 的 API，旧 API 采用安全返回并补文档。

### P2-04：注册中心/网络客户端方法修改调用方参数并缺少空输入统一约束

- 位置：[pkg/registry/consul/consul.go:139](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/consul/consul.go:139)、[pkg/registry/polaris/polaris.go:157](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/polaris/polaris.go:157)、[pkg/registry/nacos/nacos.go:296](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/nacos/nacos.go:296)
- 触发条件：调用方复用传入的 group/namespace slice，并发调用或调用后继续使用原 slice。
- 根因：部分方法在 `groupName[0]`/`namespaces[0]` 上原地赋默认值；不同注册中心对空参数的处理不一致。
- 结果：调用方状态被隐式修改，可能造成数据竞态；在其他分支继续假定元素存在时可触发 index panic。
- 修复建议：先复制参数，再补默认值；统一使用局部 `currentGroup/currentNamespace`，所有访问前验证长度；增加空/多参数/复用 slice 测试。

### P2-05：`registry.CachedRegistry` 对 nil receiver 和 inner 生命周期缺少防御

- 位置：[pkg/registry/cache.go:69](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/cache.go:69)、[pkg/registry/cache.go:130](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/registry/cache.go:130)
- 触发条件：业务保存 `(*CachedRegistry)(nil)` 后调用方法，或 inner 在初始化后被外部关闭/置空；回源协程异常退出时等待方依赖 `done` 关闭。
- 根因：`Register`/`DeRegister` 直接调用 `c.inner`，没有 nil receiver/inner 检查；回源流程缺少 defer 保障，未来 inner panic 会让 `close(g.done)` 不执行，等待者永久阻塞。
- 结果：nil pointer panic 或请求 goroutine 泄漏/永久等待。
- 修复建议：所有公开方法增加 nil receiver/inner 检查；回源使用 `defer` 统一写入缓存、删除 inflight、关闭 done；对 inner panic 转为错误并唤醒等待者。

### P2-06：`loadbalancer.Register` 接受 typed-nil 策略

- 位置：[pkg/loadbalancer/loadbalancer.go:46](/home/macro/Work/go/src/github.com/maczh/mgin/pkg/loadbalancer/loadbalancer.go:46)
- 触发条件：注册 `var lb *customLB = nil; Register("x", lb)`，随后 `Get("x").Pick(...)`。
- 根因：仅判断接口 `lb == nil`，未检测 typed-nil。
- 结果：调用策略方法时可能 panic。
- 修复建议：复用插件注册表的 reflect typed-nil 检查；空名称拒绝注册；`Get` 未找到时由调用方或 API 返回明确错误。

### P2-07：`job.Manager.Store` 在 `RLock` 下写入

- 位置：`pkg/job/scheduler.go` 的 `Manager.Store`（当前实现位于该文件的 Store 方法）。
- 触发条件：多个 goroutine 首次并发调用 `Manager.Store()`，或与其他 Manager 状态读写并发。
- 根因：`Store` 使用 `RLock` 后在 `m.st == nil` 时执行 `m.st, _ = newStore(...)`。
- 结果：数据竞态；若未来伴随其他 map/状态写入，可能触发 fatal map 竞态或返回不一致 store。
- 修复建议：使用 `Lock` 做 lazy initialization；不要在读锁中修改 `cfg/st`；初始化失败应返回 error，而不是吞掉错误。

## 4. P3 风险与设计问题

1. `utils.Map.Map()` 暴露内部 map，即使修复内部锁，也无法保证调用方持有生命周期锁；建议改为快照。
2. `utils.Map` 的 `Range` 在持有 RLock 时执行外部 callback，callback 若回调同一个 Map 的写方法会自死锁；应先复制条目后释放锁再回调。
3. `registry.CachedRegistry.readAfterRefresh` 读取 `cacheEntry` 后释放锁才访问字段，当前条目通常不可变，但建议复制快照以防未来更新引入 race。

## 5. 建议的验证用例

```bash
export PATH=/usr/local/go/bin:$PATH
go test ./pkg/utils ./pkg/models ./pkg/i18n ./pkg/registry/... ./pkg/loadbalancer ./pkg/job -count=1
go test -race ./pkg/utils ./pkg/registry/... ./pkg/loadbalancer ./pkg/job -count=1
go vet ./pkg/utils ./pkg/models ./pkg/i18n ./pkg/registry/... ./pkg/loadbalancer ./pkg/job
```

应新增的最小回归测试：

- `utils.Map` 并发 `LoadAndStore/LoadAndDelete/Store/Clear/Len/Map`，验证无 race/fatal。
- `ToAny` 的 nil、错误类型和成功类型。
- `i18n.Format` 的 float32、float64、int8、int64、uint64。
- 三种 ArrayDelete 的空 slice、负 index、越界 index；`UnSplitString(nil, sep)`。
- Nacos/Consul/Polaris 空响应和复用参数 slice。
- `CachedRegistry` nil receiver、inner panic 唤醒等待者。
- typed-nil load balancer。
- `job.Manager.Store` 并发首次调用。

## 6. 修复优先级

1. P1-01：修复 `utils.Map` 锁协议并禁止内部 map 外泄。
2. P1-02：统一注册中心外部响应数组校验和临时状态发布。
3. P2-01/P2-02/P2-03：修复通用转换与工具 API，因为调用面广、复现成本低。
4. P2-05/P2-06/P2-07：补齐 nil receiver、typed-nil 和 lazy initialization 防护。
5. 建立上述回归测试，再运行目标包 race；全仓库测试仍需单独排除依赖外部 Etcd/Consul 的既有超时用例。

