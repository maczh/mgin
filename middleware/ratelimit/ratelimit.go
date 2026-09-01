package ratelimit

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/sadlil/gologger"
)

var logger = gologger.GetLogger()

// Manager 单例限流管理器。
// 全进程唯一，集中持有所有维度的限流器实例，并通过后台协程定期回收空闲限流器，
// 避免按 IP / 路径维度限流时限流器无限增长造成内存泄漏。
type Manager struct {
	mu       sync.RWMutex
	cfg      *Config
	limiters map[string]limiter
	stopOnce sync.Once
	stopChan chan struct{}
}

var (
	instance *Manager
	initOnce sync.Once
)

// GetManager 获取单例限流管理器，首次调用时自动从 application.yml 加载配置并启动回收协程
func GetManager() *Manager {
	initOnce.Do(func() {
		instance = newManager(LoadConfig())
	})
	return instance
}

func newManager(cfg *Config) *Manager {
	m := &Manager{
		cfg:      cfg,
		limiters: make(map[string]limiter),
		stopChan: make(chan struct{}),
	}
	m.startGC()
	return m
}

// Reload 重新从配置中心/配置文件加载限流配置并重置所有限流器，用于配置热更新
func (m *Manager) Reload() {
	m.ReloadWith(LoadConfig())
}

// ReloadWith 使用给定配置替换当前配置，并清空已有限流器
func (m *Manager) ReloadWith(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.limiters = make(map[string]limiter)
	logger.Info(fmt.Sprintf("限流配置已重载, enabled=%v, 独立规则数=%d", cfg.Enabled, len(cfg.Rules)))
}

// Config 返回当前生效的限流配置
func (m *Manager) Config() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// startGC 启动空闲限流器回收协程
func (m *Manager) startGC() {
	go func() {
		ticker := time.NewTicker(defaultGCInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.gc()
			case <-m.stopChan:
				return
			}
		}
	}()
}

// gc 清理超过空闲时间未被访问的限流器
func (m *Manager) gc() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.limiters) == 0 {
		return
	}
	deadline := time.Now().Add(-m.cfg.idleTimeout)
	for k, l := range m.limiters {
		if l.lastSeen().Before(deadline) {
			delete(m.limiters, k)
		}
	}
}

// Stop 停止回收协程，通常无需调用（进程退出即可）
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

// Count 返回当前活跃限流器数量，可用于监控
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.limiters)
}

