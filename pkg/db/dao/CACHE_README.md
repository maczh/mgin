# MySQLDao 缓存使用说明

`MySQLDao` 支持通过 `CacheConfig` 配置查询结果缓存。缓存实现需要满足 `pkg/cache.ICache`，可以使用内存缓存，也可以使用 `RedisCache`。

## 配置内存缓存

```go
import (
    "time"

    cacheStore "github.com/maczh/mgin/v2/pkg/cache"
    "github.com/maczh/mgin/v2/pkg/db/dao"
)

type User struct {
    ID   int
    Name string
}

func (User) TableName() string { return "users" }

userDAO := (&dao.MySQLDao[User]{}).WithCache(dao.CacheConfig{
    Enabled: true,
    Store:   cacheStore.OnMemCache("users-cache"),
    TTL:     5 * time.Minute,
    Prefix:  "myapp:mysql:",
})
```

`TTL` 为 `0` 表示不过期，`Prefix` 为空时使用 `mgin:mysql:`。

## 配置 Redis 缓存

```go
import (
    "time"

    "github.com/maczh/mgin/v2/pkg/db"
    "github.com/maczh/mgin/v2/pkg/db/dao"
)

redisClient, err := db.Redis.GetConnection()
if err != nil {
    // 处理 Redis 连接错误
}

userDAO := (&dao.MySQLDao[User]{}).WithCache(dao.CacheConfig{
    Enabled: true,
    Store:   dao.NewRedisCache(redisClient),
    TTL:     5 * time.Minute,
})
```

Redis 必须先按项目配置初始化。`RedisCache` 使用传入的共享 Redis 客户端，不会在 `Close` 时关闭该客户端。

## 自动缓存与失效

自动缓存的方法：`All`、`One`、`Exists`、`Count`。`Where`、`Pager`、`DB`、`Alias` 和 `JOIN` 返回裸 GORM 查询，不会自动缓存。

`Create`、`MultiCreate`、`Delete`、`Updates` 和 `Save` 成功后，会清理当前 `Tag` 和实体 `TableName()` 对应的缓存。

缓存 key 由前缀、数据库标签、表名、操作类型和查询条件 MD5 组成。查询条件会排除空值，按字段名排序，并组成 `key1=value1&key2=value2` 后计算 MD5，因此同一组条件不会因 map 顺序不同而产生不同 key。

`RedisCache.Clear()` 会执行 `FLUSHDB`，会清理当前 Redis DB 的全部数据，生产环境请谨慎使用。