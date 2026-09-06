package utils

import (
	"sync"
	"time"
)

type ExpireCache struct {
	mp sync.Map
	// mu 保护 tm 的创建/销毁，并串行化对 cacheItem 的读写，
	// 避免并发 Store/Load/checkExpire 产生数据竞争或定时器丢失导致缓存永不清理
	mu      sync.Mutex
	tm      *time.Timer
	Timeout int64
}

type cacheItem struct {
	value    any
	expireAt int64
}

func (c *ExpireCache) Delete(key string) {
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
	if c.tm == nil {
		c.tm = time.AfterFunc(time.Second, func() {
			c.checkExpire()
		})
	}
	c.mu.Unlock()

	c.mp.Store(key, &cacheItem{value: value, expireAt: time.Now().Unix() + c.Timeout})
}

func (c *ExpireCache) checkExpire() {
	hasRemain := false

	now := time.Now().Unix()
	c.mp.Range(func(key, value any) bool {
		item, ok := value.(*cacheItem)
		if !ok {
			return true
		}
		if now >= item.expireAt {
			c.mp.Delete(key)
		} else {
			hasRemain = true
		}
		return true
	})

	c.mu.Lock()
	defer c.mu.Unlock()
	if hasRemain {
		c.tm = time.AfterFunc(time.Second, func() {
			c.checkExpire()
		})
	} else {
		c.tm = nil
	}
}
