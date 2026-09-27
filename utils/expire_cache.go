package utils

import (
	"sync"
	"time"
)

type ExpireCache struct {
	mp sync.Map
	// mu 保护 tm 的创建/销毁，并串行化对 cacheItem 的读写：
	// Store/Load/checkExpire 对 cacheItem.expireAt 的每一次访问都必须发生在 mu 临界区内，
	// 否则定时器 goroutine 中的 checkExpire 会与调用方 goroutine 的 Load 产生数据竞争
	// （清理定时器独立运行，即使调用方是单线程也会触发）。
	// 同时避免定时器丢失导致缓存永不清理。
	mu      sync.Mutex
	tm      *time.Timer
	Timeout int64
}

type cacheItem struct {
	value any
	// expireAt 的读写必须在 ExpireCache.mu 临界区内进行
	expireAt int64
}

func (c *ExpireCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mp.Delete(key)
}

func (c *ExpireCache) Load(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i, ok := c.mp.Load(key); ok {
		if item, ok := i.(*cacheItem); ok {
			item.expireAt = time.Now().Unix() + c.Timeout
			return item.value, true
		}
	}
	return nil, false
}

func (c *ExpireCache) Store(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tm == nil {
		c.tm = time.AfterFunc(time.Second, func() {
			c.checkExpire()
		})
	}
	c.mp.Store(key, &cacheItem{value: value, expireAt: time.Now().Unix() + c.Timeout})
}

func (c *ExpireCache) checkExpire() {
	// 全程持锁：Range 回调会读写 cacheItem.expireAt，必须与 Store/Load 互斥。
	// 死锁红线：回调内只能使用 sync.Map 原生的 c.mp.Delete(key)，
	// 严禁调用 c.Delete() / c.Load() 等会再次加 c.mu 的方法，否则在持锁状态下自锁。
	c.mu.Lock()
	defer c.mu.Unlock()

	hasRemain := false

	now := time.Now().Unix()
	c.mp.Range(func(key, value any) bool {
		item, ok := value.(*cacheItem)
		if !ok {
			// 非法的缓存项无法被 Load 使用，直接清理，避免残留导致定时器提前停止、缓存永不回收
			c.mp.Delete(key)
			return true
		}
		if now >= item.expireAt {
			// 注意：这里必须删除 map 的 key，而不是 value（cacheItem），否则过期项永远清不掉
			c.mp.Delete(key)
		} else {
			hasRemain = true
		}
		return true
	})

	if hasRemain {
		c.tm = time.AfterFunc(time.Second, func() {
			c.checkExpire()
		})
	} else {
		c.tm = nil
	}
}
