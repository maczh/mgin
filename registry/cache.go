package registry

import (
	"sync"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/sadlil/gologger"
)

var logger = gologger.GetLogger()

const (
	defaultCacheTTL     = 3 * time.Second
	defaultCacheRefresh = 1 * time.Second
)

// CachedRegistry 是在真实注册中心客户端之上的一层“服务发现缓存装饰器”。
//
// 目的：
//  1. 性能：绝大多数 GetServiceURL 调用直接命中本地缓存，零注册中心 RTT，
//     将注册中心的访问压力降低 1~2 个数量级。
//  2. 稳定性：注册中心抖动/短暂不可用时，复用上一次成功发现的实例（降级），
//     避免把所有微服务调用瞬间打挂。
//  3. 并发：对同一服务名+分组的并发发现请求做 singleflight 合并，只打一次后端。
//
// 后台有一个轻量刷新协程，仅对“仍被访问（热）”的 key 周期性重新发现，
// 以便在 TTL 窗口内让负载在多个实例间轮换，并在实例下线时及时剔除。
type CachedRegistry struct {
	delegate RegistryClient

	mu      sync.Mutex
	entries map[string]*cacheEntry
	inflight map[string]*singleFlight
	ttl     time.Duration

	refresh *time.Ticker
	stop    chan struct{}
	once    sync.Once
}

type cacheEntry struct {
	url        string
	group      string
	service    string
	groupArgs  []string
	expiresAt  time.Time
	lastAccess time.Time
}

// singleFlight 合并同一 key 的并发发现请求，leader 真正调用后端，follower 等待结果。
type singleFlight struct {
	wg sync.WaitGroup
}

// NewCachedRegistry 用装饰器包裹真实注册中心客户端。
func NewCachedRegistry(delegate RegistryClient) *CachedRegistry {
	ttl := config.Config.Discovery.CacheTTL
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	refresh := config.Config.Discovery.CacheRefresh
	if refresh <= 0 {
		refresh = defaultCacheRefresh
	}
	c := &CachedRegistry{
		delegate: delegate,
		entries:  make(map[string]*cacheEntry),
		inflight: make(map[string]*singleFlight),
		ttl:      ttl,
		stop:     make(chan struct{}),
	}
	c.refresh = time.NewTicker(refresh)
	go c.backgroundRefresh()
	return c
}

// GetServiceURL 实现 RegistryClient：优先返回缓存，缺失/过期时回源并缓存。
func (c *CachedRegistry) GetServiceURL(servicename string, groupName ...string) (string, string) {
	firstGroup := ""
	if len(groupName) > 0 {
		firstGroup = groupName[0]
	}
	key := servicename + "\x00" + firstGroup

	c.mu.Lock()
	if entry, ok := c.entries[key]; ok {
		entry.lastAccess = time.Now()
		if time.Now().Before(entry.expiresAt) {
			url, grp := entry.url, entry.group
			c.mu.Unlock()
			return url, grp
		}
	}
	sf, exists := c.inflight[key]
	if !exists {
		sf = &singleFlight{}
		sf.wg.Add(1)
		c.inflight[key] = sf
	}
	c.mu.Unlock()

	if exists {
		// 已有 leader 在回源，等待其完成后直接复用结果
		sf.wg.Wait()
		c.mu.Lock()
		if e, ok := c.entries[key]; ok {
			url, grp := e.url, e.group
			c.mu.Unlock()
			return url, grp
		}
		c.mu.Unlock()
		// leader 也未拿到实例（后端确实无实例），做一次非合并的直查兜底
	}

	url, grp := c.delegate.GetServiceURL(servicename, groupName...)
	c.mu.Lock()
	if url != "" {
		c.entries[key] = &cacheEntry{
			url:        url,
			group:      grp,
			service:    servicename,
			groupArgs:  groupName,
			expiresAt:  time.Now().Add(c.ttl),
			lastAccess: time.Now(),
		}
	} else if e, ok := c.entries[key]; ok {
		// 后端无实例但有旧缓存：降级复用，避免全部调用失败
		e.expiresAt = time.Now().Add(c.ttl)
		e.lastAccess = time.Now()
		url = e.url
		grp = e.group
		logger.Warn("服务 " + servicename + " 发现后端无实例，复用旧缓存实例: " + url)
	}
	delete(c.inflight, key)
	if !exists {
		sf.wg.Done()
	}
	c.mu.Unlock()
	return url, grp
}

// Register / DeRegister 直接透传给真实客户端，并在关闭时优雅停止后台刷新。
func (c *CachedRegistry) Register(registryConfigData []byte) {
	c.delegate.Register(registryConfigData)
}

func (c *CachedRegistry) DeRegister() {
	c.delegate.DeRegister()
	c.once.Do(func() {
		c.refresh.Stop()
		close(c.stop)
	})
}

// backgroundRefresh 周期性刷新“热”key，让负载在实例间轮换并剔除已下线的实例。
func (c *CachedRegistry) backgroundRefresh() {
	for {
		select {
		case <-c.stop:
			return
		case <-c.refresh.C:
			c.refreshHot()
		}
	}
}

func (c *CachedRegistry) refreshHot() {
	c.mu.Lock()
	keys := make([]string, 0, len(c.entries))
	svcs := make([]string, 0, len(c.entries))
	args := make([][]string, 0, len(c.entries))
	for k, e := range c.entries {
		if time.Since(e.lastAccess) < c.ttl {
			keys = append(keys, k)
			svcs = append(svcs, e.service)
			args = append(args, e.groupArgs)
		}
	}
	c.mu.Unlock()

	for i, k := range keys {
		url, grp := c.delegate.GetServiceURL(svcs[i], args[i]...)
		c.mu.Lock()
		if url != "" {
			if e, ok := c.entries[k]; ok {
				e.url = url
				e.group = grp
				e.expiresAt = time.Now().Add(c.ttl)
				e.lastAccess = time.Now()
			} else {
				c.entries[k] = &cacheEntry{
					url: url, group: grp, service: svcs[i], groupArgs: args[i],
					expiresAt: time.Now().Add(c.ttl), lastAccess: time.Now(),
				}
			}
		}
		c.mu.Unlock()
	}
}
