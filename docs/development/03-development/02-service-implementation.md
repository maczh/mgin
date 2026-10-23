# 服务实现

本文按「写一个接口」的顺序：路由 → controller → service → 数据访问 → 错误码 → 启动验证。

## 1. 路由注册

```go
// router/router.go
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin"
	"example.com/demo/controller"
)

func RegisterRoutes(app *mgin.App) {
	v1 := app.Router.Group("/api/v1")
	{
		v1.GET("/users/:id", controller.GetUser)
		v1.POST("/users", controller.CreateUser)
	}
}
```

- 路径参数用 Gin 语法（`:id`、`*path`）。
- 需要鉴权时在组上挂中间件：`v1.Use(jwt.JwtAuthorize())`；Casbin 必须在 JWT 之后。
- 探针与 `/metrics` 已由框架占用，不要再注册同名路径。

## 2. Controller：绑定、校验、响应

```go
package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/i18n"
	"example.com/demo/service"
)

var userService = &service.UserService{}

func GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(200, i18n.Error(errcode.PARAM_ERROR, errcode.ParamError))
		return
	}
	user, err := userService.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(200, i18n.ErrorWithMsg(errcode.DB_CONNECT_ERROR, errcode.DbQueryErr, err.Error()))
		return
	}
	if user == nil {
		c.JSON(200, i18n.Error(errcode.DATA_NOT_FOUND, errcode.DataNotFound))
		return
	}
	c.JSON(200, i18n.Success(user))
}
```

约定：

- HTTP 状态默认 200，业务结果放在 `status`（见 [设计决策 D4](../02-concepts/03-design-decisions.md#d4-业务错误统一-http-200--status)）。
- 不要把 SQL、DSN、堆栈放进响应；细节写 `logs.Error`。
- 参数错误用 `i18n.ParamLostError("字段")` 或 `errcode.PARAM_ERROR`。

### 请求体绑定与校验

```go
type CreateUserReq struct {
	Name  string `json:"name" binding:"required,min=2,max=32"`
	Email string `json:"email" binding:"omitempty,email"`
}

func CreateUser(c *gin.Context) {
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, i18n.ErrorWithMsg(errcode.PARAM_ERROR, errcode.ParamError, err.Error()))
		return
	}
	// ...
}
```

## 3. Service：业务与事务

```go
package service

import (
	"context"

	"gorm.io/gorm"
	"github.com/maczh/mgin/db"
	"example.com/demo/model"
)

type UserService struct{}

func (s *UserService) Get(ctx context.Context, id int64) (*model.User, error) {
	conn, err := db.Mysql.GetConnection()
	if err != nil {
		return nil, err
	}
	var user model.User
	err = conn.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &user, err
}

// 跨表写入用事务：回调内只能使用 tx
func (s *UserService) CreateWithAccount(ctx context.Context, u *model.User) error {
	_, err := db.MySQLTx(ctx, func(tx *gorm.DB) (struct{}, error) {
		if err := tx.Create(u).Error; err != nil {
			return struct{}{}, err
		}
		return struct{}{}, tx.Create(&model.Account{UserId: u.Id}).Error
	})
	return err
}
```

- service 方法接收 `context.Context`，透传给 DB/客户端，便于超时与取消。
- 事务回调里不要调用 `GetConnection()` 另开连接，否则不在同一事务。
- 多库模式必须 `GetConnection("dbName")`。

## 4. 数据访问的两种方式

| 方式 | 适用 | 示例 |
|------|------|------|
| 门面 + GORM | 复杂查询、事务、跨表 | `conn.WithContext(ctx).Where(...).Find(&users)` |
| 泛型 DAO | 单表 CRUD、需要缓存 | `userDao.WhereAll("status = ?", 1)` |

DAO 示例：

```go
var userDao = &dao.MySQLDao[model.User]{}

func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	return userDao.WithContext(&ctx).All(model.User{})
}
```

> `WithContext` 会修改 DAO 实例本身，并发场景先复制：`d := *userDao; d.WithContext(&ctx)`。

## 5. 分页

```go
conn, _ := db.Mysql.GetConnection()
var users []model.User
page, size := 1, 20

list, pageInfo, err := (&dao.MySQLDao[model.User]{}).Pager(
	conn.Model(&model.User{}).Where("status = ?", 1), page, size,
)
if err != nil { /* ... */ }

c.JSON(200, i18n.SuccessWithPage(list, pageInfo.Count, pageInfo.Index, pageInfo.Size, pageInfo.Total))
```

`Pager` 的 `size==0` 时取 20；`page<1` 报错。分页不参与缓存。

## 6. 错误码与国际化

```go
// 未启用 i18n 时，String 原样返回 messageId
c.JSON(200, i18n.Error(errcode.DATA_NOT_FOUND, errcode.DataNotFound))

// 带动态内容
c.JSON(200, i18n.ErrorWithMsg(errcode.PARAM_ERROR, errcode.ParamError, "id 必须是数字"))
```

启用方式：`NewApp(..., true)` 或 `go.i18n.enabled: true`。字典表 `{app}_i18n` 由 i18n 初始化时创建；CRUD 路由需 `go.i18n.api.enabled: true`。

## 7. 中间件挂载

```go
app.Router.Use(jwt.JwtAuthorize())        // 解析 Authorization，写入 claims
app.Router.Use(casbin.CasbinHandler())    // 依赖 claims，必须在其后
app.Router.Use(xss.RemoveXss())           // 过滤 query/body（不过滤 header）
```

只对部分路由生效时挂到路由组；探针/指标不会被这些中间件影响。

## 8. 启动与自检

```go
app := mgin.NewApp("conf/application.yml", "demo", Version, false)
app.EnableHealth()
app.EnableMetrics()
router.RegisterRoutes(app)
job.Register("syncUserJob", syncUser) // 若启用 job
app.MarkHealthStarted()
app.Run()
```

验证：

```bash
curl -s localhost:8080/health/ready
curl -s localhost:8080/metrics | grep mgin_http_requests_total
```

## 9. 常见错误

| 现象 | 原因 | 修正 |
|------|------|------|
| `status` 一直是 0 | 返回了未构造的 `models.Result{}` | 用 `i18n.Success/Error` 构造 |
| 前端拿不到错误信息 | 直接把 error 放进 `msg` 且未启用 i18n | 用 messageId + `ErrorWithMsg` |
| 事务未回滚 | 回调里用了 `db.Mysql.GetConnection()` | 只用 `tx` |
| 并发下数据串库 | 复用了同一个 `MySQLDao` 并调用 `WithContext` | 复制 DAO 或每次新建 |
| 鉴权一直 401 | JWT 密钥未配置 / 头不是 `Bearer` | 配 `go.jwt.secret`，检查头 |
