# mgin 数据库与插件 Panic 风险复审

- 审核日期：2026-11-25
- 分支：`v2`
- 审核基准提交：`aaae6a3`（`fix: preserve mongo lazy initialization`）
- 审核范围：`pkg/db/`、`pkg/plugin/builtins.go`、`pkg/plugin/plugin.go`，以及关联的退出与健康检查流程
- 审核方式：静态审查、最小复现探针、单元测试、竞态检测、`go vet`

## 一、总体结论

上一轮主要 panic 风险已基本修复，包括：

- MongoDB 单库重连失败后的 `Copy().DB()` nil 解引用。
- MongoDB、Redis 并发访问 map 导致的数据竞态。
- SQLite 关闭时忽略 `DB()` 错误造成的 nil 解引用。
- `MySQLDao` 大部分方法的 nil 接收者问题，以及 Tag 懒初始化竞态。
- Redis 订阅修改调用方 channels、handler panic 未隔离等问题。
- 插件 typed-nil、配置与 registry 为空、`used` 子串误匹配等问题。
- 插件 `Init`、`Close`、`Health` 单点 panic 的恢复与继续执行。

`pkg/plugin/builtins.go` 本身未再发现可复现的直接 panic。当前残留风险主要集中在插件注册表对 `Name()`、`Order()` 的调用边界，以及少量 DAO、连接清理和订阅取消路径。

## 二、确认的 Panic 风险

### 1. 高：插件 `Name()` 与 `Order()` 未受保护

**位置：**

