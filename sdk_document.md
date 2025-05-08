# MGin 微服务框架 SDK 详细说明文档

## 一、项目概述
MGin 是一个用于快速创建基于 RESTful 的微服务程序的框架，集成了多种功能，包括 Web 服务框架、统一配置中心、服务发现与注册中心、内置数据库连接、缓存机制等。

## 二、包说明

### 1. `mgin` 包
此包为框架的核心包，负责初始化应用和启动服务。
- **主要结构体**：
    - `App`：代表一个 MGin 应用实例，包含应用名称、版本、配置文件路径、是否启用国际化、路由引擎和框架核心实例等信息。
- **主要函数**：
    - `NewApp(configFile, appName, version string, xlang bool) *App`：创建一个新的 MGin App 实例。会处理配置文件路径，初始化配置，设置定时任务，设置 Gin 模式，初始化国际化（如果启用），并初始化基础路由。
    - `(app *App) baseRouter()`：初始化路由并添加中间件，如跟踪日志、接口日志、跨域处理、国际化支持和全局异常处理等。
    - `(app *App) Run()`：启动 HTTP 和 HTTPS 服务器，并监听系统信号以实现优雅关闭。

**使用示例**：
```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin"
)

func main() {
    app := mgin.NewApp("", "测试mgin项目", "1.0.0", false)
    app.Router.GET("/test", func(c *gin.Context) {
        c.JSON(200, map[string]string{"msg": "hello world"})
    })
    app.Run()
}
```

### 2. `mgin/middleware` 包
`mgin/middleware` 包包含多个子包，每个子包都提供了不同的中间件功能，用于增强应用的功能和安全性。以下是对该包下各个子包的详细介绍：

#### 2.1. `mgin/middleware/trace` 包
此包用于处理请求跟踪信息，为每个请求添加唯一的请求 ID，并记录客户端 IP 和用户代理信息。
- **主要结构体和函数**：
    - `PutRequestId(c *gin.Context)`：为请求添加请求 ID，处理客户端 IP 和用户代理信息，并将这些信息缓存起来。
    - `GetRequestId() string`：获取当前请求的请求 ID。
    - `GetClientIp() string`：获取客户端 IP。
    - `GetUserAgent() string`：获取用户代理信息。
    - `GetHeader(header string) string`：获取指定头部信息。
    - `SetHeader(key, value string)`：设置头部信息。
    - `GetHeaders() map[string]string`：获取所有头部信息。
    - `CopyPreHeaderToCurRoutine(preRoutineId uint64)`：从其他协程克隆头部信息到当前协程的缓存。
    - `TraceId() gin.HandlerFunc`：作为中间件使用，调用 `PutRequestId` 为请求添加跟踪信息。
    - `Headers() gin.HandlerFunc`：实际上调用 `TraceId` 函数，作用与 `TraceId` 相同。

**使用示例**：
```go
app.Router.Use(trace.TraceId())
```

#### 2.2. `mgin/middleware/postlog` 包
用于记录接口请求和响应日志，可将日志存储到数据库（如 MongoDB 或 Elasticsearch）或发送到 Kafka。
- **主要结构体和函数**：
    - `bodyLogWriter`：用于捕获响应体内容。
    - `mongo[E any]`：用于操作 MongoDB 存储日志。
    - `RequestLogger() gin.HandlerFunc`：记录请求和响应信息，并将日志发送到 Kafka 或存储到数据库。
    - `handleAccessChannel()`：处理日志通道中的日志信息。

**使用示例**：
```go
app.Router.Use(postlog.RequestLogger())
```

#### 2.3. `mgin/middleware/cors` 包
用于处理跨域请求，为应用添加跨域支持。
- **主要函数**：
    - `Cors(headers ...string) gin.HandlerFunc`：设置跨域请求的响应头，允许跨域访问。可以传入额外的允许头部信息。

**使用示例**：
```go
app.Router.Use(cors.Cors())
```

#### 2.4. `mgin/middleware/xlang` 包
用于支持国际化，根据请求头中的 `X-Lang` 参数设置当前请求的语言。
- **主要函数**：
    - `RequestLanguage() gin.HandlerFunc`：从请求头中获取 `X-Lang` 参数，设置当前请求的语言并缓存起来。
    - `GetCurrentLanguage() string`：获取当前请求的语言。

**使用示例**：
```go
if app.i18n {
    app.Router.Use(xlang.RequestLanguage())
}
```

#### 2.5. `mgin/middleware/jwt` 包
提供 JWT 认证中间件，用于验证请求头中的 JWT 令牌。
- **主要函数**：
    - `JwtAuthorize() gin.HandlerFunc`：从请求头中获取 JWT 令牌，解析并验证令牌的有效性。如果令牌无效，则返回 `401 Unauthorized` 状态码。

**使用示例**：
```go
app.Router.Use(jwt.JwtAuthorize())
```
### 3. `mgin/db` 包
`mgin/db` 包是 MGin 微服务框架中用于处理数据库连接和操作的核心包，它提供了对多种数据库和消息队列的支持，包括 MySQL、MongoDB、Redis、Elasticsearch、Kafka 和 SQLite 等。以下是对该包的详细介绍：

#### 3.1. 主要功能概述
- **自动连接数据库**：支持多种数据库和消息队列的自动连接，通过配置文件即可完成数据库的初始化。
- **多库连接支持**：MySQL、MongoDB 和 Redis 支持多库连接，可通过配置文件指定多个数据库。
- **连接池管理**：支持数据库连接池的配置，可根据需要调整连接池的大小、超时时间等参数。
- **插件化扩展**：支持通过插件自动加载外部数据库、消息队列模块，方便扩展。

#### 3.2. 主要结构体和函数

##### 3.2.1 `db.go` 文件
该文件定义了各个数据库和消息队列的客户端实例，通过这些实例可以访问相应的数据库和消息队列。
```go
var Mysql = &mysql.MysqlClient{}
var Mongo = &mongo.Mongodb{}
var Redis = &redis.RedisClient{}
var ElasticSearch = &es.ElasticSearch{}
var Kafka = &kafka.Kafka{}
var Sqlite = &sqlite.Sqlite{}
```

##### 3.2.2 `mysql` 子包
- **`MysqlClient` 结构体**：表示 MySQL 客户端，包含配置信息和数据库连接。
- **`Init` 函数**：根据配置文件初始化 MySQL 连接，支持单库和多库连接。
- **`Check` 函数**：检查 MySQL 连接是否正常。

##### 3.2.3 `mongo` 子包
- **`Mongodb` 结构体**：表示 MongoDB 客户端，包含配置信息和数据库连接。
- **`Init` 函数**：根据配置文件初始化 MongoDB 连接，支持单库和多库连接。
- **`Check` 函数**：检查 MongoDB 连接是否正常。
- **`GetConnection` 函数**：获取 MongoDB 数据库连接。

##### 3.2.4 `redis` 子包
- **`RedisClient` 结构体**：表示 Redis 客户端，包含配置信息和数据库连接。
- **`Init` 函数**：根据配置文件初始化 Redis 连接，支持单库、多库、集群和哨兵模式。
- **`Check` 函数**：检查 Redis 连接是否正常。

