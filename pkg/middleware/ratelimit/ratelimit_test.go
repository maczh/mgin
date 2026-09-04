package ratelimit

import (
	"testing"
	"time"
)

// newTestManager 构造一个用于测试的单例 Manager（绕过 yml 配置）
func newTestManager(rules ...Rule) *Manager {
	cfg := &Config{Enabled: true}
	cfg.normalize()
	m := newManager(cfg)
	for i := range rules {
		m.cfg.Rules = append(m.cfg.Rules, rules[i])
	}
	return m
}

func TestConcurrencyLimiter(t *testing.T) {
	m := newTestManager(Rule{
		Name:          "c",
		Algorithm:     AlgoConcurrency,
		Dimension:     DimGlobal,
		MaxConcurrent: 2,
	})
	key := "k"
	ok1, r1 := m.Allow(key, m.cfg.Rules[0])
	ok2, r2 := m.Allow(key, m.cfg.Rules[0])
	if !ok1 || !ok2 {
		t.Fatalf("期望前两次并发获取成功, got ok1=%v ok2=%v", ok1, ok2)
	}
	ok3, _ := m.Allow(key, m.cfg.Rules[0])
	if ok3 {
		t.Fatalf("期望第三次并发被拒绝")
	}
	r1()
	r2()
	ok4, r4 := m.Allow(key, m.cfg.Rules[0])
	if !ok4 {
		t.Fatalf("释放后期望可再次获取")
	}
	r4()
}

func TestTokenBucketLimiter(t *testing.T) {
	m := newTestManager(Rule{
		Name:      "tb",
		Algorithm: AlgoTokenBucket,
		Dimension: DimGlobal,
		Rate:      1,
		Burst:     2,
	})
	key := "tbk"
	// burst=2，初次应允许 2 次，第 3 次被限流
	if ok, _ := m.Allow(key, m.cfg.Rules[0]); !ok {
		t.Fatalf("第1次应成功")
	}
	if ok, _ := m.Allow(key, m.cfg.Rules[0]); !ok {
		t.Fatalf("第2次(burst)应成功")
	}
	if ok, _ := m.Allow(key, m.cfg.Rules[0]); ok {
		t.Fatalf("第3次应被限流")
	}
	// 等待一个令牌补充周期后应再次放行
	time.Sleep(1100 * time.Millisecond)
	if ok, r := m.Allow(key, m.cfg.Rules[0]); !ok {
		t.Fatalf("令牌补充后应放行")
	} else {
		r()
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	m := newTestManager(Rule{
		Name:      "sw",
		Algorithm: AlgoSlidingWindow,
		Dimension: DimGlobal,
		Rate:      3,
		Window:    2, // 2 秒窗口
	})
	key := "swk"
	var allowed int
	for i := 0; i < 5; i++ {
		if ok, r := m.Allow(key, m.cfg.Rules[0]); ok {
			allowed++
			r()
		}
	}
	if allowed != 3 {
		t.Fatalf("滑动窗口窗口内应允许 3 次, got %d", allowed)
	}
}

func TestConfigMatch(t *testing.T) {
	m := newTestManager(
		Rule{Name: "login", Path: "/api/login/*", Methods: []string{"POST"}},
		Rule{Name: "glob", Path: "/api/order", Methods: []string{"GET", "POST"}},
	)
	// 前缀匹配：routePath 带 /* 触发 prefixMode，匹配任意子路径
	if m.matchRule("POST", "/api/login/*", "/api/login/submit") == nil {
		t.Fatalf("期望匹配 /api/login/* 前缀规则")
	}
	// 精确匹配（方法不符应不匹配）
	if m.matchRule("DELETE", "/api/order", "/api/order") != nil {
		t.Fatalf("DELETE 方法不在 order 规则允许列表中，应不匹配")
	}
	if m.matchRule("GET", "/api/order", "/api/order") == nil {
		t.Fatalf("GET 应匹配 order 规则")
	}
	// 未配置规则路径不应匹配
	if m.matchRule("GET", "/other", "/other") != nil {
		t.Fatalf("/other 无对应规则，应不匹配")
	}
}

func TestRateLimitWith(t *testing.T) {
	// 仅验证 RateLimitWith 返回一个可用中间件且不会 panic
	mw := RateLimitWith(Rule{Name: "x", Algorithm: AlgoConcurrency, Dimension: DimGlobal, MaxConcurrent: 1})
	if mw == nil {
		t.Fatalf("RateLimitWith 应返回非 nil 中间件")
	}
}
