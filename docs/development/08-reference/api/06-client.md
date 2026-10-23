# 服务调用与负载均衡

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> HTTP 调用客户端、韧性与负载均衡策略。

## `github.com/maczh/mgin/v2/pkg/client`

```go
import "github.com/maczh/mgin/v2/pkg/client"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `StateClosed` | State = iota | 熔断器三态： Closed —— 关闭（正常）：请求正常放行，失败计数达到阈值后跳转 Open Open —— 打开（熔断）：请求被快速失败，冷却时间到达后跳转 HalfOpen HalfOpen —… |
| `StateHalfOpen` | = | 熔断器三态： Closed —— 关闭（正常）：请求正常放行，失败计数达到阈值后跳转 Open Open —— 打开（熔断）：请求被快速失败，冷却时间到达后跳转 HalfOpen HalfOpen —… |
| `StateOpen` | = | 熔断器三态： Closed —— 关闭（正常）：请求正常放行，失败计数达到阈值后跳转 Open Open —— 打开（熔断）：请求被快速失败，冷却时间到达后跳转 HalfOpen HalfOpen —… |
| `CONTENT_TYPE_FORM` | = "x-form" |  |
| `CONTENT_TYPE_JSON` | = "json" |  |
| `CONTENT_TYPE_RESTFUL` | = "restful" |  |
| `CONTENT_TYPE_FILE` | = "file" |  |

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `ErrAllInstancesCircuitOpen` | = errors.New("client: all instances circuit-open") | ErrAllInstancesCircuitOpen v2 负载均衡专用错误：当服务的全部实例都被熔断时返此错误， 调用方可以快速失败（避免对已挂掉的下游继续施压）。 |
| `ErrCircuitOpen` | = errors.New("client: 熔断器已打开，请求被快速失败") | 调用级韧性策略的错误哨兵，便于调用方用 errors.Is 判断。 |
| `ErrCallTimeout` | = errors.New("client: 微服务调用超时") | 调用级韧性策略的错误哨兵，便于调用方用 errors.Is 判断。 |

### 类型

#### `State`

```go
type State int
```

State 表示熔断器当前所处的状态。

**方法**

```go
func (s State) String() string
```

String 返回状态的中文名，便于日志阅读。

#### `CircuitBreaker`

```go
type CircuitBreaker struct
```

CircuitBreaker 一个轻量三态熔断器。

采用纯标准库实现（sync/atomic 未使用，统一用互斥锁保护状态），
不引入 gobreaker / hystrix 等第三方依赖，避免给框架增加依赖负担。

计数模型：固定窗口计数（窗口长度 interval），窗口到期或发生状态跳转时清零重新计数。
该模型足够覆盖微服务调用保护场景，且实现简单、无额外内存开销。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (cb *CircuitBreaker) Name() string
```

Name 返回熔断器名称。

```go
func (cb *CircuitBreaker) Allow() bool
```

Allow 判断当前请求是否允许放行。
熔断打开且未到冷却时间时返回 false（快速失败）；冷却时间到达时会自动切换到半开并放行探测请求。
一旦 Allow 返回 true，调用方必须在请求结束后调用 Success() 或 Failure() 归还半开配额。

```go
func (cb *CircuitBreaker) Success()
```

Success 上报一次成功结果。

```go
func (cb *CircuitBreaker) Failure()
```

Failure 上报一次失败结果。

```go
func (cb *CircuitBreaker) State() State
```

State 返回熔断器当前状态。

```go
func (cb *CircuitBreaker) Counts() (failures int, successes int)
```

Counts 返回当前计数窗口内的失败次数与成功次数。

```go
func (cb *CircuitBreaker) Reset()
```

Reset 强制把熔断器恢复到关闭状态并清零计数。

```go
func (cb *CircuitBreaker) String() string
```

String 返回熔断器的可读描述，用于错误信息与日志。

#### `Options`

