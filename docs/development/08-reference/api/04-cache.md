# 缓存

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 进程内内存缓存与磁盘缓存。

## `github.com/maczh/mgin/v2/pkg/cache`

```go
import "github.com/maczh/mgin/v2/pkg/cache"
```

### 类型

#### `Cache`

```go
type Cache struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

#### `DiskCache`

```go
type DiskCache struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (d *DiskCache) Add(key any, value any, lifeSpan time.Duration)
```

```go
func (d *DiskCache) Value(key any) (any, bool)
```

```go
func (d *DiskCache) IsExist(key any) bool
```

```go
func (d *DiskCache) Clear() bool
```

```go
func (d *DiskCache) Get(key any) (any, bool)
```

```go
func (d *DiskCache) Set(key any, value any, duration time.Duration)
```

```go
func (d *DiskCache) Range(f func(key, value any) bool)
```

```go
func (d *DiskCache) Delete(key any)
```

```go
func (d *DiskCache) Close()
```

#### `ICache`

```go
type ICache interface
```

| 方法/内嵌 | 签名 |
|-----------|------|
| `Add` | func(key any, value any, lifeSpan time.Duration) |
| `Value` | func(key any) (any, bool) |
| `IsExist` | func(key any) bool |
| `Clear` | func() bool |
| `Get` | func(key any) (any, bool) |
| `Set` | func(key any, value any, duration time.Duration) |
| `Range` | func(f func(key, value any) bool) |
| `Delete` | func(key any) |
| `Close` | func() |

#### `MemCache`

```go
type MemCache struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *MemCache) Add(key any, value any, lifeSpan time.Duration)
```

添加一个缓存
lifeSpan:缓存时间，0表示永不超时

```go
func (c *MemCache) Value(key any) (any, bool)
```

```go
func (c *MemCache) IsExist(key any) bool
```

判断key是否存在

```go
func (c *MemCache) Clear() bool
```

清空表內容

```go
func (cache *MemCache) Get(key any) (any, bool)
```

Get gets the value for the given key.

```go
func (cache *MemCache) Set(key any, value any, duration time.Duration)
```

Set sets a value for the given key with an expiration duration.
If the duration is 0 or less, it will be stored forever.

```go
func (cache *MemCache) Range(f func(key, value any) bool)
```

Range calls f sequentially for each key and value present in the cache.
If f returns false, range stops the iteration.

```go
func (cache *MemCache) Delete(key any)
```

Delete deletes the key and its value from the cache.

```go
func (cache *MemCache) Close()
```

Close closes the cache and frees up resources.

### 函数

#### `CloseCache`

```go
func CloseCache()
```

#### `New`

```go
func New(cleaningInterval time.Duration) *MemCache
```

New creates a new cache that asynchronously cleans
expired entries after the given time passes.

#### `OnDiskCache`

```go
func OnDiskCache(cachePath string) ICache
```

#### `OnGetCache`

```go
func OnGetCache(cachename string, persistent ...bool) ICache
```

初始化一个cache
cachename 缓存名字

#### `OnMemCache`

```go
func OnMemCache(cachename string) ICache
```

