package cache

import (
	"sync"
	"time"
)

type MyCache struct {
	cache map[string]*Cache
}
type Cache struct {
	items sync.Map
	close chan struct{}
}

// An item represents arbitrary data with expiration time.
type item struct {
	data    interface{}
	expires int64
}

var mc *MyCache

/*
	初始化一个cache
	cachename 缓存名字
*/
func OnGetCache(cachename string) *Cache {
	if mc == nil {
		mc = new(MyCache)
		mc.cache = make(map[string]*Cache)
	}
	if mc.cache[cachename] == nil {
		mc.cache[cachename] = New(time.Hour)
	}
	return mc.cache[cachename]
}

/*
	添加一个缓存
	lifeSpan:缓存时间，0表示永不超时
*/
func (c *Cache) Add(key interface{}, value interface{}, lifeSpan time.Duration) {
	c.Set(key, value, lifeSpan)
}

/*
	查找一个cache
	value 返回的值
*/

func (c *Cache) Value(key interface{}) (interface{}, bool) {
	return c.Get(key)
}

/*
	判断key是否存在
*/
func (c *Cache) IsExist(key interface{}) bool {
	_, exists := c.Get(key)
	return exists
}

/*
 删除一个cache
*/

/*
	清空表內容
*/
func (c *Cache) Clear() bool {
	c.Close()
	return true
}

// New creates a new cache that asynchronously cleans
// expired entries after the given time passes.
func New(cleaningInterval time.Duration) *Cache {
	cache := &Cache{
		close: make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(cleaningInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				now := time.Now().UnixNano()

				cache.items.Range(func(key, value interface{}) bool {
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
func (cache *Cache) Get(key interface{}) (interface{}, bool) {
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
func (cache *Cache) Set(key interface{}, value interface{}, duration time.Duration) {
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
func (cache *Cache) Range(f func(key, value interface{}) bool) {
	now := time.Now().UnixNano()

	fn := func(key, value interface{}) bool {
		item := value.(item)

		if item.expires > 0 && now > item.expires {
			return true
		}

		return f(key, item.data)
	}

	cache.items.Range(fn)
}

// Delete deletes the key and its value from the cache.
func (cache *Cache) Delete(key interface{}) {
	cache.items.Delete(key)
}

// Close closes the cache and frees up resources.
func (cache *Cache) Close() {
	cache.close <- struct{}{}
	cache.items = sync.Map{}
}
