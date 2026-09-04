package main

const toolVersion = "2.1.0"

// ProjectOptions 保存生成工程所需的全部选项（含推导字段）。
//
// v2-arch 适配要点：
//   - 所有模板生成的 import 路径已切换到 v2 的 pkg/... 形式（v1 的顶层包已迁入 pkg/）。
//   - 可选启用 v2 三个新能力：Health（/health/{live,ready,startup}）、
//     Metrics（/metrics + Prometheus 指标）、Otel（业务自接 OTel TracerProvider）。
//   - 生成的 controller 改用 errcode.Definition + i18n.ErrorDef 三向映射，
//     返回的 HTTP 状态码由 Definition.HTTPStatus 字段决定（不再是恒为 200）。
//   - 保留全部 v1 兼容项：mgin.NewApp / UsePlugin / MginPlugin / router/controller/service/model/dao
//     五层结构、Makefile -ldflags 注入 Version/BuildTime/GitHash。
type ProjectOptions struct {
	ProjectName  string
	Module       string
	Port         int
	DBs          []string
	MQ           []string // 消息队列 (多选): nats/kafka/mqtt/rabbit; 空表示不使用
	Registry     string   // nacos/consul/etcd/none
	ConfigCenter string   // nacos/consul/etcd/polaris/springconfig/file/none
	I18n         bool
	JWT          bool
	Casbin       bool
	Sys          bool
	OutputDir    string
	Force        bool
	MginVersion  string // 生成工程所依赖的 mgin 版本 (自动获取最新, 可 --mgin-version 覆盖)

	// v2 新能力开关
	Health   bool   // 启用 /health/{live,ready,startup} 探针
	Metrics  bool   // 启用 /metrics + Prometheus 指标
	Otel     bool   // 启用 OpenTelemetry (业务侧自接 SDK)
	LBPolicy string // 客户端负载均衡策略: round/random/least/consistent (默认 round)

	// 以下为推导字段
	Env          string     // 环境标识, 默认 test
	BaseURI      string     // 路由基础路径
	UsedList     string     // go.config.used 的逗号串
	PrefixBlock  string     // go.config.prefix 的 yaml 片段
	Components   []string   // 实际启用的组件 (有序去重)
	HasMQ        bool       // 是否启用了任意消息队列
	MQPlugins    []MQPlugin // 启用的 MQ 对应的插件模块 (用于生成 go.mod require)
	DataLayer    string     // mysql/postgres/clickhouse/memory
	DaoType      string     // 对应泛型 DAO 名称
	ServerType   string     // go.config.server_type
	ConfigServer string     // go.config.server
	ConfigToken  string     // go.config.token
}

// MQPlugin 描述一个消息队列插件模块: 既用于生成 go.mod 的 require 行,
// 也用于生成插件注册代码 (plugins.go)。
type MQPlugin struct {
	Name      string // 组件名, 同时也是 go.config.used 中的键: nats/kafka/mqtt/rabbit
	Path      string // 模块路径, 如 github.com/maczh/mgkafka
	Pkg       string // 导入后的包名, 如 mgkafka
	Singleton string // 包内导出的单例变量名, 实现 Init([]byte)/Close()/Check() error
	Version   string // 初始版本号 (执行 go mod tidy 后会自动修正)
}

// componentMeta 描述每个组件在配置中的前缀以及是否需要单独的 yml 文件。
var componentMeta = map[string]struct {
	prefix   string
	yamlFile bool
	registry bool
}{
	"mysql":         {"mysql", true, false},
	"postgres":      {"postgres", true, false},
	"sqlite":        {"", false, false}, // 前缀为数据库文件名, 无独立 yml
	"mongodb":       {"mongodb", true, false},
	"redis":         {"redis", true, false},
	"clickhouse":    {"clickhouse", true, false},
	"elasticsearch": {"elasticsearch", true, false},
	"kafka":         {"kafka", true, false},
	"nacos":         {"nacos", true, true},
	"consul":        {"consul", true, true},
	"etcd":          {"etcd", true, true},
	"nats":          {"nats", true, false},
	"mqtt":          {"mqtt", true, false},
	"rabbit":        {"rabbitmq", true, false},
}

