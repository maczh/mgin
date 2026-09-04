package cache

import (
	"sync"
	"time"

	"github.com/huandu/go-clone"
)

type MemCache struct {
	items     sync.Map
	close     chan struct{}
	closeOnce sync.Once
}

/*
添加一个缓存
lifeSpan:缓存时间，0表示永不超时
*/
func (c *MemCache) Add(key any, value any, lifeSpan time.Duration) {
	if c == nil {
		return
	}
	v := clone.Clone(value)
	c.Set(key, v, lifeSpan)
}

/*
	查找一个cache
	value 返回的值
*/

func (c *MemCache) Value(key any) (any, bool) {
	if c == nil {
		return nil, false
	}
	v, found := c.Get(key)
	return clone.Clone(v), found
}

/*
判断key是否存在
*/
func (c *MemCache) IsExist(key any) bool {
	if c == nil {
		return false
	}
	_, exists := c.Get(key)
	return exists
}

/*
 删除一个cache
*/

/*
清空表內容
*/
func (c *MemCache) Clear() bool {
	c.Close()
	return true
}

// New creates a new cache that asynchronously cleans
// expired entries after the given time passes.
func New(cleaningInterval time.Duration) *MemCache {
	if cleaningInterval <= 0 {
		cleaningInterval = time.Minute
	}
	cache := &MemCache{
		close: make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(cleaningInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				now := time.Now().UnixNano()

				cache.items.Range(func(key, value any) bool {
					item, ok := value.(item)
					if !ok {
						return true
					}

					if item.expires > 0 && now > item.expires {
						cache.items.Delete(key)
					}

					return true
				})

			case <-cache.close:
				return
			}
		}
	}()

	return cache
}

// Get gets the value for the given key.
func (cache *MemCache) Get(key any) (any, bool) {
	if cache == nil {
		return nil, false
	}
	obj, exists := cache.items.Load(key)

	if !exists {
		return nil, false
	}

	item, ok := obj.(item)
	if !ok {
		cache.items.Delete(key)
		return nil, false
	}

	if item.expires > 0 && time.Now().UnixNano() > item.expires {
		return nil, false
	}

	return item.data, true
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (cache *MemCache) Set(key any, value any, duration time.Duration) {
	if cache == nil {
		return
	}
	var expires int64

	if duration > 0 {
		expires = time.Now().Add(duration).UnixNano()
	}

	cache.items.Store(key, item{
		data:    value,
		expires: expires,
	})
}

// Range calls f sequentially for each key and value present in the cache.
// If f returns false, range stops the iteration.
func (cache *MemCache) Range(f func(key, value any) bool) {
	if cache == nil || f == nil {
		return
	}
	now := time.Now().UnixNano()

	fn := func(key, value any) bool {
		item, ok := value.(item)
		if !ok {
			return true
		}

		if item.expires > 0 && now > item.expires {
			return true
		}

		return f(key, item.data)
	}

	cache.items.Range(fn)
}

// Delete deletes the key and its value from the cache.
func (cache *MemCache) Delete(key any) {
	if cache == nil {
		return
	}
	cache.items.Delete(key)
}

// Close closes the cache and frees up resources.
func (cache *MemCache) Close() {
	if cache == nil {
		return
	}
	cache.closeOnce.Do(func() {
		if cache.close != nil {
			close(cache.close)
		}
	})
	cache.items = sync.Map{}
}
