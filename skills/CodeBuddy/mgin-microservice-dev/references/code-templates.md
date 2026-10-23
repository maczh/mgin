# 可直接复制的代码模板

> 按 jihai 实际项目惯例整理。v1 用 `github.com/maczh/mgin/...`，v2 全部改成 `github.com/maczh/mgin/v2/pkg/...`。

---

## 1. `model/vo/result.go` —— 统一响应（★ 每个项目复制一份）

jihai 各项目不 import 框架的 `models.Result`，而是复制这个文件（用 diff 验证过 `jh-ris-common/vo/result.go` 与 `jh-ris-order/model/vo/result.go` 完全相同）。

```go
package vo

import (
	"reflect"
	"time"

	"github.com/maczh/mgin/models"
)

type Response[T any] struct {
	Code    int       `json:"code"`
	Data    T         `json:"data"`
	Msg     string    `json:"msg"`
	SrvTime time.Time `json:"srvTime"`
}

type ResponsePage[T any] struct {
	Code    int          `json:"code"`
	Msg     string       `json:"msg"`
	SrvTime time.Time    `json:"srvTime"`
	Data    *PageData[T] `json:"data"`
}

type PageData[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// ★ 只有三个错误码常量
const (
	ERROR   = 500
	SUCCESS = 1
	UnLogin = 999
)

func Success(data any) Response[any] {
	return Response[any]{Code: SUCCESS, Data: data, Msg: "success", SrvTime: time.Now()}
}

func SuccessT[T any](data T) Response[T] {
	return Response[T]{Code: SUCCESS, Data: data, Msg: "success", SrvTime: time.Now()}
}

func SuccessPage(data any, page *models.ResultPage) ResponsePage[any] {
	return ResponsePage[any]{
		Code: SUCCESS,
		Data: &PageData[any]{
			List: func() []any {
				if anySlice, ok := data.([]any); ok {
					return anySlice
				}
				val := reflect.ValueOf(data)
				if val.Kind() != reflect.Slice {
					return nil
				}
				anyList := make([]any, val.Len())
				for i := 0; i < val.Len(); i++ {
					anyList[i] = val.Index(i).Interface()
				}
				return anyList
			}(),
			Total:    int64(page.Total),
			Page:     page.Index,
			PageSize: page.Size,
		},
		Msg:     "success",
		SrvTime: time.Now(),
	}
}

func Error(status int, msg string) Response[any] {
	return Response[any]{Code: status, Data: nil, Msg: msg, SrvTime: time.Now()}
}

func ErrorT[T any](status int, msg string) Response[T] {
	return Response[T]{Code: status, Msg: msg, SrvTime: time.Now()}
}

func ErrorPage(status int, msg string) ResponsePage[any] {
	return ResponsePage[any]{Code: status, Data: nil, Msg: msg, SrvTime: time.Now()}
}

func ErrorPageT[T any](status int, msg string) ResponsePage[T] {
	return ResponsePage[T]{Code: status, Data: nil, Msg: msg, SrvTime: time.Now()}
}
```

---

## 2. `model/entity.go` —— 实体

纯 MySQL：
```go
package model

import "time"

type TableBooking struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	BookingId   string    `gorm:"column:booking_id;type:varchar(64);unique;not null;comment:预订ID" json:"bookingId"`
	StoreId     string    `gorm:"column:store_id;type:varchar(64);index;not null" json:"storeId"`
	PeopleCount int       `gorm:"column:people_count;type:int(11);not null;comment:人数" json:"peopleCount"`
	Status      string    `gorm:"column:status;type:varchar(20);not null;comment:状态 BOOKED/DINING/FINISHED/CANCELLED" json:"status"`
	CreateTime  time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
	UpdateTime  time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`

	Tables []BookingTable `gorm:"-" json:"tables"`   // ★ 非持久化关联必须 gorm:"-"
}

func (TableBooking) TableName() string { return "table_booking" }
```

MySQL + Mongo 双写（jihai Order 的做法）：
```go
type Order struct {
	Mid       primitive.ObjectID `gorm:"-" json:"_id" bson:"_id"`
	Id        int                `gorm:"primaryKey" json:"id" bson:"-"`
	OrderId   string             `gorm:"column:order_id;type:varchar(64);uniqueIndex" json:"orderId" bson:"orderId"`
	// ...
	Batchs    []OrderBatch       `gorm:"-" json:"batchs,omitempty" bson:"batchs,omitempty"`
}

