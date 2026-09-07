package registry

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logs"
)

// 发现缓存的默认参数（可在 application.yml 的 go.discovery.cacheTTL / cacheRefresh 中覆盖，单位秒）。
const (
	// defaultCacheTTL 服务实例列表在本地缓存的存活时间。
	// 缓存命中时不再访问注册中心，是高并发下降低注册中心压力、降低调用延迟的关键。
	defaultCacheTTL = 3 * time.Second
	// defaultCacheRefresh 后台预热刷新间隔，必须小于 defaultCacheTTL。
	// 通过周期性刷新保持缓存"温热"，避免缓存过期瞬间的集中回源（惊群）。
	defaultCacheRefresh = 1 * time.Second
)

// cacheEntry 是单个服务在某分组下的缓存条目。
// service / groups 用于在后台刷新生效时重新向注册中心发起查询。
type cacheEntry struct {
	service   string
	groups    []string
	urls      []string
	fetchedAt time.Time
	// err 记录最近一次回源的错误。err!=nil 且 urls 为空表示"彻底不可用"；
	// err!=nil 但 urls 非空表示"本次刷新失败，但仍在服务旧数据"（降级）。
	err error
}

// inFlight 用于抑制并发冷命中的"惊群"：同一 key 只放行一个回源协程，其余等待其完成。
type inFlight struct {
	done chan struct{}
}

// CachedRegistry 是 RegistryClient 的装饰器（decorator）。
//
// 解决的核心问题：原实现每次微服务调用（client.CallCtx → selectHostByLB → GetServices）
// 都会同步打一次注册中心（Nacos /list、etcd Get、Consul Health、Polaris GET），
// 在高 QPS 下既拖慢单次调用、又给注册中心造成巨大压力，且注册中心抖动会直接击穿到业务调用。
//
// 本装饰器在内存中缓存 GetServices 结果：
//   - 读取命中缓存（未过期）直接返回，零网络开销；
//   - 后台定时预热，保持缓存温热、及时感知实例上下线；
//   - 回源失败时优先复用旧实例列表（降级），避免注册中心瞬时故障导致全部调用失败；
//   - 并发冷命中通过 inFlight 去重，只回源一次；
//   - 进程关闭（DeRegister）时停止后台刷新协程，避免泄漏。
//
// 该层对调用方与注册中心后端完全透明，不改变 RegistryClient 接口语义。
type CachedRegistry struct {
	inner RegistryClient

	mu       sync.Mutex
	entries  map[string]*cacheEntry
	inflight map[string]*inFlight

	ttl     time.Duration
	refresh time.Duration
	stop    chan struct{}
	once    sync.Once
}

// NewCachedRegistry 用缓存层包裹一个真实的 RegistryClient。
// inner 为 nil 时直接返回 nil（与 NewRegistry 的兜底逻辑保持一致）。
func NewCachedRegistry(inner RegistryClient) *CachedRegistry {
	if inner == nil {
		return nil
	}
	c := &CachedRegistry{
		inner:    inner,
		entries:  make(map[string]*cacheEntry),
		inflight: make(map[string]*inFlight),
		ttl:      resolveTTL(config.Config.Discovery.CacheTTL, defaultCacheTTL),
		refresh:  resolveRefresh(config.Config.Discovery.CacheRefresh, defaultCacheRefresh, defaultCacheTTL),
		stop:     make(chan struct{}),
	}
	go c.refreshLoop()
	return c
}

// resolveTTL 把配置里的秒数转换为 Duration，<=0 时使用默认值。
func resolveTTL(sec int, def time.Duration) time.Duration {
	if sec <= 0 {
		return def
	}
	return time.Duration(sec) * time.Second
}

// resolveRefresh 后台刷新间隔，必须 <= ttl，否则退化为 ttl 的一半以防永远不刷新。
func resolveRefresh(sec int, def, ttl time.Duration) time.Duration {
	if sec <= 0 {
		return def
	}
	d := time.Duration(sec) * time.Second
	if d >= ttl {
		// 刷新间隔不得小于 TTL，否则回源频率会超过缓存生效频率，失去意义。
		return ttl / 2
	}
	return d
}

// cacheKey 由服务名 + 分组构造，与后端 GetServices 入参保持一一对应。
func cacheKey(service string, groups []string) string {
	return service + "\x00" + strings.Join(groups, ",")
}

// Register / DeRegister 直接透传到内层客户端；DeRegister 额外停止后台刷新。
func (c *CachedRegistry) Register(data []byte) {
	c.inner.Register(data)
}

// DeRegister 停止后台刷新协程，再透传注销。
func (c *CachedRegistry) DeRegister() {
	c.once.Do(func() { close(c.stop) })
	c.inner.DeRegister()
}