```go
type Options struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Method` | string | json:"method" | 接口方法 GET\|POST\|PUT\|DELETE |
| `Protocol` | string | json:"protocol" | 协议 x-form\|json\|restful |
| `Group` | string | json:"group" | 应用分组，用于nacos中分组，不传为当前nacos分组及默认分组 |
| `Header` | any | json:"header" | 额外的头部参数 |
| `Query` | any | json:"query" | URL Query参数 |
| `Data` | any | json:"data" | x-form Postform参数 |
| `Json` | any | json:"json" | json或restful模式的body参数 |
| `Path` | map[string]string | json:"path" | restful模式的路径参数 |
| `Files` | []grequests.FileUpload |  | 文件上传数据 |
| `Retry` | bool | json:"retry" | 是否重试 |
| `LoadBalancer` | string | json:"loadbalancer,omitempty" | v2 新增：负载均衡相关字段（可选，零侵入）。 LoadBalancer 为空时走 loadbalancer.Default()（RoundRobin）。 命名常量参见 pkg/loadbalancer：roundrobin / random / leastconn / consistenthash�… |

#### `ResilienceOptions`

```go
type ResilienceOptions struct
```

ResilienceOptions 描述在 client.Call 之外叠加的韧性策略。
所有字段都是可选的；零值 / nil 表示“不使用该能力”，行为与直接调用 client.Call 完全一致。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Timeout` | time.Duration |  | Timeout 单次调用的超时时间。<=0 表示不启用超时（与现有行为一致）。 |
| `MaxRetries` | int |  | MaxRetries 除首次调用外最多重试的次数。<=0 表示不重试（与现有行为一致）。 |
| `BaseDelay` | time.Duration |  | BaseDelay 指数退避的初始延迟，<=0 时使用 defaultBaseDelay(100ms)。 |
| `MaxDelay` | time.Duration |  | MaxDelay 指数退避的延迟上限，<=0 时使用 defaultMaxDelay(2s)。 |
| `RetryOnStatus` | func(code int) bool |  | RetryOnStatus 判定哪些 HTTP 状态码可重试。 【重要限制】现有 client.Call 的返回值只有响应体字符串与 error： 非 2xx 响应（如 500/503）在 grequests 层面并不算 error，会以正�… |
| `Breaker` | *CircuitBreaker |  | Breaker 熔断器，nil 表示不启用熔断。可用 client.GetBreaker(service) 获取全局共享实例。 |

### 函数

#### `Call`

```go
func Call(service, uri string, op *Options) (string, error)
```

Call 是 v1 时代的微服务调用入口，等价于 CallCtx(context.Background(), service, uri, op)。
保留以兼容存量项目；新代码建议直接使用 CallCtx 以便传入 context（例如超时/取消）。

#### `CallCtx`

```go
func CallCtx(ctx context.Context, service, uri string, op *Options) (string, error)
```

CallCtx 是 v2 新增的"支持 context"版本微服务调用入口。

背景：v1 的 Call 不支持 context，导致：
- 无法把 ctx 透传到 grequests 底层（grequests 的 DoRegularRequest 不接受 ctx）
- 业务侧无法用 ctx 实现超时或链路取消

v2 决策（已与用户确认）：**保持 goroutine+timer 兑底**，
即调用方在 CallCtx 内部通过 ctx 控制的超时计时器来"及时返回"，但底层 HTTP 请求
不会被真正取消，仍会占用连接到对端响应或连接超时。要做到真正取消需要升级
`levigross/grequests` 或换 net/http 自封装，那是 v2.1+ 的工作。

ctx 在本函数的作用：
- 通过 grequests 兼容的 RequestOptions 携带（未来 SDK 支持时直接生效）
- 用于超时/取消的"上层兜底"（业务可在外层用 select 控制）
- 链路 trace header 由 trace.GetHeaders() 透传，与 ctx 无关

#### `CallCtxT`

```go
func CallCtxT[T any](ctx context.Context, service, uri string, op *Options) models.Result[T]
```

CallCtxT 保留 v2 原有的 models.Result[T] 响应协议，语义对齐 CallCtx。

#### `CallPageT`

```go
func CallPageT[T any](service, uri string, op *Options) vo.ResponsePage[T]
```

CallPageT 调用微服务并解析 data.list/total/page/pageSize 分页响应。

#### `CallResilient`

```go
func CallResilient(service, uri string, op *Options, ro *ResilienceOptions) (string, error)
```

CallResilient 在 client.Call 之上叠加超时、指数退避（带 jitter）重试与熔断。

幂等性保护（非常重要）：
- 幂等读（GET / HEAD / OPTIONS / TRACE）默认可重试；
- 非幂等写（POST / PUT / DELETE / PATCH）默认不重试，避免重复下单、重复扣款等副作用；
- 唯一例外：调用方显式提供 RetryOnStatus，且从错误中解析出的状态码被该回调判定为可重试；
- Options.Method 为空时，按 client.Call 的既有默认值视为 POST（非幂等，默认不重试）。

默认安全：ro 为 nil 或其所有字段为零值时，等价于直接调用一次 client.Call
（不重试、不超时、不熔断），既有调用方不受任何影响。

参数:
- service: 微服务名。
- uri: 接口路径。
- op: 调用参数，与原 client.Call 一致；函数不会修改调用方传入的 op。
- ro: 韧性策略，可为 nil。

返回值:
- string: 成功时返回响应体，与原 client.Call 一致。
- error: 失败时返回最后一次的错误；熔断打开时返回包装了 ErrCircuitOpen 的错误。

CallResilient 是 Call 的"带韧性策略"版本，等价于 CallResilientCtx(context.Background(), service, uri, op, ro)。
保留以兼容 v1 时代使用 CallResilient 的项目。

#### `CallResilientCtx`

```go
func CallResilientCtx(ctx context.Context, service, uri string, op *Options, ro *ResilienceOptions) (string, error)
```

CallResilientCtx 是 CallResilient 的 context 版本（v2 新增）。

与 CallResilient 的差异：把 context 透传到 callWithTimeout 的 goroutine 调用链，
调用方可在 ctx 中带超时或取消信号。需要 ctx 真正取消底层 HTTP 时，
当前实现是"goroutine+timer 兑底"（见 v2-design §6.2），不依赖 grequests 升级。

#### `CallResultT`

```go
func CallResultT[T any](service, uri string, op *Options) models.Result[T]
```

CallResultT preserves the legacy status/data/msg response protocol.

#### `CallT`

```go
func CallT[T any](service, uri string, op *Options) vo.Response[T]
```

CallT 调用微服务并解析 code/data/msg/srvTime 响应。

#### `GetBreaker`

```go
func GetBreaker(name string) *CircuitBreaker
```

GetBreaker 按名称获取（或首次创建）一个全局共享的熔断器。
未显式指定参数时使用默认阈值：60 秒窗口内失败 5 次即熔断，冷却 30 秒后半开探测。
需要自定义参数时请使用 NewCircuitBreaker 并把结果赋给 ResilienceOptions.Breaker。

#### `NewCircuitBreaker`

```go
func NewCircuitBreaker(name string, maxFailures int, interval, timeout time.Duration) *CircuitBreaker
```

NewCircuitBreaker 创建一个熔断器。
参数:
- name: 熔断器名称，用于日志与 String()。
- maxFailures: 计数窗口内失败次数阈值，<=0 时使用 defaultMaxFailures。
- interval: 失败计数窗口长度，<=0 时使用 defaultBreakerInterval。
- timeout: 熔断打开后的冷却时长，<=0 时使用 defaultBreakerTimeout。

#### `ResetBreakers`

```go
func ResetBreakers()
```

ResetBreakers 清空全局熔断器注册表，主要用于测试场景。

## `github.com/maczh/mgin/v2/pkg/loadbalancer`

```go
import "github.com/maczh/mgin/v2/pkg/loadbalancer"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `ErrEmptyInstances` | = errors.New("loadbalancer: 实例列表为空") | ErrEmptyInstances 在 Pick 收到空实例列表时返回，调用方应转 fallback。 |
| `RoundRobinLB` | = &RoundRobin{} | RoundRobinLB 是默认注册实例，init 时由 Register 注册。 |
| `RandomLB` | = &Random{} | RandomLB 默认注册实例。 |
| `LeastConnectionsLB` | = &LeastConnections{} | LeastConnectionsLB 默认注册实例。 |
| `ConsistentHashLB` | = &ConsistentHash{replicas: 160} | ConsistentHashLB 默认注册实例，160 副本（vnode），业界常用值。 |

