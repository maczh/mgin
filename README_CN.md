# MGin 微服务框架 应用文档

> 版本：基于 `github.com/maczh/mgin/v2`（Go 1.25），最新特性 v1.20.x
> 适用：使用 MGin 从零搭建、开发与运维 RESTful 微服务的工程人员
> 模块路径：`github.com/maczh/mgin/v2`
>
> [[英文版](README_en.md)]

## 目录


**第一部分：入门与架构**

1. [框架概述](#1-框架概述)
2. [快速开始](#2-快速开始)
   - 2.1 [安装](#21-安装)
   - 2.2 [最小可运行示例](#22-最小可运行示例)
   - 2.3 [配置文件](#23-配置文件)
3. [应用生命周期](#3-应用生命周期)
   - 3.1 [核心入口对象 `App`](#31-核心入口对象-app)
   - 3.2 [启动与优雅关闭](#32-启动与优雅关闭)
   - 3.3 [插件化加载外部数据源](#33-插件化加载外部数据源)

**第二部分：配置管理**

4. [配置体系（application.yml）](#4-配置体系applicationyml)
   - 4.1 [总览与开关](#41-总览与开关)
   - 4.2 [应用与基础配置](#42-应用与基础配置)
   - 4.3 [配置中心](#43-配置中心)
   - 4.4 [内置系统模块开关](#44-内置系统模块开关)
   - 4.5 [配置读取 API](#45-配置读取-api)
5. [统一配置中心](#5-统一配置中心)

**第三部分：数据访问**

6. [数据库与消息中间件](#6-数据库与消息中间件)
   - 6.1 [MySQL](#61-mysql)
   - 6.2 [PostgreSQL](#62-postgresql)
   - 6.3 [SQLite](#63-sqlite)
   - 6.4 [MongoDB](#64-mongodb)
   - 6.5 [Redis](#65-redis)
   - 6.6 [ClickHouse](#66-clickhouse)
   - 6.7 [ElasticSearch](#67-elasticsearch)
   - 6.8 [Kafka](#68-kafka)
7. [DAO 泛型数据访问层](#7-dao-泛型数据访问层)
   - 7.1 [关系型 DAO（以 MySQL 为例）](#71-关系型-dao以-mysql-为例)
   - 7.2 [MongoDB DAO](#72-mongodb-dao)
8. [缓存（Cache）](#8-缓存cache)

**第四部分：服务与通信**

9. [微服务调用（Client）](#9-微服务调用client)
   - 9.1 [Options 与协议](#91-options-与协议)
   - 9.2 [调用示例](#92-调用示例)
10. [注册中心与服务发现（registry）](#10-注册中心与服务发现registry)

**第五部分：请求处理与横切能力**

11. [中间件（Middleware）](#11-中间件middleware)
   - 11.1 [CORS 跨域](#111-cors-跨域)
   - 11.2 [JWT 鉴权](#112-jwt-鉴权)
   - 11.3 [Casbin 接口鉴权](#113-casbin-接口鉴权)
   - 11.4 [IP 限制（CIDR）](#114-ip-限制cidr)
   - 11.5 [并发限流](#115-并发限流)
   - 11.6 [接口访问日志](#116-接口访问日志)
   - 11.7 [Session](#117-session)
   - 11.8 [Trace 请求跟踪](#118-trace-请求跟踪)
   - 11.9 [国际化语言](#119-国际化语言)
   - 11.10 [XSS 防护](#1110-xss-防护)
12. [国际化与错误码（i18n / errcode）](#12-国际化与错误码i18n--errcode)
   - 12.1 [错误码常量（`errcode` 包）](#121-错误码常量errcode-包)
   - 12.2 [i18n 返回（需在 `NewApp(..., true)` 启用）](#122-i18n-返回需在-newapp-true-启用)
13. [统一返回结构（Result）](#13-统一返回结构result)
14. [日志（logs）](#14-日志logs)

**第六部分：内置模块与工具**

15. [内置系统管理模块与 Casbin 权限](#15-内置系统管理模块与-casbin-权限)
   - 15.1 [启用](#151-启用)
   - 15.2 [内置数据模型（表名）](#152-内置数据模型表名)
   - 15.3 [权限模型](#153-权限模型)
   - 15.4 [请求/响应 DTO](#154-请求响应-dto)
16. [工具集（utils）](#16-工具集utils)
   - 16.1 [加解密（Crypto）](#161-加解密crypto)
   - 16.2 [字符串与中文（String）](#162-字符串与中文string)
   - 16.3 [时间日期（Time）](#163-时间日期time)
   - 16.4 [序列化（JSON / XML / YAML）](#164-序列化json--xml--yaml)
   - 16.5 [Map 与 Struct 转换](#165-map-与-struct-转换)
   - 16.6 [集合与切片（Slice / Set）](#166-集合与切片slice--set)
   - 16.7 [网络与 IP（Net / IP）](#167-网络与-ipnet--ip)
   - 16.8 [文件与压缩（File / Zip）](#168-文件与压缩file--zip)
   - 16.9 [UUID 与随机（UUID / Rand）](#169-uuid-与随机uuid--rand)
   - 16.10 [并发安全与数据结构（Concurrent / DS）](#1610-并发安全与数据结构concurrent--ds)
   - 16.11 [校验与防护（Validate）](#1611-校验与防护validate)
   - 16.12 [其它（Misc）](#1612-其它misc)

**第七部分：参考与分支**

17. [最佳实践](#17-最佳实践)
18. [常见问题（FAQ）](#18-常见问题faq)
19. [版本与升级说明](#19-版本与升级说明)
20. [jh 分支（寄海版）专项说明](#20-jh-分支寄海版专项说明)
   - 20.1 [分支定位与由来](#201-分支定位与由来)
   - 20.2 [与 master-1.25 的差异总览](#202-与-master-125-的差异总览)
   - 20.3 [已移除的能力（重要）](#203-已移除的能力重要)
   - 20.4 [新增：GORM 二级缓存层（UseCache）](#204-新增：gorm-二级缓存层usecache)
   - 20.5 [新增：PostgreSQL 支持](#205-新增：postgresql-支持)
   - 20.6 [新增：ClickHouse 支持](#206-新增：clickhouse-支持)
   - 20.7 [其他改进（jh 分支）](#207-其他改进jh-分支)
   - 20.8 [已知问题与注意事项](#208-已知问题与注意事项)
   - 20.9 [升级 / 迁移建议](#209-升级--迁移建议)
21. [新增：单例限流中间件（ratelimit）](#21-新增单例限流中间件ratelimit)
22. [新增：定时任务管理器（job）](#22-新增定时任务管理器job)
23. [新增：S3 对象存储插件（storage/s3）](#23-新增s3-对象存储插件storages3)
24. [新增能力汇总（jh 分支）](#24-新增能力汇总jh-分支)

---

**第一部分：入门与架构**　（第 1–3 章）

## 1. 框架概述

MGin 是一个用于**快速创建基于 RESTful 的微服务程序**的 Go 框架，底层基于 [Gin](https://github.com/gin-gonic/gin)。它把微服务开发中常见的“脏活”预置好，让开发者只关心业务路由与数据模型：

| 能力域 | 内置支持 |
|---|---|
| Web 框架 | Gin（HTTP/HTTPS 双端口、优雅关闭） |
| 配置中心 | Nacos / Consul / Etcd / Polaris / SpringCloud Config / 本地文件 |
| 服务注册与发现 | Nacos / Consul / Etcd / Polaris |
| 关系型数据库 | MySQL、PostgreSQL、SQLite、ClickHouse（均基于 GORM v2） |
| 文档型 / KV | MongoDB（仿 mgo.v2 API）、Redis（哨兵/集群/单机） |
| 搜索 | ElasticSearch（olivere/elastic） |
| 消息队列 | Kafka |
| 缓存 | 内存缓存（带过期）、本地持久化缓存（bitcask） |
| 通用能力 | 统一返回 Result、国际化、统一错误码、JWT、Casbin 鉴权、请求跟踪、CORS、XSS 防护、限流、IP 限制、接口访问日志 |
| 内置业务 | 系统管理模块（用户/角色/部门/岗位/菜单/字典/API/配置 + RBAC） |

**设计原则**

- **约定优于配置**：绝大多数能力通过 `application.yml` 的 `go.config.used` 开关启用，框架自动按配置连接并做 5 分钟健康检查（断线自动重连）。
- **插件化数据源**：通过 `mgin.Use()` 可加载任意外部数据源（如 RabbitMQ）接入框架生命周期。
- **泛型友好**：`Result`、`CallT`、`DAO` 全面使用 Go 泛型，编译期保证类型安全。



---

## 2. 快速开始

### 2.1 安装

```bash
go get -u github.com/maczh/mgin/v2
```

### 2.2 最小可运行示例

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin/v2"
    "github.com/maczh/mgin/v2/models"
)

func main() {
    // 第一个参数为配置文件路径；传 "" 时自动使用 <可执行文件名>.yml
    // 第四个参数 xlang=true 表示启用国际化（i18n）
    app := mgin.NewApp("", "测试mgin项目", "1.0.0", false)

    app.Router.GET("/hello", func(c *gin.Context) {
        c.JSON(200, models.Success(map[string]string{"msg": "hello world"}))
    })

    app.Run() // 阻塞启动，监听系统信号做优雅关闭
}
```

### 2.3 配置文件

`NewApp` 默认读取与可执行文件同名的 `.yml`（如 `yourapp.yml`）。最简配置：

```yaml
go:
  application:
    name: yourapp
    port: 8080
```

运行：

```bash
go build -o yourapp .
./yourapp                 # 自动读取 yourapp.yml
# 或显式指定：./yourapp -f /path/to/conf.yml
# 查看版本：./yourapp -v
```



---

## 3. 应用生命周期

### 3.1 核心入口对象 `App`

```go
func mgin.NewApp(configFile, appName, version string, xlang bool) *App
func (app *App) Run()
func (app *App) GetVersion() string
```

`App` 结构（字段非导出，对外仅暴露 `Router *gin.Engine` 用于挂路由）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `Router` | `*gin.Engine` | Gin 引擎，业务路由与中间件挂载于此 |
| `MGin` | `*mgin` | 框架内核，含 `Use`/`UsePlugin`/`SafeExit` |

`NewApp` 内部流程：

1. 解析命令行 `-f`（配置文件）与 `-v`（打印版本）。
2. 调用 `mgin.Init(configFile)` 初始化配置，并按 `go.config.used` 连接所有启用的数据库与注册中心。
3. 启动 5 分钟定时器，对所有连接做 `Check()` 健康检查（断线自动重连）。
4. 根据 `go.application.debug` 设置 Gin 模式（release/debug）。
5. 若 `xlang=true`，初始化国际化模块。
6. 初始化基础路由并挂载**全局中间件**：`trace.TraceId()` → `postlog.RequestLogger()` → `cors.Cors()` → 可选 `xlang.RequestLanguage()` → `nice.Recovery`（全局异常恢复）。

### 3.2 启动与优雅关闭

`Run()` 会：

- 启动 HTTP 服务（当 `go.application.port > 0`）。
- 启动 HTTPS 服务（当配置了 `go.application.cert`）。
- 监听 `SIGINT / SIGHUP / SIGTERM / SIGQUIT`，收到信号后依次：`SafeExit()`（关闭所有 DB 与注销注册中心）→ `server.Shutdown`（5 秒超时优雅关闭）→ 退出。

### 3.3 插件化加载外部数据源

任何实现了 `MginPlugin` 接口的组件都可接入框架生命周期：

```go
type MginPlugin interface {
    Init(configData []byte)
    Close()
    Check() error
}
```

```go
// 方式一：直接传函数
mgin.MGin.Use("rabbitmq", mgrabbit.Rabbit.Init, mgrabbit.Rabbit.Close, nil)

// 方式二：传插件对象
mgin.MGin.UsePlugin("rabbitmq", rabbitPluginInstance)
```

> 注意：`Use` 的 `dbConfigName` 必须出现在 `go.config.used` 中，且对应配置文件前缀存在，否则不会加载。


















---

**第二部分：配置管理**　（第 4–5 章）

## 4. 配置体系（application.yml）

### 4.1 总览与开关

所有能力的启用都通过 `go.config.used`（逗号分隔）控制，例如：

```yaml
go:
  config:
    used: nacos,mysql,mongodb,redis,kafka   # 本次启用的组件，框架据此自动连接
```

### 4.2 应用与基础配置

```yaml
go:
  application:
    name: myapp                 # 应用名，作为注册到注册中心的服务名
    port: 8080                  # HTTP 端口
    project: myproj             # 所属项目名
    port_ssl: 8443              # HTTPS 端口（配置 cert/key 后生效）
    cert: server.crt            # SSL 证书（位于可执行文件目录）
    key: server.key             # SSL 私钥
    debug: true                 # 调试模式（=true 时 Gin 为 debug 且日志强制 debug）
    ip: 10.0.0.5                # 注册到注册中心时登记的 IP（Docker/外网场景需指定）
  discovery:
    registry: nacos             # 注册中心类型：nacos/consul/etcd/polaris，默认 nacos
    callType: json              # 微服务调用参数模式：x-form / json / restful
  jwt:
    secret: 1234567890abcdef    # JWT 签名密钥
  logger:                       # 控制台/文件日志（logs 包）
    level: debug
    out: console,file
    file: /opt/logs/myapp       # 日志文件路径前缀，自动追加 .yyyy-MM-dd.log
  log:                          # 接口访问日志与微服务调用日志
    db: mongodb                 # 日志落库：mongodb / elasticsearch
    dbName: Partner-Id          # 多库时从 header 取该参数作库名标签
    req: MyappRequestLog        # 请求日志表/索引名
    call: MyappCallLog          # 调用日志表/索引名
    kafka:
      use: true                 # 是否同时发往 kafka
      topic: myapp              # kafka 主题（可逗号分隔多个）
```

### 4.3 配置中心

```yaml
go:
  config:
    server: http://192.168.1.5:8848/   # 配置服务器地址
    server_type: nacos                 # nacos/consul/springconfig/etcd/polaris/file
    token:                             # 访问 token（polaris 必填）
    path: conf                         # 本地文件模式时配置所在目录
    env: test                          # 环境：test/prod/dev 等
    prefix:                            # 各组件配置文件名前缀
      mysql: mysql
      mongodb: mongodb
      redis: redis
      nacos: nacos
      sqlite: mytest.db
      elasticsearch: elasticsearch
      kafka: kafka
      postgres: postgres
      clickhouse: clickhouse
      etcd: etcd
      consul: consul
```

各组件实际配置文件命名规则：`<prefix>-<env>.yml`，例如 `mysql-test.yml`、`redis-prod.yml`。

### 4.4 内置系统模块开关

```yaml
go:
  sys:
    enabled: true              # 是否启用内置系统管理组件（v1.20.3+）
    initdb: true               # 是否自动建表与初始化基础数据
    baseUri: /api/v1           # 基础组件接口前缀
    casbin: true               # sys 模块内是否启用 casbin 接口鉴权
    swagger:
      enabled: true
      uri: /swagger/sys        # 完整地址 /api/v1/swagger/sys/index.html
  casbin:
    enabled: true              # 是否全局启用 casbin（供 middleware/casbin 使用）
    model_file: casbin.conf    # casbin 模型文件路径

> ⚠️ **jh 分支提示**：`jh` 分支已移除内置 `sys` 模块，故 `go.sys.*` 配置项在该分支下无意义；Casbin 仍可作为独立中间件 `middleware/casbin` 通过 `go.casbin.enabled` 启用。
```

### 4.5 配置读取 API

```go
import "github.com/maczh/mgin/v2/config"

config.Config.GetConfigString("go.application.name")   // string
config.Config.GetConfigStringArray("go.config.used")  // []string
config.Config.GetConfigInt("go.application.port")      // int
config.Config.GetConfigBool("go.application.debug")    // bool
config.Config.Exists("go.sys.enabled")                 // bool
```

全局配置实例为 `config.Config`（唯一单例）。



---

## 5. 统一配置中心

除本地文件外，MGin 支持从 Nacos / Consul / Etcd / Polaris / SpringCloud Config 拉取配置。配置键路径约定：

- Nacos：`{server}/nacos/v1/cs/configs?group={project}&&dataId={prefix.nacos}-{env}.yml`
- Etcd：`/config/{project}/{prefix.etcd}-{env}.yml`
- Consul：`/{project}/{prefix.consul}-{env}.yml`
- Polaris：`/config/v1/GetConfigFile?namespace=default&group={project}&fileName={prefix.consul}-{env}.yml`

由各组件的 `<prefix>-<env>.yml` 文件提供连接信息，框架在 `mgin.Init` 中按 `go.config.used` 拉取并初始化对应客户端。














---

**第三部分：数据访问**　（第 6–8 章）

## 6. 数据库与消息中间件

框架启动时按 `go.config.used` 自动初始化下列全局客户端（位于 `github.com/maczh/mgin/v2/db`）：

| 全局变量 | 类型 | 说明 |
|---|---|---|
| `db.Mysql` | `*mysql.MysqlClient` | MySQL（GORM） |
| `db.Pg` | `*postgres.PostgresClient` | PostgreSQL（GORM） |
| `db.Sqlite` | `*sqlite.Sqlite` | SQLite（GORM） |
| `db.Mongo` | `*mongo.Mongodb` | MongoDB |
| `db.Redis` | `*redis.RedisClient` | Redis |
| `db.Clickhouse` | `*clickhouse.ClickhouseClient` | ClickHouse（GORM） |
| `db.ElasticSearch` | `*es.ElasticSearch` | ElasticSearch |
| `db.Kafka` | `*kafka.Kafka` | Kafka |

通用方法（各客户端均具备）：`Init([]byte)`、`Check() error`、`Close()`、`GetConnection(dbName ...string)`、`IsMultiDB() bool`、`ListConnNames() []string`。

> 所有客户端在 `mgin.Init` 中依据 `go.config.used` 自动初始化；如需自定义初始化时机，也可手动调用 `db.XXX.Init(config.Config.GetConfigData(prefix))`。

### 6.1 MySQL

**配置 `mysql-test.yml`：**

```yaml
go:
  data:
    mysql: user:pwd@tcp(127.0.0.1:3306)/dbname?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true    # 打印 SQL
    mysql_cache: true    # 启用二级缓存
    mysql_cache_expired: 300   # 缓存过期秒数，默认 300（5 分钟）；0 表示不过期
    mysql_pool:          # 连接池（缺省为单一长连接）
      max: 200           # 最大连接数
      total: 1000        # 最大并发，缺省为 max*5
      timeout: 30        # 空闲连接超时（秒）
      life: 5            # 连接生命周期（分钟）
```

**多库配置 `mysql-multidb-test.yml`：**

```yaml
go:
  data:
    mysql:
      multidb: true
      dbNames: test1,test2
      test1: user1:pwd1@tcp(127.0.0.1:3306)/dbname1?charset=utf8&parseTime=True&loc=Local
      test2: user2:pwd2@tcp(127.0.0.1:3306)/dbname2?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true
    mysql_cache: true
```

**代码示例：**

```go
// 取连接（多库时传入库名）
conn, err := db.Mysql.GetConnection()
if err != nil { /* 处理 */ }
conn.Create(&user)

// 启用二级缓存（建议在启动时调用一次）
// 配置键：go.data.mysql_cache（开关） / go.data.mysql_cache_expired（秒，默认 300=5 分钟）
// 有 Redis 时用 Redis 作缓存层，否则回退为进程内内存缓存
db.Mysql.UseCache()

// 多库判断
if db.Mysql.IsMultiDB() {
    for _, name := range db.Mysql.ListConnNames() {
        _ = name
    }
}
```

### 6.2 PostgreSQL

**配置 `postgresql-test.yml`：**

```yaml
go:
  data:
    postgres:
      dsn: "host=localhost user=test password=test_pwd123 dbname=testdb port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      debug: true
      cache:
        enabled: true          # 二级缓存开关
        expired: 300           # 缓存过期秒数，默认 300（5 分钟）；依赖 go.data.redis
      pool:
        max: 200
        total: 1000
        timeout: 30
        life: 5
```

多库配置与 MySQL 同构（将 `mysql` 换成 `postgres`，字段 `dbNames` 指定库名列表，各库名作为子键放 dsn）。使用 `db.Pg.GetConnection()`、`db.Pg.UseCache()` 等。`UseCache()` 读取 `go.data.postgres.cache.enabled`（开关）与 `go.data.postgres.cache.expired`（秒，默认 300）决定缓存行为。

### 6.3 SQLite

**配置 `sqlite-test.yml`：**

```yaml
go:
  data:
    sqlite: mytest.db   # 本地文件名；缺省使用 <App.Name>.db
```

```go
conn := db.Sqlite.GetConnection()
db.Sqlite.UseCache()
```

### 6.4 MongoDB

基于 `github.com/maczh/mgo`（仿 mgo.v2 API）。**配置 `mongodb-test.yml`：**

```yaml
go:
  data:
    mongodb:
      uri: mongodb://user:pwd@127.0.0.1:27017/dbname
      db: dbname
      debug: true
    mongo_pool:
      max: 20
```

复制集：`mongodb://user:pwd@host1:27017,host2:27017/dbname?replicaSet=rs0`。

多库：将 `mongodb` 设为 `multidb: true` + `dbNames` + 各库名子键（`uri`/`db`）。

**代码示例：**

```go
// 取库连接（返回的是 session 的 Copy，用后需归还）
database, err := db.Mongo.GetConnection()
if err != nil { /* 处理 */ }
defer db.Mongo.ReturnConnection(database) // 关键：用完归还 session

// 多库
db2, _ := db.Mongo.GetConnection("test2")
defer db.Mongo.ReturnConnection(db2)
```

### 6.5 Redis

支持**单机 / 哨兵 / 集群**三种模式，底层 go-redis v7。**配置 `redis-test.yml`：**

```yaml
go:
  data:
    redis:
      uri: 127.0.0.1:6379[,127.0.0.1:6380]   # uri 与 host+port 二选一；多 ip 即集群
      host: 127.0.0.1[,127.0.0.1]
      port: 6379[,6380]
      password: ""
      database: 1
      master: mymaster          # 哨兵模式 master 名
      timeout: 1000
    redis_pool:
      min: 3
      max: 200
      idle: 10
      timeout: 300
```

多库配置：将 `redis` 设为 `multidb: true` + `dbNames` + 各库名子键（同样支持 uri/host/port/master）。

**代码示例：**

```go
client, err := db.Redis.GetConnection()
if err != nil { /* 处理 */ }

client.Set("key", "value", 10*time.Second)
val, _ := client.Get("key").Result()

// 多库
client2, _ := db.Redis.GetConnection("test2")

// 断线自动重连的 pattern 订阅
db.Redis.PSubscribe("", func(msg *redis.Message, dbName string) {
    logs.Info("收到消息: %s", msg.Payload)
}, "news.*")
```

### 6.6 ClickHouse

基于 GORM 驱动 `gorm.io/driver/clickhouse`，API 与 MySQL 同构。配置键 `go.data.clickhouse`（单库 DSN）或 `go.data.clickhouse.<dbName>`（多库）、`go.data.clickhouse_cache`（缓存开关）。使用 `db.Clickhouse.GetConnection()`、`UseCache()` 等。

```go
dao := &dao.ClickhouseDao[Event]{}
_ = dao.Create(&event)
rows, page, _ := dao.Pager(db.Clickhouse.GetConnection(), 1, 20)
```

> ⚠️ **jh 分支已知问题**：`ClickhouseDao.MultiCreate` 在批量插入时误用 `db.Mysql.GetConnection`，会把数据写到 **MySQL** 而非 ClickHouse（截至 `v1.25.11-jh`）。修复前请勿对 ClickHouse 使用 `MultiCreate`，改用单条 `Create` 或自行取 `db.Clickhouse.GetConnection()` 操作。另有 `UseCache()` 未设置 Redis 过期时间，详见[第 20 章](#20-jh-分支寄海版专项说明)。

### 6.7 ElasticSearch

基于 olivere/elastic v6。**配置 `elasticsearch-test.yml`：**

```yaml
go:
  elasticsearch:
    uri: http://127.0.0.1:9200
    user: elastic
    password: "********"
```

**CRUD（索引名规则 `database_table`，table 为空则为 `database`）：**

```go
// 新增文档（searchFields 指定可被搜索的字段）
id, err := db.ElasticSearch.AddDocument("mydb", "user", map[string]any{
    "name": "张三", "age": 18,
}, []string{"name"})

// 批量新增
db.ElasticSearch.AddDocuments("mydb", "user", []map[string]any{ /* ... */ }, []string{"name"})

// 更新 / 删除
db.ElasticSearch.UpdateDocument("mydb", "user", id, map[string]any{"age": 19})
db.ElasticSearch.DeleteDocument("mydb", "user", id)
db.ElasticSearch.DeleteTable("mydb", "user")
db.ElasticSearch.DeleteDatabase("mydb")
```

> 索引与 IK/ngram mapping 由框架自动创建，无需手动建索引。

### 6.8 Kafka

基于 Shopify/sarama。**配置 `kafka-test.yml`：**

```yaml
go:
  data:
    kafka:
      servers: "127.0.0.1:9092,127.0.0.1:9093"   # 集群逗号分隔
      ack: all              # no / local / all
      auto_commit: true
      partitioner: hash     # hash / random / round-robin
      version: 2.8.1
```

**发送与消费：**

```go
// 发送
db.Kafka.Send("my_topic", "测试消息")
db.Kafka.SendMsgs("my_topic", []string{"a", "b"})

// 创建主题
db.Kafka.CreateTopic("my_topic")

// 消费（基于消费组，断线自动重连；一个 topic 对应一个 groupId）
err := db.Kafka.MessageListener("my_group_id", "my_topic", func(msg string) error {
    logs.Info("收到Kafka消息: %s", msg)
    return nil
})
```

更多底层入口：`GetProducer() / GetConsumer() / GetAdminClient() / GetConsumerGroup(id)`。



---

## 7. DAO 泛型数据访问层

为关系型与文档型数据库提供类型安全的 CRUD。位于 `github.com/maczh/mgin/v2/db/dao`。

| DAO | 适用 | 构造 |
|---|---|---|
| `dao.MySQLDao[E]` | MySQL | `&dao.MySQLDao[E]{}`，E 需实现 `schema.Tabler`（有 `TableName()`） |
| `dao.PostgresDao[E]` | PostgreSQL | 同上 |
| `dao.ClickhouseDao[E]` | ClickHouse | 同上 |
| `dao.MgoDao[E]` | MongoDB | 需先设置 `CollectionName` 字段 |

### 7.1 关系型 DAO（以 MySQL 为例）

```go
type User struct {
    ID     int    `gorm:"column:id;primaryKey"`
    Name   string `gorm:"column:name"`
    Status int    `gorm:"column:status"`
}
func (User) TableName() string { return "sys_user" }

// 构造 DAO（单库可省略 Tag；多库通过 Tag 闭包路由到指定库名）
userDao := &dao.MySQLDao[User]{}
// 多库示例：
// userDao := &dao.MySQLDao[User]{Tag: func() string { return "test2" }}

// 增
userDao.Create(&User{Name: "张三", Status: 1})
userDao.MultiCreate([]*User{&u1, &u2})

// 改
userDao.Updates(&User{Name: "李四"})            // 按主键更新
userDao.Save(&user)                             // 全量保存

// 删
userDao.Delete(User{Name: "张三"})              // 按条件删除

// 查
list, err := userDao.All(User{Status: 1},
    dao.QueryOption{Preloads: []string{"Role"}, OrderBy: []string{"id desc"}})
one, err := userDao.One(User{Name: "张三"})      // 查不到返回 (nil, nil)
exists := userDao.Exists(User{Name: "张三"})
count, err := userDao.Count(User{Status: 1})

// 分页：先 Where 取 *gorm.DB，再 Pager
db2 := userDao.Where("status = ?", 1)
rows, page, err := userDao.Pager(db2, 1, 20)     // page.Index/Size/Total/Count

// 调试与上下文
userDao.Debug().All(User{})
userDao.WithContext(&ctx).All(User{})
```

> `One` / `Count` 查询单条无记录时**不返回 error**（返回 `nil` / `0`），便于上层直接判空。

### 7.2 MongoDB DAO

```go
type MgoUser struct { ID primitive.ObjectID `bson:"_id"` }
mgoDao := &dao.MgoDao[MgoUser]{CollectionName: "user"}
mgoDao.Insert(&MgoUser{})
mgoDao.All(bson.M{"name": "张三"})
mgoDao.One(bson.M{"_id": id})
mgoDao.Updates(id, MgoUser{})
mgoDao.Delete(bson.M{"name": "张三"})
mgoDao.Pager(bson.M{}, "name", 1, 20)
```







---

## 8. 缓存（Cache）

位于 `github.com/maczh/mgin/v2/cache`，统一接口 `ICache`：

```go
type ICache interface {
    Add(key, value any, lifeSpan time.Duration)
    Set(key, value any, duration time.Duration)
    Get(key any) (any, bool)
    Value(key any) (any, bool)
    IsExist(key any) bool
    Delete(key any)
    Clear() bool
    Range(f func(key, value any) bool)
    Close()
}
```

**工厂函数（按名称单例）：**

```go
// 第二个参数 true 返回磁盘持久化缓存（bitcask），缺省 false 为内存缓存
c := cache.OnGetCache("mycache", false)
c.Set("k", "v", 5*time.Minute)
v, ok := c.Get("k")

// 显式构造
mem := cache.OnMemCache("name")
disk := cache.OnDiskCache("/path/to/cache")  // 持久化到磁盘
cache.CloseCache()                            // 释放所有缓存实例
```

> 内存缓存 `New(cleaningInterval)` 后台清理过期项；磁盘缓存基于 bitcask，**`Get` 返回 `string` 类型**，需自行转换为目标类型。














---

**第四部分：服务与通信**　（第 9–10 章）

## 9. 微服务调用（Client）

位于 `github.com/maczh/mgin/v2/client`。底层通过 `registry.Registry.GetServiceURL` 做服务发现，自动透传 trace header。

### 9.1 Options 与协议

```go
type Options struct {
    Method   string                 // GET/POST/PUT/DELETE，缺省 POST
    Protocol string                 // x-form / json / restful / file
    Group    string                 // 注册中心分组（nacos 等）
    Header   any                    // 额外请求头（struct/map）
    Query    any                    // URL Query 参数
    Data     any                    // x-form 的 PostForm 参数
    Json     any                    // json / restful 的 body
    Path     map[string]string      // restful 路径参数 {key}=value
    Files    []grequests.FileUpload // 文件上传
    Retry    bool                   // 失败是否重试一次
}
```

协议常量：`client.CONTENT_TYPE_FORM` / `CONTENT_TYPE_JSON` / `CONTENT_TYPE_RESTFUL` / `CONTENT_TYPE_FILE`。`Protocol` 缺省取 `go.discovery.callType`。

### 9.2 调用示例

```go
// 非泛型：返回原始字符串
resp, err := client.Call("user-service", "/api/v1/user/get", &client.Options{
    Method:   "POST",
    Protocol: client.CONTENT_TYPE_JSON,
    Json:     map[string]any{"id": 1},
    Query:    map[string]any{"trace": "x"},
})

// 泛型：直接解析为 models.Result[T]
var user User
result := client.CallT[User]("user-service", "/api/v1/user/get", &client.Options{
    Protocol: client.CONTENT_TYPE_JSON,
    Json:     map[string]any{"id": 1},
})
if result.Status != 1 {
    // 处理错误（失败 result.Status = -1）
}
user = result.Data

// restful 风格
result := client.CallT[Order]("order-service", "/api/v1/order/{oid}", &client.Options{
    Protocol: client.CONTENT_TYPE_RESTFUL,
    Method:   "GET",
    Path:     map[string]string{"oid": "1001"},
})

// 文件上传
result := client.CallT[any]("file-service", "/upload", &client.Options{
    Files: []grequests.FileUpload{{FileName: "a.png", FileReader: f}},
})
```

> 调用失败返回 `models.ErrorT[T](-1, "...")`，message 为 `"Service error"` 或 `"微服务获取X服务主机IP端口失败"`。`Retry: true` 时失败自动重试一次。



---

## 10. 注册中心与服务发现（registry）

`NewApp` 按 `go.discovery.registry` 创建注册客户端，并在 `mgin.Init` 中执行 `Register`。

```go
var Registry RegistryClient   // 全局单例

type RegistryClient interface {
    Register(registryConfigData []byte)
    GetServiceURL(servicename string, groupName ...string) (string, string) // 返回 host:port, group
    DeRegister()
}
func NewRegistry() RegistryClient  // 内部按配置类型返回实现
```

支持实现：`nacos` / `consul` / `etcd` / `polaris`（对应 `registry/nacos`、`registry/consul`、`registry/etcd`、`registry/polaris`）。`client.Call` 内部正是调用 `GetServiceURL` 完成发现，并随机选取健康实例。

**各注册中心配置（`*-test.yml`）：**

```yaml
# nacos
go:
  nacos:
    server: 127.0.0.1
    port: 8848
    clusterName: DEFAULT
    group: OpenApi
    weight: 1
    lan: true          # 以内网地址注册
    lanNet: 192.168.3. # 网段前缀
```
其余（etcd/consul/polaris）结构类似，`polaris` 额外需要 `namespace` 与 `token`。其中 consul 的 `port` 对应其 HTTP API 端口（如 8500）。

> 📌 **jh 分支增强**：etcd 注册自 `v1.21.17-jh` 起改用**租约（lease）+ keepalive 自动续租**，实例离线后注册信息可自动清理；Nacos 注册自 `v1.21.15-jh` 起改用社区 `nacos-sdk-go`，不再依赖阿里云 SDK。














---

**第五部分：请求处理与横切能力**　（第 11–14 章）

## 11. 中间件（Middleware）

基础中间件由 `NewApp` 自动挂载：`trace` → `postlog` → `cors` → `xlang`(可选) → `recovery`。业务鉴权中间件（jwt/casbin）需自行按需挂载。

### 11.1 CORS 跨域

```go
app.Router.Use(cors.Cors())                       // 默认允许常用头
app.Router.Use(cors.Cors("X-My-Header"))          // 追加自定义头
```

默认允许头部：`Content-Type, AccessToken, X-CSRF-Token, Authorization, Token`；自动放行 OPTIONS。

> 📌 **jh 分支增强**：自 `v1.21.15-jh` 起支持**外部配置跨域规则**（不再仅内置固定头），可通过配置注入允许的源/方法/头。

### 11.2 JWT 鉴权

```go
app.Router.Use(jwt.JwtAuthorize())
```

从 `Authorization` 头取 token，用 `config.Config.Jwt.Secret` 校验。白名单路径：`/docs/`、`/swagger/`、`go.sys.swagger.uri` 自动放行。

### 11.3 Casbin 接口鉴权

```go
app.Router.Use(casbin.CasbinHandler())
```

要求 `go.casbin.enabled=true`。从 JWT claims 取 `userId`/`roleId`，调用 `casbin.Casbin.GetEnforcer().Enforce(...)` 做 RBAC 鉴权（未授权返回 401/403）。`casbin.Casbin.UnAuthPath`（`[]CasbinInfo{{Path, Method}}`）可配置免鉴权白名单。

### 11.4 IP 限制（CIDR）

```go
app.Router.Use(iplimit.CIDR("192.168.1.0/24,10.0.0.0/8"))  // 仅允许指定网段
```

- `iplimit.DisableLogging`：是否关闭日志。
- `iplimit.TrustedHeaderField`：从可信代理头取真实 IP。

### 11.5 并发限流

```go
app.Router.Use(limit.MaxAllowed(100))   // 最多 100 个并发请求
```

### 11.6 接口访问日志

```go
app.Router.Use(postlog.RequestLogger())  // 默认已挂载
```

异步写日志到 MongoDB / ElasticSearch（取决于 `go.log.db`），或发往 Kafka（`go.log.kafka.use`）。支持按 header 中 `go.log.dbName` 指定参数切库。

### 11.7 Session

```go
app.Router.Use(session.New())                  // 默认配置
store := session.FromContext(c)                // 取会话
session.Destroy(c)                             // 销毁
store, _ = session.Refresh(c)                  // 刷新
```

### 11.8 Trace 请求跟踪

```go
app.Router.Use(trace.TraceId())   // 默认已挂载，写入 X-Request-ID
```

请求级上下文工具（跨协程共享）：`trace.GetRequestId()`、`trace.GetClientIp()`、`trace.GetUserAgent()`、`trace.GetHeader(h)`、`trace.SetHeader(k,v)`、`trace.GetHeaders()`、`trace.CopyPreHeaderToCurRoutine(id)`。这些 header 会在 `client.Call` 时被自动透传，实现链路追踪。

### 11.9 国际化语言

```go
app.Router.Use(xlang.RequestLanguage())   // 默认在 xlang=true 时挂载
xlang.GetCurrentLanguage()                // 取当前语言，默认 zh-cn
```

### 11.10 XSS 防护

```go
xssMw := &xss.XssMw{
    FieldsToSkip: []string{"content"},     // 跳过的字段
    BmPolicy:     "UGCPolicy",             // StrictPolicy（默认）/ UGCPolicy
}
app.Router.Use(xssMw.RemoveXss())
```

对 POST/PUT 的 JSON、form、multipart 及 GET query 做 bluemonday 净化，自动跳过 `password` 字段。



---

## 12. 国际化与错误码（i18n / errcode）

### 12.1 错误码常量（`errcode` 包）

整型状态码：

```go
errcode.URI_NOT_FOUND        = 1000
errcode.SYSTEM_ERROR         = 1001
errcode.DB_CONNECT_ERROR     = 1002
errcode.REQUEST_PARAMETER_LOST = 1003
errcode.DATA_NOT_FOUND       = 1004
errcode.USER_NOT_FOUND       = 1005
errcode.PASSWORD_ERROR       = 1006
errcode.VERIFY_CODE_ERROR    = 1007
errcode.TOKEN_ERROR          = 1008
errcode.AUTHENTICATION_FAILURE = 1009
errcode.SERVICE_UNAVAILABLE  = 1010
```

多语言消息 ID（`errcode` 包常量，作为 i18n 的 key）：

```go
errcode.UrlNotFound   = "404"
errcode.SystemError   = "系统异常"
errcode.ParamLost     = "参数不可为空"
errcode.ParamError    = "参数错误"
errcode.Success       = "success"
// 还有 DbConnectErr / DbInsertErr / DbUpdateErr / DbDeleteErr /
// DataNotFound / ConnectFail / ServiceUnavailable / DbQueryErr 等
```

### 12.2 i18n 返回（需在 `NewApp(..., true)` 启用）

```go
i18n.Error(code int, messageId string) models.Result[any]
i18n.ErrorT[T](code int, messageId string) models.Result[T]
i18n.ErrorWithMsg(code, messageId, msg string) models.Result[any]
i18n.Success[T](data T) models.Result[T]
i18n.SuccessWithPage[T](data T, count, index, size, total int) models.Result[T]

i18n.String("success")                  // 按当前协程语言取文本
i18n.Format("welcome", name)            // 模板 {} 占位替换
i18n.ParamLostError("userId")           // 参数缺失快捷错误
i18n.CheckParametersLost(params, "a", "b") // 批量校验参数非空
```

**示例：**

```go
func GetUser(c *gin.Context) {
    id := c.Query("id")
    if id == "" {
        c.JSON(200, i18n.ParamLostError("id"))
        return
    }
    user, err := db.User.Get(id)
    if err != nil {
        c.JSON(200, i18n.Error(errcode.DATA_NOT_FOUND, errcode.DataNotFound))
        return
    }
    c.JSON(200, i18n.Success(user))
}
```

> 多语言数据源为外部 `x-lang` 微服务，框架在 `i18n.Init()` 时拉取并每 5 分钟刷新；内置中文消息 ID 见 `errcode` 包。







---

## 13. 统一返回结构（Result）

位于 `github.com/maczh/mgin/v2/models`，所有接口建议返回 `models.Result[T]`（由 Gin 以 `c.JSON(200, result)` 输出）：

```go
type Result[T any] struct {
    Status int         `json:"status"`            // 1=成功，非1=失败
    Msg    string      `json:"msg"`
    Data   T           `json:"data,omitempty"`
    Page   *ResultPage `json:"page,omitempty"`
}
type ResultPage struct {
    Count int `json:"count"`  // 总页数
    Index int `json:"index"`  // 当前页
    Size  int `json:"size"`   // 每页大小
    Total int `json:"total"`  // 总记录数
}
```

构造器：

```go
models.Success(data)                       // Status=1, Msg="Success"
models.SuccessWithMsg("ok", data)
models.SuccessWithPage(data, count, index, size, total)
models.SuccessPage(data, &models.ResultPage{...})
models.Error(status, msg)                  // Result[any]
models.ErrorT[T](status, msg)             // 指定泛型

// 类型转换
r.ToAny()            // Result[T] -> Result[any]
models.ToAny[T](r)   // Result[any] -> Result[T]（断言 Data 类型必须一致）
```

> 框架的 `NoRoute`（404）与全局 `recoveryHandler`（panic）均使用 `i18n.Error` 返回标准 Result，保证错误格式统一。







---

## 14. 日志（logs）

```go
logs.Debug("用户 %s 登录", name)
logs.Info("连接 %s 成功", "MySQL")
logs.Warn("缓存未命中")
logs.Error("MySQL check failed： {}", err.Error())
```

- 级别受 `go.logger.level`（debug/info/warn/error）控制；`go.application.debug=true` 时强制 debug。
- 输出目标由 `go.logger.out`（console,file）决定，文件路径前缀 `go.logger.file`。
- 日志自动注入 `traceId`，便于链路排查。
- 输出器类型（`logs` 内部）：`console` / `file` / `es` / `simple` / `color`。


















---

**第六部分：内置模块与工具**　（第 15–16 章）

## 15. 内置系统管理模块与 Casbin 权限

> v1.20.3+，仅需配置即可启用，自动建表并自带 Swagger 文档。

> ⚠️ **jh 分支（寄海版）不适用本章**：`jh` 分支已**整体移除内置 `sys` 模块**（含 controller/dao/service/route/middle 全套代码）与 Swagger 文档（`docs/docs.go`、`docs/swagger.json/yaml` 一并删除），目的是**缩小编译产物体积**。因此在 `jh` 分支下：`go.sys.*` 配置项不生效、无内置用户/角色/权限等 API、不内置 Swagger。如需这些能力，请使用 `master-1.25` 等非 `jh` 分支。详见[第 20 章](#20-jh-分支寄海版专项说明)。

### 15.1 启用

```yaml
go:
  sys:
    enabled: true
    initdb: true
    baseUri: /api/v1
    casbin: true
    swagger:
      enabled: true
      uri: /swagger/sys
```

### 15.2 内置数据模型（表名）

| 模型 | 表名 | 说明 |
|---|---|---|
| `SysUser` | `sys_user` | 用户（LoginName 唯一） |
| `SysRole` | `sys_role` | 角色（RoleIdent 唯一） |
| `SysUserExt` | `sys_user_ext` | 用户扩展（部门/岗位/角色关联） |
| `SysDept` | `sys_dept` | 部门（树形） |
| `SysPost` | `sys_post` | 岗位 |
| `SysDict` | `sys_dict` | 字典 |
| `SysResource` | `sys_resource` | 前端菜单/资源树 |
| `SysApi` | `sys_api` | 后端接口（NeedAuth/NeedLog） |
| `SysRoleApi` | `sys_role_api` | 角色-接口 |
| `SysRoleResource` | `sys_role_resource` | 角色-资源（菜单） |
| `SysUserOnline` | `sys_user_online` | 在线用户 |
| `SysSocial` | `sys_social` | 第三方登录 |
| `SysConfig` | `sys_config` | 动态配置（Key 唯一） |

公共基类 `BaseModel` 含 `DelFlag, CreateAt, UpdateAt, UpdateBy, CreateBy, TenantId`。

### 15.3 权限模型

- **登录与 Token**：`SysUser` + JWT（`jwt.JwtAuthorize` 校验）。
- **接口鉴权（二选一）**：
  - Casbin 模式（推荐）：`go.casbin.enabled=true` + `go.sys.casbin=true`，由 `middleware/casbin` 做 RBAC 拦截。策略通过 `casbin.Casbin` 管理：
    ```go
    casbin.Casbin.UpdateCasbin(roleId, []casbin.CasbinInfo{{Path:"/api/v1/user", Method:"GET"}})
    casbin.Casbin.GetPolicyPathByRoleId(roleId)
    casbin.Casbin.AddPolicy([][]string{...})
    casbin.Casbin.RemoveFilteredPolicy("1")
    casbin.Casbin.FreshCasbin()
    ```
    `casbin.conf`（仓库根）为 RBAC 模型：`r.sub,p.sub=role; r.obj=path; r.act=method`，matcher 使用 `keyMatch2` 支持路径通配。
  - 角色接口表模式：通过 `SysRoleApi` 关联，由业务自行校验。

### 15.4 请求/响应 DTO

`models/sys/request` 与 `models/sys/vo` 提供常用 DTO，如 `RegisterReq`、`LoginReq`、`CreateRoleReq`、`ListSysUserReq`、`BindRoleApiReq`、`CreateResourceReq` 等，可直接用于 Controller 绑定。



---

## 16. 工具集（utils）

`github.com/maczh/mgin/v2/utils` 提供了大量开箱即用的工具函数与泛型数据结构，覆盖加解密、字符串/中文、时间日期、序列化、Map/Struct 转换、集合、网络/IP、文件与压缩、UUID/随机、并发安全结构与参数校验等。全部为包级导出函数，按需引用，例如：

```go
s  := utils.MD5Encode("hello")                    // "5d41402abc4b2a76b9719d911017c592"
id := utils.NewUUIDString()                       // "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
ok := utils.IsChinaMobileString("13800138000")    // true
```

> 📌 约定：加解密类函数一般同时提供「字节切片版」（`[]byte`/error）与「字符串版」（忽略错误、失败返回空串或提示），可按场景选择。

### 16.1 加解密（Crypto）

#### AES（CBC / ECB）
| 函数 | 说明 |
| --- | --- |
| `AESBase64Encrypt(origin, key string, iv []byte) (string, error)` | CBC 模式，输出标准 Base64 字符串 |
| `AESBase64Decrypt(encrypt, key string, iv []byte) (string, error)` | 对应解密 |
| `AesEncrypt(origData, key []byte) ([]byte, error)` | CBC，IV 取 `key[:16]` |
| `AesDecrypt(encrypted, key []byte) ([]byte, error)` | 对应解密 |
| `AESEncrypt(data, key string) string` / `AESDecrypt(data, key string) string` | string 封装，失败返回 `""` |
| `AesEncryptEcb(src, key string) (string, error)` / `AesDecryptEcb(crypted, key string) (string, error)` | ECB 模式 |
| `NewECBEncrypter(b)` / `NewECBDecrypter(b)` | 自定义 ECB BlockMode |
| `PKCS5Padding` / `PKCS5UnPadding` | 填充 / 去填充辅助 |

> 密钥长度需为 16 / 24 / 32 字节；`AesEncrypt` 系列以 `key` 前 16 字节作 IV。

#### DES / 3DES
| 函数 | 说明 |
| --- | --- |
| `DesEncrypt / DesDecrypt(origData, key []byte) ([]byte, error)` | DES CBC，key 需 8 字节 |
| `DesEcbEncrypt / DesEcbDecrypt(src, key []byte) ([]byte, error)` | DES ECB |
| `DESEncrypt / DESDecrypt(data, key string) string` | string 封装 |
| `TripleDesEncrypt / TripleDesDecrypt(origData, key []byte, pcks5 bool) ([]byte, error)` | 3DES CBC，IV = `key[:8]` |
| `TripleEcbDesEncrypt / TripleEcbDesDecrypt(origData, key []byte) ([]byte, error)` | 3DES ECB（k1/k2/k3 各 8 字节） |

#### RSA
- 便捷函数（自动 Base64 编解码）：`PublicEncrypt(data, publicKey string) (string, error)`、`PriKeyEncrypt`、`PublicDecrypt`、`PriKeyDecrypt`。
- 实例方式：包级全局 `var RSA = &RSASecurity{}`。
  - `SetPublicKey(str)` / `SetPrivateKey(str)` 加载 PEM（支持 PKIX / PKCS1 / PKCS8）；
  - `PubKeyENCTYPT / PriKeyENCTYPT / PubKeyDECRYPT / PriKeyDECRYPT(input []byte)` 加解密；
  - `SignSha1WithRsa / SignSha256WithRsa(data string) (string, error)` → Base64 签名；
  - `SignSha256WithRsaHex`（Hex）/ `SignSha256WithRsaUrlSafe`（URL 安全 Base64）；
  - `VerifySignSha1WithRsa / VerifySignSha256WithRsa(data, sign string) error` 验签。

#### 摘要与 HMAC
| 函数 | 说明 |
| --- | --- |
| `MD5Encode(content string) string` | Hex 小写 32 位 |
| `FileMD5(filename string) (string, error)` | 文件 MD5 |
| `MapMD5(m map[string]string) string` | 按键字典序拼接 `k=v&`（排除 `sign` 与空值）后 MD5，常用于签名 |
| `Sha1(src)` / `Sha256(src) string` | Hex 小写 |
| `FileSha256(filename string) (string, error)` | 文件 SHA256 |
| `HmacSHA1 / HmacSHA256(key, data []byte) []byte` | 原始字节 |
| `HmacSHA1Hex / HmacSHA256Hex(key, data string) string` | Hex |
| `HmacSHA1Base64 / HmacSHA256Base64(key, data string) string` | 标准 Base64 |

#### Base64 / JWT
- `Base64Encode(str) string` / `Base64Decode(str) string`：标准 `StdEncoding`（带填充）。
- `GenerateToken(claims jwt.MapClaims) (string, error)`：HS256 签名，固定 `exp = now+24h`，密钥取 `config.Config.Jwt.Secret`。
- `ValidateToken(tokenStr string) (*jwt.Token, error)`：校验令牌。

### 16.2 字符串与中文（String）
| 函数 | 说明 |
| --- | --- |
| `IsEmpty / IsNotEmpty / IsBlank / IsNotBlank(text)` | 空 / 空白判定 |
| `Left / Right(src, size)`、`LeftJustin / RightJustin / CenterJustin` | 截断 / 补齐 |
| `Length(text) int` | 按 rune 计长（合并变音标记） |
| `Lowercase / Uppercase(s)`、`UpperFirst / LowerFirst(s)`、`UpperWord(s)` | 大小写 |
| `IsNumeric / IsAlphabet / IsAlphaNum` | 字符类别 |
| `StrPos / BytePos / RunePos`、`IsStartOf / IsEndOf` | 查找 |
| `ReplacePunctuation(src, rep)`、`AddSpaceBetweenCharsAndNumbers` | 标点处理 |
| `AnyToString(i any) (string, error)`、`StringToAny[T](src) (T, error)` | 泛型类型转换 |
| `SubChineseString(str, begin, length)`、`ChineseLength(str)`、`IsChinese(str)` | 中文按 rune 安全截取 |
| `DBCtoSBC(s)` | 全角转半角 |
| `UnicodeEmojiDecode / UnicodeEmojiCode / TrimEmoji(s)` | Emoji 编解码 / 清除 |

### 16.3 时间日期（Time）
| 函数 | 说明 |
| --- | --- |
| `ToDateTimeString / ToDateString / ToTimeString(t)` | `2006-01-02 15:04:05` / 日期 / 时间 |
| `GetNowDateTime() / GetDate()` | 以 **CST（+08:00）** 返回当前时间串 |
| `DateFormat(t, format)`、`ConvertDateFormat(str, format)` | 自定义 / 解析格式化（`yyyy/MM/dd` 等） |
| `GormTimeFormat(t string)` | GORM 时间串规整（去 `T` 与 `+08:00`） |
| `TimeSubDays(t1, t2) int`、`WeekByDate(t) int` | 相差天数 / 年内周次 |
| `Get0Hour / Get0Yesterday / Get0Tomorrow / Get0Week / Get0Month ...` | 各粒度归零时间点 |
| `GroupByWeekDate(start, end) []WeekDate` | 按周切片 |
| `WaitNextMinute()` | 阻塞至下一分钟 0 秒 |

### 16.4 序列化（JSON / XML / YAML）
| 函数 | 说明 |
| --- | --- |
| `ToJSON(o any) string` | 序列化（还原 `<` `>` `&` 转义） |
| `ToJSONPretty(o any) string` | 美化（2 空格缩进） |
| `FromJSON(j string, o any) *any` | 反序列化（失败记日志返回 nil） |
| `JSONPretty / CompactJSON(in string) string` | 美化 / 压缩 |
| `DecodeXMLToMap(r)` / `EncodeXMLFromMap(w, m, root)` | XML ↔ map |
| `LoadYaml(filename, cfg)` / `StoreYaml(filename, cfg)` | YAML 读写（写时自动建目录） |

### 16.5 Map 与 Struct 转换
| 函数 | 说明 |
| --- | --- |
| `MapItoS / MapStoI`、`Exists / Existi` | map 互转、键存在判断 |
| `Map2Struct(input, output)` | `mapstructure` 弱类型转换（含 duration hook） |
| `MapGet(input, field) interface{}` | 点号嵌套取值（`a.b.c`） |
| `Struct2StringMap(obj)` / `AnyToMap(obj)` | 结构体转 `map[string]string` |
| `Struct2Map(obj)` / `Struct2MapString(obj)` | 结构体转 map（递归） |
| `Clone(src, dst)` / `CopyStruct(src, dst)` | JSON 深拷贝 / 反射逐字段拷贝 |
| `DeepCopy[D any](src any) D` | 泛型深拷贝 |
| `GetStructFields / GetStructJsonTags(obj)` | 反射取字段名 / json tag |
| `SortMapByValue / SortMapByValueDesc(src)` | 按值排序（值需为 float64） |

### 16.6 集合与切片（Slice / Set）
| 函数 | 说明 |
| --- | --- |
| `StringArrayContains / IntArrayContains / Float64ArrayContains` | 包含判定 |
| `StringArrayDelete / IntArrayDelete / ...` | 按索引删除 |
| `SliceContains / SliceContainsInt / SliceContainsInt64 / SliceContainsString` | 泛型包含 |
| `SliceMerge(Int/Int64/String)`、`SliceUnique(Int/Int64/String)` | 合并 / 去重 |
| `SliceSumInt / SliceSumInt64 / SliceSumFloat64` | 求和 |
| `ArrayStr2Int / ArrayInt2Str`、`UnSplitString(src, sep)` | 类型互转 / 拼接 |
| `UnionStringSlice / IntersectStringSlice / DifferenceStringSlice` | 并集 / 交集 / 差集 |
| `StringUnique / Int64Unique`、`TrimSpaceStrInArray` | 去重 / 去空格判定 |
| `NewHashSet() *HashSet`（`Add / Exists / Remove / Members`） | 字符串集合 |

### 16.7 网络与 IP（Net / IP）
| 函数 | 说明 |
| --- | --- |
| `GetLocalIpAddress() string` | 首个非回环、非 `169.254.x` 的 IPv4，否则 `127.0.0.1` |
| `LocalIPv4s() ([]string, error)`、`GetIPv4ByInterface(name)` | 全部 / 指定网卡 IPv4 |
| `IsIntranetIP(ip) bool` | 内网判定（`10.*` / `192.168.*` / `172.16-31.*`） |
| `IsPortUse(port int) bool` | 端口占用判定（注意：输出非空时返回 `true`） |
| `UrlEncode(raw) string` / `UrlDecode(encoded) (string, error)` | RFC3986 风格编解码 |

### 16.8 文件与压缩（File / Zip）
| 函数 | 说明 |
| --- | --- |
| `SelfPath() / SelfDir()`、`Basename / Dir / Ext(file)` | 路径解析 |
| `InsureDir(path)`、`IsFile / IsDir / IsExist(path)` | 目录确保 / 存在判定 |
| `ReadFileToBytes / ReadFileToString`、`WriteBytesToFile / WriteStringToFile` | 读写（写时自动建目录） |
| `FileSize / FileMTime`、`DirsUnder / FilesUnder(dir)` | 文件信息 / 列举 |
| `SearchFile(filename, paths...)`、`RealPath(file)` | 查找 / 真实路径 |
| `DownloadFile(fileUrl, localPath) (string, error)` | HTTP 下载（`grequests`） |
| `SftpConnect / SftpUploadFile / SftpClose` | SFTP 上传（密码认证） |
| `ZipFiles(filename, files, srcpath, aliasnames)` | 多文件 ZIP |
| `Compress / Decompress(data []byte)` | Gzip 压缩 / 解压 |
| `Utf8ToGbk / GbkToUtf8`、`ClearUtf8BOM(str)` | 编码转换 / 去 BOM |

### 16.9 UUID 与随机（UUID / Rand）
| 函数 | 说明 |
| --- | --- |
| `NewUUID() (UUID, error)` / `MustNewUUID() UUID` | 生成 v4 UUID |
| `UUIDFromString(s)`、`IsValidUUIDString(s)` | 解析 / 校验（RFC4122） |
| `UUID.String() / Simple()`、`NewUUIDString() / SimpleUUID()` | 标准 / 无分隔串 |
| `GetRandomString(l)` / `GetRandomCaseString(l)` / `GetRandomHexString(l)` / `GetRandomIntString(l)` | 随机串（数字 / 大小写+符号 / 十六进制 / 纯数字） |
| `GenerateRandString(source, l)` | 自定义字符集随机串 |
| `GetUUIDString()` | 基于 `gofrs/uuid` 的 v4 串 |

### 16.10 并发安全与数据结构（Concurrent / DS）
| 函数 / 类型 | 说明 |
| --- | --- |
| `NewSafeGo(fn)`（`SetGoBeforeHandler / SetCallBeforeHandler / Run`） | panic 安全协程（recover 并打印彩色堆栈） |
| `GetGoroutineID() uint64` | 当前 goroutine ID（调试用） |
| `Map[T any]`（`Load / Store / Range / Delete / LoadAndStore / LoadAndDelete / Len / Clear`） | 泛型并发安全 map |
| `LinkList[T any]`（`Add / Push / Pop / Enqueue / Dequeue / Get / GetAll / Walk / Size`） | 泛型双向链表 |
| `RingBuffer[T any]`（`Write / Read / Latest / Oldest / Overwrite`） | 泛型环形缓冲（覆盖式） |
| `HashSet`（`Add / Exists / Remove / Members`） | 字符串集合（见 16.6） |
| `Values`（`Put / Get / GetAll / Merge / Clear`） | 并发键值容器 |
| `ExpireCache`（`Store / Load / Delete`，`Timeout` 秒） | 带过期的内存缓存 |
| `NewLimitQueue()` + `LimitFreqSingle(queue, count, window) bool` | 单机滑动窗口限流 |

### 16.11 校验与防护（Validate）
| 函数 | 说明 |
| --- | --- |
| `IsChinaMobile / Mail / UserName / Nickname / ChineseName(...)` | 手机号 / 邮箱 / 用户名 / 昵称 / 中文名（含 `...String` 与 `[]byte` 两种入参） |
| `IsChineseNameEx(s) (string, bool)` | 不规范间隔符自动规整为 `·` 并返回修正结果 |
| `IsIdCard(cardNo string) bool` | 15 / 18 位身份证（末位可 X） |
| `CheckSqlValidate(content string) (bool, string)` | SQL 注入关键字黑名单（命中返回疑似串） |
| `AddPortsToFirewall(ports []int)` | linux `firewall-cmd` 放通端口（仅 linux） |

### 16.12 其它（Misc）
| 函数 | 说明 |
| --- | --- |
| `AppName() / AppDir()` | 可执行文件名 / 所在目录 |
| `GinParamMap(c *gin.Context) map[string]string` | 聚合 GET Query 与 POST 表单参数 |
| `GinHeaders(c *gin.Context) map[string]string` | 请求头转 map |
| `CmdExec(name, arg...) (string, error)`、`CmdRunWithTimeout(cmd, timeout)` | 执行系统命令 |
| `DisplaySize(raw float64) string` | 字节数 → 人类可读（`B/K/M/G/T/P/E`） |
| `IfThen / IfThenElse / DefaultIfNil / FirstNonNil` | 条件 / 空值取值工具 |

> ⚠️ **实现备注（供生产参考）**：经源码核对，`utils` 部分函数存在已知边界问题 —— `LinkList.Get` 越界会 `panic`；`ExpireCache.checkExpire` 存在误用 value 作 key 的清理 bug；`sqlvalidate` 仅为关键字黑名单且首字符命中会漏判；`SftpConnect` 的 `HostKeyCallback` 恒返回 nil（存在中间人风险）；`IsPortUse` 语义与命名相反（输出非空返回 `true`）。生产使用前建议阅读对应源码。


















---

**第七部分：参考与分支**　（第 17–20 章）

## 17. 最佳实践

1. **统一返回**：所有 Controller 一律返回 `models.Result[T]`（或 `i18n.Error/ErrorT`），前端按 `status` 判成功，避免散落的 HTTP 状态码。
2. **参数校验**：优先使用 `i18n.CheckParametersLost(params, "a","b")` 做非空校验，配合 `utils` 的手机号/邮箱校验。
3. **DAO 优于裸 SQL**：业务用 `dao.MySQLDao[T]` 等泛型 DAO，多库通过 `Tag` 闭包路由，减少样板代码。
4. **MongoDB 连接归还**：`db.Mongo.GetConnection()` 返回的是 session Copy，**务必 `defer db.Mongo.ReturnConnection(db)`**。
5. **Redis 缓存层**：启用 MySQL 二级缓存时建议同时启用 Redis，缓存命中率更高且支持多实例共享。
6. **链路追踪**：跨服务调用务必通过 `client.Call/CallT`，其自动透传 trace header（`X-Request-ID`），配合 `postlog` 日志可串联全链路。
7. **优雅关闭**：不要自管 `os.Signal`，交给 `app.Run()`，它会先 `SafeExit()` 再 `Shutdown`。
8. **多环境配置**：用 `go.config.env` + `<prefix>-<env>.yml` 分离环境，敏感信息走 Nacos/Polaris 等配置中心。
9. **中间件顺序**：`jwt` / `casbin` 等鉴权中间件应放在 `postlog` 之后、业务路由之前；白名单路径（swagger/doc）已在 jwt/casbin 中预设。
10. **XSS**：对外接收富文本/用户内容的接口建议挂载 `xss.RemoveXss()`，使用 `UGCPolicy`。



---

## 18. 常见问题（FAQ）

**Q：组件启用了但连不上？**
A：检查 `go.config.used` 是否包含该组件名，且对应 `<prefix>-<env>.yml` 存在且命名正确；连接失败会在启动时打印 `加载{}失败，配置文件中未使用` 或 `{}配置错误`。

**Q：MongoDB 报 session 耗尽？**
A：未在 `GetConnection()` 后用 `ReturnConnection` 归还。务必 `defer db.Mongo.ReturnConnection(db)`。

**Q：微服务调用报“获取主机IP端口失败”？**
A：目标服务未注册到注册中心，或 `go.discovery.registry` 类型不匹配；确认服务名与分组正确。

**Q：i18n 文本没生效？**
A：`NewApp` 第四参数需为 `true`，且需配置 `x-lang` 微服务；内置中文走 `errcode` 常量兜底。

**Q：多库如何切换？**
A：配置 `multidb:true` + `dbNames`；运行时 `GetConnection("test2")` 或 DAO 的 `Tag` 闭包返回库名。`IsMultiDB()` / `ListConnNames()` 可用于判断。

**Q：Gin 模式怎么切生产？**
A：设 `go.application.debug=false`（或删掉），框架自动用 `gin.ReleaseMode`。



---

## 19. 版本与升级说明

- **v1.20.3**：内置系统管理模块，仅需 yml 开启，自动建表，自带 Swagger 文档。
- **v1.20.1**：新增 `App` 对象，极大简化创建一个新 MGin 应用。
- **v1.19.42**：持久化缓存改为 bitcask，并与内存缓存通过 `ICache` 接口标准化。
- **v1.19.38**：新增 Redis 断线重连 `PSubscribe`；Kafka 消费者增加重连。
- **v1.19.36**：Redis 支持 cluster 集群、哨兵模式与单机。
- **v1.19.21**：DAO 层查询单条无记录不返回 error，返回 nil。
- **v1.19.19**：新增 mongo/mysql 的 DAO，新增 `CopyStruct`。
- **v1.19.10**：mysql/mongo/redis 多库新增 `IsMultiDB` 与 `ListConnNames`。
- **v1.19.9**：postlog 支持多库按 header 切库。
- **v1.19.8**：`client.Options` 各参数改为 `any` 类型。
- **v1.19.7**：新增 `Struct2Map` 与 `AnyToMap`。
- **v1.19.5**：Nacos 订阅统一管理与统一退订。
- **v1.19.3**：增加跨域自定义标头支持。
- **v1.19.1**：`Result` 实现 any 与泛型 T 互转。
- **v1.19.0**：支持 Go 1.19，`Result` 改用泛型；重构 `client.Call`，支持泛型返回 `CallT[T]`。


> 本文档基于源码 `mgin.go / app.go / config / db / client / cache / middleware / i18n / errcode / logs / registry / models / utils` 编写，覆盖 MGin 的核心能力与常用 API。更多底层细节请直接阅读对应包源码。



---

## 20. jh 分支（寄海版）专项说明

> 本章针对 **`jh` 分支（寄海版）** 做专项补充说明。该分支是 MGin 的一个**精简变体**，相对 `master-1.25` 做了“删模块 + 加能力”两类改动。下文结论均基于 `jh` 分支源码（最新 tag `v1.25.11-jh`）核对。

### 20.1 分支定位与由来

- 分支命名 `jh` 取自「寄海」（Jihai）的拼音首字母。
- 设计目标：在保留 MGin 核心微服务能力的**同时，移除内置系统管理模块与 Swagger 文档**，从而**显著缩小编译产物体积**（删除约 1.6 万行 `sys` 相关代码）。
- 适用场景：不需要内置用户/权限体系、希望镜像体积更小或自行实现管理后台的服务。

### 20.2 与 master-1.25 的差异总览

| 维度 | master-1.25 | jh 分支 |
|---|---|---|
| 内置 sys 模块 | ✅ 有 | ❌ 已移除 |
| Swagger 文档 | ✅ `docs/` 内置 | ❌ 已移除 |
| MySQL/SQLite/ClickHouse 二级缓存 | ❌ | ✅ `UseCache()` |
| PostgreSQL 支持 | ❌ | ✅ `db.Pg` + `PostgresDao` |
| ClickHouse 支持 | ❌ | ✅ `db.Clickhouse` + `ClickhouseDao` |
| `NewApp` `-v` 版本参数 | ❌ | ✅ |
| GIN 运行模式 | 硬编码 `debug` | 由 `go.application.debug` 控制 |
| MongoDB 驱动 | `maczh/mgo` | 官方 `mongo-driver` |
| etcd 注册 | 普通注册 | 带租约 + keepalive 续租 |
| Nacos 注册 | 阿里云 SDK | 社区 `nacos-sdk-go` |
| CORS | 固定头 | 支持外部配置 |

### 20.3 已移除的能力（重要）

在 `jh` 分支中，以下目录与能力已被**整体删除**，请勿依赖：

- `sys/controller`、`sys/dao`、`sys/service`、`sys/route`、`sys/middle` 整套内置系统管理（用户、角色、部门、字典、岗位、API、资源、角色-API、角色-资源、在线用户、第三方登录、动态配置、验证码等）。
- Swagger：`docs/docs.go`、`docs/swagger.json`、`docs/swagger.yaml` 及 `app.go` 中的 Swagger 路由注册。
- `app.go` 中 `baseRouter()` 不再注册 sys 路由与 `JwtAuthorize`/`ApiAuth` 中间件。

因此：

- 配置项 `go.sys.*`（enabled / initdb / baseUri / casbin / swagger）在 `jh` 分支**不生效**；
- Casbin 仍可作为独立中间件 `middleware/casbin` + `go.casbin.enabled` 使用；
- 若业务需要内置用户/权限体系，请切换到 `master-1.25`（或自行合并 sys 模块）。

### 20.4 新增：GORM 二级缓存层（UseCache）

`jh` 分支为关系型数据库客户端新增了基于 [go-gorm/caches/v4](https://github.com/go-gorm/caches) 的读缓存：

- 实现：新增 `db/mysql/redisCacher.go` 定义 `RedisCacher`（实现 `Get/Store/Invalidate`，底层用 Redis 存储）。
- 开关与过期：各客户端提供 `UseCache() bool`，按配置决定是否挂载缓存插件：

| 客户端 | 开关配置键 | 过期配置键 | 默认过期 | 无 Redis 时 |
|---|---|---|---|---|
| MySQL | `go.data.mysql_cache` | `go.data.mysql_cache_expired`（秒） | 300s(5min) | 进程内内存缓存 |
| SQLite | 始终（若启用） | 固定 300s | 300s | 进程内内存缓存 |
| PostgreSQL | `go.data.postgres.cache.enabled` | `go.data.postgres.cache.expired`（秒） | 300s(5min) | 进程内内存缓存 |
| ClickHouse | `go.data.clickhouse_cache` | — | 见 20.8 | 进程内内存缓存 |

- 用法（建议 `mgin.Init` 之后、业务请求之前调用一次）：

  ```go
  db.Mysql.UseCache()      // MySQL
  db.Pg.UseCache()         // PostgreSQL
  db.Sqlite.UseCache()     // SQLite
  db.Clickhouse.UseCache() // ClickHouse
  ```

- 注意：缓存为**只读查询缓存**，写入/更新会触发 `Invalidate`（按 `caches.IdentifierPrefix` 前缀扫描删除），适合读多写少场景。

### 20.5 新增：PostgreSQL 支持

- 全局客户端：`db.Pg = &postgres.PostgresClient{}`（包 `github.com/maczh/mgin/v2/db/postgres`）。
- 配置前缀：`go.config.prefix.postgres`（默认可不配，组件级配置键见 `go.data.postgres.*`）。
- 泛型 DAO：`PostgresDao[E schema.Tabler]`，API 与 `MySQLDao` 完全一致：`Create / MultiCreate / Delete / Updates / Save / All / One / Exists / Count / Pager / Where / Debug / WithContext`。

  ```go
  dao := &dao.PostgresDao[User]{}
  dao.Tag = func() string { return "testdb" } // 多库时指定库名
  _ = dao.Create(&user)
  ```

- 连接配置 `postgres-test.yml` 见 [5.2](#52-postgresql)。

### 20.6 新增：ClickHouse 支持

- 全局客户端：`db.Clickhouse = &clickhouse.ClickhouseClient{}`（包 `github.com/maczh/mgin/v2/db/clickhouse`）。
- 泛型 DAO：`ClickhouseDao[E schema.Tabler]`，API 与 `MySQLDao` 一致。
- 配置前缀：`go.config.prefix.clickhouse`，组件级 DSN 键 `go.data.clickhouse`（单库）或 `go.data.clickhouse.<dbName>`（多库）。

  ```go
  dao := &dao.ClickhouseDao[Event]{}
  _ = dao.Create(&event)
  rows, page, _ := dao.Pager(db.Clickhouse.GetConnection(), 1, 20)
  ```

- 二级缓存：`UseCache()` 读取 `go.data.clickhouse_cache`，有 Redis 时使用 `RedisCacher`；否则回退内存。

### 20.7 其他改进（jh 分支）

1. **版本参数 `-v` 与 `GetVersion()`**：`NewApp(configFile, appName, version string, xlang bool)` 新增命令行 `-v` 开关，传入时打印 `appName, 版本号: version` 并直接退出；运行时可用 `app.GetVersion()` 获取版本。
2. **GIN 运行模式**：不再硬编码 `debug`，改为 `go.application.debug=true` 时使用 `gin.DebugMode`，否则 `gin.ReleaseMode`（生产建议关闭 debug）。
3. **etcd 注册增强**：新增租约（lease）与 keepalive 自动续租（v1.21.17-jh），避免实例离线后注册信息残留。
4. **Nacos 注册改造**：改用社区 `nacos-sdk-go`，不再依赖阿里云 SDK（v1.21.15-jh），降低依赖体积与冲突风险。
5. **外部 CORS 配置**：允许通过配置注入跨域规则，而非仅内置固定头（v1.21.15-jh）。
6. **MongoDB 驱动替换**：彻底替换 `maczh/mgo` 为官方 `go.mongodb.org/mongo-driver`（v1.25.2-jh）。

### 20.8 已知问题与注意事项

- **⚠️ ClickHouse `ClickhouseDao.MultiCreate` 缺陷**：当前实现（截至 `v1.25.11-jh`）在批量插入时误用 `db.Mysql.GetConnection(receiver.Tag())`，会导致批量数据被写入 **MySQL** 而非 ClickHouse。在官方修复前，请避免对 ClickHouse 使用 `MultiCreate`，改用单条 `Create` 或在业务层直接取 `db.Clickhouse.GetConnection()` 操作。
- **ClickHouse 缓存过期**：`UseCache()` 构造 `RedisCacher` 时**未设置 `Expiration`**（区别于 MySQL/Postgres 的 5 分钟默认），等价于 Redis `SET` 不带过期，缓存键**不会自动过期**。如需 TTL，需自行在 `redisCacher.go` 中补充 `Expiration`。
- **PostgreSQL 缓存键层级**：缓存开关是 `go.data.postgres.cache.enabled`（嵌套 `cache` 下），并非顶层 `go.data.postgres.cache`。
- **依赖变化**：`jh` 分支 `go.mod` 新增 `github.com/go-gorm/caches/v4`、`gorm.io/driver/postgres`、`gorm.io/driver/clickhouse` 等，且 Redis 客户端为 `github.com/go-redis/redis/v7`（与 `RedisCacher` 对应）。升级时注意最小 Go 版本与依赖冲突。

### 20.9 升级 / 迁移建议

- 从 `master-1.25` 迁移到 `jh`：若原项目依赖内置 `sys` 模块（用户/权限/验证码）或 Swagger，**不可直接切换**——需自行补齐管理后台，或保留 `master-1.25`。其余 Web/配置/注册/缓存/DAO 用法基本兼容。
- 使用二级缓存：在 `mgin.Init` 后调用对应 `UseCache()`；确保 `go.config.used` 包含 `redis`（以启用 Redis 缓存层，否则为进程内缓存）。
- 启用 PostgreSQL/ClickHouse：在 `go.config.used` 中加入 `postgres` / `clickhouse`，并配置对应 `<prefix>-<env>.yml`。

---

## 21. 新增：单例限流中间件（ratelimit）

`jh` 分支内置了一个**配置驱动、单例管理**的限流中间件 `middleware/ratelimit`，支持多种算法与多维度限流，适用于保护接口不被突发流量打垮。

### 21.1 特性

- **多算法可选**：令牌桶（token_bucket）、滑动日志窗口（sliding_window）、最大并发数（concurrency）。
- **多维度限流**：全局（global）、按 IP（ip）、按路径（path）、按 IP+路径（ip_path）、按请求头（header）。
- **规则化**：每条规则可独立指定算法、维度、阈值、限流响应码与提示文案。
- **白名单**：支持按 IP / 前缀路径放行，不受限流约束。
- **空闲回收**：长时间不命中的限流器会被后台 GC 协程回收，避免按 IP/路径限流导致内存无限增长。
- **编程式限流**：除 HTTP 中间件外，还提供 `Allow(key, rule)` 供非 HTTP 场景（如消费队列、定时任务）复用同一套限流逻辑。

### 21.2 配置（application.yml）

在 `go.config.used` 中加入 `ratelimit`，并在 `go.ratelimit` 节点下配置：

```yaml
go:
  config:
    used: "...,ratelimit"
    prefix:
      ratelimit: "go.ratelimit"
  ratelimit:
    enabled: true
    # 空闲限流器回收间隔（秒），默认 600
    idleTimeout: 600
    # 全局白名单开关（满足任一白名单即放行）
    whitelist:
      - "/health"
      - "/actuator"
    whiteIps:
      - "127.0.0.1"
    # 规则列表，按声明顺序匹配，命中即生效
    rules:
      - name: "全局令牌桶"
        algorithm: "token_bucket"     # token_bucket | sliding_window | concurrency
        dimension: "global"           # global | ip | path | ip_path | header
        rate: 100                     # 令牌桶：每秒补充令牌数；滑动窗口：窗口内允许次数
        burst: 20                     # 令牌桶：突发容量（仅 token_bucket）
        window: 1                     # 滑动窗口：窗口大小（秒，仅 sliding_window）
        httpStatus: 429              # 触发限流时的 HTTP 状态码
        code: 1011                    # 触发限流时的业务错误码
        message: "请求过于频繁，请稍后再试"
      - name: "登录接口按 IP 限流"
        path: "/api/login/*"          # 支持精确匹配、前缀"/*"、Gin 路由模板
        methods: ["POST"]
        algorithm: "sliding_window"
        dimension: "ip"
        rate: 5                       # 每个 IP 在 60 秒内最多 5 次
        window: 60
        httpStatus: 429
        code: 1011
        message: "登录尝试过于频繁"
      - name: "上传并发限制"
        path: "/api/upload"
        algorithm: "concurrency"
        dimension: "global"
        maxConcurrent: 10             # 同时最多 10 个上传在进行
```

### 21.3 使用方式

```go
import "github.com/maczh/mgin/v2/middleware/ratelimit"

// 方式一：读取 yml 配置（go.ratelimit.enabled 为 true 时生效）
app.Router.Use(ratelimit.RateLimit())

// 方式二：纯代码指定规则（不走配置文件）
app.Router.Use(ratelimit.RateLimitWith(ratelimit.Rule{
    Name:      "全局并发",
    Algorithm: ratelimit.AlgoConcurrency,
    Dimension: ratelimit.DimGlobal,
    MaxConcurrent: 50,
}))
```

触发限流时，中间件会写入 `X-RateLimit-Limit` / `X-RateLimit-Remaining` / `X-RateLimit-Retry-After` 响应头，并返回配置的 `httpStatus` 与 `code/message`。

非 HTTP 场景编程式限流：

```go
ok, release := ratelimit.Allow("my-key", ratelimit.Rule{
    Algorithm: ratelimit.AlgoTokenBucket,
    Rate:      10,
    Burst:     5,
})
if !ok {
    return errors.New("限流")
}
defer release()
// ... 受保护的业务逻辑
```

---

## 22. 新增：定时任务管理器（job）

`jh` 分支内置了一个**类 xxl-job** 的定时任务管理器 `job`，任务配置与执行日志持久化在当前 GORM 数据库中（按 **MySQL → PostgreSQL → SQLite** 优先级自动选择连接），纯单机调度。

### 22.1 启用

在 `go.config.used` 中加入 `job`。调度器在 `mgin.Init` 阶段自动启动（前提是 `go.job.enabled: true`）；所有执行器需在此之前通过 `job.Register` 注册：

```yaml
go:
  config:
    used: "mysql,job"
  job:
    enabled: true
    dbName: ""          # 多库模式指定库名；单库留空
    initdb: true        # 是否自动建表（默认 true）
    tablePrefix: "mgin_"# 表名前缀（默认 mgin_）
    scanInterval: 1     # 调度扫描间隔（秒）
    refreshInterval: 30 # 从数据库同步任务配置的间隔（秒）
    logRetainDays: 30   # 执行日志保留天数，0 不清理
    maxConcurrent: 50   # 全局最大并发执行数
    maxSerialQueue: 10  # serial 策略最大排队数
    timezone: "Asia/Shanghai"  # 调度时区
```

建表：`mgin_job_info`（任务配置）、`mgin_job_log`（执行日志）。

### 22.2 注册执行器

执行器即一个 `func(*job.Context) error`，通过 `job.Register(name, handler)` 注册，`name` 需与数据库 `handler_name` 字段一致：

```go
import "github.com/maczh/mgin/v2/job"

func init() {
    job.Register("syncUserJob", func(ctx *job.Context) error {
        ctx.Log("开始同步用户, 参数=%s", ctx.Param)
        if err := userService.Sync(ctx.Ctx()); err != nil {
            return err // 返回 error 触发重试
        }
        return nil
    })
}
```

`job.Context` 提供：`Ctx()`（标准库 context，支持超时取消）、`Log(format, ...)`（写入执行日志明细）、`ParamMap()`（解析 `a=1&b=2` 参数）、`Done()` / `Err()`（感知取消）。

### 22.3 调度类型与策略

| 配置项 | 取值 | 说明 |
|---|---|---|
| scheduleType | `cron` | cron 表达式（支持 5 段、6 段、`@every 5m`、`@daily`） |
| | `fixed_rate` | 间隔秒数，从上次**开始执行**起算 |
| | `fixed_delay` | 间隔秒数，从上次**执行结束**起算 |
| | `once` | 一次性，配置执行时间 `2006-01-02 15:04:05` |
| blockStrategy | `serial` / `discard` / `concurrent` / `cover` | 上一次未结束时的处理方式 |
| misfireStrategy | `do_nothing` / `fire_now` | 错过调度时刻是否立即补偿 |
| timeout / retryCount / retryInterval | 秒 / 次数 / 秒 | 超时中断、失败重试 |

### 22.4 管理接口（Gin 路由组）

调用 `job.RouterGroup(r)` 将管理接口挂载到任意路由组下（如 `/job`）：

```go
app.Router.Use(...) // 前置鉴权中间件
job.GetManager()    // 确保管理器已初始化
job.RouterGroup(app.Router.Group("/job"))
```

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/job/list?group=&keyword=&status=&index=&size=` | 任务列表（分页） |
| GET | `/job/:id` | 任务详情 |
| POST | `/job` | 新增任务 |
| PUT | `/job` | 更新任务 |
| DELETE | `/job/:id` | 删除任务 |
| POST | `/job/:id/start` | 启动任务 |
| POST | `/job/:id/stop` | 停止任务 |
| POST | `/job/:id/trigger` | 手动触发一次（`{"param":"可选覆盖参数"}`） |
| GET | `/job/handlers` | 已注册执行器列表 |
| GET | `/job/log?jobId=&jobName=&status=&index=&size=` | 执行日志列表（分页） |

所有接口统一返回 `models.Result`（成功 `status:1`）。

### 22.5 健康检查与关闭

`mgin` 已在 `checkAll`（每 5 分钟）中对运行中的调度器做 DB 探活，并在 `SafeExit` 时调用 `job.Stop()` 优雅等待在途任务结束。也可在代码中手动控制：

```go
job.Start()        // 启动（若未在 mgin.Init 中启动）
job.Stop()         // 停止
job.GetManager().IsRunning() // 是否运行中
logID, err := job.GetManager().Trigger(id, "") // 手动触发
```

---

## 23. 新增：S3 对象存储插件（storage/s3）

`jh` 分支内置 S3 对象存储插件 `storage/s3`，基于 **aws-sdk-go-v2**，兼容 AWS S3 与 MinIO 等 S3 兼容服务，支持多 bucket、分片上传、预签名 URL。

### 23.1 启用

在 `go.config.used` 中加入 `s3`。插件在 `mgin.Init` 阶段读取 `go.s3` 节点完成初始化，并在 `SafeExit` 时统一关闭：

```yaml
go:
  config:
    used: "...,s3"
  s3:
    enabled: true
    endpoint: "https://s3.amazonaws.com"   # MinIO 示例: http://localhost:9000
    region: "cn-north-1"
    accessKey: "AKIDEXAMPLE"
    secretKey: "SECRET"
    # sessionToken: ""                      # 临时凭证时填写
    pathStyle: true                         # MinIO 等自建服务必须为 true
    ssl: true
    maxRetries: 3
    uploadPartSize: 16777216                # 分片大小（字节），默认 16MB
    downloadPartSize: 16777216
    maxUploadParts: 10
    maxDownloadParts: 10
    presignExpiry: 3600                     # 预签名 URL 有效期（秒）
    singleBucket: "my-bucket"               # 单桶模式（不配置 buckets 时使用）
    buckets:                                # 多桶模式
      - name: "public-assets"
        public: true
        defaultContentType: "image/png"
      - name: "private-files"
        public: false
```

### 23.2 基本使用

```go
import "github.com/maczh/mgin/v2/storage/s3"

b := s3.GetS3().Default()        // 默认桶（singleBucket 或 buckets[0]）
b = s3.GetS3().Get("public-assets") // 指定桶

// 上传
_ = b.Upload(ctx, "avatars/1.png", bytes.NewReader(data), "image/png")

// 下载
buf := manager.NewWriteAtBuffer([]byte{})
_ = b.Download(ctx, "avatars/1.png", buf)

// 删除 / 判断存在
_ = b.Delete(ctx, "avatars/1.png")
exists, _ := b.Exists(ctx, "avatars/1.png")

// 列举
list, _ := b.List(ctx, "avatars/", 100)

// 预签名下载 / 上传 URL（客户端直传直下）
url, _ := b.Presign(ctx, "avatars/1.png")
upUrl, _ := b.PresignUpload(ctx, "avatars/2.png")

// 分片上传（超大文件）
etag, _ := b.UploadMultipart(ctx, "big.iso", "", bytes.NewReader(huge), 16*1024*1024)
```

> 说明：`Upload` 对传入的 `*bytes.Reader` 会自动判断是否超过单片大小并改用分片上传；大文件下载由 `manager.Downloader` 自动分片。ContentType 缺省时按扩展名推断，再缺省为 `application/octet-stream`。

---

## 24. 新增能力汇总（jh 分支）

下表汇总 `jh` 分支在 `master-1.25` 之外的**新增能力**（含上文 21–23 章）：

| 能力 | 包 / 入口 | 启用开关 | 说明 |
|---|---|---|---|
| GORM 二级缓存 | `db.Mysql.UseCache()` 等 | `go.data.*.cache.enabled` | 见 20.4 |
| PostgreSQL 支持 | `db.Pg` / `PostgresDao` | `postgres` | 见 20.5 |
| ClickHouse 支持 | `db.Clickhouse` / `ClickhouseDao` | `clickhouse` | 见 20.6 |
| 单例限流中间件 | `middleware/ratelimit` | `ratelimit` | 多算法 + 多维度，见 21 |
| 定时任务管理器 | `job` | `job` | 类 xxl-job，DB 持久化，见 22 |
| S3 对象存储插件 | `storage/s3` | `s3` | aws-sdk-go-v2，兼容 MinIO，见 23 |

上述新增能力均通过 `go.config.used` 中的开关按需启用，未启用时不会产生任何连接或副作用。