func (Order) TableName() string      { return "jh_ris_order" }
func (Order) CollectionName() string { return "jh_ris_order_detail" }
```

**约定**：`gorm:` 写全（column/type/index/uniqueIndex/comment/autoCreateTime）；`json:` 小驼峰；**必须有 `TableName()`**。

## 3. `model/ro/req.go` —— 入参

```go
package ro

type CreateBookingReq struct {
	StoreId     string `json:"storeId"`
	BookingDate string `json:"bookingDate"`
	PeopleCount int    `json:"peopleCount"`
	Remark      string `json:"remark"`
	Token       string `json:"-"`        // 从 header 来，不参与 body 绑定
}

type ListBookingReq struct {
	StoreId string `json:"storeId" form:"storeId"`
	Status  string `json:"status" form:"status"`
	Page    int    `json:"page" form:"page"`
	Size    int    `json:"size" form:"size"`
}
```

---

## 4. `mysql/enter.go` —— DAO 层 + 初始化

```go
package mysql

import (
	"time"

	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/db/dao"
	"github.com/maczh/mgin/logs"
	"gorm.io/gorm/schema"
	"jihaihotpot.com/jihai/myservice/model"
)

var (
	TableBookingDao = &dao.MySQLDao[model.TableBooking]{}   // v1 用值类型 dao.MySQLDao[T]{}
	BookingTableDao = &dao.MySQLDao[model.BookingTable]{}
)

func applyMySQLDaoCache[T schema.Tabler](target *dao.MySQLDao[T], config dao.CacheConfig) {
	*target = *target.WithCache(config)
}

func MySQLInit() {
	tables := []interface{}{
		&model.TableBooking{},
		&model.BookingTable{},
	}
	conn, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("获取数据库连接失败:{}", err)
		return
	}
	if err = conn.AutoMigrate(tables...); err != nil {
		logs.Error("创建数据库表失败:{}", err)
		return
	}
	logs.Info("MySQL数据库初始化成功")

	// 可选：挂 Redis 二级缓存
	rds, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("获取Redis连接失败:{}", err)
		return
	}
	cfg := dao.CacheConfig{Enabled: true, Store: dao.NewRedisCache(rds), TTL: 5 * time.Minute, Prefix: "cache:mysql"}
	applyMySQLDaoCache(TableBookingDao, cfg)
	applyMySQLDaoCache(BookingTableDao, cfg)
}
```

### 给泛型 DAO 扩展业务方法（无法给外部包加方法，用内嵌）

```go
// mysql/setting.go
type BookingSettingDaoType struct {
	*dao.MySQLDao[model.BookingSetting]
}

func (d *BookingSettingDaoType) GetByKey(k string) (*model.BookingSetting, error) {
	return d.One(model.BookingSetting{K: k})     // 未命中返回 (nil, nil)
}

func (d *BookingSettingDaoType) Upsert(m *model.BookingSetting) error {
	existing, err := d.GetByKey(m.K)
	if err != nil {
		return err
	}
	if existing != nil {
		existing.V = m.V
		return d.Updates(existing)
	}
	return d.Create(m)
}

func (d *BookingSettingDaoType) List() ([]model.BookingSetting, error) {
	return d.All(model.BookingSetting{}, dao.QueryOption{OrderBy: []string{"k"}})
}
// 实例化：var BookingSettingDao = &BookingSettingDaoType{MySQLDao: &dao.MySQLDao[model.BookingSetting]{}}
```

---

## 5. `service/` —— 业务层

```go
package admin

import (
	"jihaihotpot.com/jihai/myservice/model"
	"jihaihotpot.com/jihai/myservice/mysql"
	"jihaihotpot.com/jihai/jh-ris-common/vo"
)

type timeslotService struct{}

var TimeSlot = &timeslotService{}     // 供 controller 调用的单例

