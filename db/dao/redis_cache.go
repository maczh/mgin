package dao

import (
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v7"
	cacheStore "github.com/maczh/mgin/cache"
)

// RedisCache adapts a go-redis client to the cache store used by MySQLDao.
type RedisCache struct {
	Client redis.UniversalClient
}

func NewRedisCache(client redis.UniversalClient) *RedisCache {
	return &RedisCache{Client: client}
}

func (c *RedisCache) Add(key interface{}, value interface{}, lifeSpan time.Duration) {
	c.Set(key, value, lifeSpan)
}

func (c *RedisCache) Value(key interface{}) (interface{}, bool) { return c.Get(key) }

func (c *RedisCache) IsExist(key interface{}) bool {
	_, ok := c.Get(key)
	return ok
}

func (c *RedisCache) Clear() bool {
	if c.Client == nil {
		return false
	}
	return c.Client.FlushDB().Err() == nil
}

func (c *RedisCache) Get(key interface{}) (interface{}, bool) {
	if c.Client == nil {
		return nil, false
	}
	value, err := c.Client.Get(redisKey(key)).Result()
	if err != nil {
		return nil, false
	}
	return []byte(value), true
}

func (c *RedisCache) Set(key interface{}, value interface{}, duration time.Duration) {
	if c.Client == nil {
		return
	}
	data, err := cacheBytes(value)
	if err == nil {
		_ = c.Client.Set(redisKey(key), data, duration).Err()
	}
}

func (c *RedisCache) Range(f func(key, value interface{}) bool) {
	if c.Client == nil {
		return
	}
	var cursor uint64
	for {
		keys, next, err := c.Client.Scan(cursor, "*", 0).Result()
		if err != nil {
			return
		}
		for _, key := range keys {
			value, _ := c.Get(key)
			if !f(key, value) {
				return
			}
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}

func (c *RedisCache) Delete(key interface{}) {
	if c.Client != nil {
		_ = c.Client.Del(redisKey(key)).Err()
	}
}

func (c *RedisCache) Close() {
}

func redisKey(key interface{}) string {
	if value, ok := key.(string); ok {
		return value
	}
	return toString(key)
}

func toString(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func cacheBytes(value interface{}) ([]byte, error) {
	if data, ok := value.([]byte); ok {
		return data, nil
	}
	return json.Marshal(value)
}

var _ cacheStore.ICache = (*RedisCache)(nil)
