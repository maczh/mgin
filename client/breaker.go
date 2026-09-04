package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/maczh/mgin/logs"
)

// State 表示熔断器当前所处的状态。
type State int

// 熔断器三态：
//
//	Closed   —— 关闭（正常）：请求正常放行，失败计数达到阈值后跳转 Open
//	Open     —— 打开（熔断）：请求被快速失败，冷却时间到达后跳转 HalfOpen
//	HalfOpen —— 半开（探测）：放行少量请求做探测，成功则回到 Closed，失败则立刻回到 Open
const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

// String 返回状态的中文名，便于日志阅读。
func (s State) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateHalfOpen:
		return "HalfOpen"
	case StateOpen:
		return "Open"
	default:
		return "Unknown"
	}
}

// 全局共享熔断器的默认参数。
const (
	// defaultMaxFailures 计数窗口内失败次数达到该值时熔断。
	defaultMaxFailures = 5
	// defaultBreakerInterval 失败计数窗口长度。
	defaultBreakerInterval = 60 * time.Second
	// defaultBreakerTimeout 熔断打开后的冷却时长，冷却结束后进入半开。
	defaultBreakerTimeout = 30 * time.Second
	// defaultMaxHalfOpen 半开状态下允许同时放行的探测请求数。
	defaultMaxHalfOpen = 1
)

// CircuitBreaker 一个轻量三态熔断器。
//
// 采用纯标准库实现（sync/atomic 未使用，统一用互斥锁保护状态），
// 不引入 gobreaker / hystrix 等第三方依赖，避免给框架增加依赖负担。
//
// 计数模型：固定窗口计数（窗口长度 interval），窗口到期或发生状态跳转时清零重新计数。
// 该模型足够覆盖微服务调用保护场景，且实现简单、无额外内存开销。
type CircuitBreaker struct {
	name         string
	maxFailures  int
	interval     time.Duration
	timeout      time.Duration
	maxHalfOpen  int
	mu           sync.Mutex
	state        State
	failures     int
	successes    int
	windowStart  time.Time
	openedAt     time.Time
	halfOpenUsed int
}

// NewCircuitBreaker 创建一个熔断器。
// 参数:
//   - name: 熔断器名称，用于日志与 String()。
//   - maxFailures: 计数窗口内失败次数阈值，<=0 时使用 defaultMaxFailures。
//   - interval: 失败计数窗口长度，<=0 时使用 defaultBreakerInterval。
//   - timeout: 熔断打开后的冷却时长，<=0 时使用 defaultBreakerTimeout。
func NewCircuitBreaker(name string, maxFailures int, interval, timeout time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = defaultMaxFailures
	}
	if interval <= 0 {
		interval = defaultBreakerInterval
	}
	if timeout <= 0 {
		timeout = defaultBreakerTimeout
	}
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		interval:    interval,
		timeout:     timeout,
		maxHalfOpen: defaultMaxHalfOpen,
		state:       StateClosed,
		windowStart: time.Now(),
	}
}

// Name 返回熔断器名称。
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// Allow 判断当前请求是否允许放行。
// 熔断打开且未到冷却时间时返回 false（快速失败）；冷却时间到达时会自动切换到半开并放行探测请求。
// 一旦 Allow 返回 true，调用方必须在请求结束后调用 Success() 或 Failure() 归还半开配额。
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.openedAt) < cb.timeout {
			return false
		}
		// 冷却结束，进入半开放行探测请求
		cb.toHalfOpenLocked()
		cb.halfOpenUsed++
		logs.Warn("熔断器{}由 Open 转入 HalfOpen，放行探测请求", cb.name)
		return true
	case StateHalfOpen:
		if cb.halfOpenUsed >= cb.maxHalfOpen {
			return false
		}
		cb.halfOpenUsed++
		return true
	default:
		return true
	}
}

// Success 上报一次成功结果。
func (cb *CircuitBreaker) Success() {
	cb.result(true)
}