### 类型

#### `LoadBalancer`

```go
type LoadBalancer interface
```

LoadBalancer 是所有负载均衡策略的统一接口。

参数：
- instances：可用实例 URL 列表（来自 registry.GetServices）
- key：路由键（用于一致性哈希），空字符串时退化为非一致性策略

返回：被选中的实例 URL；调用方负责发起调用。

| 方法/内嵌 | 签名 |
|-----------|------|
| `Pick` | func(instances []string, key string) (string, error) |

#### `RoundRobin`

```go
type RoundRobin struct
```

RoundRobin 轮询策略。线程安全。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (r *RoundRobin) Pick(instances []string, _ string) (string, error)
```

Pick 按内部计数器选一个实例。

#### `Random`

```go
type Random struct
```

Random 随机策略。线程安全。

**方法**

```go
func (r *Random) Pick(instances []string, _ string) (string, error)
```

Pick 随机选一个实例。

#### `LeastConnections`

```go
type LeastConnections struct
```

LeastConnections 最少连接策略。线程安全。
用 sync.Map 跟踪每个实例当前的活跃请求数。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (l *LeastConnections) Pick(instances []string, _ string) (string, error)
```

Pick 选活跃连接最少的实例（并发安全）。
注意：v2.1 当前实现只读 conns（不调用 Inc），只用作"最少连接选实例"的快照判断；
真实的活跃连接跟踪由业务侧在请求开始/结束时显式调用 Inc 完成（返回的 dec 函数）。