##### 3.2.5 `es` 子包
- **`ElasticSearch` 结构体**：表示 Elasticsearch 客户端，包含配置信息和数据库连接。
- **`Init` 函数**：根据配置文件初始化 Elasticsearch 连接。
- **`Check` 函数**：检查 Elasticsearch 连接是否正常。

##### 3.2.6 `kafka` 子包
- **`Kafka` 结构体**：表示 Kafka 客户端，包含配置信息和数据库连接。
- **`Init` 函数**：根据配置文件初始化 Kafka 连接。
- **`Send` 函数**：向 Kafka 主题发送消息。
- **`MessageListener` 函数**：侦听 Kafka 主题消息并处理。

##### 3.2.7 `sqlite` 子包
- **`Sqlite` 结构体**：表示 SQLite 客户端，包含配置信息和数据库连接。
- **`Init` 函数**：根据配置文件初始化 SQLite 连接。

#### 3.3. 使用方法

##### 3.3.1 初始化数据库连接
在项目启动时，调用 `mgin.Init` 函数初始化数据库连接。

##### 3.3.2 数据库操作
通过 `db` 包中定义的客户端实例进行数据库操作。
```go
// MySQL 插入数据
dao := &dao.MySQLDao[MyEntity]{}
err := dao.Create(&myEntity)
if err != nil {
    // 处理错误
}

// MongoDB 插入数据
mgoDao := dao.MgoDao[MyEntity]{
    CollectionName: "my_collection",
}
err = mgoDao.Insert(&myEntity)
if err != nil {
    // 处理错误
}
```

#### 3.4. 配置文件示例

##### 3.4.1 MySQL 配置
```yaml
go:
  data:
    mysql: user:pwd@tcp(xxx.xxx.xxx.xxx:3306)/dbname?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true   #打开调试模式
    mysql_pool:     #连接池设置,若无此项则使用单一长连接
      max: 200      #实际最大连接数
      total: 1000   #最大并发数,不填默认为最大连接数5倍
      timeout: 30   #空闲连接超时，秒，默认60秒
      life: 5       #连接生命周期，分钟，默认60分钟
```

##### 3.4.2 MongoDB 配置
```yaml
go:
  data:
    mongodb:
      uri: mongodb://user:pwd@xxx.xxx.xxx.xxx:port/dbname #当使用复制集时 mongodb://user:pwd@192.168..3.5:27017,192.168.3.6:27017/dbname?replicaSet=replsetname
      db: dbname
      debug: true   #打开调试模式
    mongo_pool:     #连接池设置,若无此项则使用单一长连接
      max: 20       #最大连接数
```



### 4.mgin/config包

`mgin/config` 包主要负责配置文件的读取和管理，支持从不同的配置中心（如 Nacos、Consul、Etcd 等）或本地文件加载配置。以下是该包中所有结构体和方法的详细说明及调用范例。

#### 4.1 结构体说明

##### 4.1.1. `config` 结构体
该结构体包含了应用的所有配置信息，包括应用信息、配置服务器信息、日志信息、JWT 密钥等。
```go
type config struct {
    Cnf       *koanf.Koanf
    App       app       `json:"app" bson:"app"`
    Config    appConfig `json:"config" bson:"config"`
    Log       appLog    `json:"log" bson:"log"`
    Logger    appLogger `json:"logger" bson:"logger"`
    Discovery discovery `json:"discovery" bson:"discovery"`
    Jwt       jwtConfig `json:"jwt" bson:"jwt"`
}
```
**字段说明**：
- `Cnf`：`koanf.Koanf` 类型的指针，用于管理配置数据。
- `App`：`app` 结构体，包含应用的基本信息。
- `Config`：`appConfig` 结构体，包含配置服务器的信息。
- `Log`：`appLog` 结构体，包含接口访问日志和微服务调用请求日志的配置信息。
- `Logger`：`appLogger` 结构体，包含日志输出的配置信息。
- `Discovery`：`discovery` 结构体，包含微服务的服务发现与注册中心的配置信息。
- `Jwt`：`jwtConfig` 结构体，包含 JWT 密钥的配置信息。

##### 4.1.2. `app` 结构体
包含应用的基本信息，如应用名称、项目名称、端口号等。
```go
type app struct {
    Name    string `json:"name" bson:"name"`
    Project string `json:"project" bson:"project"`
    Port    int    `json:"port" bson:"port"`
    PortSSL int    `json:"portSSL" bson:"portSSL"`
    Cert    string `json:"cert" bson:"cert"`
    Key     string `json:"key" bson:"key"`
    Debug   bool   `json:"debug" bson:"debug"`
    IpAddr  string `json:"ipAddr" bson:"ipAddr"`
}
```

##### 4.1.3. `appConfig` 结构体
包含配置服务器的信息，如服务器地址、类型、环境、使用的配置等。
```go
type appConfig struct {
    Server string `json:"server" bson:"server"`
    Type   string `json:"type" bson:"type"`
    Path   string `json:"path" bson:"path"`
    Env    string `json:"env" bson:"env"`
    Used   string `json:"used" bson:"used"`
    Prefix struct {
        Mysql         string `json:"mysql" bson:"mysql"`
        Mongodb       string `json:"mongodb" bson:"mongodb"`
        Redis         string `json:"redis" bson:"redis"`
        Nacos         string `json:"nacos" bson:"nacos"`
        Elasticsearch string `json:"elasticsearch" bson:"elasticsearch"`
        Kafka         string `json:"kafka" bson:"kafka"`
        Sqlite        string `json:"sqlite" bson:"sqlite"`
        Etcd          string `json:"etcd" bson:"etcd"`
        Consul        string `json:"consul" bson:"consul"`
    } `json:"prefix" bson:"prefix"`
}
```

##### 4.1.4. `appLogger` 结构体
包含日志输出的配置信息，如日志级别、输出位置、文件路径等。
```go
type appLogger struct {
    Level string `json:"level" bson:"level"`
    Out   string `json:"out" bson:"out"`
    File  string `json:"file" bson:"file"`
}
```

##### 4.1.5. `appLog` 结构体
包含接口访问日志和微服务调用请求日志的配置信息，如日志库、表名、Kafka 配置等。
```go
type appLog struct {
    RequestTableName string `json:"request" bson:"request"`
    CallTableName    string `json:"call" bson:"call"`
    LogDb            string `json:"logDb" bson:"logDb"`
    DbName           string `json:"dbName" bson:"dbName"`
    Kafka            struct {
        Use   bool   `json:"use" bson:"use"`
        Topic string `json:"topic" bson:"topic"`
    } `json:"kafka" bson:"kafka"`
}
```

##### 4.1.6. `discovery` 结构体
包含微服务的服务发现与注册中心的配置信息，如注册中心类型、调用参数模式等。
```go
type discovery struct {
    Registry string `json:"registry" bson:"registry"`
    CallType string `json:"callType" bson:"callType"`
}
```

##### 4.1.7. `jwtConfig` 结构体
包含 JWT 密钥的配置信息。
```go
type jwtConfig struct {
    Secret string `json:"secret" bson:"secret"`
}
```

#### 4.2方法说明及调用范例