- [pkg/plugin/plugin.go:49](../pkg/plugin/plugin.go#L49)：`Register` 直接调用 `Name()`。
- [pkg/plugin/plugin.go:109](../pkg/plugin/plugin.go#L109)：排序比较器直接调用 `Order()`。
- [pkg/plugin/plugin.go:122](../pkg/plugin/plugin.go#L122)、[131](../pkg/plugin/plugin.go#L131)、[136](../pkg/plugin/plugin.go#L136)、[138](../pkg/plugin/plugin.go#L138)：初始化流程多处直接调用 `Name()`。
- [pkg/plugin/plugin.go:191](../pkg/plugin/plugin.go#L191)、[197](../pkg/plugin/plugin.go#L197)：健康检查直接调用 `Name()`。
- [pkg/plugin/plugin.go:214](../pkg/plugin/plugin.go#L214)、[223](../pkg/plugin/plugin.go#L223)、[232](../pkg/plugin/plugin.go#L232)：recover 分支再次调用 `Name()`。

**问题：**

现有 `safeInit`、`safeClose`、`safeHealth` 只包住了插件对应的生命周期方法，没有覆盖：

1. 注册阶段的 `Name()`。
2. 排序阶段的 `Order()`。
3. 日志、错误信息、健康检查 map key 阶段的 `Name()`。
4. panic 恢复信息组装阶段的 `Name()`。

因此，自定义插件的方法 panic 仍可能穿透整个注册、启动、健康检查或关闭流程。特别是插件 `Init()` 已 panic 后，recover 分支中的 `Name()` 再次 panic，会造成二次 panic，使恢复机制失效。

**最小复现输出：**

```text
plugin.Register bad Name: PANIC: name panic
plugin.InitAll bad Order: PANIC: order panic
plugin.InitAll late bad Name: PANIC: late name panic
plugin.InitAll Name panic inside recover: PANIC: name panic inside recover
```

注意：只有一个插件时，`sort.Slice` 可能不执行比较器，`Order()` 风险需要至少两个插件才能稳定触发。

**建议：**

- 注册时通过 `safeName` 获取名称并保存；失败则拒绝注册。
- 排序前逐个安全获取 `Order()`，记录错误并跳过异常插件，继续处理其他插件。
- 日志、错误信息和健康检查统一使用注册时保存的名称。
- recover 分支禁止再次调用插件方法。
- 可预先构造包含 `plugin/name/order/err` 的排序条目，避免在比较器中执行插件代码。

### 2. 中：`MySQLDao.WithTx` 仍会解引用 nil 接收者

**位置：** [pkg/db/dao/mysql.go:60](../pkg/db/dao/mysql.go#L60)

```go
func (m *MySQLDao[E]) WithTx(tx *gorm.DB) *MySQLDao[E] {
    clone := *m
    clone.tx = tx
    return &clone
}
```

`Debug`、`WithContext`、`WithCache` 均已增加 nil 检查，只有 `WithTx` 遗漏。nil 接收者调用时会直接 panic。

**最小复现输出：**

```text
nil MySQLDao.WithTx: PANIC: runtime error: invalid memory address or nil pointer dereference
```

**建议：**

```go
func (m *MySQLDao[E]) WithTx(tx *gorm.DB) *MySQLDao[E] {
    if m == nil {
        return nil
    }
    clone := *m
    clone.tx = tx
    return &clone
}
```

### 3. 中：Mongo `ReturnConnection` 对零值 `Database` 会 panic

**位置：** [pkg/db/mongo/mgo.go:262](../pkg/db/mongo/mgo.go#L262)

```go
func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
    if conn == nil {
        return
    }
    conn.Session().Close()
}
```

`conn != nil` 不代表 `conn.Session() != nil`。传入 `&mgo.Database{}` 时会触发 nil 解引用。

正常 `GetConnection` 返回的数据库对象不会触发该问题，但 `ReturnConnection` 是导出的公共清理接口，应保持防御性。

**最小复现输出：**

```text
Mongo ReturnConnection zero Database: PANIC: runtime error: invalid memory address or nil pointer dereference
```

**建议：**

```go
func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
    if conn == nil {
        return
    }
    session := conn.Session()
    if session != nil {
        session.Close()
    }
}
```

## 三、Redis 订阅取消路径缺陷

该问题不会直接 panic，但会造成连接、goroutine 和重连循环泄漏。

**位置：**

- [pkg/db/redis/redis.go:367](../pkg/db/redis/redis.go#L367)：初始 `pubsub.Receive()` 阻塞。
- [pkg/db/redis/redis.go:394](../pkg/db/redis/redis.go#L394)：handler 返回后检查 context 并直接退出。

**问题一：握手阶段取消不及时**

`PSubscribeContext` 在调用 `pubsub.Receive()` 等待订阅响应时没有监听 `ctx.Done()`。如果 Redis 接受连接但不返回订阅响应，context 取消后连接仍会阻塞到网络读超时。

**问题二：handler 内取消导致漏关 Pub/Sub**

handler 内取消 context 后，消息循环执行到尾部：

```go
if ctx.Err() != nil {
    return
}
```

该分支没有调用 `pubsub.Close()`，连接会一直保留到客户端关闭或外部回收。

**最小复现输出：**

```text
subscription still open 300ms after handler cancellation
```

**建议：**

- Pub/Sub 创建后，确保所有退出路径都关闭它。
- 初始 `Receive()` 等待期间同时监听 `ctx.Done()`，取消时主动关闭 Pub/Sub。
- 尾部 `ctx.Err() != nil` 分支先关闭 Pub/Sub 再返回。
- `GetConnection` 返回客户端前补充 `client != nil` 检查，避免后台 goroutine 解引用 nil client。

## 四、相关生命周期风险

以下问题不属于直接 panic，但会放大健康检查、退出和资源泄漏风险。

### 1. 持锁执行网络 I/O

- MongoDB 在 [pkg/db/mongo/mgo.go:95](../pkg/db/mongo/mgo.go#L95)、[116](../pkg/db/mongo/mgo.go#L116) 持写锁执行 Dial。
- Redis 在 [pkg/db/redis/redis.go:176](../pkg/db/redis/redis.go#L176) 持写锁执行 Ping。
- Redis `Check` 在 [pkg/db/redis/redis.go:231](../pkg/db/redis/redis.go#L231) 持读锁执行 Ping。

依赖挂死时，其他读写调用和 `Close()` 都会被长时间阻塞。

**建议：** 参照 MySQL 的实现，在锁内只复制配置和连接引用，网络 I/O 在锁外执行，发布连接前重新检查关闭状态。

### 2. Mongo、Redis 关闭后可能复活

Mongo 的 `Check`、`GetConnection` 在连接为空且保留 `confData` 时会重新调用 `Init`；Redis 也没有 MySQL 式的终止状态。关闭后若仍有健康检查或业务调用，可能重新建立连接。

**建议：** 增加 `closed bool` 或等价状态；`Close` 后所有初始化、重连、健康检查统一返回 `ErrClosed`。

### 3. Mongo 冗余初始化清空多库标签

[pkg/db/mongo/mgo.go:52](../pkg/db/mongo/mgo.go#L52) 在判断连接是否为空之前就执行：

```go
m.tags = make([]string, 0)
```

当 `conns` 已非空时，再次 `Init` 不会重连，但会清空 `tags`，导致 `ListConnNames` 为空，多库默认连接选择失败。

**建议：** 只在真正重建连接集合时重置并重新填充 `tags`。

### 4. Redis 空 map 健康检查假阳性

[pkg/db/redis/redis.go:227](../pkg/db/redis/redis.go#L227) 的 `Check()` 在 `clients` 为空时循环不执行，最终返回 `nil`。关闭或初始化失败后，健康检查可能错误地认为 Redis 正常。

**建议：** 启用状态下没有可用客户端时返回明确错误。

### 5. 健康检查与关闭流程重叠

[app.go:117](../app.go#L117) 启动健康检查 goroutine，但 [app.go:380](../app.go#L380) 退出时只停止 ticker 并关闭 channel，没有等待在途 `checkAll` 完成。随后执行 `SafeExit`，健康检查可能与插件关闭重叠。

**建议：** 使用 `sync.WaitGroup` 或专门的退出完成 channel，在关闭插件前等待健康检查退出，并设置合理超时。

## 五、验证结果

以下命令均通过：

```text
/usr/local/go/bin/go test ./pkg/db/... ./pkg/plugin -count=1
/usr/local/go/bin/go test -race ./pkg/db/... ./pkg/plugin -count=1
/usr/local/go/bin/go vet ./pkg/db/... ./pkg/plugin
```

复现探针均为临时文件，验证后已删除；本次复审未修改源码。

## 六、建议修复顺序

1. 插件 `Name()`、`Order()` 安全边界与 recover 二次 panic。
2. `MySQLDao.WithTx` nil 检查。
3. Mongo `ReturnConnection` Session nil 检查。
4. Redis 订阅取消与 Pub/Sub 清理。
5. Mongo、Redis 关闭状态及锁外网络 I/O。
6. Mongo tags 保留、Redis 空 map 健康检查、健康检查退出等待。
