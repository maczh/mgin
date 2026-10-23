# 服务开发：从路由到数据库

本章按一次真实开发顺序组织：**建模型 → 写 DAO → 写 service → 写 controller → 挂路由 → 统一响应**。
所有代码基于框架当前签名，可直接放入脚手架工程。

---

## 1. 建模型

```go
// model/order.go
package model

import "time"

type Order struct {
    Id        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    OrderNo   string    `gorm:"size:64;uniqueIndex;not null" json:"orderNo"`
    UserId    int64     `gorm:"index;not null" json:"userId"`
    Amount    int64     `gorm:"not null;comment:分" json:"amount"`
    Status    int8      `gorm:"index;default:0" json:"status"`
    CreateAt  time.Time `gorm:"autoCreateTime" json:"createAt"`
    UpdateAt  time.Time `gorm:"autoUpdateTime" json:"updateAt"`
}

func (Order) TableName() string { return "biz_order" }
```

要点：

- `TableName()` 必须有**值接收者**，DAO 的泛型约束是 `schema.Tabler`；
- 主键字段名建议 `Id`（GORM 默认识别 `ID`/`Id`）；
- 金额用整数分，避免浮点；
- 时间字段用 `time.Time` + `autoCreateTime/autoUpdateTime`。

---

## 2. 写 DAO

```go
// dao/order.go
package dao

import (
    "example.com/hello/model"
    "github.com/maczh/mgin/v2/pkg/db/dao"
)

var OrderDao = &dao.MySQLDao[model.Order]{
    Tag: func() string { return "order_db" }, // 单库时可省略
}
```

常用操作：

```go
// 创建
err := dao.OrderDao.Create(&order)

// 按主键查（One 接收一个带主键的实体）
got, err := dao.OrderDao.One(model.Order{Id: 1})

// 条件查询
list, err := dao.OrderDao.WhereAll("user_id = ? AND status = ?", 1001, 1)

// 分页（需要自己构造 *gorm.DB 查询）
conn, _ := db.Mysql.GetConnection("order_db")
page, err := dao.OrderDao.Pager(conn.Model(&model.Order{}).Where("user_id = ?", 1001), 1, 20)

// 更新（按主键，Updates 会忽略零值字段）
order.Status = 2
err = dao.OrderDao.Updates(&order)
```

### 2.1 开启查询缓存

```go
import (
    "time"
    "github.com/maczh/mgin/v2/pkg/db"
    "github.com/maczh/mgin/v2/pkg/db/dao"
)

conn, _ := db.Redis.GetConnection()
cached := dao.OrderDao.WithCache(dao.CacheConfig{
    Enabled: true,
    Store:   dao.NewRedisCache(conn),
    TTL:     time.Minute,
    Prefix:  "order:",
})

list, err := cached.WhereAll("status = ?", 1)  // 命中缓存
```

约束：

- 只对 `All/One/Exists/Count/Where*` 生效；
- 写方法成功后按**表名前缀**清理缓存；
- `WithTx` **不会禁用读缓存，也不会在事务内清理缓存**：事务内的写操作若未提交，
  缓存可能在 TTL 内继续返回旧值。强一致场景请勿对同一 DAO 同时开启缓存与事务，
  或在事务提交后手动失效。
- `RedisCache.Clear()` 是 `FLUSHDB`，生产禁止随手调用。

### 2.2 事务

```go
import (
    "context"
    "github.com/maczh/mgin/v2/pkg/db"
    "gorm.io/gorm"
)

func CreateOrder(ctx context.Context, o *model.Order, item *model.OrderItem) error {
    _, err := db.MySQLTx(ctx, func(tx *gorm.DB) (struct{}, error) {
        if err := tx.Create(o).Error; err != nil {
            return struct{}{}, err
        }
        item.OrderId = o.Id
        if err := tx.Create(item).Error; err != nil {
            return struct{}{}, err
        }
        return struct{}{}, nil
    }, "order_db")
    return err
}
```

`MySQLTx` 的第三个参数是多库时的库名；单库可省略。回调返回 error 会回滚。

> ⚠️ 若同时使用查询缓存：事务内的写不会触发缓存失效（`clearCache` 在 `tx != nil` 时直接返回），
> 读路径也不会跳过缓存。涉及强一致读写的表，建议事务路径使用不带 `WithCache` 的 DAO 实例。

---

## 3. 控制器与路由

```go
// controller/order.go
package controller

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin/v2/pkg/models"
    "example.com/hello/dao"
    "example.com/hello/model"
)

func GetOrder(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, models.Error(1003, "参数错误"))
        return
    }
    o, err := dao.OrderDao.One(model.Order{Id: id})
    if err != nil || o == nil {
        c.JSON(http.StatusNotFound, models.Error(1004, "订单不存在"))
        return
    }
    c.JSON(http.StatusOK, models.Success(o))
}

type CreateOrderReq struct {
    OrderNo string `json:"orderNo" binding:"required,max=64"`
    UserId  int64  `json:"userId" binding:"required,gt=0"`
    Amount  int64  `json:"amount" binding:"required,gt=0"`
}

func CreateOrder(c *gin.Context) {
    var req CreateOrderReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, models.Error(1003, err.Error()))
        return
    }
    o := &model.Order{OrderNo: req.OrderNo, UserId: req.UserId, Amount: req.Amount}
    if err := dao.OrderDao.Create(o); err != nil {
        c.JSON(http.StatusInternalServerError, models.Error(1001, "创建失败"))
        return
    }
    c.JSON(http.StatusOK, models.Success(o))
}
```