##### 4.2.1. `Init` 方法
用于初始化配置，从指定的配置文件中读取配置信息，并将其存储在 `config` 结构体中。
```go
func (c *config) Init(cf string)
```
**调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
}
```

##### 4.2.2. `GetConfigString` 方法
用于获取指定名称的字符串类型的配置信息。
```go
func (c *config) GetConfigString(name string) string
```
**调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
    appName := config.Config.GetConfigString("go.application.name")
    fmt.Println("应用名称:", appName)
}
```

##### 4.2.3. `GetConfigInt` 方法
用于获取指定名称的整数类型的配置信息。
```go
func (c *config) GetConfigInt(name string) int
```
**调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
    port := config.Config.GetConfigInt("go.application.port")
    fmt.Println("应用端口:", port)
}
```

##### 4.2.4. `GetConfigBool` 方法
用于获取指定名称的布尔类型的配置信息。
```go
func (c *config) GetConfigBool(name string) bool
```
**调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
    debug := config.Config.GetConfigBool("go.application.debug")
    fmt.Println("是否为调试模式:", debug)
}
```

##### 4.2.5. `Exists` 方法
用于检查指定名称的配置信息是否存在。
```go
func (c *config) Exists(name string) bool
```
**调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
    exists := config.Config.Exists("go.application.name")
    fmt.Println("配置项是否存在:", exists)
}
```

##### 4.2.6. `GetConfigUrl` 方法
根据配置中心类型和配置前缀生成配置文件的访问路径。
```go
func (c *config) GetConfigUrl(prefix string) string
```
**调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
    mysqlConfigUrl := config.Config.GetConfigUrl("mysql")
    fmt.Println("MySQL 配置文件访问路径:", mysqlConfigUrl)
}
```

##### 4.2.7. `GetConfigData` 方法
根据配置中心类型和配置前缀获取配置数据。
```go
func (c *config) GetConfigData(prefix string) []byte
```
**调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
)

func main() {
    config.Config.Init("application.yml")
    mysqlConfigData := config.Config.GetConfigData("mysql")
    fmt.Println("MySQL 配置数据:", string(mysqlConfigData))
}
```

`mgin/config` 包提供了一系列结构体和方法，用于方便地读取和管理应用的配置信息。通过这些结构体和方法，开发者可以轻松地从不同的配置中心或本地文件加载配置，并获取所需的配置数据。

#### 4.3 配置文件说明

##### 4.3.1 主配置文件（`模块名.yml`）

包含应用的基本配置、服务发现与注册中心配置、统一配置中心配置、JWT 密钥、日志配置等。

##### 4.3.2 数据库配置文件

- **MySQL**：`mysql-test.yml` 和 `mysql-multidb-test.yml` 分别用于单库和多库配置。
- **MongoDB**：`mongodb-test.yml` 和 `mongodb-multidb-test.yml` 分别用于单库和多库配置。
- **Redis**：`redis-test.yml` 和 `redis-multidb-test.yml` 分别用于单库和多库配置，支持集群模式和哨兵模式。
- **Nacos**：`nacos-test.yml` 用于 Nacos 配置。
- **Etcd**：`etcd-test.yml` 用于 Etcd 配置。
- **Consul**：`consul-test.yml` 用于 Consul 配置。
- **Elasticsearch**：`elasticsearch-test.yml` 用于 Elasticsearch 配置。
- **Kafka**：`Kafka-test.yml` 用于 Kafka 配置。

### 5. mgin/registry包

`mgin/registry` 包主要负责微服务的服务发现与注册功能，支持多种注册中心，如 Nacos、Etcd 和 Consul。下面将详细介绍该包中的结构体、字段和方法。

#### 5.1. 结构体和字段说明

##### 5.1.1 `RegistryClient` 接口
```go
type RegistryClient interface {
    Register(etcdConfigData []byte)
    GetServiceURL(servicename string, groupName ...string) (string, string)
    DeRegister()
}
```
- **字段说明**：
    - `Register(etcdConfigData []byte)`：向注册中心注册服务，接收配置数据作为参数。
    - `GetServiceURL(servicename string, groupName ...string) (string, string)`：从注册中心获取指定服务的 URL，可指定服务组名。
    - `DeRegister()`：从注册中心注销服务。

##### 5.1.2 `NewRegistry` 函数
- **字段说明**：
    - 根据配置文件中指定的注册中心类型（`config.Config.Discovery.Registry`）创建对应的注册中心客户端实例。

#### 5.2. 方法详细说明及调用范例

##### 5.2.1 `Register` 方法
该方法用于向注册中心注册服务，不同的注册中心实现方式不同。

**Nacos 注册示例**：

```go
package main

import (
    "github.com/maczh/mgin/config"
    "github.com/maczh/mgin/registry"
)

func main() {
    config.Config.Init("application.yml")
    registry.Registry = registry.NewRegistry()
    nacosConfigData := config.Config.GetConfigData(config.Config.Config.Prefix.Nacos)
    registry.Registry.Register(nacosConfigData)
}
```
**Etcd 注册示例**：

```go
package main

import (
    "github.com/maczh/mgin/config"
    "github.com/maczh/mgin/registry"
)

func main() {
    config.Config.Init("application.yml")
    registry.Registry = registry.NewRegistry()
    etcdConfigData := config.Config.GetConfigData(config.Config.Config.Prefix.Etcd)
    registry.Registry.Register(etcdConfigData)
}
```
**Consul 注册示例**：
```go
package main

import (
    "github.com/maczh/mgin/config"
    "github.com/maczh/mgin/registry"
)

func main() {
    config.Config.Init("application.yml")
    registry.Registry = registry.NewRegistry()
    consulConfigData := config.Config.GetConfigData(config.Config.Config.Prefix.Consul)
    registry.Registry.Register(consulConfigData)
}
```

##### 5.2.2 `GetServiceURL` 方法
该方法用于从注册中心获取指定服务的 URL。
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/config"
    "github.com/maczh/mgin/registry"
)

func main() {
    config.Config.Init("application.yml")
    registry.Registry = registry.NewRegistry()
    serviceName := "my-service"
    url, _ := registry.Registry.GetServiceURL(serviceName)
    fmt.Println("服务 URL:", url)
}
```

##### 5.2.3 `DeRegister` 方法
该方法用于从注册中心注销服务。
```go
package main

import (
    "github.com/maczh/mgin/config"
    "github.com/maczh/mgin/registry"
)

func main() {
    config.Config.Init("application.yml")
    registry.Registry = registry.NewRegistry()
    registry.Registry.DeRegister()
}
```

#### 5.3. 配置说明
```yaml
go:
  discovery:                      
    registry: nacos                    #微服务的服务发现与注册中心类型 nacos,consul,etcd,默认是 nacos
    callType: json                     #微服务调用参数模式 x-form,json,restful 三种模式可选
  config:                               #统一配置服务器相关
    server: http://192.168.1.5:8848/    #配置服务器地址
    server_type: nacos                  #配置服务器类型 nacos,consul,springconfig,etcd,file; 
                                        #nacos地址 http://{go.config.server}/nacos/v1/cs/configs?group={go.application.project}&&dataId={go.config.prefix.nacos}-{go.config.env}.yml
                                        #etcd key /config/{go.application.project}/{go.config.prefix.etcd}-{go.config.env}.yml
                                        #consul key /{go.application.project}/{go.config.prefix.consul}-{go.config.env}.yml
    env: test                           #配置环境 一般常用test/prod/dev等，跟相应配置文件匹配
    used: nacos,mysql,mongodb,redis,kafka     #当前应用启用的配置
    prefix:                             #配置文件名前缀定义
      mysql: mysql                      #mysql对应的配置文件名前缀，如当前配置中对应的配置文件名为 mysql-test.yml
      mongodb: mongodb
      redis: redis
      nacos: nacos
      sqlite: mytest.db                 #SQLite本地文件名
      elasticsearch: elasticsearch
      kafka: kafka

```

