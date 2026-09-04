package main

const toolVersion = "1.0.0"

// ProjectOptions 保存生成工程所需的全部选项（含推导字段）。
type ProjectOptions struct {
	ProjectName  string
	Module       string
	Port         int
	DBs          []string
	MQ           []string // 消息队列(多选): nats/kafka/mqtt/rabbit; 空表示不使用
	Registry     string   // nacos/consul/etcd/none
	ConfigCenter string   // nacos/consul/etcd/polaris/springconfig/file/none
	I18n         bool
	JWT          bool
	Casbin       bool
	Sys          bool
	OutputDir    string
	Force        bool
	MginVersion  string // 生成工程所依赖的 mgin 版本(自动获取最新, 可 --mgin-version 覆盖)

	// 以下为推导字段
	Env          string     // 环境标识, 默认 test
	BaseURI      string     // 路由基础路径
	UsedList     string     // go.config.used 的逗号串
	PrefixBlock  string     // go.config.prefix 的 yaml 片段
	Components   []string   // 实际启用的组件(有序去重)
	HasMQ        bool       // 是否启用了任意消息队列
	MQPlugins    []MQPlugin // 启用的 MQ 对应的插件模块(用于生成 go.mod require)
	DataLayer    string     // mysql/postgres/clickhouse/memory
	DaoType      string     // 对应泛型 DAO 名称
	ServerType   string     // go.config.server_type
	ConfigServer string     // go.config.server
	ConfigToken  string     // go.config.token
}