func (s *timeslotService) Create(req model.TimeSlotConfig) vo.Response[any] {
	if req.TimeSlot == "" {
		return vo.Error(vo.ERROR, "时段标识不能为空")
	}
	if mysql.TimeSlotConfigDao.Exists(model.TimeSlotConfig{TimeSlot: req.TimeSlot}) {
		return vo.Error(vo.ERROR, "时段标识已存在")
	}
	if err := mysql.TimeSlotConfigDao.Create(&req); err != nil {
		return vo.Error(vo.ERROR, err.Error())
	}
	return vo.Success(req)
}

func (s *timeslotService) Get(id int64) vo.Response[any] {
	one, err := mysql.TimeSlotConfigDao.One(model.TimeSlotConfig{ID: id})
	if err != nil {
		return vo.Error(vo.ERROR, err.Error())
	}
	if one == nil {
		return vo.Error(vo.ERROR, "时段配置不存在")
	}
	return vo.Success(one)
}
```

分页版（`Where` 链式 + `Pager`）：
```go
func (s *logService) List(req ro.ListLogReq) vo.ResponsePage[any] {
	db := mysql.OperateLogDao.Where("1 = 1")
	if req.StoreId != "" {
		db = db.Where("store_id = ?", req.StoreId)
	}
	if req.Sort == "ASC" {
		db = db.Order("id ASC")
	} else {
		db = db.Order("id DESC")
	}
	list, rpage, err := mysql.OperateLogDao.Pager(db, req.Page, req.Size)
	if err != nil {
		return vo.ErrorPage(500, "查询失败:"+err.Error())
	}
	return vo.SuccessPage(list, rpage)
}
```

返回 `error` 的变体（给 MQ handler / job 用）：
```go
func (s *bookingService) GenSummary(orderId string) error {
	// ...
}
```

---

## 6. `controller/` + `router/`

```go
// controller/admin/timeslot.go
package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"jihaihotpot.com/jihai/myservice/model"
	"jihaihotpot.com/jihai/myservice/model/ro"
	adminservice "jihaihotpot.com/jihai/myservice/service/admin"
	"jihaihotpot.com/jihai/jh-ris-common/vo"
)

type timeslotController struct{}

var TimeSlot = &timeslotController{}

// Create 新增时段配置
// @Summary 新增时段配置
// @Tags 后台-时段管理
// @Accept json
// @Produce json
// @Param req body model.TimeSlotConfig true "时段配置"
// @Success 200 {object} vo.Response[model.TimeSlotConfig]
// @Failure 400 {object} vo.Response[any]
// @Router /api/myservice/v1/admin/timeslot/save [post]
func (c *timeslotController) Create(ctx *gin.Context) vo.Response[any] {
	var req model.TimeSlotConfig
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return vo.Error(vo.ERROR, "参数解析错误:"+err.Error())
	}
	return adminservice.TimeSlot.Create(req)
}

func (c *timeslotController) Get(ctx *gin.Context) vo.Response[any] {
	id, err := strconv.ParseInt(ctx.Query("id"), 10, 64)
	if err != nil {
		return vo.Error(vo.ERROR, "时段ID无效")
	}
	return adminservice.TimeSlot.Get(id)
}
```

```go
// controller/enter.go
package controller

import (
	"jihaihotpot.com/jihai/myservice/controller/admin"
	"jihaihotpot.com/jihai/myservice/controller/mp"
)

var (
	Admin = admin.AdminController
	Mp    = mp.MpController
)
```

```go
// router/router.go（v2）
package router

import (
	"github.com/maczh/mgin/v2"
	adminrouter "jihaihotpot.com/jihai/myservice/router/admin"
	mprouter "jihaihotpot.com/jihai/myservice/router/mp"
)

func RegisterRoutes(app *mgin.App) {
	adminrouter.Registers(app.Router.Group("/api/myservice/v1/admin"))
	mprouter.Registers(app.Router.Group("/api/myservice/v1/mp"))
}
```

```go
// router/admin/router.go
package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"jihaihotpot.com/jihai/myservice/controller/admin"
)

type bookingAdminRouter struct{}

func (r bookingAdminRouter) Register(prefix *gin.RouterGroup) {
	prefix.POST("/timeslot/save", func(c *gin.Context) {
		c.JSON(http.StatusOK, admin.TimeSlot.Create(c))
	})
	prefix.GET("/timeslot/get", func(c *gin.Context) {
		c.JSON(http.StatusOK, admin.TimeSlot.Get(c))
	})
	settingAdminRouter{}.Register(prefix)
}