#### 5.4. 自动注册

微服务在调用mgin.Init()方法时就根据配置中的注册中心及配置中心中的对应的注册中心配置文件自动读取并且进行自动注册，无需人工注册，在程序退出时将会自动从注册中心注销服务实例。

### 6. `mgin/client` 包
用于调用其他微服务。通过 `Options` 结构体配置请求参数，支持多种请求方法、协议和参数类型。

`mgin/client` 包主要负责微服务之间的调用，提供了统一的调用接口，支持多种协议和参数模式。以下是对该包中所有结构体、字段和方法的详细说明及调用范例。

#### 6.1. 结构体和字段说明

##### 6.1.1 `Options` 结构体
该结构体用于封装微服务调用的相关选项。
```go
type Options struct {
    Method   string                 `json:"method"`   // 接口方法 GET|POST|PUT|DELETE
    Protocol string                 `json:"protocol"` // 协议 x-form|json|restful
    Group    string                 `json:"group"`    // 应用分组，用于nacos中分组，不传为当前nacos分组及默认分组
    Header   any                    `json:"header"`   // 额外的头部参数
    Query    any                    `json:"query"`    // URL Query参数
    Data     any                    `json:"data"`     // x-form Postform参数
    Json     any                    `json:"json"`     // json或restful模式的body参数
    Path     map[string]string      `json:"path"`     // restful模式的路径参数
    Files    []grequests.FileUpload // 文件上传数据
    Retry    bool                   `json:"retry"`    // 是否重试
}
```
- **字段说明**：
    - `Method`：HTTP请求方法，如 `GET`、`POST`、`PUT`、`DELETE`。
    - `Protocol`：请求协议，支持 `x-form`、`json`、`restful`。
    - `Group`：应用分组，用于Nacos中的分组。
    - `Header`：额外的HTTP头部参数。
    - `Query`：URL查询参数。
    - `Data`：`x-form` 模式下的表单数据。
    - `Json`：`json` 或 `restful` 模式下的请求体数据。
    - `Path`：`restful` 模式下的路径参数。
    - `Files`：文件上传数据。
    - `Retry`：是否重试请求。

#### 6.2. 方法详细说明及调用范例

##### 6.2.1 `Call` 方法
该方法用于发起微服务调用。
```go
func Call(service, uri string, op *Options) (string, error)
```
- **参数说明**：
    - `service`：目标微服务的名称。
    - `uri`：目标微服务的接口路径。
    - `op`：调用选项，包含请求方法、协议、头部参数等。
- **返回值说明**：
    - `string`：响应结果的字符串表示。
    - `error`：调用过程中出现的错误。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/client"
    "github.com/maczh/mgin/models"
)

func main() {
    op := &client.Options{
        Method:   "GET",
        Protocol: "json",
        Query:    map[string]string{"key": "value"},
    }
    resp, err := client.Call("target-service", "/api/path", op)
    if err != nil {
        fmt.Println("调用失败:", err)
    } else {
        fmt.Println("调用成功:", resp)
    }
}
```

##### 6.2.2 `CallT` 方法
该方法是 `Call` 方法的泛型版本，用于发起微服务调用并将响应结果解析为指定类型。
```go
func CallT[T any](service, uri string, op *Options) models.Result[T]
```
- **参数说明**：
    - `service`：目标微服务的名称。
    - `uri`：目标微服务的接口路径。
    - `op`：调用选项，包含请求方法、协议、头部参数等。
- **返回值说明**：
    - `models.Result[T]`：泛型结果类型，包含响应数据和错误信息。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/client"
    "github.com/maczh/mgin/models"
)

type ResponseData struct {
    Message string `json:"message"`
}

func main() {
    op := &client.Options{
        Method:   "GET",
        Protocol: "json",
        Query:    map[string]string{"key": "value"},
    }
    result := client.CallT[ResponseData]("target-service", "/api/path", op)
    if result.Code != 0 {
        fmt.Println("调用失败:", result.Msg)
    } else {
        fmt.Println("调用成功:", result.Data.Message)
    }
}
```



### 7.mgin/cache包

`mgin/cache` 包提供了丰富的缓存功能，支持内存式缓存和本地持久化缓存。通过 `OnGetCache` 方法可以方便地创建不同类型的缓存实例，并使用 `ICache` 接口定义的方法进行缓存操作。

`mgin/cache` 包为 MGin 微服务框架提供了缓存功能，支持内存式缓存和本地持久化缓存（使用 `bitcask`）。以下是该包中所有结构体、字段和方法的详细说明及调用范例。

#### 7.1. 结构体和字段说明

##### 7.1.1 `Cache` 结构体
该结构体用于管理不同类型的缓存实例。
```go
type Cache struct {
    cache     sync.Map
    cacheType sync.Map
    db        sync.Map
}
```
- **字段说明**：
    - `cache`：存储内存式缓存实例。
    - `cacheType`：记录每个缓存的类型（`mem` 或 `disk`）。
    - `db`：存储本地持久化缓存实例。

##### 7.1.2 `MemCache` 结构体
用于实现内存式缓存。
```go
type MemCache struct {
    items sync.Map
    close chan struct{}
}
```
- **字段说明**：
    - `items`：存储缓存项。
    - `close`：用于关闭缓存的通道。

##### 7.1.3 `DiskCache` 结构体
用于实现本地持久化缓存。
```go
type DiskCache struct {
    db *bitcask.Bitcask
}
```
- **字段说明**：
    - `db`：`bitcask` 数据库实例。

#### 7.2. 方法详细说明及调用范例

##### 7.2.1 `OnGetCache` 方法
用于初始化一个缓存实例，可选择内存式或本地持久化缓存。
```go
func OnGetCache(cachename string, persistent ...bool) ICache
```
- **参数说明**：
    - `cachename`：缓存名称。
    - `persistent`：可选参数，`true` 表示使用本地持久化缓存，`false` 或不传入表示使用内存式缓存。
- **返回值说明**：
    - `ICache`：缓存实例。
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
)

func main() {
    // 使用内存式缓存
    memCache := cache.OnGetCache("my-mem-cache")

    // 使用本地持久化缓存
    diskCache := cache.OnGetCache("my-disk-cache", true)
}
```

##### 7.2.2 `OnDiskCache` 方法
用于初始化一个本地持久化缓存实例。
```go
func OnDiskCache(cachePath string) ICache
```
- **参数说明**：
    - `cachePath`：缓存文件路径。
- **返回值说明**：
    - `ICache`：本地持久化缓存实例。
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
)

func main() {
    diskCache := cache.OnDiskCache("my-cache.db")
}
```

##### 7.2.3 `OnMemCache` 方法
用于初始化一个内存式缓存实例。
```go
func OnMemCache(cachename string) ICache
```
- **参数说明**：
    - `cachename`：缓存名称。
