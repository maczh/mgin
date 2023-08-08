package cache

import (
	"crypto/md5"
	"fmt"
	"github.com/akrylysov/pogreb"
	"github.com/huandu/go-clone"
	"github.com/sadlil/gologger"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MyCache struct {
	cache     map[string]*Cache
	cacheFile map[string]string
	db        map[string]*pogreb.DB
}

type Cache struct {
	items sync.Map
	close chan struct{}
}

// An item represents arbitrary data with expiration time.
type item struct {
	data    any
	expires int64
}

var mc *MyCache
var logger = gologger.GetLogger()

func OnDiskCache(cachePath string) *pogreb.DB {
	key := md5Encode(cachePath)
	if mc == nil {
		mc = new(MyCache)
		mc.cache = make(map[string]*Cache)
		mc.cacheFile = make(map[string]string)
		mc.db = make(map[string]*pogreb.DB)
	}
	if db, ok := mc.db[key]; ok {
		return db
	}
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if cachePath == "" {
		cachePath = fmt.Sprintf("%s/cache.db", path)
	} else {
		if !(strings.HasPrefix(cachePath, "/") || cachePath[1:2] == ":") {
			cachePath = fmt.Sprintf("%s/%s", path, cachePath)
		}
	}
	mc.cacheFile[key] = cachePath
	db, err := pogreb.Open(cachePath, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("file %s open failed: %s", cachePath, err.Error()))
		return nil
	}
	mc.db[key] = db
	return db
}

/*
初始化一个cache
cachename 缓存名字
*/
func OnGetCache(cachename string) *Cache {
	if mc == nil {
		mc = new(MyCache)
		mc.cache = make(map[string]*Cache)
		mc.cacheFile = make(map[string]string)
		mc.db = make(map[string]*pogreb.DB)
	}
	if mc.cache[cachename] == nil {
		mc.cache[cachename] = New(time.Minute)
	}
	return mc.cache[cachename]
}

/*
添加一个缓存
lifeSpan:缓存时间，0表示永不超时
*/
func (c *Cache) Add(key any, value any, lifeSpan time.Duration) {
	v := clone.Clone(value)
	c.Set(key, v, lifeSpan)
}

/*
	查找一个cache
	value 返回的值
*/

func (c *Cache) Value(key any) (any, bool) {
	v, found := c.Get(key)
	return clone.Clone(v), found
}

/*
判断key是否存在
*/
func (c *Cache) IsExist(key any) bool {
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
func (cache *Cache) Get(key any) (any, bool) {
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
func (cache *Cache) Set(key any, value any, duration time.Duration) {
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
func (cache *Cache) Range(f func(key, value any) bool) {
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
func (cache *Cache) Delete(key any) {
	cache.items.Delete(key)
}

// Close closes the cache and frees up resources.
func (cache *Cache) Close() {
	cache.close <- struct{}{}
	cache.items = sync.Map{}
}

func CloseCache() {
	if len(mc.db) > 0 {
		for k, db := range mc.db {
			db.Sync()
			db.Close()
			delete(mc.db, k)
		}
	}
}

func md5Encode(content string) (md string) {
	h := md5.New()
	_, _ = io.WriteString(h, content)
	md = fmt.Sprintf("%x", h.Sum(nil))
	return
}