// mqPlugins 定义每种消息队列对应的外部插件模块。
// modulePath: go.mod 的 require 路径; pkgName: 导入后的包名 (用于访问单例);
// singleton: 包内导出的单例变量名 (实现 mgin.MginPlugin 接口: Init/Close/Check);
// version: 初始版本号, 执行 `go mod tidy` 后会自动修正为可用版本。
// 配置键以各插件实际约定为准 (nats/kafka/mqtt 为 go.data.<name>, rabbit 为 go.rabbitmq)。
var mqPlugins = map[string]struct {
	modulePath string
	pkgName    string
	singleton  string
	version    string
}{
	"nats":   {"github.com/maczh/nats", "nats", "NATS", "v0.0.2"},
	"kafka":  {"github.com/maczh/mgkafka", "mgkafka", "Kafka", "v1.1.2"},
	"mqtt":   {"github.com/maczh/mqtt", "mqtt", "MQTT", "v1.1.6"},
	"rabbit": {"github.com/maczh/mgrabbit", "mgrabbit", "Rabbit", "v0.1.1"},
}

// mqOrder 是消息队列的固定输出顺序, 保证同一组选择生成的文件内容稳定一致。
var mqOrder = []string{"nats", "kafka", "mqtt", "rabbit"}

// lbStrategies 客户端负载均衡策略白名单, 留作未来校验。
var lbStrategies = []string{"round", "random", "least", "consistent"}

// 各组件的独立配置文件模板 (基于 <prefix>-<env>.yml)。
var componentTemplates = map[string]string{
	"mysql": `go:
  data:
    mysql: user:password@tcp(127.0.0.1:3306)/{{.ProjectName}}?charset=utf8mb4&parseTime=True&loc=Local
    mysql_debug: true
    mysql_cache: true   #打开缓存
    mysql_pool:     #连接池设置,若无此项则使用单一长连接
      max: 200      #实际最大连接数
      total: 1000   #最大并发数,不填默认为最大连接数5倍
      timeout: 30   #空闲连接超时，秒，默认60秒
      life: 5       #连接生命周期，分钟，默认60分钟
`,
	"postgres": `go:
  data:
    postgres:
      dsn: "host=127.0.0.1 user=postgres password=postgres dbname={{.ProjectName}} port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      debug: true
	  cache: true   #打开缓存
	  pool:     #连接池设置,若无此项则使用单一长连接
        max: 200      #实际最大连接数
        total: 1000   #最大并发数,不填默认为最大连接数5倍
        timeout: 30   #空闲连接超时，秒，默认60秒
        life: 5       #连接生命周期，分钟，默认60分钟
`,
	"mongodb": `go:
  data:
    mongodb:
    mongodb:
      uri: mongodb://user:password@127.0.0.1:27017/{{.ProjectName}} #当使用复制集时 mongodb://user:pwd@192.168..3.5:27017,192.168.3.6:27017/dbname?replicaSet=replsetname
      db: {{.ProjectName}}
      debug: true   #打开调试模式
    mongo_pool:     #连接池设置,若无此项则使用单一长连接
      max: 200       #最大连接数
`,
	"redis": `go:
  data:
    redis:
      host: 127.0.0.1
      port: 6379
      password: ""
      database: 0
    redis_pool:
      min: 3
      max: 100
      idle: 10
      timeout: 300
`,
	"clickhouse": `go:
  data:
    clickhouse: "clickhouse://127.0.0.1:9000/{{.ProjectName}}?username=default&password="
`,
	"elasticsearch": `go:
  elasticsearch:
    uri: http://127.0.0.1:9200
    user: elastic
    password: ""
`,
	"kafka": `go:
  data:
    kafka:
      servers: "127.0.0.1:9092"
      ack: all
      auto_commit: true
      partitioner: hash
      version: 3.7.1
`,
	"nats": `go:
  data:
    nats:
      multi: false
      uri: "nats://127.0.0.1:4222"
      user: user
      password: password
`,
	"mqtt": `go:
  data:
    mqtt:
      multi: false
      broker: "tcp://127.0.0.1:1883"
      clientId: {{.ProjectName}}-client
      username: user
      password: password
`,
	"rabbit": `go:
  rabbitmq:
    multi: false
    uri: "amqp://user:password@127.0.0.1:5672/vhost"
    exchange: "exchange"
`,
	"nacos": `go:
  nacos:
    server: 127.0.0.1
    port: 8848
    clusterName: DEFAULT
    group: DEFAULT_GROUP
    weight: 1
    lan: true
	lanNet: 192.168.113.    #网段前缀
`,
	"consul": `go:
  consul:
    server: 127.0.0.1
    port: 8500
	clusterName: DEFAULT
	group: DEFAULT_GROUP
	weight: 1
	lan: true
	lanNet: 192.168.113.    #网段前缀
`,
	"etcd": `go:
  etcd:
    server:  127.0.0.1   #etcd服务IP
    port: 2379            #etcd端口
    clusterName: DEFAULT
    group: DEFAULT_GROUP    #根据项目不同配置不同分组
    weight: 1
    lan: false   #以内网地址注册，否则以公网地址注册
    lanNet: 192.168.113.    #网段前缀
`,
	"polaris": `go:
  polaris:
    server: 127.0.0.1
    port: 8080
    namespace: default
    token: "polaris-token"
    weight: 1
    lan: true
	lanNet: 192.168.113.    #网段前缀
`,
}