- **返回值说明**：
    - `ICache`：内存式缓存实例。
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
}
```

##### 7.2.4 `CloseCache` 方法
用于关闭所有缓存实例。
```go
func CloseCache()
```
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    diskCache := cache.OnDiskCache("my-cache.db")

    // 关闭所有缓存
    cache.CloseCache()
}
```

#### 7.3. 缓存接口 `ICache` 方法说明

`ICache` 接口定义了缓存操作的基本方法，`MemCache` 和 `DiskCache` 都实现了该接口。

##### 7.3.1 `Add` 方法
用于向缓存中添加一个键值对，并设置过期时间。
```go
func (c *MemCache) Add(key any, value any, lifeSpan time.Duration)
func (d *DiskCache) Add(key any, value any, lifeSpan time.Duration)
```
- **参数说明**：
    - `key`：缓存键。
    - `value`：缓存值。
    - `lifeSpan`：过期时间，`0` 表示永不超时。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/cache"
    "time"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 5*time.Minute)

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key2", "value2", 10*time.Minute)
}
```

##### 7.3.2 `Value` 方法
用于获取缓存中指定键的值。
```go
func (c *MemCache) Value(key any) (any, bool)
func (d *DiskCache) Value(key any) (any, bool)
```
- **参数说明**：
    - `key`：缓存键。
- **返回值说明**：
    - `any`：缓存值。
    - `bool`：表示键是否存在。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 0)
    value, exists := memCache.Value("key1")
    if exists {
        fmt.Println("内存式缓存值:", value)
    }

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key2", "value2", 0)
    value, exists = diskCache.Value("key2")
    if exists {
        fmt.Println("本地持久化缓存值:", value)
    }
}
```

##### 7.3.3 `IsExist` 方法
用于判断缓存中指定键是否存在。
```go
func (c *MemCache) IsExist(key any) bool
func (d *DiskCache) IsExist(key any) bool
```
- **参数说明**：
    - `key`：缓存键。
- **返回值说明**：
    - `bool`：表示键是否存在。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 0)
    exists := memCache.IsExist("key1")
    fmt.Println("内存式缓存键是否存在:", exists)

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key2", "value2", 0)
    exists = diskCache.IsExist("key2")
    fmt.Println("本地持久化缓存键是否存在:", exists)
}
```

##### 7.3.4 `Clear` 方法
用于清空缓存中的所有数据。
```go
func (c *MemCache) Clear() bool
func (d *DiskCache) Clear() bool
```
- **返回值说明**：
    - `bool`：表示清空操作是否成功。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 0)
    success := memCache.Clear()
    fmt.Println("内存式缓存清空结果:", success)

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key2", "value2", 0)
    success = diskCache.Clear()
    fmt.Println("本地持久化缓存清空结果:", success)
}
```

##### 7.3.5 `Get` 方法
用于获取缓存中指定键的值，与 `Value` 方法功能相同。
```go
func (c *MemCache) Get(key any) (any, bool)
func (d *DiskCache) Get(key any) (any, bool)
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 0)
    value, exists := memCache.Get("key1")
    if exists {
        fmt.Println("内存式缓存值:", value)
    }

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key2", "value2", 0)
    value, exists = diskCache.Get("key2")
    if exists {
        fmt.Println("本地持久化缓存值:", value)
    }
}
```

##### 7.3.6 `Set` 方法
用于设置缓存中指定键的值，并设置过期时间。
```go
func (c *MemCache) Set(key any, value any, duration time.Duration)
func (d *DiskCache) Set(key any, value any, duration time.Duration)
```
- **参数说明**：
    - `key`：缓存键。
    - `value`：缓存值。
    - `duration`：过期时间，`0` 表示永不超时。
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
    "time"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Set("key1", "value1", 5*time.Minute)

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Set("key2", "value2", 10*time.Minute)
}
```

##### 7.3.7 `Range` 方法
用于遍历缓存中的所有键值对。
```go
func (c *MemCache) Range(f func(key, value any) bool)
func (d *DiskCache) Range(f func(key, value any) bool)
```
- **参数说明**：
    - `f`：遍历函数，返回 `false` 时停止遍历。
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 0)
    memCache.Add("key2", "value2", 0)
    memCache.Range(func(key, value any) bool {
        fmt.Println("内存式缓存键值对:", key, value)
        return true
    })

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key3", "value3", 0)
    diskCache.Add("key4", "value4", 0)
    diskCache.Range(func(key, value any) bool {
        fmt.Println("本地持久化缓存键值对:", key, value)
        return true
    })
}
```

##### 7.3.8 `Delete` 方法
用于删除缓存中指定键的值。
```go
func (c *MemCache) Delete(key any)
func (d *DiskCache) Delete(key any)
```
- **参数说明**：
    - `key`：缓存键。
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Add("key1", "value1", 0)
    memCache.Delete("key1")

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Add("key2", "value2", 0)
    diskCache.Delete("key2")
}
```

##### 7.3.9 `Close` 方法
用于关闭缓存实例。
```go
func (c *MemCache) Close()
func (d *DiskCache) Close()
```
- **调用范例**：
```go
package main

import (
    "github.com/maczh/mgin/cache"
)

func main() {
    memCache := cache.OnMemCache("my-mem-cache")
    memCache.Close()

    diskCache := cache.OnDiskCache("my-cache.db")
    diskCache.Close()
}
```

`mgin/cache` 包提供了丰富的缓存功能，支持内存式缓存和本地持久化缓存。通过 `OnGetCache` 方法可以方便地创建不同类型的缓存实例，并使用 `ICache` 接口定义的方法进行缓存操作。

### 8. Mgin/models包

`mgin/models` 包包含了一些在 MGin 微服务框架中常用的数据模型和通用返回结果类。以下是该包中所有结构体、字段和方法的详细说明及调用范例。

#### 8.1. `Result` 结构体
该结构体是通用返回结果类，使用了泛型 `T` 来表示返回的数据类型。
```go
type Result[T any] struct {
    Status int         `json:"status" bson:"status"`
    Msg    string      `json:"msg" bson:"msg"`
    Data   T           `json:"data,omitempty" bson:"data,omitempty"`
    Page   *ResultPage `json:"page,omitempty" bson:"page,omitempty"`
}
```
- **字段说明**：
    - `Status`：返回状态码，通常 `1` 表示成功，其他值表示失败。
    - `Msg`：返回消息，用于描述操作结果。
    - `Data`：返回的数据，类型为泛型 `T`。
    - `Page`：分页信息，类型为 `*ResultPage`，可选字段。

#### 8.2. 相关方法及调用范例

##### 8.2.1 `Success` 方法
用于创建一个成功的返回结果，不包含分页信息。
```go
func Success[T any](data T) Result[T]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    data := "hello world"
    result := models.Success(data)
    fmt.Println(result)
}
```

##### 8.2.2 `SuccessWithMsg` 方法
用于创建一个成功的返回结果，包含自定义消息，不包含分页信息。
```go
func SuccessWithMsg[T any](msg string, data T) Result[T]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    data := "hello world"
    msg := "操作成功"
    result := models.SuccessWithMsg(msg, data)
    fmt.Println(result)
}
```

##### 8.2.3 `SuccessPage` 方法
用于创建一个成功的返回结果，包含分页信息。
```go
func SuccessPage[T any](data T, page *ResultPage) Result[T]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    data := "hello world"
    page := &models.ResultPage{
        Count: 10,
        Index: 1,
        Size:  10,
        Total: 100,
    }
    result := models.SuccessPage(data, page)
    fmt.Println(result)
}
```

##### 8.2.4 `SuccessWithPage` 方法
用于创建一个成功的返回结果，包含分页信息，通过参数指定分页数据。
```go
func SuccessWithPage[T any](data T, count, index, size, total int) Result[T]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    data := "hello world"
    count := 10
    index := 1
    size := 10
    total := 100
    result := models.SuccessWithPage(data, count, index, size, total)
    fmt.Println(result)
}
```

##### 8.2.5 `Error` 方法
用于创建一个失败的返回结果，返回类型为 `Result[any]`。
```go
func Error(s int, m string) Result[any]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    status := 500
    msg := "服务器内部错误"
    result := models.Error(status, msg)
    fmt.Println(result)
}
```

##### 8.2.6 `ErrorT` 方法
用于创建一个失败的返回结果，使用泛型 `T`。
```go
func ErrorT[T any](s int, m string) Result[T]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    status := 500
    msg := "服务器内部错误"
    result := models.ErrorT[string](status, msg)
    fmt.Println(result)
}
```

##### 8.2.7 `ToAny` 方法
用于将 `Result[T]` 类型转换为 `Result[any]` 类型。
```go
func (r Result[T]) ToAny() Result[any]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    data := "hello world"
    result := models.Success(data)
    anyResult := result.ToAny()
    fmt.Println(anyResult)
}
```

##### 8.2.8 `ToAny` 函数
用于将 `Result[any]` 类型转换为 `Result[T]` 类型，需要确保 `Data` 的类型一致，否则断言可能会 `panic`。
```go
func ToAny[T any](result Result[any]) Result[T]
```
- **调用范例**：
```go
package main

