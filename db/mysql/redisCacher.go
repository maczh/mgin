package mysql

import (
	"context"
	"fmt"
	"time"

	caches "github.com/go-gorm/caches/v4"
	"github.com/go-redis/redis/v7"
)

// 1. 定义你的 Redis Cacher 结构体
type RedisCacher struct {
	Rdb redis.UniversalClient
}

// 2. 实现 Get 方法
func (c *RedisCacher) Get(ctx context.Context, key string, q *caches.Query[any]) (*caches.Query[any], error) {
	res, err := c.Rdb.Get(key).Result()
	if err == redis.Nil {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if err := q.Unmarshal([]byte(res)); err != nil {
		return nil, err
	}

	return q, nil
}

func (c *RedisCacher) Store(ctx context.Context, key string, val *caches.Query[any]) error {
	res, err := val.Marshal()
	if err != nil {
		return err
	}

	c.Rdb.Set(key, res, 1*time.Hour) // Set proper cache time
	return nil
}

func (c *RedisCacher) Invalidate(ctx context.Context) error {
	var (
		cursor uint64
		keys   []string
	)
	for {
		var (
			k   []string
			err error
		)
		k, cursor, err = c.Rdb.Scan(cursor, fmt.Sprintf("%s*", caches.IdentifierPrefix), 0).Result()
		if err != nil {
			return err
		}
		keys = append(keys, k...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) > 0 {
		if _, err := c.Rdb.Del(keys...).Result(); err != nil {
			return err
		}
	}
	return nil
}