func Registers(group *gin.RouterGroup) { bookingAdminRouter{}.Register(group) }
```

v1 版路由注册（传 `*gin.Engine`）：
```go
func Register(engine *gin.Engine) {
	admin.Register(engine.Group("/api/myservice/v1/admin"))
	mp.Register(engine.Group("/api/myservice/v1/mp"))
}
```

**文件下载型 controller**（不走统一响应，直接写流，没有返回值）：
```go
func (c *adminController) Export(ctx *gin.Context) {
	resp := service.Admin.Export(req)
	if resp.Code != vo.SUCCESS {
		ctx.JSON(http.StatusOK, resp)
		return
	}
	ctx.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	// RFC 5987：用 filename* 承载 UTF-8 中文名，避免各浏览器下载文件名乱码
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(fileName)))
	ctx.Writer.Write(file.Content)
}
```

---

## 7. `ms/` —— 跨服务调用

每个项目复制一份 `client.go`（jihai 全系代码完全相同，只是 v2 换 import 路径）：

```go
package ms

import (
	"github.com/maczh/mgin/client"
)

const BASE_SERVICE = "baseServ"     // 基础服务

// 或用框架的 client 包直接调（推荐新项目直接用这个）
func GetTable(tableId int) (*model.Table, error) {
	result := client.CallT[model.Table](BASE_SERVICE, "/api/table/info", &client.Options{
		Method:   "GET",
		Protocol: client.CONTENT_TYPE_JSON,
		Query:    map[string]string{"id": fmt.Sprintf("%d", tableId)},
	})
	if result.Status != 1 {
		return nil, errors.New(result.Msg)
	}
	return &result.Data, nil
}
```

jihai 老项目是自己实现 `Call/CallT/CallPageT`（用 `registry.Registry.GetServiceURL` + grequests，额外落 Mongo 调用日志）。**新项目直接用 `client.CallT` 即可**——它已经做了 trace 透传和服务发现。

---

## 8. `mw/jwt.go` —— 鉴权中间件

```go
package mw

import (
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/utils"
	"jihaihotpot.com/jihai/jh-ris-common/vo"
	"jihaihotpot.com/jihai/myservice/ms"
)

func XTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1) 文档与后台路径放行
		if strings.HasPrefix(c.Request.RequestURI, "/api/myservice/v1/docs/") ||
			strings.HasPrefix(c.Request.RequestURI, "/api/myservice/v1/admin/") {
			c.Next()
			return
		}
		token := c.Request.Header.Get("x-token")      // ★ 小写 header，不是 Authorization
		if token == "" {
			c.AbortWithStatusJSON(401, vo.Error(401, "token is empty"))
			return
		}
		// 2) App 端：本地 JWT 校验
		if strings.HasPrefix(c.Request.RequestURI, "/api/myservice/v1/app/") {
			tk, _ := utils.ValidateToken(token)
			user := tk.Claims.(jwt.MapClaims)
			if user == nil {
				c.AbortWithStatusJSON(401, vo.Error(401, "token is invalid"))
				return
			}
			c.Set("user", user)
			c.Next()
			return
		}
		// 3) 小程序端：调会员服务换会员信息
		if strings.HasPrefix(c.Request.RequestURI, "/api/myservice/v1/mp/") {
			member, err := ms.GetMemberByToken(token)
			if err != nil {
				c.AbortWithStatusJSON(401, vo.Error(401, err.Error()))
				return
			}
			c.Set("member", *member)
			c.Next()
		}
	}
}
```

controller 里取登录态：
```go
// JWT MapClaims
user, ok := ctx.Get("user")
um := user.(jwt.MapClaims)
operatorId := int(um["ID"].(float64))

// 业务实体
member, ok := ctx.Get("member")
m := member.(model.Member)
```

---

## 9. `mq/` —— 消息订阅

```go
package mq

// Register 集中注册所有订阅，main.go 里调一次
func Register() {
	registerOrderSummary()
	registerTableEvent()
}
```

NATS：
```go
import _nats "github.com/nats-io/nats.go"
import "github.com/maczh/nats"