import (
    "fmt"
    "github.com/maczh/mgin/models"
)

func main() {
    data := "hello world"
    anyResult := models.Success(data).ToAny()
    result := models.ToAny[string](anyResult)
    fmt.Println(result)
}
```

#### 8.2. `ResultPage` 结构体
该结构体用于表示分页信息。
```go
type ResultPage struct {
    Count int `json:"count"` //总页数
    Index int `json:"index"` //页号
    Size  int `json:"size"`  //分页大小
    Total int `json:"total"` //总记录数
}
```
- **字段说明**：
    - `Count`：总页数。
    - `Index`：当前页号。
    - `Size`：每页的记录数。
    - `Total`：总记录数。

#### 8.3. `PostLog` 结构体
该结构体用于记录接口访问日志。
```go
type PostLog struct {
    //ID            bson.ObjectId     `bson:"_id"`
    Time          string            `json:"time" bson:"time"`
    RequestId     string            `json:"requestId" bson:"requestId"`
    ResponseTime  string            `json:"responseTime" bson:"responseTime"`
    TTL           int               `json:"ttl" bson:"ttl"`
    AppName       string            `json:"appName" bson:"appName"`
    Apiname       string            `json:"apiName" bson:"apiName"`
    Method        string            `json:"method" bson:"method"`
    ContentType   string            `json:"contentType" bson:"contentType"`
    Uri           string            `json:"uri" bson:"uri"`
    ClientIP      string            `json:"clientIP" bson:"clientIP"`
    RequestHeader map[string]string `json:"requestHeader" bson:"requestHeader"`
    RequestParam  any               `json:"requestParam" bson:"requestParam"`
    RequestBody   any               `json:"requestBody" bson:"requestBody"`
    ResponseStr   string            `json:"responseStr" bson:"responseStr"`
    ResponseMap   any               `json:"responseMap" bson:"responseMap"`
}
```
- **字段说明**：
    - `Time`：请求时间。
    - `RequestId`：请求 ID。
    - `ResponseTime`：响应时间。
    - `TTL`：响应耗时（毫秒）。
    - `AppName`：应用名称。
    - `Apiname`：接口名称。
    - `Method`：请求方法。
    - `ContentType`：请求内容类型。
    - `Uri`：请求 URI。
    - `ClientIP`：客户端 IP 地址。
    - `RequestHeader`：请求头信息。
    - `RequestParam`：请求参数。
    - `RequestBody`：请求体。
    - `ResponseStr`：响应字符串。
    - `ResponseMap`：响应数据。

#### 8.4. `Timestamp` 结构体
该结构体用于处理时间戳，在 `mgin/models/timestamp.go` 文件中有相关方法。
```go
// 以下是部分相关方法
func (t Timestamp) Value() (driver.Value, error) {
    return time.Time(t), nil
}

func (t *Timestamp) Scan(v any) error {
    value, ok := v.(time.Time)
    if ok {
        *t = Timestamp(value)
        return nil
    }
    return fmt.Errorf("can not convert %v to timestamp", v)
}