```go
// router/router.go
func RegisterRoutes(app *mgin.App) {
    v1 := app.Router.Group("/api/v1")
    v1.GET("/orders/:id", controller.GetOrder)
    v1.POST("/orders", controller.CreateOrder)
}
```

### 3.1 分页响应

```go
c.JSON(http.StatusOK, models.SuccessPage(list, &models.ResultPage{
    Count: pageCount, Index: page, Size: size, Total: total,
}))
```

JSON：

```json
{"status":1,"msg":"Success","data":[...],"page":{"count":5,"index":1,"size":20,"total":97}}
```

---

## 4. 服务层与依赖边界

推荐 controller 只做三件事：**绑定/校验 → 调 service → 写响应**。
service 不接收 `*gin.Context`，而是接收普通参数与 `context.Context`：

```go
// service/order.go
type OrderService struct{}

func (OrderService) Create(ctx context.Context, req CreateOrderReq) (*model.Order, error) {
    // 业务校验、事务、跨服务调用
    return nil, nil
}
```

这样可以：

- 用表驱动单测 service，不启动 HTTP；
- 在 job/CLI 中复用同一逻辑；
- 避免把 `c.Set("claims")` 之类的 Web 细节渗到业务层。

---

## 5. 统一响应与错误码

### 5.1 两套响应模型

| 模型 | 结构 | 适用 |
|------|------|------|
| `models.Result[T]` | `status`/`msg`/`data`/`page` | 推荐；与 `i18n`、`client` 一致 |
| `vo.Response[T]` | `code`/`data`/`msg`/`srvTime` | 需要服务时间戳的对外 API |

**同一服务内只选一套**，否则客户端要写两套解析。

### 5.2 注册业务错误码

```go
// errcode/order.go
package errcode

import (
    "github.com/maczh/mgin/v2/pkg/errcode"
)

var DefOrderNotFound = errcode.New("order", 4101, 404, "order.not_found")

func init() { errcode.RegisterDef(DefOrderNotFound) }
```

```go
// handler
c.JSON(DefOrderNotFound.HTTPStatus, models.Error(DefOrderNotFound.Code, "订单不存在"))
```

启用 i18n 时：

```go
c.JSON(DefOrderNotFound.HTTPStatus, i18n.ErrorDef(DefOrderNotFound, id))
```

`i18n.ErrorDef` 会按 `X-Lang` 查字典 `order.not_found`；缺失时返回 key 原文，不 panic。

### 5.3 错误码规范

- 1000–1999 框架内置，不要占用；
- 2000+ 按模块分段（如 4100–4199 订单）；
- `HTTPStatus` 必须真实：参数错 400、未认证 401、无权限 403、不存在 404、冲突 409、限流 429、依赖不可用 503；
- 响应体同时给 `code` 与可读 `msg`，便于前端分支与用户提示。

### 5.4 不要这样做

```go
c.JSON(200, gin.H{"code": 500, "msg": "失败"})   // HTTP 永远 200，网关无法重试
c.JSON(500, gin.H{"error": err.Error()})         // 泄露 SQL/堆栈；无业务码
```

---

## 6. 中间件挂载

```go
// main.go，在 RegisterRoutes 之前
app.Router.Use(
    jwt.JwtAuthorize(),      // 登录校验
    casbin.CasbinHandler(),  // 接口授权
    session.New(...),        // 会话
    metrics.Middleware(),    // HTTP 指标
)
```

顺序原则：

1. 全局通用（trace/postlog/cors/recovery）已由框架挂好，不要重复挂；
2. 鉴权类放在业务路由之前；
3. `metrics.Middleware()` 放在鉴权之后，避免未授权请求污染业务指标（按需）；
4. 需要按路由组细粒度控制时，用 `v1.Use(...)` 而不是全局 `Use`。

---

## 7. 生命周期与后台任务

```go
func main() {
    app := mgin.NewApp(...)

    // 业务初始化（DB 迁移、缓存预热、注册任务处理器）
    if err := bootstrap(); err != nil {
        logs.Error("bootstrap: {}", err.Error())
        os.Exit(1)
    }

    // 注册定时任务处理器（任务记录由 pkg/job 管理）
    job.Register("order.timeout", func(ctx *job.Context) error {
        return service.CloseTimeoutOrders(ctx.Ctx())
    })

    health.MarkStarted()
    app.Run()
}
```

- 业务初始化失败应**主动退出**，不要带着半初始化状态启动；
- 长时间后台任务用 `app` 之外的 `context.WithCancel`，并在退出前取消；
- 注册 `job` 处理器必须在 `job.Start()`（插件初始化）之后或之前均可，
  但必须在任务首次触发之前。

---

## 8. 检查清单

- [ ] 模型实现 `TableName()`，主键/索引命名一致
- [ ] DAO 不跨层调用 service
- [ ] 事务回调内只用 `tx`，不混用全局连接
- [ ] 所有响应走统一模型，HTTP 状态码与业务码匹配
- [ ] 错误响应不包含 SQL、堆栈、内部地址
- [ ] 探针/指标在业务中间件之前启用
- [ ] 退出时无未取消的 goroutine