func registerOrderSummary() {
	nats.NATS.SubscribeQueue("", "myservice.order.>", "order.summary.gen", func(msg *_nats.Msg) {
		var m model.Message[any]
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			logs.Error("反序列化消息体错误:{}", err.Error())
			return
		}
		if err := service.Admin.GenSummary(m.OrderId); err != nil {
			logs.Error("生成摘要失败:{}", err.Error())
		}
	})
}
```

Kafka：
```go
kafka.Kafka.Send("biz.table.occupy."+storeId, string(payload))
kafka.Kafka.MessageListener("myservice-up-"+storeId, "biz.table.up."+storeId, func(topic, raw string) error {
	var msg model.Message[model.TableOccupyRespData]
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil      // ★ 解析失败返回 nil，避免死信循环
	}
	return service.HandleResult(msg.Data)
})
```

MQTT：
```go
import paho "github.com/eclipse/paho.mqtt.golang"

mqtt.MQTT.Subscribe("", "biz/table/open/#", 1, func(client paho.Client, msg paho.Message) {
	// msg.Topic() / msg.Payload()
})
mqtt.MQTT.Publish("", "biz/table/open/"+storeId, 0, false, []byte(payload))
```

用 `sync.Map` 记录已订阅的 topic 避免重复订阅（jihai booking 的做法）。

---

## 10. `version.go` + `Makefile`

```go
package main

// 以下变量由构建系统(Makefile)通过 -ldflags 注入, 不要在代码中赋值。
var (
	Version   string
	BuildNum  int
	BuildTime string
	GitHash   string
)
```

```makefile
BINARY=myservice
VERSION=$(shell git tag -l 'v[0-9]*' --sort=-v:refname 2>/dev/null | head -n 1)
BUILD_TIME_SHORT=$(shell date '+%Y%m%d%H%M%S')
BUILD_TIME=$(shell date '+%Y-%m-%d %H:%M:%S')
GIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS=-ldflags "-X main.Version=${VERSION}-build${BUILD_TIME_SHORT} -X 'main.BuildTime=${BUILD_TIME}' -X main.GitHash=${GIT_HASH}"

.PHONY: tidy
tidy:
	GOPROXY=https://goproxy.cn go mod tidy

.PHONY: build
build: tidy
	go build ${LDFLAGS} -o ${BINARY}

.PHONY: linux
linux: tidy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY}
	upx ${BINARY}

.PHONY: run
run:
	go run ${LDFLAGS} .

.PHONY: test
test:
	go test -count=1 ./...

.PHONY: lint
lint:
	go vet ./...
```

---

## 11. Swagger

main.go 顶部（新项目用小写标准格式）：
```go
// @title myservice API 文档
// @version 1.0.0
// @description 某某服务
// @termsOfService http://swagger.io/terms/
// @contact.name xxx
// @license.name MIT
```
+ `import _ "module/docs"` + `app.Router.GET("/api/myservice/v1/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))`

生成：
```bash
swag init -g main.go -o docs
```
⚠️ 泛型 `vo.Response[any]` 会被 swag 渲染成 `vo.Response-any`。

---

## 12. go.mod 基线

```
module jihaihotpot.com/jihai/myservice

go 1.25.7

replace jihaihotpot.com/jihai/jh-ris-common v0.0.1 => ../jh-ris-common

require (
	github.com/gin-gonic/gin v1.12.0
	github.com/maczh/mgin v1.25.22-jh           // v2: github.com/maczh/mgin/v2 v2.0.7
	github.com/maczh/mgkafka v1.1.1             // 按需
	github.com/maczh/mqtt v1.1.6
	github.com/maczh/nats v0.0.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/levigross/grequests v0.0.0-20250606031859-3f3c12e4e704
	github.com/robfig/cron/v3 v3.0.1
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.1
	github.com/swaggo/swag v1.16.6
	go.mongodb.org/mongo-driver v1.17.9
	gorm.io/gorm v1.31.1
)
```
mgin 用 go module 正常拉取（带 `-jh` 后缀的私有版本）。本地 replace 注释保留，调试时打开：
```
// replace github.com/maczh/mgin v1.25.22-jh => ../../../github.com/maczh/mgin
```
