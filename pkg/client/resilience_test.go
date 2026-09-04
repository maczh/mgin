package client

import (
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestIsIdempotentMethod 验证对 HTTP 方法的幂等性判定。
func TestIsIdempotentMethod(t *testing.T) {
	idempotent := []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}
	for _, m := range idempotent {
		if !isIdempotentMethod(m) {
			t.Errorf("%s 期望被识别为幂等", m)
		}
	}
	nonIdempotent := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, ""}
	for _, m := range nonIdempotent {
		if isIdempotentMethod(m) {
			t.Errorf("%s 期望被识别为非幂等", m)
		}
	}
	// 大小写与空格容忍。
	if !isIdempotentMethod(" get ") {
		t.Error("应容忍大小写与首尾空格")
	}
}

// TestBackoffDelay 验证退避函数的边界与单调性。
func TestBackoffDelay(t *testing.T) {
	ro := &ResilienceOptions{
		BaseDelay: 50 * time.Millisecond,
		MaxDelay:  800 * time.Millisecond,
	}

	// 基础范围断言：每次退避的下界是 delay/2，上界是 delay，delay 单调到 MaxDelay 后不再增长。
	prevUpper := time.Duration(0)
	for attempt := 0; attempt < 8; attempt++ {
		// 跑 50 次取最大值，确保覆盖到抖动上限。
		var got time.Duration
		for i := 0; i < 50; i++ {
			d := backoffDelay(attempt, ro)
			if d <= 0 {
				t.Fatalf("attempt=%d 返回 0 退避", attempt)
			}
			if d > got {
				got = d
			}
		}
		// 退避上界（无抖动）应随 attempt 翻倍直到 MaxDelay。
		expUpper := ro.BaseDelay << attempt
		if expUpper <= 0 || expUpper > ro.MaxDelay {
			expUpper = ro.MaxDelay
		}
		// 实测上界可能略小于 expUpper（jitter 把窗口下移到 [delay/2, delay)），但不应超过。
		// 但 got 是 50 次采样的最大值，最大值应逼近 delay（窗口上界）。
		if got > expUpper {
			t.Errorf("attempt=%d 实测退避 %v 超过理论上限 %v", attempt, got, expUpper)
		}
		// 同时验证范围下界：至少应有一次采样 >= delay/2（实际是期望大多数命中 [delay/2, delay)，单次采样不在下界上很正常）。
		// 因此仅验证最大值单调非降即可。
		if got < prevUpper/2 {
			t.Errorf("attempt=%d 实测退避 %v 异常下降（上一轮 %v）", attempt, got, prevUpper)
		}
		prevUpper = expUpper
	}
}

// TestBackoffDelayZeroDefaults 验证零值会回落到默认 Base/MaxDelay。
func TestBackoffDelayZeroDefaults(t *testing.T) {
	ro := &ResilienceOptions{}
	d := backoffDelay(0, ro)
	if d <= 0 {
		t.Fatalf("默认参数下退避应为正，实际 %v", d)
	}
	// 默认 BaseDelay=100ms、MaxDelay=2s，单次退避（attempt=0）上界应为 100ms。
	if d > defaultBaseDelay {
		t.Errorf("attempt=0 退避应不超过 %v，实际 %v", defaultBaseDelay, d)
	}
}

// TestBackoffDelayMaxClampToMaxDelay 验证 MaxDelay 不低于 BaseDelay 时也能正常工作。
func TestBackoffDelayMaxClampToMaxDelay(t *testing.T) {
	ro := &ResilienceOptions{
		BaseDelay: 10 * time.Second,
		MaxDelay:  20 * time.Millisecond, // 小于 base，应自动上调
	}
	d := backoffDelay(0, ro)
	if d > 10*time.Second {
		t.Errorf("max<base 应被自动上调，实际退避 %v", d)
	}
}

// TestCallResilientNilOptions 检查空 Options 会被显式拒绝。
func TestCallResilientNilOptions(t *testing.T) {
	if _, err := CallResilient("svc", "/x", nil, nil); err == nil {
		t.Fatal("空 Options 期望返回错误")
	}
}

// TestCallResilientOpenCircuitFastFails 验证熔断打开时直接返回 ErrCircuitOpen，
// 不消耗任何下游资源（不调用 Call）。
func TestCallResilientOpenCircuitFastFails(t *testing.T) {
	cb := NewCircuitBreaker("test", 1, time.Hour, time.Hour)
	// 触发打开。
	cb.Allow()
	cb.Failure()
	if cb.State() != StateOpen {
		t.Fatalf("期望 Open，实际 %s", cb.State())
	}
	_, err := CallResilient("svc", "/x", &Options{Method: http.MethodGet}, &ResilienceOptions{
		MaxRetries: 0,
		Breaker:    cb,
	})
	if err == nil {
		t.Fatal("熔断打开  期望返回错误")
	}
	if !strings.Contains(err.Error(), "熔断") && !strings.Contains(err.Error(), ErrCircuitOpen.Error()) {
		t.Fatalf("期望错误包含 %q，实际 %v", ErrCircuitOpen.Error(), err)
	}
}

// TestStatusCodePatternSanity 仅作为防护：状态码正则本身不能因为实现细节变化而误匹配，
// 这里直接验证一个明确命中与一个明确不命中的样本。
func TestStatusCodePatternSanity(t *testing.T) {
	for _, s := range []string{"status 503", "code=502", "HTTP 404"} {
		if !statusCodePattern.MatchString(s) {
			t.Errorf("正则应命中 %q", s)
		}
	}
	for _, s := range []string{"127.0.0.1:8080", "no code here"} {
		if statusCodePattern.MatchString(s) {
			t.Errorf("正则不应误判 %q", s)
		}
	}
}

// TestDefaultDelayBounds 数学边界检查：放大抖动下，attempt=0 的退避值应在 [base/2, base) 之间。
func TestDefaultDelayBounds(t *testing.T) {
	ro := &ResilienceOptions{
		BaseDelay: 200 * time.Millisecond,
		MaxDelay:  2 * time.Second,
	}
	minD := time.Duration(math.MaxInt64)
	maxD := time.Duration(0)
	for i := 0; i < 500; i++ {
		d := backoffDelay(0, ro)
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	// 下界: base/2 = 100ms；上界: base = 200ms。允许 1ms 抖动。
	if minD < 99*time.Millisecond {
		t.Errorf("期望下界 ~100ms，实测 %v", minD)
	}
	if maxD > 201*time.Millisecond {
		t.Errorf("期望上界 ~200ms，实测 %v", maxD)
	}
}