// casbinModel 是 RBAC 模型文件内容，启用 Casbin 时写入 conf/casbin.conf。
const casbinModel = `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act
`

// ============================================================================
// 以下为工程内各源码文件的模板 (v2-arch 适配版)
//
// 关键变更点 (相对 v1 脚手架):
//   1. 所有 import 路径已切换到 v2 的 pkg/... 形式
//   2. tmplMain 默认注册 health/metrics 端点 (按配置开关)
//   3. tmplController 改用 errcode.Definition + i18n.ErrorDef 三向映射
//   4. tmplService 演示 client.CallCtx 替代 Call (v2 新 ctx 透传)
//   5. tmplGoMod 保留 v1 兼容性, 同时注释 v2 新增能力
// ============================================================================

// tmplMain 程序入口。
//
// v2-arch 适配:
//   - mgin.NewApp 仍为顶层入口 (v2 兼容保留)。
//   - go.framework.* / go.application.* 配置项由 mgin 内置读取。
//   - 当 --health/--metrics 开关开启时, 由框架的 baseRouter() 最早注册位置自动挂载,
//     不会被后续业务中间件拦截 (探活不被 401, /metrics 不被鉴权)。
//   - 业务可在所有初始化完成后调用 health.MarkStarted() 切换 /health/startup 到 200。
const tmplMain = `package main

import (
	"{{.Module}}/router"

	"github.com/maczh/mgin/v2"
	"github.com/maczh/mgin/v2/pkg/health"
)

// @title {{.ProjectName}} API 文档
// @version 1.0.0
// @description 基于 MGin v2 微服务框架自动生成的服务接口
// @termsOfService http://swagger.io/terms/
// @contact.name {{.ProjectName}}
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @BasePath /api/v1
func main() {
	// 参数: 配置文件路径, 应用名, 版本号 (由 Makefile 通过 -ldflags 注入 main.Version), 是否启用国际化 (xlang)
	app := mgin.NewApp("conf/application.yml", "{{.ProjectName}}", Version, {{.I18n}})
	if app == nil {
		return
	}
{{if .HasMQ}}	// 注册消息队列插件 (见 plugins.go)
	registerMQPlugins(app)
{{end}}{{if or .Health .Metrics}}
	// 显式挂载 v2 新增的探针/指标端点 (与配置文件 go.framework.* 等效)。
	// 不调用也无所谓: 配置 enabled=true 时框架自动挂载; 这里调用是为业务
	// 提供"默认开启"的便利, 关闭只需去掉配置 + 删本行。
	{{- if .Health}}
	app.EnableHealth()
	{{- end}}
	{{- if .Metrics}}
	app.EnableMetrics()
	{{- end}}
{{end}}
	router.RegisterRoutes(app)
{{if .Health}}
	// 标记启动完成, 让 K8s startup 探针从 503 切换到 200。
	// 在监听启动后调用 (业务侧 init 完毕即可, 不需要等服务 ready)。
	health.MarkStarted()
{{end}}
	app.Run()
}
`