```go
func (l *LeastConnections) Inc(instance string) (func(), bool)
```

Inc 业务侧在请求开始时调用以让 LB 跟踪活跃连接数；返回的"减"函数应在请求结束时调用。

与 Pick 共享同一个 sync.Map；Inc 第一次调用会 LoadOrStore 创建条目（值 0），
随后 Add(1) 增加。典型用法：

dec, _ := lb.Inc(best)
defer dec()

#### `ConsistentHash`

```go
type ConsistentHash struct
```

ConsistentHash 一致性哈希策略。线程安全。
key 相同的请求会路由到同一实例，适合"同一 session 走同一服务"的场景。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *ConsistentHash) Pick(instances []string, key string) (string, error)
```

Pick 对 key 做 FNV-1a 哈希后取模选实例。
若 key 为空字符串则退化为 RoundRobin 行为。

### 函数

#### `Default`

```go
func Default() LoadBalancer
```

Default 取全局默认策略；若未设置则返回 RoundRobinLB。

#### `Get`

```go
func Get(name string) LoadBalancer
```

Get 按名称取策略；未找到返回 nil。

#### `Register`

```go
func Register(name string, lb LoadBalancer)
```

Register 把一个策略注册到全局表。重复注册会覆盖。
一般由各策略实现文件在 init() 中调用。

#### `SetDefault`

```go
func SetDefault(lb LoadBalancer)
```

SetDefault 设置全局默认策略。传入 nil 等同于 RoundRobin。

