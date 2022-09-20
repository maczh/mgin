package cache

import (
	"github.com/muesli/cache2go"
	"time"
)

/**
base of  "github.com/muesli/cache2go"
Author : Not Mine
*/

type MyCache struct {
	cache *cache2go.CacheTable
}

var mc *MyCache

/*
	初始化一个cache
	cachename 缓存名字
*/
func OnGetCache(cachename string) *MyCache {
	if mc == nil {
		mc = new(MyCache)
	}
	mc.cache = cache2go.Cache(cachename)
	return mc
}

/*
	添加一个缓存
	lifeSpan:缓存时间，0表示永不超时
*/
func (mc *MyCache) Add(key interface{}, value interface{}, lifeSpan time.Duration) *cache2go.CacheItem {
	return mc.cache.Add(key, lifeSpan, value)
}

/*
	查找一个cache
	value 返回的值
*/

func (mc *MyCache) Value(key interface{}) (value interface{}, b bool) {
	b = false
	res, err := mc.cache.Value(key)
	if err == nil {
		value = res.Data()
		b = true
		return
	}
	return
}

/*
	判断key是否存在
*/
func (mc *MyCache) IsExist(key interface{}) bool {
	return mc.cache.Exists(key)
}

/*
 删除一个cache
*/
func (mc *MyCache) Delete(key interface{}) error {
	_, err := mc.cache.Delete(key)
	return err
}

/*
	获取原始cache2go操作类
*/
func (mc *MyCache) GetCache2go() *cache2go.CacheTable {
	return mc.cache
}

/*
	清空表內容
*/
func (mc *MyCache) Clear() bool {
	mc.cache.Flush()
	return true
}