// tmplPlugins 生成插件注册文件 plugins.go。
//
// mgin 通过 app.MGin.UsePlugin(name, plugin) 挂载外部插件:
// name 必须出现在 go.config.used 中, 框架据此从 go.config.prefix.<name> 指定的
// 配置文件读取数据并调用 Init; Close 在进程退出时调用。
//
// v2 兼容: v2 的 plugin.Plugin 接口 (Name/Order/Init/Close/Health/Enabled) 是
// 新规范, 但 MginPlugin (Init([]byte)/Close()/Check() error) 仍保留并 100% 兼容。
// 外部 MQ 插件 (nats/kafka/mqtt/rabbit) 当前都使用 MginPlugin 接口, 因此本文件
// 在 v2 框架下继续生效。若新写插件, 建议直接实现 plugin.Plugin 接口并用
// plugin.Register() 注册 (按 Order 自动管理 Init/Close)。
const tmplPlugins = `package main

import (
	"github.com/maczh/mgin/v2"{{range .MQPlugins}}
	"{{.Path}}"{{end}}
)

// registerMQPlugins 注册消息队列插件 (兼容 v1/v2)。
// 注意: mgin 框架自带 sarama 版 kafka 客户端, 当 go.config.used 含 kafka 时,
// 框架会先初始化内置客户端, 再初始化本文件中的 mgkafka 插件。
// 若仅需使用 mgkafka 插件, 可从 go.config.used 中去掉 kafka 并同步删除 conf/kafka-<env>.yml。
func registerMQPlugins(app *mgin.App) { {{- range .MQPlugins}}
	app.MGin.UsePlugin("{{.Name}}", {{.Pkg}}.{{.Singleton}})
{{- end}}
}
`

// tmplRouter 路由与全局中间件注册。
//
// v2-arch 适配: 中间件路径改为 pkg/middleware/jwt、pkg/middleware/casbin。
// v2 也保留了原 middleware/jwt 的所有公开 API, 业务代码无需改。
const tmplRouter = `package router

import (
	"github.com/maczh/mgin/v2"
	"{{.Module}}/controller"
{{if .JWT}}	"github.com/maczh/mgin/v2/pkg/middleware/jwt"
{{end}}{{if .Casbin}}	"github.com/maczh/mgin/v2/pkg/middleware/casbin"
{{end}})

// RegisterRoutes 注册所有业务路由与全局中间件
func RegisterRoutes(app *mgin.App) {
{{if .JWT}}	// JWT 鉴权中间件 (白名单: /swagger/ /docs/ 及 sys 文档路径已自动放行)
	app.Router.Use(jwt.JwtAuthorize())
{{end}}{{if .Casbin}}	// Casbin 接口级 RBAC 鉴权 (需 go.casbin.enabled=true)
	app.Router.Use(casbin.CasbinHandler())
{{end}}
	v1 := app.Router.Group("{{.BaseURI}}")
	{
		v1.GET("/products", controller.ListProducts)
		v1.GET("/products/:id", controller.GetProduct)
		// TODO: 在此追加更多业务路由
	}
}
`

