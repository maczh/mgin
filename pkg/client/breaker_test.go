package client

import (
	"errors"
	"testing"
	"time"
)

// TestCircuitBreakerStateTransitions 验证三态熔断器的关键状态流转。
//
// 时间相关断言只检查相对量（如“Open 状态至少持续到超时时刻”），不依赖 wall clock，
// 避免在 CI 中出现偶发抖动。
func TestCircuitBreakerStateTransitions(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, time.Hour, 50*time.Millisecond)
	if cb.State() != StateClosed {
		t.Fatalf("初始状态应为 Closed，实际 %s", cb.State())
	}

	// 连续 3 次失败应打开熔断器。
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("Closed 状态下第 %d 次 Allow 应放行", i+1)
		}
		cb.Failure()
	}
	if cb.State() != StateOpen {
		t.Fatalf("3 次失败后期望 Open，实际 %s", cb.State())
	}
	// Open 状态下应快速失败。
	if cb.Allow() {
		t.Fatal("Open 状态下 Allow 应返回 false")
	}

	// 等待冷却时间到期，进入 HalfOpen 放行一次探测。
	time.Sleep(60 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("冷却结束后期望 HalfOpen 放行一次")
	}
	// 探测失败应立刻回到 Open。
	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatalf("HalfOpen 探测失败后期望回到 Open，实际 %s", cb.State())
	}

	// 再冷却、放行一次、探测成功 → Closed。
	time.Sleep(60 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("二次冷却后期望 HalfOpen 放行")
	}
	cb.Success()
	if cb.State() != StateClosed {
		t.Fatalf("HalfOpen 探测成功后期望 Closed，实际 %s", cb.State())
	}
}

// TestCircuitBreakerFailureAccumulation 验证窗口内失败按“计数累计”打开熔断：
// Success 不会清零 failures 计数器（breaker 采用固定窗口计数）。
// 本测试与实现的“固定窗口计数”语义对齐。
func TestCircuitBreakerFailureAccumulation(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, time.Hour, time.Second)
	// 1 次失败 + 1 次成功（成功不应清零失败计数）
	if !cb.Allow() {
		t.Fatal("Closed 状态应放行")
	}
	cb.Failure()
	if !cb.Allow() {
		t.Fatal("Closed 状态应放行")
	}
	cb.Success()
	if cb.State() != StateClosed {
		t.Fatalf("成功不应改变 Closed 状态，实际 %s", cb.State())
	}
	// 再来 2 次失败（累计 3 次）应触发 Open
	if !cb.Allow() {
		t.Fatal("Closed 状态应放行")
	}
	cb.Failure()
	if cb.State() != StateClosed {
		t.Fatalf("累计 2 次失败仍应保持 Closed，实际 %s", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("Closed 状态应放行")
	}
	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatalf("累计 3 次失败后期望 Open，实际 %s", cb.State())
	}
}

// TestCircuitBreakerDefaults 验证非法参数会回落到默认值。
func TestCircuitBreakerDefaults(t *testing.T) {
	cb := NewCircuitBreaker("d", 0, 0, 0)
	if cb.maxFailures != defaultMaxFailures {
		t.Fatalf("maxFailures 应回落到 %d，实际 %d", defaultMaxFailures, cb.maxFailures)
	}
	if cb.interval != defaultBreakerInterval {
		t.Fatalf("interval 应回落到 %v，实际 %v", defaultBreakerInterval, cb.interval)
	}
	if cb.timeout != defaultBreakerTimeout {
		t.Fatalf("timeout 应回落到 %v，实际 %v", defaultBreakerTimeout, cb.timeout)
	}
}

// TestExtractStatusCode 验证从错误文本中解析 HTTP 状态码的能力。
func TestExtractStatusCode(t *testing.T) {
	cases := []struct {
		err    error
		want   int
		wantOK bool
	}{
		{errors.New("some random error"), 0, false},
		{errors.New("server returned status code 503"), 503, true},
		{errors.New("HTTP 500 Internal Server Error"), 500, true},
		{errors.New("upstream code=502 bad gateway"), 502, true},
		{errors.New("status:404 not found"), 404, true},
		// 状态码必须是 100-599，否则视为未解析。
		{errors.New("status code 999"), 0, false},
		// 纯端口号不应被误判。
		{errors.New("dial tcp 127.0.0.1:8080: connection refused"), 0, false},
	}
	for _, c := range cases {
		got, ok := extractStatusCode(c.err)
		if ok != c.wantOK || got != c.want {
			t.Errorf("extractStatusCode(%q) = (%d,%v)，期望 (%d,%v)", c.err.Error(), got, ok, c.want, c.wantOK)
		}
	}
}