// Failure 上报一次失败结果。
func (cb *CircuitBreaker) Failure() {
	cb.result(false)
}

// State 返回熔断器当前状态。
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen && time.Since(cb.openedAt) >= cb.timeout {
		return StateHalfOpen
	}
	return cb.state
}

// Counts 返回当前计数窗口内的失败次数与成功次数。
func (cb *CircuitBreaker) Counts() (failures int, successes int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures, cb.successes
}

// Reset 强制把熔断器恢复到关闭状态并清零计数。
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.toClosedLocked()
}

// String 返回熔断器的可读描述，用于错误信息与日志。
func (cb *CircuitBreaker) String() string {
	failures, successes := cb.Counts()
	return fmt.Sprintf("breaker[%s] state=%s failures=%d successes=%d", cb.name, cb.State(), failures, successes)
}

// result 在锁内更新计数并驱动状态跳转。
func (cb *CircuitBreaker) result(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.rollWindowLocked()
	// 归还半开配额
	if cb.state == StateHalfOpen && cb.halfOpenUsed > 0 {
		cb.halfOpenUsed--
	}
	if success {
		cb.successes++
		if cb.state == StateHalfOpen && cb.successes >= cb.maxHalfOpen {
			cb.toClosedLocked()
			logs.Info("熔断器{}探测成功，由 HalfOpen 转入 Closed", cb.name)
		}
		return
	}
	cb.failures++
	switch cb.state {
	case StateHalfOpen:
		cb.toOpenLocked()
		logs.Warn("熔断器{}探测失败，由 HalfOpen 转入 Open", cb.name)
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.toOpenLocked()
			logs.Warn("熔断器{}窗口内失败{}次，达到阈值{}，转入 Open", cb.name, cb.failures, cb.maxFailures)
		}
	}
}

// rollWindowLocked 计数窗口到期时清零计数。调用方必须持有锁。
func (cb *CircuitBreaker) rollWindowLocked() {
	if cb.interval <= 0 {
		return
	}
	if time.Since(cb.windowStart) < cb.interval {
		return
	}
	cb.failures = 0
	cb.successes = 0
	cb.windowStart = time.Now()
}

// toOpenLocked 切换到打开状态。调用方必须持有锁。
func (cb *CircuitBreaker) toOpenLocked() {
	cb.state = StateOpen
	cb.openedAt = time.Now()
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenUsed = 0
	cb.windowStart = time.Now()
}

// toHalfOpenLocked 切换到半开状态。调用方必须持有锁。
func (cb *CircuitBreaker) toHalfOpenLocked() {
	cb.state = StateHalfOpen
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenUsed = 0
	cb.windowStart = time.Now()
}

// toClosedLocked 切换到关闭状态。调用方必须持有锁。
func (cb *CircuitBreaker) toClosedLocked() {
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenUsed = 0
	cb.windowStart = time.Now()
}

// 全局熔断器注册表：同一个下游服务在所有调用点共享一个熔断器，
// 避免每个调用点各自计数导致熔断阈值被放大。
var (
	breakerMu sync.Mutex
	breakers  = make(map[string]*CircuitBreaker)
)

// GetBreaker 按名称获取（或首次创建）一个全局共享的熔断器。
// 未显式指定参数时使用默认阈值：60 秒窗口内失败 5 次即熔断，冷却 30 秒后半开探测。
// 需要自定义参数时请使用 NewCircuitBreaker 并把结果赋给 ResilienceOptions.Breaker。
func GetBreaker(name string) *CircuitBreaker {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	if cb, ok := breakers[name]; ok {
		return cb
	}
	cb := NewCircuitBreaker(name, defaultMaxFailures, defaultBreakerInterval, defaultBreakerTimeout)
	breakers[name] = cb
	return cb
}

// ResetBreakers 清空全局熔断器注册表，主要用于测试场景。
func ResetBreakers() {
	breakerMu.Lock()
	defer breakerMu.Unlock()
	breakers = make(map[string]*CircuitBreaker)
}