// tmplController HTTP 处理函数。
//
// v2-arch 适配:
//   - 用 errcode.Definition {Code, HTTPStatus, MessageKey, Module} 表达业务错误码;
//   - 用 i18n.ErrorDef(def, args...) 渲染 (HTTP 状态码由 Definition.HTTPStatus 决定,
//     业务码正确码由 Definition.Code 决定, 文案由 i18n 取, 三向映射);
const tmplController = `package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/errcode"
	"github.com/maczh/mgin/v2/pkg/i18n"
	"{{.Module}}/service"
)

// 业务错误码定义 (集中管理, 便于前端/API 文档对齐)。
// i18n 文案键需要在 conf/application.yml 的 i18n 节点配置, 此处给的是默认 key,
// 业务可在 conf/i18n-<env>.yml 中翻译。
var (
	ErrInvalidParam  = errcode.New("PRODUCT", 4001, 400, "product.param.invalid")
	ErrProductMiss  = errcode.New("PRODUCT", 4004, 404, "product.not_found")
	ErrInternalBusy  = errcode.New("SYSTEM",  5000, 500, "system.busy")
)

var productService = &service.ProductService{}

// ListProducts 查询商品列表
// @Summary 查询商品列表
// @Tags 商品
// @Produce json
// @Success 200 {object} models.Result
// @Router /products [get]
func ListProducts(c *gin.Context) {
	list, err := productService.List(c.Request.Context())
	if err != nil {
		// v2 三向映射: i18n.ErrorDef 把 errcode.Definition 的 HTTPStatus/Code/MessageKey
		// 三者字段分别填入 HTTP 响应状态码 / 业务返回码 / i18n 文案, 不再返 200。
		c.JSON(ErrInternalBusy.HTTPStatus, i18n.ErrorDef(ErrInternalBusy, err.Error()))
		return
	}
	c.JSON(200, models_Result(list))
}

// GetProduct 查询商品详情
// @Summary 查询商品详情
// @Tags 商品
// @Produce json
// @Param id path int true "商品ID"
// @Success 200 {object} models.Result
// @Router /products/{id} [get]
func GetProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(ErrInvalidParam.HTTPStatus, i18n.ErrorDef(ErrInvalidParam))
		return
	}
	product, err := productService.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(ErrInternalBusy.HTTPStatus, i18n.ErrorDef(ErrInternalBusy, err.Error()))
		return
	}
	if product == nil {
		c.JSON(ErrProductMiss.HTTPStatus, i18n.ErrorDef(ErrProductMiss))
		return
	}
	c.JSON(200, models_Result(product))
}

// models_Result 是 models.Success 的本地别名, 避免在每个文件重复 import models;
// 业务复杂时可改为直接导入 "github.com/maczh/mgin/v2/pkg/models"。
func models_Result(data any) any {
	return map[string]any{"code": 0, "msg": "ok", "data": data}
}
`

// tmplModel 含反引号(GORM 标签)，必须用双引号字符串定义。
const tmplModel = "package model\n\nimport \"time\"\n\n// Product 商品示例模型 (GORM)\ntype Product struct {\n" +
	"\tId        int64     `gorm:\"column:id;primaryKey;autoIncrement\" json:\"id\"`\n" +
	"\tName      string    `gorm:\"column:name\" json:\"name\"`\n" +
	"\tPrice     float64   `gorm:\"column:price\" json:\"price\"`\n" +
	"\tStock     int       `gorm:\"column:stock\" json:\"stock\"`\n" +
	"\tCreatedAt time.Time `gorm:\"column:created_at\" json:\"createdAt\"`\n" +
	"\tUpdatedAt time.Time `gorm:\"column:updated_at\" json:\"updatedAt\"`\n" +
	"}\n\n// TableName 指定表名\nfunc (Product) TableName() string {\n\treturn \"product\"\n}\n"

// tmplDao 数据访问 (v2 Dao[E] 接口)。
//
// v2-arch 适配: Dao[E] 接口已扩展为 7 方法 (Insert/Where/Find/FindById/Update/Delete/Count),
// 这里的 dao.ProductDao 仍以泛型方式实例化, 业务可直接用新接口方法。
const tmplDao = `package dao

import (
	"github.com/maczh/mgin/v2/pkg/db/dao"
	"{{.Module}}/model"
)

// ProductDao 商品数据访问, 基于 mgin 泛型 DAO ({{.DataLayer}})。
// v2 Dao[E] 已扩展 (Insert/Where/Find/FindById/Update/Delete/Count),
// 这里仍用泛型实例化, 业务可直接访问新接口方法。
var ProductDao = &dao.{{.DaoType}}[model.Product]{}
`