// GetServiceURL v1 单实例入口：复用缓存的实例列表，随机选 1 个（保持原语义）。
// 缓存未命中或回源失败时返回 ("", "")，与原 v1 行为一致。
func (c *CachedRegistry) GetServiceURL(servicename string, groupName ...string) (string, string) {
	urls, err := c.GetServices(servicename, groupName...)
	if err != nil || len(urls) == 0 {
		return "", ""
	}
	idx := rand.Intn(len(urls))
	grp := ""
	if len(groupName) > 0 {
		grp = groupName[0]
	}
	return urls[idx], grp
}

// GetServices v2 多实例入口：优先返回未过期的缓存；
// 未命中或已过期时回源，并通过 inFlight 抑制并发惊群；
// 回源出错但存在旧数据则降级返回旧数据，彻底无数据才返回错误。
func (c *CachedRegistry) GetServices(servicename string, groupName ...string) ([]string, error) {
	key := cacheKey(servicename, groupName)

	// 1) 快速路径：命中且未过期
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && time.Since(e.fetchedAt) < c.ttl {
		urls, err := e.urls, e.err
		c.mu.Unlock()
		if err != nil && len(urls) == 0 {
			return nil, err
		}
		return urls, nil
	}
	// 2) 已有协程在回源，等待其完成
	if g, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-g.done
		return c.readAfterRefresh(key)
	}
	// 3) 由本协程负责回源
	g := &inFlight{done: make(chan struct{})}
	c.inflight[key] = g
	c.mu.Unlock()

	urls, err := c.inner.GetServices(servicename, groupName...)

	c.mu.Lock()
	if err != nil && len(urls) == 0 {
		// 回源彻底失败：若仍有旧数据则保留（降级），否则记录错误。
		if old, ok := c.entries[key]; ok && len(old.urls) > 0 {
			delete(c.inflight, key)
			close(g.done)
			c.mu.Unlock()
			logs.Warn("服务发现刷新失败，复用旧实例列表 key={} err={}", key, err.Error())
			return old.urls, nil
		}
		c.entries[key] = &cacheEntry{
			service:   servicename,
			groups:    groupName,
			urls:      nil,
			fetchedAt: time.Now(),
			err:       err,
		}
	} else {
		c.entries[key] = &cacheEntry{
			service:   servicename,
			groups:    groupName,
			urls:      urls,
			fetchedAt: time.Now(),
			err:       err,
		}
	}
	delete(c.inflight, key)
	close(g.done)
	c.mu.Unlock()

	if err != nil && len(urls) == 0 {
		return nil, err
	}
	return urls, nil
}

// readAfterRefresh 在等待其他协程回源完成后读取最新结果。
func (c *CachedRegistry) readAfterRefresh(key string) ([]string, error) {
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("服务发现缓存缺失: %s", key)
	}
	if e.err != nil && len(e.urls) == 0 {
		return nil, e.err
	}
	return e.urls, nil
}

// refreshLoop 后台定时刷新所有已知 key（预热），使缓存始终温热。
// 回源失败时保留旧数据，仅当旧数据也不存在时才记录错误。
func (c *CachedRegistry) refreshLoop() {
	ticker := time.NewTicker(c.refresh)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.refreshAll()
		}
	}
}

// refreshAll 对当前所有已缓存 key 做一次回源刷新。
func (c *CachedRegistry) refreshAll() {
	c.mu.Lock()
	snapshot := make([]*cacheEntry, 0, len(c.entries))
	for _, e := range c.entries {
		snapshot = append(snapshot, e)
	}
	c.mu.Unlock()

	for _, e := range snapshot {
		// 用 inFlight 去重，避免与并发读请求重复回源。
		c.mu.Lock()
		key := cacheKey(e.service, e.groups)
		if g, ok := c.inflight[key]; ok {
			c.mu.Unlock()
			<-g.done
			continue
		}
		g := &inFlight{done: make(chan struct{})}
		c.inflight[key] = g
		c.mu.Unlock()

		urls, err := c.inner.GetServices(e.service, e.groups...)

		c.mu.Lock()
		if err != nil && len(urls) == 0 {
			if len(e.urls) == 0 {
				// 旧数据也没有，记录错误并刷新时间戳以免频繁重试。
				e.err = err
				e.fetchedAt = time.Now()
			}
			// 有旧数据：保留，仅更新 err，fetchedAt 不变（旧数据继续生效）。
		} else {
			e.urls = urls
			e.err = err
			e.fetchedAt = time.Now()
		}
		delete(c.inflight, key)
		close(g.done)
		c.mu.Unlock()
	}
}
