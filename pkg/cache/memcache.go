package cache

import (
	"github.com/huandu/go-clone"
	"sync"
	"time"
)

type MemCache struct {
	items sync.Map
	close chan struct{}
}

/*
添加一个缓存
lifeSpan:缓存时间，0表示永不超时
*/
func (c *MemCache) Add(key any, value any, lifeSpan time.Duration) {
	v := clone.Clone(value)
	c.Set(key, v, lifeSpan)
}

/*
	查找一个cache
	value 返回的值
*/

func (c *MemCache) Value(key any) (any, bool) {
	v, found := c.Get(key)
	return clone.Clone(v), found
}

/*
判断key是否存在
*/
func (c *MemCache) IsExist(key any) bool {
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
					item := value.(item)

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
	obj, exists := cache.items.Load(key)

	if !exists {
		return nil, false
	}

	item := obj.(item)

	if item.expires > 0 && time.Now().UnixNano() > item.expires {
		return nil, false
	}

	return item.data, true
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (cache *MemCache) Set(key any, value any, duration time.Duration) {
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
	now := time.Now().UnixNano()

	fn := func(key, value any) bool {
		item := value.(item)

		if item.expires > 0 && now > item.expires {
			return true
		}

		return f(key, item.data)
	}

	cache.items.Range(fn)
}

// Delete deletes the key and its value from the cache.
func (cache *MemCache) Delete(key any) {
	cache.items.Delete(key)
}

// Close closes the cache and frees up resources.
func (cache *MemCache) Close() {
	cache.close <- struct{}{}
	cache.items = sync.Map{}
}
