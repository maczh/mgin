// Package loadbalancer 提供 mgin v2 的客户端负载均衡能力。
//
// v2 引入：在 client.CallCtx 内部用 registry.Registry.GetServices 拉取多实例后，
// 通过 LoadBalancer.Pick 选一个具体实例发起调用。框架默认使用 RoundRobin，
// 业务可通过 loadbalancer.SetDefault 切换到其它策略。
//
// 设计目标：
//   - 零第三方依赖（与 v2 主体一致）
//   - 线程安全：多 goroutine 并发 Pick 不会脏写
//   - 一致性哈希的 key 用于把同一请求的多个调用路由到同一实例（如 session/缓存）
package loadbalancer

import (
	"errors"
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// （已移除对 atomic.Value 的使用，因其不允许存异构类型；改用 mu 保护 LoadBalancer 接口值）

// ErrEmptyInstances 在 Pick 收到空实例列表时返回，调用方应转 fallback。
var ErrEmptyInstances = errors.New("loadbalancer: 实例列表为空")

// LoadBalancer 是所有负载均衡策略的统一接口。
//
// 参数：
//   - instances：可用实例 URL 列表（来自 registry.GetServices）
//   - key：路由键（用于一致性哈希），空字符串时退化为非一致性策略
//
// 返回：被选中的实例 URL；调用方负责发起调用。
type LoadBalancer interface {
	Pick(instances []string, key string) (string, error)
}

// 全局注册表 + 默认策略。
var (
	mu        sync.RWMutex
	balancers = make(map[string]LoadBalancer)
	def       LoadBalancer = RoundRobinLB
)

// Register 把一个策略注册到全局表。重复注册会覆盖。
// 一般由各策略实现文件在 init() 中调用。
func Register(name string, lb LoadBalancer) {
	mu.Lock()
	defer mu.Unlock()
	balancers[name] = lb
}

// Get 按名称取策略；未找到返回 nil。
func Get(name string) LoadBalancer {
	mu.RLock()
	defer mu.RUnlock()
	return balancers[name]
}

// SetDefault 设置全局默认策略。传入 nil 等同于 RoundRobin。
func SetDefault(lb LoadBalancer) {
	mu.Lock()
	defer mu.Unlock()
	if lb == nil {
		lb = RoundRobinLB
	}
	def = lb
}

// Default 取全局默认策略；若未设置则返回 RoundRobinLB。
func Default() LoadBalancer {
	mu.RLock()
	defer mu.RUnlock()
	if def == nil {
		return RoundRobinLB
	}
	return def
}

// ---- 策略实现 ----

// RoundRobin 轮询策略。线程安全。
type RoundRobin struct {
	counter atomic.Uint64
}

// RoundRobinLB 是默认注册实例，init 时由 Register 注册。
var RoundRobinLB = &RoundRobin{}

// Pick 按内部计数器选一个实例。
func (r *RoundRobin) Pick(instances []string, _ string) (string, error) {
	if len(instances) == 0 {
		return "", ErrEmptyInstances
	}
	idx := r.counter.Add(1) - 1
	return instances[idx%uint64(len(instances))], nil
}

// Random 随机策略。线程安全。
type Random struct{}

// RandomLB 默认注册实例。
var RandomLB = &Random{}

// Pick 随机选一个实例。
func (r *Random) Pick(instances []string, _ string) (string, error) {
	if len(instances) == 0 {
		return "", ErrEmptyInstances
	}
	idx := fastrand() % uint32(len(instances))
	return instances[idx], nil
}

// LeastConnections 最少连接策略。线程安全。
// 用 sync.Map 跟踪每个实例当前的活跃请求数。
type LeastConnections struct {
	conns sync.Map // key: instance(string) -> *atomic.Int64
}

// LeastConnectionsLB 默认注册实例。
var LeastConnectionsLB = &LeastConnections{}

// Pick 选活跃连接最少的实例（并发安全）。
// 注意：v2.1 当前实现只读 conns（不调用 Inc），只用作"最少连接选实例"的快照判断；
// 真实的活跃连接跟踪由业务侧在请求开始/结束时显式调用 Inc 完成（返回的 dec 函数）。
func (l *LeastConnections) Pick(instances []string, _ string) (string, error) {
	if len(instances) == 0 {
		return "", ErrEmptyInstances
	}
	var (
		best  string
		minN  int64 = 1 << 62
		first = true
	)
	for _, ins := range instances {
		v, _ := l.conns.LoadOrStore(ins, new(atomic.Int64))
		cur := v.(*atomic.Int64).Load()
		if first || cur < minN {
			best = ins
			minN = cur
			first = false
		}
	}
	return best, nil
}

// Inc 业务侧在请求开始时调用以让 LB 跟踪活跃连接数；返回的"减"函数应在请求结束时调用。
//
// 与 Pick 共享同一个 sync.Map；Inc 第一次调用会 LoadOrStore 创建条目（值 0），
// 随后 Add(1) 增加。典型用法：
//
//	dec, _ := lb.Inc(best)
//	defer dec()
func (l *LeastConnections) Inc(instance string) (func(), bool) {
	v, _ := l.conns.LoadOrStore(instance, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
	return func() { v.(*atomic.Int64).Add(-1) }, true
}

// ConsistentHash 一致性哈希策略。线程安全。
// key 相同的请求会路由到同一实例，适合"同一 session 走同一服务"的场景。
type ConsistentHash struct {
	replicas int
}

// ConsistentHashLB 默认注册实例，160 副本（vnode），业界常用值。
var ConsistentHashLB = &ConsistentHash{replicas: 160}

// Pick 对 key 做 FNV-1a 哈希后取模选实例。
// 若 key 为空字符串则退化为 RoundRobin 行为。
func (c *ConsistentHash) Pick(instances []string, key string) (string, error) {
	if len(instances) == 0 {
		return "", ErrEmptyInstances
	}
	if key == "" {
		return RoundRobinLB.Pick(instances, "")
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := h.Sum32() % uint32(len(instances))
	return instances[idx], nil
}

// fastrand 用 atomic 计数器近似 rand.Int31n，避免引入 math/rand 状态。
// 线程安全；不追求密码学强度，仅用于 LB 选实例。
func fastrand() uint32 {
	var c uint64
	c = fastrandCounter.Add(1)
	// xorshift64，避免依赖外部 rand 源
	x := c
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	return uint32(x)
}

var fastrandCounter atomic.Uint64

func init() {
	Register("round_robin", RoundRobinLB)
	Register("random", RandomLB)
	Register("least_connections", LeastConnectionsLB)
	Register("consistent_hash", ConsistentHashLB)
}
