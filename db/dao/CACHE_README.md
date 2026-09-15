# MySQLDao 缓存使用说明

`MySQLDao` 支持为查询结果配置缓存。缓存通过 `cache.ICache` 接口接入，因此可以使用框架内置的内存缓存，也可以使用 `RedisCache` 连接 Redis。

## 1. 开启缓存

通过 `WithCache` 为 DAO 配置缓存：

```go
import (
    "time"

    cacheStore "github.com/maczh/mgin/cache"
    "github.com/maczh/mgin/db/dao"
)

type User struct {
    ID   int
    Name string
}

func (User) TableName() string {
    return "users"
}

userDAO := (&dao.MySQLDao[User]{}).WithCache(dao.CacheConfig{
    Enabled: true,
    Store:   cacheStore.OnMemCache("users-cache"),
    TTL:     5 * time.Minute,
    Prefix:  "myapp:mysql:",
})
```

`CacheConfig` 字段说明：

| 字段 | 说明 |
| --- | --- |
| `Enabled` | 是否启用缓存。必须为 `true`。 |
| `Store` | 缓存实现，必须实现 `cache.ICache`。为空时不使用缓存。 |
| `TTL` | 缓存有效期。`0` 表示不过期，具体行为由缓存实现决定。 |
| `Prefix` | 缓存 key 前缀。为空时使用 `mgin:mysql:`。 |

也可以先创建 DAO，再调用 `WithCache`：

```go
dao := &dao.MySQLDao[User]{}
dao.WithCache(dao.CacheConfig{
    Enabled: true,
    Store:   cacheStore.OnMemCache("users-cache"),
    TTL:     time.Minute,
})
```

## 2. 使用 Redis 缓存

使用项目已有的 Redis 客户端获取连接，再通过 `NewRedisCache` 转换为 DAO 所需的缓存接口：

```go
import (
    "time"

    "github.com/maczh/mgin/db"
    "github.com/maczh/mgin/db/dao"
)

redisClient, err := db.Redis.GetConnection()
if err != nil {
    // 处理 Redis 连接错误
}

userDAO := (&dao.MySQLDao[User]{}).WithCache(dao.CacheConfig{
    Enabled: true,
    Store:   dao.NewRedisCache(redisClient),
    TTL:     5 * time.Minute,
    Prefix:  "myapp:mysql:",
})
```

Redis 需要先按照项目的正常配置流程启用，并确保 `db.Redis` 已初始化。`RedisCache` 使用传入的 Redis 客户端读写数据，不会在 `Close` 时关闭由应用管理的共享客户端。

## 3. 自动缓存的方法

以下方法支持自动读取和写入缓存：

```go
users, err := userDAO.All(User{Name: "alice"})
user, err := userDAO.One(User{ID: 1})
exists := userDAO.Exists(User{ID: 1})
count, err := userDAO.Count(User{Name: "alice"})
```

以下方法当前不自动缓存：

- `Where`：返回裸 `*gorm.DB`，调用方可能继续追加任意查询条件。
- `Pager`：接收外部 `*gorm.DB`，当前只执行数据库查询。
- `DB`、`Alias`、`JOIN`：返回裸 `*gorm.DB`。

如需缓存这些自定义查询，可以在业务层使用 `cache.ICache` 自行生成和管理缓存键。

## 4. 缓存键规则

缓存键的基本结构为：

```text
{Prefix}{Tag}:{TableName}:{Operation}:{MD5}
```

例如：

```text
myapp:mysql::users:one:9f86d081884c7d659a2feaa0c55ad015
```

MD5 的输入是查询条件转换后的字符串：

```text
key1=value1&key2=value2
```

生成规则：

1. 查询条件字段按 key 的字典序排序。
2. `nil`、空字符串、空数组和空对象会被排除。
3. 不同查询条件会生成不同 MD5。
4. map 字段的插入顺序不会影响最终缓存键。
5. `All` 的 `Preloads` 和 `OrderBy` 也会参与缓存键计算。
6. `one`、`all`、`exists`、`count` 使用不同的操作标识，互不复用缓存结果。

## 5. 写操作与缓存失效

以下写操作成功后，会清理当前 DAO 实体表关联的缓存：

- `Create`
- `MultiCreate`
- `Delete`
- `Updates`
- `Save`

清理范围由缓存前缀、数据库 `Tag` 和实体的 `TableName()` 确定，不会清理其他实体表的缓存。例如，更新 `User` 只会清理 `users` 表对应的缓存。

如果多个 DAO 使用不同的 `Prefix`，它们的缓存空间相互独立；如果多个实例需要共享缓存，应使用同一个 Redis 实例和相同的 `Prefix`。

## 6. 注意事项

- 缓存只在数据库写操作成功后清理，数据库操作失败不会清理缓存。
- 建议为查询结果设置合理的 `TTL`，避免长时间缓存旧数据。
- `Exists` 会缓存 `bool` 结果，包括不存在记录的结果，因此写操作失效非常重要。
- 查询条件中包含不能被 JSON 序列化的值时，该次查询会跳过缓存并直接访问数据库。
- `Prefix` 应避免包含空格、换行或不稳定的运行时信息。
- `RedisCache.Clear()` 会执行 Redis `FLUSHDB`，会清理当前 Redis DB 中的全部数据，生产环境应谨慎调用。