// MQPlugin 描述一个消息队列插件模块: 既用于生成 go.mod 的 require 行,
// 也用于生成插件注册代码(plugins.go)。
type MQPlugin struct {
	Name      string // 组件名, 同时也是 go.config.used 中的键: nats/kafka/mqtt/rabbit
	Path      string // 模块路径, 如 github.com/maczh/mgkafka
	Pkg       string // 导入后的包名, 如 mgkafka
	Singleton string // 包内导出的单例变量名, 实现 Init([]byte)/Close()/Check() error
	Version   string // 初始版本号(执行 go mod tidy 后会自动修正)
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
// modulePath: go.mod 的 require 路径; pkgName: 导入后的包名(用于访问单例);
// singleton: 包内导出的单例变量名(实现 mgin.MginPlugin 接口: Init/Close/Check);
// version: 初始版本号, 执行 `go mod tidy` 后会自动修正为可用版本。
// 配置键以各插件实际约定为准(nats/kafka/mqtt 为 go.data.<name>, rabbit 为 go.rabbitmq)。
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

// 各组件的独立配置文件模板（基于 <prefix>-<env>.yml）。
var componentTemplates = map[string]string{
	"mysql": `go:
  data:
    mysql: root:123456@tcp(127.0.0.1:3306)/{{.ProjectName}}?charset=utf8mb4&parseTime=True&loc=Local
    mysql_debug: true
`,
	"postgres": `go:
  data:
    postgres:
      dsn: "host=127.0.0.1 user=postgres password=postgres dbname={{.ProjectName}} port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      debug: true
`,
	"mongodb": `go:
  data:
    mongodb:
      uri: mongodb://user:password@127.0.0.1:27017
      db: {{.ProjectName}}
      debug: true
`,
	"redis": `go:
  data:
    redis:
      host: 127.0.0.1
      port: 6379
      password: ""
      database: 0
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
      version: 2.8.1
`,
	"nats": `go:
  data:
    nats:
      multi: false
      uri: "nats://127.0.0.1:4222"
      user: ""
      password: ""
`,
	"mqtt": `go:
  data:
    mqtt:
      multi: false
      broker: "tcp://127.0.0.1:1883"
      clientId: {{.ProjectName}}
      username: ""
      password: ""
`,
	"rabbit": `go:
  rabbitmq:
    multi: false
    uri: "amqp://guest:guest@127.0.0.1:5672/"
    exchange: ""
`,
	"nacos": `go:
  nacos:
    server: 127.0.0.1
    port: 8848
    clusterName: DEFAULT
    group: DEFAULT_GROUP
    weight: 1
    lan: true
`,
	"consul": `go:
  consul:
    server: 127.0.0.1
    port: 8500
`,
	"etcd": `go:
  etcd:
    server: 127.0.0.1:2379
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

// 以下为工程内各源码文件的模板。

const tmplMain = `package main

import (
	"github.com/maczh/mgin"
	"{{.Module}}/router"
)

// @title {{.ProjectName}} API 文档
// @version 1.0.0
// @description 基于 MGin 微服务框架自动生成的服务接口
// @termsOfService http://swagger.io/terms/
// @contact.name {{.ProjectName}}
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @BasePath /api/v1
func main() {
	// 参数: 配置文件路径, 应用名, 版本号(由 Makefile 通过 -ldflags 注入 main.Version), 是否启用国际化(xlang)
	app := mgin.NewApp("conf/application.yml", "{{.ProjectName}}", Version, {{.I18n}})
	if app == nil {
		return
	}
{{if .HasMQ}}	// 注册消息队列插件(见 plugins.go)
	registerMQPlugins(app)
{{end}}	router.RegisterRoutes(app)
	app.Run()
}
`

// tmplPlugins 生成插件注册文件 plugins.go。
// mgin 通过 app.MGin.Use(name, init, close, check) 挂载外部插件:
// name 必须出现在 go.config.used 中, 框架据此从 go.config.prefix.<name> 指定的
// 配置文件读取数据并调用 Init; Close 在进程退出时调用; check 传 nil 表示跳过健康检查。
const tmplPlugins = `package main

import (
	"github.com/maczh/mgin"{{range .MQPlugins}}
	"{{.Path}}"{{end}}
)

// registerMQPlugins 注册消息队列插件。
// 注意: mgin 框架自带 sarama 版 kafka 客户端, 当 go.config.used 含 kafka 时,
// 框架会先初始化内置客户端, 再初始化本文件中的 mgkafka 插件。
// 若仅需使用 mgkafka 插件, 可从 go.config.used 中去掉 kafka 并同步删除 conf/kafka-<env>.yml。
func registerMQPlugins(app *mgin.App) { {{- range .MQPlugins}}
	app.MGin.UsePlugin("{{.Name}}", {{.Pkg}}.{{.Singleton}})
{{- end}}
}
`

const tmplRouter = `package router

import (
	"github.com/maczh/mgin"
	"{{.Module}}/controller"
{{if .JWT}}	"github.com/maczh/mgin/pkg/middleware/jwt"
{{end}}{{if .Casbin}}	"github.com/maczh/mgin/pkg/middleware/casbin"
{{end}})

// RegisterRoutes 注册所有业务路由与全局中间件
func RegisterRoutes(app *mgin.App) {
{{if .JWT}}	// JWT 鉴权中间件(白名单: /swagger/ /docs/ 及 sys 文档路径已自动放行)
	app.Router.Use(jwt.JwtAuthorize())
{{end}}{{if .Casbin}}	// Casbin 接口级 RBAC 鉴权(需 go.casbin.enabled=true)
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

const tmplController = `package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/pkg/models"
	"{{.Module}}/service"
)

var productService = &service.ProductService{}

// ListProducts 查询商品列表
// @Summary 查询商品列表
// @Tags 商品
// @Produce json
// @Success 200 {object} models.Result
// @Router /products [get]
func ListProducts(c *gin.Context) {
	list, err := productService.List()
	if err != nil {
		c.JSON(200, models.Error(500, err.Error()))
		return
	}
	c.JSON(200, models.Success(list))
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
		c.JSON(200, models.Error(400, "参数错误"))
		return
	}
	product, err := productService.Get(id)
	if err != nil {
		c.JSON(200, models.Error(500, err.Error()))
		return
	}
	if product == nil {
		c.JSON(200, models.Error(-1, "商品不存在"))
		return
	}
	c.JSON(200, models.Success(product))
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

const tmplDao = `package dao

import (
	"github.com/maczh/mgin/pkg/db/dao"
	"{{.Module}}/model"
)

// ProductDao 商品数据访问, 基于 mgin 泛型 DAO ({{.DataLayer}})
var ProductDao = &dao.{{.DaoType}}[model.Product]{}
`

const tmplServiceDB = `package service

import (
	"{{.Module}}/dao"
	"{{.Module}}/model"
)

// ProductService 商品业务层
type ProductService struct{}

// List 查询商品列表
func (s *ProductService) List() ([]model.Product, error) {
	return dao.ProductDao.All(model.Product{})
}

// Get 根据 ID 查询商品
func (s *ProductService) Get(id int64) (*model.Product, error) {
	return dao.ProductDao.One(model.Product{Id: id})
}
`

const tmplServiceMemory = `package service

import (
	"{{.Module}}/model"
)

// ProductService 商品业务层
// 示例采用内存数据；接入数据库时, 将本文件替换为基于 {{.Module}}/dao 的实现即可。
type ProductService struct{}

var mockProducts = []model.Product{
	{Id: 1, Name: "示例商品A", Price: 99.9, Stock: 100},
	{Id: 2, Name: "示例商品B", Price: 199.9, Stock: 50},
}

// List 查询商品列表
func (s *ProductService) List() ([]model.Product, error) {
	return mockProducts, nil
}

// Get 根据 ID 查询商品
func (s *ProductService) Get(id int64) (*model.Product, error) {
	for i := range mockProducts {
		if mockProducts[i].Id == id {
			return &mockProducts[i], nil
		}
	}
	return nil, nil
}
`

const tmplGoMod = `module {{.Module}}

go 1.25

require (
	github.com/gin-gonic/gin v1.11.0
	github.com/maczh/mgin {{.MginVersion}}{{range .MQPlugins}}
	{{.Path}} {{.Version}}{{end}}
)

// mgin 版本由脚手架自动获取最新发布版(离线时回退到默认版本);
// 消息队列插件(如启用)版本为已知初始值, 执行 ` + "`go mod tidy`" + ` 后会自动修正为可用版本(需联网)
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
// 其中的 {{.ProjectName}} 占位符由 scaffold 通过 strings.ReplaceAll 替换, 不走 text/template,
// 以避免其中的 shell 变量 $() / ${} 被模板引擎误解析。
const tmplMakefile = "# Makefile for {{.ProjectName}}\n" +
	"BINARY={{.ProjectName}}\n" +
	"VERSION=$(shell git tag 2>/dev/null | tail -n 1)\n" +
	"BUILD_TIME_SHORT=$(shell date '+%Y%m%d%H%M%S')\n" +
	"BUILD_TIME=$(shell date '+%Y-%m-%d %H:%M:%S')\n" +
	"GIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)\n" +
	"LDFLAGS=-ldflags \"-X main.Version=${VERSION}-build${BUILD_TIME_SHORT} -X 'main.BuildTime=${BUILD_TIME}' -X main.GitHash=${GIT_HASH}\"\n" +
	"\n" +
	".PHONY: build\n" +
	"build:\n" +
	"\tGOPROXY=https://goproxy.cn go mod tidy\n" +
	"\tgo build ${LDFLAGS} -o ${BINARY}\n" +
	"\n" +
	".PHONY: linux\n" +
	"linux:\n" +
	"\tGOPROXY=https://goproxy.cn go mod tidy\n" +
	"\tCGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY}\n" +
	"\tupx ${BINARY}\n" +
	"\n" +
	".PHONY: run\n" +
	"run:\n" +
	"\tgo run ${LDFLAGS} .\n"

// buildComponentFileName 返回组件的独立配置文件名（仅用于生成提示，无副作用）。
func buildComponentFileName(prefix, env string) string {
	return prefix + "-" + env + ".yml"
}
