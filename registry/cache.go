package registry

import (
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/sadlil/gologger"
)

var logger = gologger.GetLogger()

const (
	defaultCacheTTL = 5 * time.Second
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
// 缓存条目保留 5 秒，过期后删除；只有下一次请求到来时才重新查询注册中心。
type CachedRegistry struct {
	delegate RegistryClient

	mu       sync.Mutex
	entries  map[string]*cacheEntry
	inflight map[string]*singleFlight
	ttl      time.Duration
	strategy SelectionStrategy
	sequence uint64
	watchers map[string]func()
	stopped  bool

	once sync.Once
}
type cacheEntry struct {
	url        string
	group      string
	service    string
	groupArgs  []string
	expiresAt  time.Time
}

// singleFlight 合并同一 key 的并发发现请求，leader 真正调用后端，follower 等待结果。
type singleFlight struct {
	wg sync.WaitGroup
}

// NewCachedRegistry 用装饰器包裹真实注册中心客户端。
func NewCachedRegistry(delegate RegistryClient) *CachedRegistry {
	if delegate == nil {
		delegate = &noopRegistryClient{}
	}
	c := &CachedRegistry{
		delegate: delegate,
		entries:  make(map[string]*cacheEntry),
		inflight: make(map[string]*singleFlight),
		watchers: make(map[string]func()),
		ttl:      defaultCacheTTL,
	}
	c.strategy = StrategyRandom
	if config.Config != nil {
		if v := config.Config.GetConfigString("go.discovery.strategy"); v != "" {
			c.strategy = SelectionStrategy(v)
		}
	}
	if subscriber, ok := delegate.(OfflineSubscriber); ok {
		_ = subscriber.SubscribeOffline(func(service, group string) { c.Invalidate(service, group) })
	}
	return c
}

// Invalidate removes all cached groups for a service. An empty group matches
// every group and is useful for registry events that omit group metadata.
func (c *CachedRegistry) Invalidate(service, group string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.entries {
		if entry.service == service && (group == "" || entry.group == group) {
			delete(c.entries, key)
		}
	}
}

func (c *CachedRegistry) watch(service, group string) {
	watcher, ok := c.delegate.(ServiceWatcher)
	if !ok {
		return
	}
	key := service + "\x00" + group
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	if _, exists := c.watchers[key]; exists {
		c.mu.Unlock()
		return
	}
	c.watchers[key] = nil
	c.mu.Unlock()
	cancel, err := watcher.WatchService(service, group, func() { c.Invalidate(service, group) })
	if err != nil || cancel == nil {
		c.mu.Lock()
		delete(c.watchers, key)
		c.mu.Unlock()
		return
	}
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		cancel()
		return
	}
	c.watchers[key] = cancel
	c.mu.Unlock()
}

// GetServiceInstances exposes a complete instance list when the delegate
// supports it, applying the configured local selection strategy.
func (c *CachedRegistry) GetServiceInstances(service string, group ...string) ([]ServiceInstance, string) {
	p, ok := c.delegate.(InstanceProvider)
	if !ok {
		url, grp := c.GetServiceURL(service, group...)
		if url == "" {
			return nil, grp
		}
		return []ServiceInstance{{URL: url, Weight: 1}}, grp
	}
	instances, grp := p.GetServiceInstances(service, group...)
	return selectInstances(instances, c.strategy, &c.sequence), grp
}

func selectInstances(in []ServiceInstance, strategy SelectionStrategy, sequence *uint64) []ServiceInstance {
	if len(in) < 2 {
		return in
	}
	copyIn := append([]ServiceInstance(nil), in...)
	switch strategy {
	case StrategyRoundRobin:
		i := int(atomic.AddUint64(sequence, 1)-1) % len(copyIn)
		return []ServiceInstance{copyIn[i]}
	case StrategyWeighted:
		total := 0
		for _, v := range copyIn {
			w := v.Weight
			if w < 1 {
				w = 1
			}
			total += w
		}
		if total > 0 {
			n := int(atomic.AddUint64(sequence, 1)-1) % total
			for _, v := range copyIn {
				w := v.Weight
				if w < 1 {
					w = 1
				}
				if n < w {
					return []ServiceInstance{v}
				}
				n -= w
			}
		}
	}
	return []ServiceInstance{copyIn[rand.Intn(len(copyIn))]}
}

// GetServiceURL 实现 RegistryClient：优先返回缓存，缺失/过期时回源并缓存。
func (c *CachedRegistry) GetServiceURL(servicename string, groupName ...string) (string, string) {
	key := servicename + "\x00" + strings.Join(groupName, "\x00")

	c.mu.Lock()
	if entry, ok := c.entries[key]; ok {
		if time.Now().Before(entry.expiresAt) {
			url, grp := entry.url, entry.group
			c.mu.Unlock()
			return url, grp
		}
		delete(c.entries, key)
	}
	c.mu.Unlock()
	if provider, ok := c.delegate.(InstanceProvider); ok {
		instances, group := provider.GetServiceInstances(servicename, groupName...)
		selected := selectInstances(instances, c.strategy, &c.sequence)
		if len(selected) > 0 && selected[0].URL != "" {
			c.mu.Lock()
			c.entries[key] = &cacheEntry{url: selected[0].URL, group: group, service: servicename, groupArgs: groupName, expiresAt: time.Now().Add(c.ttl)}
			c.mu.Unlock()
			c.watch(servicename, group)
			return selected[0].URL, group
		}
	}
	c.mu.Lock()
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
	c.watch(servicename, grp)
	c.mu.Lock()
	if url != "" {
		c.entries[key] = &cacheEntry{
			url:        url,
			group:      grp,
			service:    servicename,
			groupArgs:  groupName,
			expiresAt:  time.Now().Add(c.ttl),
		}
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
	if c.delegate == nil {
		return
	}
	c.delegate.Register(registryConfigData)
}

func (c *CachedRegistry) DeRegister() {
	c.once.Do(func() {
		c.mu.Lock()
		for key, cancel := range c.watchers {
			if cancel != nil {
				cancel()
			}
			delete(c.watchers, key)
		}
		c.entries = make(map[string]*cacheEntry)
		c.mu.Unlock()
		if c.delegate != nil {
			c.delegate.DeRegister()
		}
	})
}