// tmplServiceDB 业务层 (DB 版)。
//
// v2-arch 适配:
//   - 业务方法接受 context.Context (Go 惯例, 配合 gin 与 client.CallCtx);
//   - 用 dao.ProductDao.All(model.Product{}) / One(model.Product{Id: id}) 查数据;
//   - 演示用法: 直接把 ctx 屏蔽, 让 demo 在没有真实请求时也能跑;
//     真实业务应把 ctx 透传到 dao.ProductDao.WithContext(&ctx).All(...) 调用链。
const tmplServiceDB = `package service

import (
	"context"

	"{{.Module}}/dao"
	"{{.Module}}/model"
)

// ProductService 商品业务层
type ProductService struct{}

// List 查询商品列表
//
// v2 起, 业务方法接受 context.Context 以便:
//   1. 透传给 DAO (gorm 链路支持 WithContext 自动取消);
//   2. 透传给 client.CallCtx 跨服务调用;
//   3. 被中间件/超时控制自然终止。
func (s *ProductService) List(ctx context.Context) ([]model.Product, error) {
	_ = ctx // v2: 演示用法 — 真实业务应将 ctx 透传到 dao.WithContext(&ctx).All(...)
	return dao.ProductDao.All(model.Product{})
}

// Get 根据 ID 查询商品
func (s *ProductService) Get(ctx context.Context, id int64) (*model.Product, error) {
	_ = ctx
	return dao.ProductDao.One(model.Product{Id: id})
}
`

// tmplServiceMemory 内存数据版业务层。
const tmplServiceMemory = `package service

import (
	"context"

	"{{.Module}}/model"
)

// ProductService 商品业务层
// 示例采用内存数据; 接入数据库时, 将本文件替换为基于 {{.Module}}/dao 的实现即可。
type ProductService struct{}

var mockProducts = []model.Product{
	{Id: 1, Name: "示例商品A", Price: 99.9, Stock: 100},
	{Id: 2, Name: "示例商品B", Price: 199.9, Stock: 50},
}

// List 查询商品列表
func (s *ProductService) List(ctx context.Context) ([]model.Product, error) {
	_ = ctx
	return mockProducts, nil
}

// Get 根据 ID 查询商品
func (s *ProductService) Get(ctx context.Context, id int64) (*model.Product, error) {
	_ = ctx
	for i := range mockProducts {
		if mockProducts[i].Id == id {
			return &mockProducts[i], nil
		}
	}
	return nil, nil
}
`

// tmplGoMod 模块依赖。
//
// v2-arch 适配: 模块名仍为 github.com/maczh/mgin/v2, 但生成工程显式声明 Go 1.25
// (v2 已升级), 并把 gin v1.11.0 作为基础依赖列出。v2 内部依赖 (如 otel/prometheus)
// 不会被工程直接引用, 所以不需要在 require 中声明。
const tmplGoMod = `module {{.Module}}

go 1.25

require (
	github.com/gin-gonic/gin v1.11.0
	github.com/maczh/mgin/v2 {{.MginVersion}}{{range .MQPlugins}}
	{{.Path}} {{.Version}}{{end}}
)

// mgin 版本由脚手架自动获取最新发布版 (离线时回退到默认版本);
// 消息队列插件 (如启用) 版本为已知初始值, 执行 ` + "`go mod tidy`" + ` 后会自动修正为可用版本 (需联网)。
`

// tmplVersion 定义由构建系统通过 -ldflags "-X main.Version=..." 注入的版本变量。
// 参考 jihaihotpot.com/jihai/jh-ris-order/version.go。
const tmplVersion = `package main

// 以下变量由构建系统(Makefile)通过 -ldflags 注入, 不要在代码中赋值。
var (
	Version   string // 完整版本号
	BuildNum  int    // 自增编译次数
	BuildTime string // 编译时间
	GitHash   string // git 提交哈希
)
`