// getLimiter 按 key 获取限流器，不存在则按规则创建（double-check 保证并发安全）
func (m *Manager) getLimiter(key string, r *Rule) limiter {
	m.mu.RLock()
	l, ok := m.limiters[key]
	m.mu.RUnlock()
	if ok {
		return l
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok = m.limiters[key]; ok {
		return l
	}
	l = newLimiter(r)
	m.limiters[key] = l
	return l
}

// matchRule 为请求匹配限流规则：优先按声明顺序匹配独立规则，未命中则回落到全局规则
func (m *Manager) matchRule(method, routePath, urlPath string) *Rule {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()
	for i := range cfg.Rules {
		if cfg.Rules[i].match(method, routePath, urlPath) {
			return &cfg.Rules[i]
		}
	}
	if cfg.global.limited() {
		return &cfg.global
	}
	return nil
}

// skip 判断请求是否命中白名单
func (m *Manager) skip(clientIp, urlPath string) bool {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()
	for _, p := range cfg.Whitelist {
		p = strings.TrimSpace(p)
		if p != "" && strings.HasPrefix(urlPath, p) {
			return true
		}
	}
	for _, ip := range cfg.WhiteIps {
		if strings.TrimSpace(ip) == clientIp {
			return true
		}
	}
	return false
}

// limiterKey 按维度生成限流器 key
func limiterKey(r *Rule, c *gin.Context) string {
	switch r.Dimension {
	case DimIP:
		return r.Name + "|ip:" + c.ClientIP()
	case DimPath:
		return r.Name + "|path:" + routeOf(c)
	case DimIPPath:
		return r.Name + "|ip:" + c.ClientIP() + "|path:" + routeOf(c)
	case DimHeader:
		key := r.HeaderKey
		if key == "" {
			key = "Authorization"
		}
		return r.Name + "|h:" + key + ":" + c.GetHeader(key)
	default:
		return r.Name + "|global"
	}
}

// routeOf 优先取 gin 路由模板，避免路径参数导致限流器数量膨胀
func routeOf(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return c.Request.URL.Path
}

// RateLimit 返回限流中间件，配置从 application.yml 的 go.ratelimit 节点读取。
//
//	app.Router.Use(ratelimit.RateLimit())
//
// 未配置或 enabled=false 时中间件直接放行，无任何性能开销。
func RateLimit() gin.HandlerFunc {
	return GetManager().Handler()
}

// RateLimitWith 使用代码指定的规则构建限流中间件（不读取 yml），
// 适用于对某个路由组单独施加限流策略的场景：
//
//	api.Use(ratelimit.RateLimitWith(ratelimit.Rule{
//	    Algorithm: ratelimit.AlgoConcurrency, MaxConcurrent: 10,
//	}))
func RateLimitWith(rules ...Rule) gin.HandlerFunc {
	cfg := &Config{Enabled: true, Rules: rules}
	cfg.normalize()
	m := newManager(cfg)
	return m.Handler()
}

// Handler 生成该管理器对应的 gin 中间件
func (m *Manager) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.mu.RLock()
		enabled := m.cfg.Enabled
		withHeaders := m.cfg.Headers
		m.mu.RUnlock()
		if !enabled {
			c.Next()
			return
		}
		urlPath := c.Request.URL.Path
		if m.skip(c.ClientIP(), urlPath) {
			c.Next()
			return
		}
		rule := m.matchRule(c.Request.Method, c.FullPath(), urlPath)
		if rule == nil || !rule.limited() {
			c.Next()
			return
		}

		l := m.getLimiter(limiterKey(rule, c), rule)
		ok, retry := l.acquire(rule.waitTimeout())
		if !ok {
			if withHeaders {
				writeHeaders(c, rule, 0, retry)
			}
			logger.Debug(fmt.Sprintf("[RateLimit] 规则[%s] 算法[%s] 拒绝请求 %s %s from %s",
				rule.Name, rule.Algorithm, c.Request.Method, urlPath, c.ClientIP()))
			c.AbortWithStatusJSON(rule.HttpStatus, models.Error(rule.Code, rule.Message))
			return
		}
		// 并发数算法需要在请求处理结束后归还槽位
		if rule.Algorithm == AlgoConcurrency {
			defer l.release()
		}
		if withHeaders {
			writeHeaders(c, rule, l.remaining(), 0)
		}
		c.Next()
	}
}

// writeHeaders 输出限流相关响应头，便于客户端做退避
func writeHeaders(c *gin.Context, r *Rule, remaining int, retry time.Duration) {
	limit := int(r.Rate)
	if r.Algorithm == AlgoConcurrency {
		limit = r.MaxConcurrent
	}
	c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
	c.Header("X-RateLimit-Policy", string(r.Algorithm))
	if retry > 0 {
		sec := int(retry.Seconds())
		if sec < 1 {
			sec = 1
		}
		c.Header("Retry-After", strconv.Itoa(sec))
	}
}

// Allow 提供非 HTTP 场景（如 MQ 消费、定时任务、RPC 调用）的编程式限流入口。
// key 为自定义限流维度标识，rule 为限流规则。返回 true 表示放行。
// 使用 concurrency 算法时，务必在处理结束后调用返回的 release 函数归还槽位。
func (m *Manager) Allow(key string, rule Rule) (bool, func()) {
	rule.normalize()
	if !rule.limited() {
		return true, func() {}
	}
	if rule.Name == "" {
		rule.Name = "custom"
	}
	l := m.getLimiter(rule.Name+"|"+key, &rule)
	ok, _ := l.acquire(rule.waitTimeout())
	if !ok {
		return false, func() {}
	}
	if rule.Algorithm == AlgoConcurrency {
		return true, l.release
	}
	return true, func() {}
}

// Allow 单例管理器的编程式限流入口，等价于 GetManager().Allow(...)
func Allow(key string, rule Rule) (bool, func()) {
	return GetManager().Allow(key, rule)
}