func (t Timestamp) Time() time.Time {
    return time.Time(t)
}
```
- **方法说明**：
    - `Value` 方法：将 `Timestamp` 类型转换为 `driver.Value` 类型，用于数据库操作。
    - `Scan` 方法：将数据库查询结果转换为 `Timestamp` 类型。
    - `Time` 方法：将 `Timestamp` 类型转换为 `time.Time` 类型。

### 9. mgin/utils包

`mgin/utils` 包包含了多个实用工具函数，以下是这些函数的功能、入参和出参介绍：

#### 9.1 app.go
##### `AppName()`
- **功能**：获取当前应用程序的名称。
- **入参**：无
- **出参**：
    - `string`：当前应用程序的名称

##### `AppDir()`
- **功能**：获取当前应用程序的目录。
- **入参**：无
- **出参**：
    - `string`：当前应用程序的目录

#### 9.2 fileutil.go
##### `SelfPath()`
- **功能**：获取编译后的可执行文件的绝对路径。
- **入参**：无
- **出参**：
    - `string`：可执行文件的绝对路径

##### `SelfDir()`
- **功能**：获取编译后的可执行文件的目录。
- **入参**：无
- **出参**：
    - `string`：可执行文件的目录

##### `Basename(file string)`
- **功能**：获取文件路径的基本名称。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `string`：文件路径的基本名称

##### `Dir(file string)`
- **功能**：获取文件路径的目录名称。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `string`：文件路径的目录名称

##### `InsureDir(path string)`
- **功能**：确保指定目录存在，如果不存在则创建。
- **入参**：
    - `path string`：目录路径
- **出参**：
    - `error`：如果创建目录时出错，返回错误信息

##### `Ext(file string)`
- **功能**：获取文件路径的扩展名。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `string`：文件路径的扩展名

##### `Rename(file string, to string)`
- **功能**：重命名文件。
- **入参**：
    - `file string`：原文件路径
    - `to string`：新文件路径
- **出参**：
    - `error`：如果重命名文件时出错，返回错误信息

##### `Unlink(file string)`
- **功能**：删除文件。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `error`：如果删除文件时出错，返回错误信息

##### `IsFile(filePath string)`
- **功能**：检查指定路径是否为文件。
- **入参**：
    - `filePath string`：文件路径
- **出参**：
    - `bool`：如果是文件，返回 `true`；否则返回 `false`

##### `IsExist(path string)`
- **功能**：检查指定文件或目录是否存在。
- **入参**：
    - `path string`：文件或目录路径
- **出参**：
    - `bool`：如果存在，返回 `true`；否则返回 `false`

##### `SearchFile(filename string, paths ...string)`
- **功能**：在指定路径中搜索文件。
- **入参**：
    - `filename string`：文件名
    - `paths ...string`：搜索路径列表
- **出参**：
    - `fullPath string`：文件的完整路径
    - `err error`：如果文件未找到，返回错误信息

##### `RealPath(file string)`
- **功能**：获取文件的绝对路径。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `string`：文件的绝对路径
    - `error`：如果获取绝对路径时出错，返回错误信息

##### `FileMTime(file string)`
- **功能**：获取文件的修改时间。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `int64`：文件的修改时间（Unix时间戳）
    - `error`：如果获取修改时间时出错，返回错误信息

##### `FileSize(file string)`
- **功能**：获取文件的大小（字节数）。
- **入参**：
    - `file string`：文件路径
- **出参**：
    - `int64`：文件的大小（字节数）
    - `error`：如果获取文件大小时出错，返回错误信息

##### `DirsUnder(dirPath string)`
- **功能**：列出指定目录下的所有子目录。
- **入参**：
    - `dirPath string`：目录路径
- **出参**：
    - `[]string`：子目录名称列表
    - `error`：如果列出子目录时出错，返回错误信息

##### `FilesUnder(dirPath string)`
- **功能**：列出指定目录下的所有文件。
- **入参**：
    - `dirPath string`：目录路径
- **出参**：
    - `[]string`：文件名称列表
    - `error`：如果列出文件时出错，返回错误信息

##### `ReadFileToBytes(filePath string)`
- **功能**：从文件中读取字节数据。
- **入参**：
    - `filePath string`：文件路径
- **出参**：
    - `[]byte`：文件的字节数据
    - `error`：如果读取文件时出错，返回错误信息

##### `ReadFileToString(filePath string)`
- **功能**：从文件中读取字符串数据。
- **入参**：
    - `filePath string`：文件路径
- **出参**：
    - `string`：文件的字符串数据
    - `error`：如果读取文件时出错，返回错误信息

##### `WriteBytesToFile(filePath string, b []byte)`
- **功能**：将字节数据写入文件。
- **入参**：
    - `filePath string`：文件路径
    - `b []byte`：字节数据
- **出参**：
    - `int`：写入的字节数
    - `error`：如果写入文件时出错，返回错误信息

##### `WriteStringToFile(filePath string, s string)`
- **功能**：将字符串数据写入文件。
- **入参**：
    - `filePath string`：文件路径
    - `s string`：字符串数据
- **出参**：
    - `int`：写入的字节数
    - `error`：如果写入文件时出错，返回错误信息

##### `IsDir(s string)`
- **功能**：检查指定路径是否为目录。
- **入参**：
    - `s string`：路径
- **出参**：
    - `bool`：如果是目录，返回 `true`；否则返回 `false`

#### 9.3 hashset.go
##### `(set *HashSet) Members()`
- **功能**：获取哈希集中的所有成员。
- **入参**：
    - `set *HashSet`：哈希集指针
- **出参**：
    - `[]string`：哈希集中的所有成员

#### 9.4 linklist.go
##### `(l *LinkList[T]) Get(index int)`
- **功能**：获取链表中指定索引位置的元素。
- **入参**：
    - `l *LinkList[T]`：链表指针
    - `index int`：索引位置
- **出参**：
    - `T`：指定索引位置的元素

##### `(l *LinkList[T]) GetAll()`
- **功能**：获取链表中的所有元素。
- **入参**：
    - `l *LinkList[T]`：链表指针
- **出参**：
    - `[]T`：链表中的所有元素

##### `(l *LinkList[T]) Walk(fn func(v T) bool)`
- **功能**：遍历链表中的元素，并对每个元素执行指定的函数。
- **入参**：
    - `l *LinkList[T]`：链表指针
    - `fn func(v T) bool`：处理函数，返回 `false` 时停止遍历
- **出参**：无

#### 9.5 maputil.go
##### `MapItoS(src map[string]any)`
- **功能**：将 `map[string]any` 类型的映射转换为 `map[string]string` 类型的映射。
- **入参**：
    - `src map[string]any`：源映射
- **出参**：
    - `map[string]string`：转换后的映射

##### `MapStoI(src map[string]string)`
- **功能**：将 `map[string]string` 类型的映射转换为 `map[string]any` 类型的映射。
- **入参**：
    - `src map[string]string`：源映射
- **出参**：
    - `map[string]any`：转换后的映射

##### `Exists(src map[string]string, key string)`
- **功能**：检查指定键是否存在于 `map[string]string` 类型的映射中。
- **入参**：
    - `src map[string]string`：映射
    - `key string`：键
- **出参**：
    - `bool`：如果键存在，返回 `true`；否则返回 `false`

##### `Existi(src map[string]any, key string)`
- **功能**：检查指定键是否存在于 `map[string]any` 类型的映射中。
- **入参**：
    - `src map[string]any`：映射
    - `key string`：键
- **出参**：
    - `bool`：如果键存在，返回 `true`；否则返回 `false`

##### `SortMapByValue(src map[string]any)`
- **功能**：按值对 `map[string]any` 类型的映射进行升序排序。
- **入参**：
    - `src map[string]any`：源映射
- **出参**：
    - `PairList`：排序后的键值对列表

##### `SortMapByValueDesc(src map[string]any)`
- **功能**：按值对 `map[string]any` 类型的映射进行降序排序。
- **入参**：
    - `src map[string]any`：源映射
- **出参**：
    - `PairList`：排序后的键值对列表

##### `Map2Struct(input interface{}, output interface{})`
- **功能**：将输入结构转换为输出结构，输出结构必须是指向 `map` 或 `struct` 的指针。
- **入参**：
    - `input interface{}`：输入结构
    - `output interface{}`：输出结构指针
- **出参**：
    - `error`：如果转换过程中出错，返回错误信息

##### `MapGet(input interface{}, fieldName string)`
- **功能**：获取 `map` 中的字段，支持嵌套结构获取。
- **入参**：
    - `input interface{}`：输入结构
    - `fieldName string`：字段名，支持嵌套结构，如 `fieldName.subFieldName.xx`
- **出参**：
    - `interface{}`：字段值，如果字段不存在，返回 `nil`

#### 9.6 md5util.go
##### `MD5Encode(content string)`
- **功能**：对字符串进行 MD5 编码。
- **入参**：
    - `content string`：待编码的字符串
- **出参**：
    - `string`：MD5 编码后的字符串

##### `FileMD5(filename string)`
- **功能**：计算文件的 MD5 值。
- **入参**：
    - `filename string`：文件名
- **出参**：
    - `string`：文件的 MD5 值
    - `error`：如果计算 MD5 值时出错，返回错误信息

##### `MapMD5(m map[string]string)`
- **功能**：对 `map[string]string` 类型的映射进行 MD5 编码。
- **入参**：
    - `m map[string]string`：映射
- **出参**：
    - `string`：MD5 编码后的字符串

#### 9.7 routineutil.go
##### `GetGoroutineID()`
- **功能**：获取当前 goroutine 的 ID。
- **入参**：无
- **出参**：
    - `uint64`：当前 goroutine 的 ID

#### 9.8 uuidutil.go
##### `UUIDFromString(s string)`
- **功能**：从字符串中解析出 UUID。
- **入参**：
    - `s string`：UUID 字符串
- **出参**：
    - `UUID`：解析后的 UUID
    - `error`：如果解析过程中出错，返回错误信息

##### `IsValidUUIDString(s string)`
- **功能**：检查指定字符串是否为有效的 UUID 字符串。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `bool`：如果是有效的 UUID 字符串，返回 `true`；否则返回 `false`

##### `MustNewUUID()`
- **功能**：生成一个新的 UUID，如果出错则 panic。
- **入参**：无
- **出参**：
    - `UUID`：新生成的 UUID

##### `NewUUID()`
- **功能**：生成一个新的 UUID。
- **入参**：无
- **出参**：
    - `UUID`：新生成的 UUID
    - `error`：如果生成 UUID 时出错，返回错误信息

##### `(uuid UUID) Copy()`
- **功能**：复制一个 UUID。
- **入参**：
    - `uuid UUID`：待复制的 UUID
- **出参**：
    - `UUID`：复制后的 UUID

##### `(uuid UUID) Raw()`
- **功能**：获取 UUID 的原始字节数据。
- **入参**：
    - `uuid UUID`：UUID
- **出参**：
    - `[16]byte`：UUID 的原始字节数据

##### `(uuid UUID) String()`
- **功能**：将 UUID 转换为标准格式的字符串。
- **入参**：
    - `uuid UUID`：UUID
- **出参**：
    - `string`：标准格式的 UUID 字符串

##### `(uuid UUID) Simple()`
- **功能**：将 UUID 转换为简单格式的字符串。
- **入参**：
    - `uuid UUID`：UUID
- **出参**：
    - `string`：简单格式的 UUID 字符串

##### `NewUUIDString()`
- **功能**：生成一个新的 UUID 字符串。
- **入参**：无
- **出参**：
    - `string`：新生成的 UUID 字符串

##### `SimpleUUID()`
- **功能**：生成一个新的简单格式的 UUID 字符串。
- **入参**：无
- **出参**：
    - `string`：新生成的简单格式的 UUID 字符串

#### 9.9 values.go
##### `(p *Values) Put(id string, val any)`
- **功能**：向 `Values` 中添加一个键值对。
- **入参**：
    - `p *Values`：`Values` 指针
    - `id string`：键
    - `val any`：值
- **出参**：无

##### `(p *Values) Get(id string)`
- **功能**：从 `Values` 中获取指定键的值。
- **入参**：
    - `p *Values`：`Values` 指针
    - `id string`：键
- **出参**：
    - `any`：键对应的值

##### `(p *Values) GetAll()`
- **功能**：获取 `Values` 中的所有键值对。
- **入参**：
    - `p *Values`：`Values` 指针
- **出参**：
    - `any`：所有键值对

##### `(p *Values) Merge(props map[string]any)`
- **功能**：将一个 `map[string]any` 类型的映射合并到 `Values` 中。
- **入参**：
    - `p *Values`：`Values` 指针
    - `props map[string]any`：待合并的映射
- **出参**：无

##### `(p *Values) Clear()`
- **功能**：清空 `Values` 中的所有键值对。
- **入参**：
    - `p *Values`：`Values` 指针
- **出参**：无

#### 9.10 ymlutil.go
##### `LoadYaml(filename string, cfg any)`
- **功能**：从 YAML 文件中加载配置。
- **入参**：
    - `filename string`：YAML 文件名
    - `cfg any`：配置对象指针
- **出参**：
    - `error`：如果加载配置时出错，返回错误信息

##### `StoreYaml(filename string, cfg any)`
- **功能**：将配置保存到 YAML 文件中。
- **入参**：
    - `filename string`：YAML 文件名
    - `cfg any`：配置对象
- **出参**：
    - `error`：如果保存配置时出错，返回错误信息

#### 9.11 checkutil.go
##### `IsChinaMobile(b []byte)`
- **功能**：检查字节切片是否为合法的中国手机号。
- **入参**：
    - `b []byte`：字节切片
- **出参**：
    - `bool`：如果是合法的中国手机号，返回 `true`；否则返回 `false`

##### `IsChinaMobileString(s string)`
- **功能**：检查字符串是否为合法的中国手机号。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `bool`：如果是合法的中国手机号，返回 `true`；否则返回 `false`

##### `IsNickname(b []byte)`
- **功能**：检查字节切片是否为合法的昵称。
- **入参**：
    - `b []byte`：字节切片
- **出参**：
    - `bool`：如果是合法的昵称，返回 `true`；否则返回 `false`

##### `IsNicknameString(s string)`
- **功能**：检查字符串是否为合法的昵称。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `bool`：如果是合法的昵称，返回 `true`；否则返回 `false`

##### `IsUserName(b []byte)`
- **功能**：检查字节切片是否为合法的用户名。
- **入参**：
    - `b []byte`：字节切片
- **出参**：
    - `bool`：如果是合法的用户名，返回 `true`；否则返回 `false`

##### `IsUserNameString(s string)`
- **功能**：检查字符串是否为合法的用户名。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `bool`：如果是合法的用户名，返回 `true`；否则返回 `false`

##### `IsMail(b []byte)`
- **功能**：检查字节切片是否为合法的电子邮箱地址。
- **入参**：
    - `b []byte`：字节切片
- **出参**：
    - `bool`：如果是合法的电子邮箱地址，返回 `true`；否则返回 `false`

##### `IsMailString(s string)`
- **功能**：检查字符串是否为合法的电子邮箱地址。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `bool`：如果是合法的电子邮箱地址，返回 `true`；否则返回 `false`

##### `IsChineseName(b []byte)`
- **功能**：检查字节切片是否为有效的中文姓名。
- **入参**：
    - `b []byte`：字节切片
- **出参**：
    - `bool`：如果是有效的中文姓名，返回 `true`；否则返回 `false`

##### `IsChineseNameString(s string)`
- **功能**：检查字符串是否为有效的中文姓名。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `bool`：如果是有效的中文姓名，返回 `true`；否则返回 `false`

##### `IsChineseNameEx(b []byte)`
- **功能**：检查字节切片是否为有效的中文姓名，并自动修正不规范的间隔符。
- **入参**：
    - `b []byte`：字节切片
- **出参**：
    - `[]byte`：修正后的字节切片
    - `bool`：如果是有效的中文姓名，返回 `true`；否则返回 `false`

##### `IsChineseNameStringEx(s string)`
- **功能**：检查字符串是否为有效的中文姓名，并自动修正不规范的间隔符。
- **入参**：
    - `s string`：字符串
- **出参**：
    - `string`：修正后的字符串
    - `bool`：如果是有效的中文姓名，返回 `true`；否则返回 `false`

##### `IsIdCard(cardNo string)`
- **功能**：检查字符串是否为 18 或 15 位身份证号码。
- **入参**：
    - `cardNo string`：身份证号码字符串
- **出参**：
    - `bool`：如果是 18 或 15 位身份证号码，返回 `true`；否则返回 `false`