// tmplMakefile 是工程根目录的 Makefile, 参考 jihaihotpot.com/jihai/jh-ris-order/Makefile。
//
// 的 v2 时代改造点:
//   - 新增 `make test` / `make test-race` / `make lint` / `make cover` / `make build-multi-os` target;
//   - 保留原 build/linux/run target;
//   - 保留 -ldflags 注入 Version/BuildTime/GitHash;
//   - LDFLAGS 的 $() / ${} 是 shell 变量, 不能走 text/template, 因此仍用字符串替换占位符 {{.ProjectName}}。
const tmplMakefile = "# Makefile for {{.ProjectName}}\n" +
	"BINARY={{.ProjectName}}\n" +
	"VERSION=$(shell git tag 2>/dev/null | tail -n 1)\n" +
	"BUILD_TIME_SHORT=$(shell date '+%Y%m%d%H%M%S')\n" +
	"BUILD_TIME=$(shell date '+%Y-%m-%d %H:%M:%S')\n" +
	"GIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)\n" +
	"LDFLAGS=-ldflags \"-X main.Version=${VERSION}-build${BUILD_TIME_SHORT} -X 'main.BuildTime=${BUILD_TIME}' -X main.GitHash=${GIT_HASH}\"\n" +
	"\n" +
	".PHONY: tidy\n" +
	"tidy:\n" +
	"\tGOPROXY=https://goproxy.cn go mod tidy\n" +
	"\n" +
	".PHONY: build\n" +
	"build: tidy\n" +
	"\tgo build ${LDFLAGS} -o ${BINARY}\n" +
	"\n" +
	".PHONY: linux\n" +
	"linux: tidy\n" +
	"\tCGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY}\n" +
	"\tupx ${BINARY}\n" +
	"\n" +
	".PHONY: build-multi-os\n" +
	"build-multi-os: tidy\n" +
	"\t@mkdir -p dist\n" +
	"\tCGO_ENABLED=0 GOOS=linux   GOARCH=amd64        go build ${LDFLAGS} -o dist/${BINARY}-linux-amd64\n" +
	"\tCGO_ENABLED=0 GOOS=linux   GOARCH=arm64        go build ${LDFLAGS} -o dist/${BINARY}-linux-arm64\n" +
	"\tCGO_ENABLED=0 GOOS=darwin  GOARCH=amd64        go build ${LDFLAGS} -o dist/${BINARY}-darwin-amd64\n" +
	"\tCGO_ENABLED=0 GOOS=darwin  GOARCH=arm64        go build ${LDFLAGS} -o dist/${BINARY}-darwin-arm64\n" +
	"\tCGO_ENABLED=0 GOOS=windows GOARCH=amd64        go build ${LDFLAGS} -o dist/${BINARY}-windows-amd64.exe\n" +
	"\t@ls -la dist/\n" +
	"\n" +
	".PHONY: run\n" +
	"run:\n" +
	"\tgo run ${LDFLAGS} .\n" +
	"\n" +
	".PHONY: test\n" +
	"test:\n" +
	"\tgo test -count=1 ./...\n" +
	"\n" +
	".PHONY: test-race\n" +
	"test-race:\n" +
	"\tgo test -race -count=1 ./...\n" +
	"\n" +
	".PHONY: cover\n" +
	"cover:\n" +
	"\tgo test -count=1 -coverprofile=coverage.out ./...\n" +
	"\tgo tool cover -html=coverage.out -o coverage.html\n" +
	"\t@echo \"coverage report: coverage.html\"\n" +
	"\n" +
	".PHONY: lint\n" +
	"lint:\n" +
	"\tgo vet ./...\n" +
	"\t@which golangci-lint >/dev/null 2>&1 && golangci-lint run --timeout=5m || echo \"golangci-lint 未安装, 仅执行 go vet; 建议安装: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest\"\n" +
	"\n" +
	".PHONY: clean\n" +
	"clean:\n" +
	"\trm -f ${BINARY}\n" +
	"\trm -rf dist coverage.out coverage.html\n"

// buildComponentFileName 返回组件的独立配置文件名（仅用于生成提示，无副作用）。
func buildComponentFileName(prefix, env string) string {
	return prefix + "-" + env + ".yml"
}
