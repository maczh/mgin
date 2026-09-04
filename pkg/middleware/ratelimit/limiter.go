package ratelimit

import (
	"math"
	"sync"
	"time"
)

// limiter 限流器统一接口。
// acquire 返回 (是否放行, 建议重试等待时长)；release 用于并发数算法归还令牌。
type limiter interface {
	acquire(wait time.Duration) (bool, time.Duration)
	release()
	remaining() int
	lastSeen() time.Time
}

// ---------------------------------------------------------------- 令牌桶

// tokenBucket 令牌桶限流器。
// 以 rate 个/秒的速度往容量为 capacity 的桶中补充令牌，请求到来时取走一个令牌，
// 取不到则被限流。可以平滑地允许 capacity 大小的突发流量。
type tokenBucket struct {
	mu       sync.Mutex
	capacity float64
	rate     float64 // 每秒补充令牌数
	tokens   float64
	last     time.Time
	seen     time.Time
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	now := time.Now()
	return &tokenBucket{
		capacity: float64(burst),
		rate:     rate,
		tokens:   float64(burst),
		last:     now,
		seen:     now,
	}
}

// refill 按流逝时间补充令牌，调用方需持锁
func (t *tokenBucket) refill(now time.Time) {
	elapsed := now.Sub(t.last).Seconds()
	if elapsed <= 0 {
		return
	}
	t.tokens += elapsed * t.rate
	if t.tokens > t.capacity {
		t.tokens = t.capacity
	}
	t.last = now
}

func (t *tokenBucket) acquire(wait time.Duration) (bool, time.Duration) {
	t.mu.Lock()
	now := time.Now()
	t.seen = now
	t.refill(now)
	if t.tokens >= 1 {
		t.tokens--
		t.mu.Unlock()
		return true, 0
	}
	// 计算补足一个令牌所需时间
	need := (1 - t.tokens) / t.rate
	retry := time.Duration(need * float64(time.Second))
	if wait <= 0 || retry > wait {
		t.mu.Unlock()
		return false, retry
	}
	// 允许排队等待：预扣令牌，避免并发下超发
	t.tokens--
	t.mu.Unlock()
	timer := time.NewTimer(retry)
	<-timer.C
	timer.Stop()
	return true, 0
}

func (t *tokenBucket) release() {}

func (t *tokenBucket) remaining() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.refill(time.Now())
	if t.tokens < 0 {
		return 0
	}
	return int(math.Floor(t.tokens))
}

func (t *tokenBucket) lastSeen() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.seen
}

// ---------------------------------------------------------------- 滑动窗口

// slidingWindow 滑动日志窗口限流器。
// 用容量为 limit 的环形缓冲记录最近 limit 次放行的时间戳，
// 桶满时比较最早的时间戳是否已滑出窗口，精确保证「任意 window 时间内不超过 limit 次」。
// 相比固定窗口计数器，不存在窗口边界处的双倍突刺问题。
type slidingWindow struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	ring   []int64 // 环形缓冲，存放纳秒时间戳
	idx    int     // 指向最早的记录（缓冲已满时）
	count  int
	seen   time.Time
}

func newSlidingWindow(limit int, window time.Duration) *slidingWindow {
	if limit < 1 {
		limit = 1
	}
	return &slidingWindow{
		limit:  limit,
		window: window,
		ring:   make([]int64, limit),
		seen:   time.Now(),
	}
}

// try 尝试放行一次，返回 (是否放行, 需等待时长)，调用方需持锁
func (s *slidingWindow) try(now time.Time) (bool, time.Duration) {
	nowNano := now.UnixNano()
	cutoff := nowNano - int64(s.window)
	// 缓冲未满，直接放行
	if s.count < s.limit {
		s.ring[s.idx] = nowNano
		s.idx = (s.idx + 1) % s.limit
		s.count++
		return true, 0
	}
	// 缓冲已满，idx 指向最早的记录
	earliest := s.ring[s.idx]
	if earliest > cutoff {
		// 最早的请求仍在窗口内，需等到它滑出窗口
		return false, time.Duration(earliest - cutoff)
	}
	s.ring[s.idx] = nowNano
	s.idx = (s.idx + 1) % s.limit
	return true, 0
}

func (s *slidingWindow) acquire(wait time.Duration) (bool, time.Duration) {
	s.mu.Lock()
	now := time.Now()
	s.seen = now
	ok, retry := s.try(now)
	s.mu.Unlock()
	if ok {
		return true, 0
	}
	if wait <= 0 || retry > wait {
		return false, retry
	}
	// 排队等待后重试一次
	timer := time.NewTimer(retry)
	<-timer.C
	timer.Stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	ok, retry = s.try(time.Now())
	return ok, retry
}

func (s *slidingWindow) release() {}

func (s *slidingWindow) remaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().UnixNano() - int64(s.window)
	used := 0
	for i := 0; i < s.count; i++ {
		if s.ring[i] > cutoff {
			used++
		}
	}
	if used > s.limit {
		return 0
	}
	return s.limit - used
}

func (s *slidingWindow) lastSeen() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen
}

// ---------------------------------------------------------------- 最大并发数

// concurrencyLimiter 并发数限流器，基于带缓冲 channel 实现的信号量。
// 请求进入时占用一个槽位，请求处理结束后归还，控制同时在处理中的请求数量。
type concurrencyLimiter struct {
	sem  chan struct{}
	mu   sync.Mutex
	seen time.Time
}

func newConcurrencyLimiter(max int) *concurrencyLimiter {
	if max < 1 {
		max = 1
	}
	return &concurrencyLimiter{
		sem:  make(chan struct{}, max),
		seen: time.Now(),
	}
}

func (c *concurrencyLimiter) acquire(wait time.Duration) (bool, time.Duration) {
	c.touch()
	// 先做一次非阻塞尝试
	select {
	case c.sem <- struct{}{}:
		return true, 0
	default:
	}
	if wait <= 0 {
		return false, 0
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case c.sem <- struct{}{}:
		return true, 0
	case <-timer.C:
		return false, 0
	}
}

func (c *concurrencyLimiter) release() {
	select {
	case <-c.sem:
	default:
	}
}

func (c *concurrencyLimiter) remaining() int {
	return cap(c.sem) - len(c.sem)
}

func (c *concurrencyLimiter) touch() {
	c.mu.Lock()
	c.seen = time.Now()
	c.mu.Unlock()
}

func (c *concurrencyLimiter) lastSeen() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 仍有请求在处理中时视为活跃，避免被 GC 回收
	if len(c.sem) > 0 {
		return time.Now()
	}
	return c.seen
}

// newLimiter 按规则创建对应算法的限流器
func newLimiter(r *Rule) limiter {
	switch r.Algorithm {
	case AlgoSlidingWindow:
		return newSlidingWindow(int(r.Rate), r.window())
	case AlgoConcurrency:
		return newConcurrencyLimiter(r.MaxConcurrent)
	default:
		return newTokenBucket(r.Rate, r.Burst)
	}
}
