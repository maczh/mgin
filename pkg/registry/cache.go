package registry

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

)

// 发现缓存参数。
const (
	// defaultCacheTTL 服务实例列表在本地缓存的存活时间。
	defaultCacheTTL = 5 * time.Second
)

// cacheEntry 是单个服务在某分组下的缓存条目。
// service / groups 用于主动失效时定位缓存条目。
type cacheEntry struct {
	service   string
	groups    []string
	urls      []string
	fetchedAt time.Time
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
	//   - 缓存过期后删除，仅在下一次请求到来时按需回源；
//   - 并发冷命中通过 inFlight 去重，只回源一次；
//
// 该层对调用方与注册中心后端完全透明，不改变 RegistryClient 接口语义。
type CachedRegistry struct {
	inner RegistryClient

	mu       sync.Mutex
	entries  map[string]*cacheEntry
	inflight map[string]*inFlight

	ttl time.Duration
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
		ttl:      defaultCacheTTL,
	}
	return c
}

// cacheKey 由服务名 + 分组构造，与后端 GetServices 入参保持一一对应。
func cacheKey(service string, groups []string) string {
	return service + "\x00" + strings.Join(groups, ",")
}

// Register / DeRegister 直接透传到内层客户端。
func (c *CachedRegistry) Register(data []byte) {
	c.inner.Register(data)
}

// DeRegister 透传注销。
func (c *CachedRegistry) DeRegister() {
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

// GetServices v2 多实例入口：优先返回未过期的缓存；过期后删除并按需回源。
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
		return append([]string(nil), urls...), nil
	}
	delete(c.entries, key)
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
			urls:      append([]string(nil), urls...),
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
	return append([]string(nil), urls...), nil
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
	return append([]string(nil), e.urls...), nil
}
