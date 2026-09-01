# MGin Microservice Framework — Application Documentation

> Version: based on `github.com/maczh/mgin` (Go 1.25), latest features v1.20.x
> Audience: engineers building, developing, and operating RESTful microservices with MGin
> Module path: `github.com/maczh/mgin`

## Table of Contents


**Part I — Getting Started & Architecture**

1. [Framework Overview](#1-framework-overview)
2. [Quick Start](#2-quick-start)
   - 2.1 [Install](#21-install)
   - 2.2 [Minimal Runnable Example](#22-minimal-runnable-example)
   - 2.3 [Configuration File](#23-configuration-file)
3. [Application Lifecycle](#3-application-lifecycle)
   - 3.1 [Core Entry Object `App`](#31-core-entry-object-app)
   - 3.2 [Startup and Graceful Shutdown](#32-startup-and-graceful-shutdown)
   - 3.3 [Plug-in External Data Sources](#33-plug-in-external-data-sources)

**Part II — Configuration**

4. [Configuration System (application.yml)](#4-configuration-system-applicationyml)
   - 4.1 [Overview and Switches](#41-overview-and-switches)
   - 4.2 [Application and Base Configuration](#42-application-and-base-configuration)
   - 4.3 [Configuration Center](#43-configuration-center)
   - 4.4 [Built-in System Module Switches](#44-built-in-system-module-switches)
   - 4.5 [Configuration Reading API](#45-configuration-reading-api)
5. [Unified Configuration Center](#5-unified-configuration-center)

**Part III — Data Access**

6. [Databases and Message Middleware](#6-databases-and-message-middleware)
   - 6.1 [MySQL](#61-mysql)
   - 6.2 [PostgreSQL](#62-postgresql)
   - 6.3 [SQLite](#63-sqlite)
   - 6.4 [MongoDB](#64-mongodb)
   - 6.5 [Redis](#65-redis)
   - 6.6 [ClickHouse](#66-clickhouse)
   - 6.7 [ElasticSearch](#67-elasticsearch)
   - 6.8 [Kafka](#68-kafka)
7. [DAO Generic Data Access Layer](#7-dao-generic-data-access-layer)
   - 7.1 [Relational DAO (MySQL example)](#71-relational-dao-mysql-example)
   - 7.2 [MongoDB DAO](#72-mongodb-dao)
8. [Cache](#8-cache)

**Part IV — Services & Communication**

9. [Microservice Client](#9-microservice-client)
   - 9.1 [Options and Protocols](#91-options-and-protocols)
   - 9.2 [Call Examples](#92-call-examples)
10. [Service Registry and Discovery](#10-service-registry-and-discovery)

**Part V — Request & Cross-cutting**

11. [Middleware](#11-middleware)
   - 11.1 [CORS](#111-cors)
   - 11.2 [JWT Authorization](#112-jwt-authorization)
   - 11.3 [Casbin Interface Authorization](#113-casbin-interface-authorization)
   - 11.4 [IP Restriction (CIDR)](#114-ip-restriction-cidr)
   - 11.5 [Concurrency Rate Limiting](#115-concurrency-rate-limiting)
   - 11.6 [Request Access Logging](#116-request-access-logging)
   - 11.7 [Session](#117-session)
   - 11.8 [Trace / Request Tracking](#118-trace--request-tracking)
   - 11.9 [Internationalization Language](#119-internationalization-language)
   - 11.10 [XSS Protection](#1110-xss-protection)
12. [Internationalization and Error Codes (i18n / errcode)](#12-internationalization-and-error-codes-i18n--errcode)
   - 12.1 [Error Code Constants (`errcode` package)](#121-error-code-constants-errcode-package)
   - 12.2 [i18n Responses (require `NewApp(..., true)`)](#122-i18n-responses-require-newapp-true)
13. [Unified Response Structure (Result)](#13-unified-response-structure-result)
14. [Logging](#14-logging)

**Part VI — Built-in Modules & Utilities**

15. [Built-in System Module and Casbin Authorization](#15-built-in-system-module-and-casbin-authorization)
   - 15.1 [Enable](#151-enable)
   - 15.2 [Built-in Data Models (table names)](#152-built-in-data-models-table-names)
   - 15.3 [Authorization Models](#153-authorization-models)
   - 15.4 [Request / Response DTOs](#154-request--response-dtos)
16. [Utilities (utils)](#16-utilities-utils)
   - 16.1 [Crypto](#161-crypto)
   - 16.2 [String & Chinese](#162-string--chinese)
   - 16.3 [Date & Time](#163-date--time)
   - 16.4 [Serialization (JSON / XML / YAML)](#164-serialization-json--xml--yaml)
   - 16.5 [Map & Struct Conversion](#165-map--struct-conversion)
   - 16.6 [Collections & Slices](#166-collections--slices)
   - 16.7 [Network & IP](#167-network--ip)
   - 16.8 [File & Compression](#168-file--compression)
   - 16.9 [UUID & Random](#169-uuid--random)
   - 16.10 [Concurrency & Data Structures](#1610-concurrency--data-structures)
   - 16.11 [Validation & Protection](#1611-validation--protection)
   - 16.12 [Misc](#1612-misc)

**Part VII — Reference & Branches**

17. [Best Practices](#17-best-practices)
18. [FAQ](#18-faq)
19. [Version and Upgrade Notes](#19-version-and-upgrade-notes)
20. [jh Branch (Jihai Edition) Notes](#20-jh-branch-jihai-edition-notes)
   - 20.1 [Purpose and Origin](#201-purpose-and-origin)
   - 20.2 [Difference Overview vs master-1.25](#202-difference-overview-vs-master-125)
   - 20.3 [Removed Capabilities (Important)](#203-removed-capabilities-important)
   - 20.4 [New: GORM L2 Cache Layer (UseCache)](#204-new-gorm-l2-cache-layer-usecache)
   - 20.5 [New: PostgreSQL Support](#205-new-postgresql-support)
   - 20.6 [New: ClickHouse Support](#206-new-clickhouse-support)
   - 20.7 [Other Improvements (jh branch)](#207-other-improvements-jh-branch)
   - 20.8 [Known Issues and Cautions](#208-known-issues-and-cautions)
   - 20.9 [Upgrade / Migration Advice](#209-upgrade--migration-advice)
21. [New: Singleton Rate-Limit Middleware (ratelimit)](#21-new-singleton-rate-limit-middleware-ratelimit)
22. [New: Job Scheduler (job)](#22-new-job-scheduler-job)
23. [New: S3 Object Storage Plugin (storage/s3)](#23-new-s3-object-storage-plugin-storages3)
24. [New Capabilities Summary (jh branch)](#24-new-capabilities-summary-jh-branch)

---

**Part I — Getting Started & Architecture**　（第 1–3 章）

## 1. Framework Overview

MGin is a Go framework for **quickly building RESTful microservice programs**, built on top of [Gin](https://github.com/gin-gonic/gin). It pre-wires the common "plumbing" of microservice development so developers only focus on business routes and data models:

| Capability | Built-in Support |
|---|---|
| Web framework | Gin (HTTP/HTTPS dual port, graceful shutdown) |
| Configuration center | Nacos / Consul / Etcd / Polaris / SpringCloud Config / local file |
| Service registry & discovery | Nacos / Consul / Etcd / Polaris |
| Relational databases | MySQL, PostgreSQL, SQLite, ClickHouse (all via GORM v2) |
| Document / KV | MongoDB (mgo.v2-like API), Redis (sentinel/cluster/standalone) |
| Search | ElasticSearch (olivere/elastic) |
| Message queue | Kafka |
| Cache | In-memory cache (with expiration), local persistent cache (bitcask) |
| Common capabilities | Unified Result, i18n, unified error codes, JWT, Casbin auth, request tracing, CORS, XSS protection, rate limiting, IP restriction, request access logging |
| Built-in business | System management module (user/role/dept/post/menu/dict/API/config + RBAC) |

**Design principles**

- **Convention over configuration**: most capabilities are enabled via the `go.config.used` switch in `application.yml`; the framework automatically connects and runs a 5-minute health check (with auto-reconnect) for each enabled component.
- **Pluggable data sources**: any external data source (e.g. RabbitMQ) can be plugged into the framework lifecycle via `mgin.Use()`.
- **Generics-friendly**: `Result`, `CallT`, and the DAOs are fully generic, ensuring type safety at compile time.



---

## 2. Quick Start

### 2.1 Install

```bash
go get -u github.com/maczh/mgin
```

### 2.2 Minimal Runnable Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/maczh/mgin"
    "github.com/maczh/mgin/models"
)

func main() {
    // 1st arg = config file path; pass "" to auto-use <executable-name>.yml
    // 4th arg xlang=true enables internationalization (i18n)
    app := mgin.NewApp("", "test mgin project", "1.0.0", false)

    app.Router.GET("/hello", func(c *gin.Context) {
        c.JSON(200, models.Success(map[string]string{"msg": "hello world"}))
    })

    app.Run() // blocks and starts, listens for system signals for graceful shutdown
}
```

### 2.3 Configuration File

`NewApp` reads a `.yml` file with the same name as the executable by default (e.g. `yourapp.yml`). Minimal configuration:

```yaml
go:
  application:
    name: yourapp
    port: 8080
```

Run:

```bash
go build -o yourapp .
./yourapp                 # automatically reads yourapp.yml
# or specify explicitly: ./yourapp -f /path/to/conf.yml
# print version: ./yourapp -v
```



---

## 3. Application Lifecycle

### 3.1 Core Entry Object `App`

```go
func mgin.NewApp(configFile, appName, version string, xlang bool) *App
func (app *App) Run()
func (app *App) GetVersion() string
```

`App` struct (fields are unexported; only `Router *gin.Engine` is exposed for mounting routes):

| Field | Type | Description |
|---|---|---|
| `Router` | `*gin.Engine` | Gin engine; mount business routes and middleware here |
| `MGin` | `*mgin` | Framework kernel, with `Use`/`UsePlugin`/`SafeExit` |

`NewApp` internal flow:

1. Parse command-line flags `-f` (config file) and `-v` (print version).
2. Call `mgin.Init(configFile)` to initialize config and connect all enabled databases/registries per `go.config.used`.
3. Start a 5-minute timer that runs `Check()` health checks on all connections (auto-reconnect on failure).
4. Set Gin mode (release/debug) based on `go.application.debug`.
5. If `xlang=true`, initialize the i18n module.
6. Initialize base router and mount **global middleware**: `trace.TraceId()` → `postlog.RequestLogger()` → `cors.Cors()` → optional `xlang.RequestLanguage()` → `nice.Recovery` (global panic recovery).

### 3.2 Startup and Graceful Shutdown

`Run()` will:

- Start the HTTP server (when `go.application.port > 0`).
- Start the HTTPS server (when `go.application.cert` is configured).
- Listen for `SIGINT / SIGHUP / SIGTERM / SIGQUIT`; on signal it proceeds: `SafeExit()` (close all DBs and deregister from registry) → `server.Shutdown` (graceful shutdown with 5s timeout) → exit.

### 3.3 Plug-in External Data Sources

Any component implementing the `MginPlugin` interface can be plugged into the framework lifecycle:

```go
type MginPlugin interface {
    Init(configData []byte)
    Close()
    Check() error
}
```

```go
// Method 1: pass functions directly
mgin.MGin.Use("rabbitmq", mgrabbit.Rabbit.Init, mgrabbit.Rabbit.Close, nil)

// Method 2: pass a plugin object
mgin.MGin.UsePlugin("rabbitmq", rabbitPluginInstance)
```

> Note: the `dbConfigName` in `Use` must appear in `go.config.used`, and the corresponding config prefix file must exist, otherwise it will not be loaded.


















---

**Part II — Configuration**　（第 4–5 章）

## 4. Configuration System (application.yml)

### 4.1 Overview and Switches

All capabilities are enabled via `go.config.used` (comma-separated), e.g.:

```yaml
go:
  config:
    used: nacos,mysql,mongodb,redis,kafka   # enabled components; framework connects them automatically
```

### 4.2 Application and Base Configuration

```yaml
go:
  application:
    name: myapp                 # app name, used as the service name when registering
    port: 8080                  # HTTP port
    project: myproj             # owning project name
    port_ssl: 8443              # HTTPS port (effective when cert/key configured)
    cert: server.crt            # SSL certificate (in executable directory)
    key: server.key             # SSL private key
    debug: true                 # debug mode (=true => Gin debug + logs forced to debug)
    ip: 10.0.0.5                # IP registered to registry (needed for Docker/external network)
  discovery:
    registry: nacos             # registry type: nacos/consul/etcd/polaris, default nacos
    callType: json              # microservice call param mode: x-form / json / restful
  jwt:
    secret: 1234567890abcdef    # JWT signing secret
  logger:                       # console/file logging (logs package)
    level: debug
    out: console,file
    file: /opt/logs/myapp       # log file path prefix, auto-appended with .yyyy-MM-dd.log
  log:                          # request access log and microservice call log
    db: mongodb                 # log store: mongodb / elasticsearch
    dbName: Partner-Id          # in multi-DB, the header param used as DB name tag
    req: MyappRequestLog        # request log table/index name
    call: MyappCallLog          # call log table/index name
    kafka:
      use: true                 # whether to also send to kafka
      topic: myapp              # kafka topic (multiple topics comma-separated)
```

### 4.3 Configuration Center

```yaml
go:
  config:
    server: http://192.168.1.5:8848/   # config server address
    server_type: nacos                 # nacos/consul/springconfig/etcd/polaris/file
    token:                             # access token (required for polaris)
    path: conf                         # local file mode: directory of config files
    env: test                          # environment: test/prod/dev etc.
    prefix:                            # config file name prefixes per component
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

Naming rule for each component's config file: `<prefix>-<env>.yml`, e.g. `mysql-test.yml`, `redis-prod.yml`.

### 4.4 Built-in System Module Switches

```yaml
go:
  sys:
    enabled: true              # enable built-in system module (v1.20.3+)
    initdb: true               # auto create tables and seed base data
    baseUri: /api/v1           # base API path
    casbin: true               # enable casbin in sys module
    swagger:
      enabled: true
      uri: /swagger/sys        # full address /api/v1/swagger/sys/index.html
  casbin:
    enabled: true              # globally enable casbin (for middleware/casbin)
    model_file: casbin.conf    # casbin model file path

> ⚠️ **jh branch note**: the `jh` branch removed the built-in `sys` module, so `go.sys.*` config is meaningless there; Casbin can still be used as a standalone middleware `middleware/casbin` via `go.casbin.enabled`.
```

### 4.5 Configuration Reading API

```go
import "github.com/maczh/mgin/config"

config.Config.GetConfigString("go.application.name")   // string
config.Config.GetConfigStringArray("go.config.used")   // []string
config.Config.GetConfigInt("go.application.port")       // int
config.Config.GetConfigBool("go.application.debug")     // bool
config.Config.Exists("go.sys.enabled")                  // bool
```

The global config singleton is `config.Config`.



---

## 5. Unified Configuration Center

Besides local files, MGin can pull config from Nacos / Consul / Etcd / Polaris / SpringCloud Config. Path conventions:

- Nacos: `{server}/nacos/v1/cs/configs?group={project}&&dataId={prefix.nacos}-{env}.yml`
- Etcd: `/config/{project}/{prefix.etcd}-{env}.yml`
- Consul: `/{project}/{prefix.consul}-{env}.yml`
- Polaris: `/config/v1/GetConfigFile?namespace=default&group={project}&fileName={prefix.consul}-{env}.yml`

Each component's `<prefix>-<env>.yml` provides connection info; the framework pulls and initializes the corresponding client in `mgin.Init` per `go.config.used`.














---

**Part III — Data Access**　（第 6–8 章）

## 6. Databases and Message Middleware

On startup the framework auto-initializes the following global clients (in `github.com/maczh/mgin/db`) per `go.config.used`:

| Global variable | Type | Description |
|---|---|---|
| `db.Mysql` | `*mysql.MysqlClient` | MySQL (GORM) |
| `db.Pg` | `*postgres.PostgresClient` | PostgreSQL (GORM) |
| `db.Sqlite` | `*sqlite.Sqlite` | SQLite (GORM) |
| `db.Mongo` | `*mongo.Mongodb` | MongoDB |
| `db.Redis` | `*redis.RedisClient` | Redis |
| `db.Clickhouse` | `*clickhouse.ClickhouseClient` | ClickHouse (GORM) |
| `db.ElasticSearch` | `*es.ElasticSearch` | ElasticSearch |
| `db.Kafka` | `*kafka.Kafka` | Kafka |

Common methods (all clients provide): `Init([]byte)`, `Check() error`, `Close()`, `GetConnection(dbName ...string)`, `IsMultiDB() bool`, `ListConnNames() []string`.

> All clients are auto-initialized in `mgin.Init` based on `go.config.used`. To control timing manually, you may call `db.XXX.Init(config.Config.GetConfigData(prefix))`.

### 6.1 MySQL

**Config `mysql-test.yml`:**

```yaml
go:
  data:
    mysql: user:pwd@tcp(127.0.0.1:3306)/dbname?charset=utf8&parseTime=True&loc=Local
    mysql_debug: true    # print SQL
    mysql_cache: true    # enable L2 cache
    mysql_cache_expired: 300   # cache TTL in seconds, default 300 (5 min); 0 = no expiry
    mysql_pool:          # connection pool (single long connection if omitted)
      max: 200           # max connections
      total: 1000        # max concurrency, default max*5
      timeout: 30        # idle connection timeout (seconds)
      life: 5            # connection lifetime (minutes)
```

**Multi-DB config `mysql-multidb-test.yml`:**

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

**Code example:**

```go
// get connection (pass DB name for multi-DB)
conn, err := db.Mysql.GetConnection()
if err != nil { /* handle */ }
conn.Create(&user)

// enable L2 cache (call once at startup; recommended)
// config keys: go.data.mysql_cache (switch) / go.data.mysql_cache_expired (seconds, default 300=5min)
// uses Redis as cache when available, else falls back to in-process memory cache
db.Mysql.UseCache()

// multi-DB checks
if db.Mysql.IsMultiDB() {
    for _, name := range db.Mysql.ListConnNames() {
        _ = name
    }
}
```

### 6.2 PostgreSQL

**Config `postgresql-test.yml`:**

```yaml
go:
  data:
    postgres:
      dsn: "host=localhost user=test password=test_pwd123 dbname=testdb port=5432 sslmode=disable TimeZone=Asia/Shanghai"
      debug: true
      cache:
        enabled: true          # L2 cache switch
        expired: 300           # cache TTL in seconds, default 300 (5 min); depends on go.data.redis
      pool:
        max: 200
        total: 1000
        timeout: 30
        life: 5
```

Multi-DB config mirrors MySQL (replace `mysql` with `postgres`, use `dbNames` + per-DB sub-keys holding `dsn`). Use `db.Pg.GetConnection()`, `db.Pg.UseCache()`, etc. `UseCache()` reads `go.data.postgres.cache.enabled` (switch) and `go.data.postgres.cache.expired` (seconds, default 300) to decide caching behavior.

### 6.3 SQLite

**Config `sqlite-test.yml`:**

```yaml
go:
  data:
    sqlite: mytest.db   # local file name; defaults to <App.Name>.db
```

```go
conn := db.Sqlite.GetConnection()
db.Sqlite.UseCache()
```

### 6.4 MongoDB

Based on `github.com/maczh/mgo` (mgo.v2-like API). **Config `mongodb-test.yml`:**

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

Replica set: `mongodb://user:pwd@host1:27017,host2:27017/dbname?replicaSet=rs0`.

Multi-DB: set `mongodb` to `multidb: true` + `dbNames` + per-DB sub-keys (`uri`/`db`).

**Code example:**

```go
// get DB connection (returns a Copy of the session; must be returned after use)
database, err := db.Mongo.GetConnection()
if err != nil { /* handle */ }
defer db.Mongo.ReturnConnection(database) // important: return the session when done

// multi-DB
db2, _ := db.Mongo.GetConnection("test2")
defer db.Mongo.ReturnConnection(db2)
```

### 6.5 Redis

Supports **standalone / sentinel / cluster** modes, on go-redis v7. **Config `redis-test.yml`:**

```yaml
go:
  data:
    redis:
      uri: 127.0.0.1:6379[,127.0.0.1:6380]   # uri or host+port; multiple ips => cluster
      host: 127.0.0.1[,127.0.0.1]
      port: 6379[,6380]
      password: ""
      database: 1
      master: mymaster          # sentinel mode master name
      timeout: 1000
    redis_pool:
      min: 3
      max: 200
      idle: 10
      timeout: 300
```

Multi-DB config: set `redis` to `multidb: true` + `dbNames` + per-DB sub-keys (also support uri/host/port/master).

**Code example:**

```go
client, err := db.Redis.GetConnection()
if err != nil { /* handle */ }

client.Set("key", "value", 10*time.Second)
val, _ := client.Get("key").Result()

// multi-DB
client2, _ := db.Redis.GetConnection("test2")

// pattern subscribe with auto-reconnect on disconnect
db.Redis.PSubscribe("", func(msg *redis.Message, dbName string) {
    logs.Info("received message: %s", msg.Payload)
}, "news.*")
```

### 6.6 ClickHouse

Based on GORM driver `gorm.io/driver/clickhouse`, API mirrors MySQL. Config keys: `go.data.clickhouse` (single-DB DSN) or `go.data.clickhouse.<dbName>` (multi-DB), plus `go.data.clickhouse_cache` (cache switch). Use `db.Clickhouse.GetConnection()`, `UseCache()`, etc.

```go
dao := &dao.ClickhouseDao[Event]{}
_ = dao.Create(&event)
rows, page, _ := dao.Pager(db.Clickhouse.GetConnection(), 1, 20)
```

> ⚠️ **Known issue on the `jh` branch**: `ClickhouseDao.MultiCreate` mistakenly uses `db.Mysql.GetConnection` during bulk insert, writing data to **MySQL** instead of ClickHouse (as of `v1.25.11-jh`). Until fixed, avoid `MultiCreate` for ClickHouse — use single `Create` or fetch `db.Clickhouse.GetConnection()` directly. Also `UseCache()` sets no Redis TTL; see [Chapter 20](#20-jh-branch-jihai-edition-notes).

### 6.7 ElasticSearch

Based on olivere/elastic v6. **Config `elasticsearch-test.yml`:**

```yaml
go:
  elasticsearch:
    uri: http://127.0.0.1:9200
    user: elastic
    password: "********"
```

**CRUD (index naming rule `database_table`, or `database` when table is empty):**

```go
// add document (searchFields specifies which fields are searchable)
id, err := db.ElasticSearch.AddDocument("mydb", "user", map[string]any{
    "name": "Zhang San", "age": 18,
}, []string{"name"})

// bulk add
db.ElasticSearch.AddDocuments("mydb", "user", []map[string]any{ /* ... */ }, []string{"name"})

// update / delete
db.ElasticSearch.UpdateDocument("mydb", "user", id, map[string]any{"age": 19})
db.ElasticSearch.DeleteDocument("mydb", "user", id)
db.ElasticSearch.DeleteTable("mydb", "user")
db.ElasticSearch.DeleteDatabase("mydb")
```

> Index and IK/ngram mappings are auto-created by the framework; no manual index creation needed.

### 6.8 Kafka

Based on Shopify/sarama. **Config `kafka-test.yml`:**

```yaml
go:
  data:
    kafka:
      servers: "127.0.0.1:9092,127.0.0.1:9093"   # cluster, comma-separated
      ack: all              # no / local / all
      auto_commit: true
      partitioner: hash     # hash / random / round-robin
      version: 2.8.1
```

**Produce and consume:**

```go
// produce
db.Kafka.Send("my_topic", "test message")
db.Kafka.SendMsgs("my_topic", []string{"a", "b"})

// create topic
db.Kafka.CreateTopic("my_topic")

// consume (consumer-group based, auto-reconnect; one topic => one groupId)
err := db.Kafka.MessageListener("my_group_id", "my_topic", func(msg string) error {
    logs.Info("received Kafka message: %s", msg)
    return nil
})
```

Lower-level entry points: `GetProducer() / GetConsumer() / GetAdminClient() / GetConsumerGroup(id)`.



---

## 7. DAO Generic Data Access Layer

Type-safe CRUD for relational and document databases, in `github.com/maczh/mgin/db/dao`.

| DAO | For | Construct |
|---|---|---|
| `dao.MySQLDao[E]` | MySQL | `&dao.MySQLDao[E]{}`, E must implement `schema.Tabler` (has `TableName()`) |
| `dao.PostgresDao[E]` | PostgreSQL | same |
| `dao.ClickhouseDao[E]` | ClickHouse | same |
| `dao.MgoDao[E]` | MongoDB | set `CollectionName` field first |

### 7.1 Relational DAO (MySQL example)

```go
type User struct {
    ID     int    `gorm:"column:id;primaryKey"`
    Name   string `gorm:"column:name"`
    Status int    `gorm:"column:status"`
}
func (User) TableName() string { return "sys_user" }

// construct DAO (single DB can omit Tag; multi-DB routes via the Tag closure)
userDao := &dao.MySQLDao[User]{}
// multi-DB example:
// userDao := &dao.MySQLDao[User]{Tag: func() string { return "test2" }}

// create
userDao.Create(&User{Name: "Zhang San", Status: 1})
userDao.MultiCreate([]*User{&u1, &u2})

// update
userDao.Updates(&User{Name: "Li Si"})            // update by primary key
userDao.Save(&user)                              // full save

// delete
userDao.Delete(User{Name: "Zhang San"})          // delete by condition

// read
list, err := userDao.All(User{Status: 1},
    dao.QueryOption{Preloads: []string{"Role"}, OrderBy: []string{"id desc"}})
one, err := userDao.One(User{Name: "Zhang San"}) // returns (nil, nil) if not found
exists := userDao.Exists(User{Name: "Zhang San"})
count, err := userDao.Count(User{Status: 1})

// paginate: first Where to get *gorm.DB, then Pager
db2 := userDao.Where("status = ?", 1)
rows, page, err := userDao.Pager(db2, 1, 20)     // page.Index/Size/Total/Count

// debug and context
userDao.Debug().All(User{})
userDao.WithContext(&ctx).All(User{})
```

> `One` / `Count` return no error when a single record is not found (return `nil` / `0`), so the caller can check emptiness directly.

### 7.2 MongoDB DAO

```go
type MgoUser struct { ID primitive.ObjectID `bson:"_id"` }
mgoDao := &dao.MgoDao[MgoUser]{CollectionName: "user"}
mgoDao.Insert(&MgoUser{})
mgoDao.All(bson.M{"name": "Zhang San"})
mgoDao.One(bson.M{"_id": id})
mgoDao.Updates(id, MgoUser{})
mgoDao.Delete(bson.M{"name": "Zhang San"})
mgoDao.Pager(bson.M{}, "name", 1, 20)
```







---

## 8. Cache

In `github.com/maczh/mgin/cache`, unified interface `ICache`:

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

**Factory functions (named singletons):**

```go
// 2nd arg true => persistent disk cache (bitcask); default false => in-memory
c := cache.OnGetCache("mycache", false)
c.Set("k", "v", 5*time.Minute)
v, ok := c.Get("k")

// explicit constructors
mem := cache.OnMemCache("name")
disk := cache.OnDiskCache("/path/to/cache")  // persisted to disk
cache.CloseCache()                            // release all cache instances
```

> In-memory cache `New(cleaningInterval)` cleans expired items in the background; disk cache is bitcask-based, and **`Get` returns `string`** — you must convert to the target type yourself.














---

**Part IV — Services & Communication**　（第 9–10 章）

## 9. Microservice Client

In `github.com/maczh/mgin/client`. Under the hood it uses `registry.Registry.GetServiceURL` for service discovery and auto-propagates trace headers.

### 9.1 Options and Protocols

```go
type Options struct {
    Method   string                 // GET/POST/PUT/DELETE, default POST
    Protocol string                 // x-form / json / restful / file
    Group    string                 // registry group (e.g. nacos)
    Header   any                    // extra headers (struct/map)
    Query    any                    // URL Query params
    Data     any                    // x-form PostForm params
    Json     any                    // json / restful body
    Path     map[string]string      // restful path params {key}=value
    Files    []grequests.FileUpload // file uploads
    Retry    bool                   // retry once on failure
}
```

Protocol constants: `client.CONTENT_TYPE_FORM` / `CONTENT_TYPE_JSON` / `CONTENT_TYPE_RESTFUL` / `CONTENT_TYPE_FILE`. `Protocol` defaults to `go.discovery.callType`.

### 9.2 Call Examples

```go
// non-generic: returns raw string
resp, err := client.Call("user-service", "/api/v1/user/get", &client.Options{
    Method:   "POST",
    Protocol: client.CONTENT_TYPE_JSON,
    Json:     map[string]any{"id": 1},
    Query:    map[string]any{"trace": "x"},
})

// generic: parsed directly into models.Result[T]
var user User
result := client.CallT[User]("user-service", "/api/v1/user/get", &client.Options{
    Protocol: client.CONTENT_TYPE_JSON,
    Json:     map[string]any{"id": 1},
})
if result.Status != 1 {
    // handle error (failed result.Status = -1)
}
user = result.Data

// restful style
result := client.CallT[Order]("order-service", "/api/v1/order/{oid}", &client.Options{
    Protocol: client.CONTENT_TYPE_RESTFUL,
    Method:   "GET",
    Path:     map[string]string{"oid": "1001"},
})

// file upload
result := client.CallT[any]("file-service", "/upload", &client.Options{
    Files: []grequests.FileUpload{{FileName: "a.png", FileReader: f}},
})
```

> A failed call returns `models.ErrorT[T](-1, "...")` with message `"Service error"` or `"failed to get host:port of X service"`. With `Retry: true`, it retries once on failure.



---

## 10. Service Registry and Discovery

`NewApp` creates the registry client per `go.discovery.registry` and calls `Register` in `mgin.Init`.

```go
var Registry RegistryClient   // global singleton

type RegistryClient interface {
    Register(registryConfigData []byte)
    GetServiceURL(servicename string, groupName ...string) (string, string) // returns host:port, group
    DeRegister()
}
func NewRegistry() RegistryClient  // returns impl by config type internally
```

Implementations: `nacos` / `consul` / `etcd` / `polaris` (in `registry/nacos`, `registry/consul`, `registry/etcd`, `registry/polaris`). `client.Call` uses `GetServiceURL` internally and picks a random healthy instance.

**Per-registry config (`*-test.yml`):**

```yaml
# nacos
go:
  nacos:
    server: 127.0.0.1
    port: 8848
    clusterName: DEFAULT
    group: OpenApi
    weight: 1
    lan: true          # register with internal IP
    lanNet: 192.168.3. # network prefix
```

The others (etcd/consul/polaris) are structured similarly; `polaris` additionally needs `namespace` and `token`. For consul, `port` is its HTTP API port (e.g. 8500).

> 📌 **jh branch enhancements**: etcd registration switched to **lease + keepalive auto-renewal** since `v1.21.17-jh` (stale registrations are auto-cleaned when an instance goes offline); Nacos registration switched to the community `nacos-sdk-go` since `v1.21.15-jh`, dropping the Alibaba Cloud SDK dependency.














---

**Part V — Request & Cross-cutting**　（第 11–14 章）

## 11. Middleware

Base middleware is auto-mounted by `NewApp`: `trace` → `postlog` → `cors` → `xlang`(optional) → `recovery`. Business auth middleware (jwt/casbin) must be mounted as needed.

### 11.1 CORS

```go
app.Router.Use(cors.Cors())                       // default allow common headers
app.Router.Use(cors.Cors("X-My-Header"))          // append custom headers
```

Default allowed headers: `Content-Type, AccessToken, X-CSRF-Token, Authorization, Token`; OPTIONS is auto-passed.

> 📌 **jh branch enhancement**: since `v1.21.15-jh`, **external CORS configuration** is supported (no longer limited to the built-in fixed headers) — allowed origins/methods/headers can be injected via config.

### 11.2 JWT Authorization

```go
app.Router.Use(jwt.JwtAuthorize())
```

Reads the token from the `Authorization` header and validates it with `config.Config.Jwt.Secret`. Whitelist paths auto-passed: `/docs/`, `/swagger/`, `go.sys.swagger.uri`.

### 11.3 Casbin Interface Authorization

```go
app.Router.Use(casbin.CasbinHandler())
```

Requires `go.casbin.enabled=true`. Reads `userId`/`roleId` from JWT claims and calls `casbin.Casbin.GetEnforcer().Enforce(...)` for RBAC (returns 401/403 on denial). `casbin.Casbin.UnAuthPath` (`[]CasbinInfo{{Path, Method}}`) configures auth-exempt whitelist.

### 11.4 IP Restriction (CIDR)

```go
app.Router.Use(iplimit.CIDR("192.168.1.0/24,10.0.0.0/8"))  // allow only these ranges
```

- `iplimit.DisableLogging`: disable logging.
- `iplimit.TrustedHeaderField`: read real IP from a trusted proxy header.

### 11.5 Concurrency Rate Limiting

```go
app.Router.Use(limit.MaxAllowed(100))   // at most 100 concurrent requests
```

### 11.6 Request Access Logging

```go
app.Router.Use(postlog.RequestLogger())  // mounted by default
```

Asynchronously writes request/response logs to MongoDB / ElasticSearch (per `go.log.db`) or to Kafka (`go.log.kafka.use`). Supports switching DB by a header param named in `go.log.dbName`.

### 11.7 Session

```go
app.Router.Use(session.New())                  // default config
store := session.FromContext(c)                // get session
session.Destroy(c)                             // destroy
store, _ = session.Refresh(c)                  // refresh
```

### 11.8 Trace / Request Tracking

```go
app.Router.Use(trace.TraceId())   // mounted by default, writes X-Request-ID
```

Per-request context helpers (shared across goroutines): `trace.GetRequestId()`, `trace.GetClientIp()`, `trace.GetUserAgent()`, `trace.GetHeader(h)`, `trace.SetHeader(k,v)`, `trace.GetHeaders()`, `trace.CopyPreHeaderToCurRoutine(id)`. These headers are auto-propagated during `client.Call`, enabling distributed tracing.

### 11.9 Internationalization Language

```go
app.Router.Use(xlang.RequestLanguage())   // mounted by default when xlang=true
xlang.GetCurrentLanguage()                // current language, default zh-cn
```

### 11.10 XSS Protection

```go
xssMw := &xss.XssMw{
    FieldsToSkip: []string{"content"},     // skip these fields
    BmPolicy:     "UGCPolicy",             // StrictPolicy (default) / UGCPolicy
}
app.Router.Use(xssMw.RemoveXss())
```

Sanitizes POST/PUT JSON, form, multipart, and GET query via bluemonday, auto-skipping the `password` field.



---

## 12. Internationalization and Error Codes (i18n / errcode)

### 12.1 Error Code Constants (`errcode` package)

Integer status codes:

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

Message IDs (`errcode` package constants, used as i18n keys):

```go
errcode.UrlNotFound   = "404"
errcode.SystemError   = "system error"
errcode.ParamLost     = "parameter cannot be empty"
errcode.ParamError    = "parameter error"
errcode.Success       = "success"
// also: DbConnectErr / DbInsertErr / DbUpdateErr / DbDeleteErr /
// DataNotFound / ConnectFail / ServiceUnavailable / DbQueryErr, etc.
```

### 12.2 i18n Responses (require `NewApp(..., true)`)

```go
i18n.Error(code int, messageId string) models.Result[any]
i18n.ErrorT[T](code int, messageId string) models.Result[T]
i18n.ErrorWithMsg(code, messageId, msg string) models.Result[any]
i18n.Success[T](data T) models.Result[T]
i18n.SuccessWithPage[T](data T, count, index, size, total int) models.Result[T]

i18n.String("success")                  // text by current goroutine language
i18n.Format("welcome", name)            // {} placeholder substitution
i18n.ParamLostError("userId")           // quick missing-param error
i18n.CheckParametersLost(params, "a", "b") // batch non-empty check
```

**Example:**

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

> The multilingual data source is an external `x-lang` microservice. MGin fetches it in `i18n.Init()` and refreshes every 5 minutes; built-in Chinese message IDs are in the `errcode` package.







---

## 13. Unified Response Structure (Result)

In `github.com/maczh/mgin/models`. All endpoints should return `models.Result[T]` (output via `c.JSON(200, result)`):

```go
type Result[T any] struct {
    Status int         `json:"status"`            // 1=success, non-1=failure
    Msg    string      `json:"msg"`
    Data   T           `json:"data,omitempty"`
    Page   *ResultPage ` json:"page,omitempty"`
}
type ResultPage struct {
    Count int `json:"count"`  // total pages
    Index int `json:"index"`  // current page
    Size  int `json:"size"`   // page size
    Total int `json:"total"`  // total records
}
```

Constructors:

```go
models.Success(data)                       // Status=1, Msg="Success"
models.SuccessWithMsg("ok", data)
models.SuccessWithPage(data, count, index, size, total)
models.SuccessPage(data, &models.ResultPage{...})
models.Error(status, msg)                  // Result[any]
models.ErrorT[T](status, msg)             // typed generic

// type conversion
r.ToAny()            // Result[T] -> Result[any]
models.ToAny[T](r)   // Result[any] -> Result[T] (Data type must match)
```

> The framework's `NoRoute` (404) and global `recoveryHandler` (panic) both use `i18n.Error` to return a standard Result, ensuring a uniform error format.







---

## 14. Logging

```go
logs.Debug("user %s logged in", name)
logs.Info("connected to %s", "MySQL")
logs.Warn("cache miss")
logs.Error("MySQL check failed： {}", err.Error())
```

- Level controlled by `go.logger.level` (debug/info/warn/error); `go.application.debug=true` forces debug.
- Output target governed by `go.logger.out` (console,file), path prefix `go.logger.file`.
- Logs auto-inject `traceId` for correlation.
- Output modes (internal): `console` / `file` / `es` / `simple` / `color`.


















---

**Part VI — Built-in Modules & Utilities**　（第 15–16 章）

## 15. Built-in System Module and Casbin Authorization

> v1.20.3+, enabled via config only, auto-creates tables and ships with Swagger docs.

> ⚠️ **Not applicable on the `jh` branch (Jihai edition)**: the `jh` branch has **removed the built-in `sys` module** (the entire `sys/controller`, `sys/dao`, `sys/service`, `sys/route`, `sys/middle` tree) and the Swagger docs (`docs/docs.go`, `docs/swagger.json/yaml` are also deleted) to **shrink the compiled binary**. Therefore on `jh`: `go.sys.*` config has no effect, there are no built-in user/role/permission APIs, and Swagger is not bundled. If you need these, use a non-`jh` branch such as `master-1.25`. See [Chapter 20](#20-jh-branch-jihai-edition-notes).

### 15.1 Enable

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

### 15.2 Built-in Data Models (table names)

| Model | Table | Description |
|---|---|---|
| `SysUser` | `sys_user` | user (LoginName unique) |
| `SysRole` | `sys_role` | role (RoleIdent unique) |
| `SysUserExt` | `sys_user_ext` | user extension (dept/post/role association) |
| `SysDept` | `sys_dept` | department (tree) |
| `SysPost` | `sys_post` | post |
| `SysDict` | `sys_dict` | dictionary |
| `SysResource` | `sys_resource` | frontend menu/resource tree |
| `SysApi` | `sys_api` | backend API (NeedAuth/NeedLog) |
| `SysRoleApi` | `sys_role_api` | role-API |
| `SysRoleResource` | `sys_role_resource` | role-resource (menu) |
| `SysUserOnline` | `sys_user_online` | online users |
| `SysSocial` | `sys_social` | third-party login |
| `SysConfig` | `sys_config` | dynamic config (Key unique) |

Common base `BaseModel` includes `DelFlag, CreateAt, UpdateAt, UpdateBy, CreateBy, TenantId`.

### 15.3 Authorization Models

- **Login & Token**: `SysUser` + JWT (`jwt.JwtAuthorize` validates).
- **Interface authorization (choose one)**:
  - Casbin (recommended): `go.casbin.enabled=true` + `go.sys.casbin=true`, with `middleware/casbin` doing RBAC interception. Manage policies via `casbin.Casbin`:
    ```go
    casbin.Casbin.UpdateCasbin(roleId, []casbin.CasbinInfo{{Path:"/api/v1/user", Method:"GET"}})
    casbin.Casbin.GetPolicyPathByRoleId(roleId)
    casbin.Casbin.AddPolicy([][]string{...})
    casbin.Casbin.RemoveFilteredPolicy("1")
    casbin.Casbin.FreshCasbin()
    ```
    `casbin.conf` (repo root) is an RBAC model: `r.sub,p.sub=role; r.obj=path; r.act=method`, matcher uses `keyMatch2` for path wildcards.
  - Role-API table mode: associate via `SysRoleApi`, validated by business logic.

### 15.4 Request / Response DTOs

`models/sys/request` and `models/sys/vo` provide common DTOs such as `RegisterReq`, `LoginReq`, `CreateRoleReq`, `ListSysUserReq`, `BindRoleApiReq`, `CreateResourceReq`, etc., ready to bind in Controllers.



---

## 16. Utilities (utils)

`github.com/maczh/mgin/utils` provides a large set of ready-to-use helper functions and generic data structures, covering cryptography, string/Chinese handling, date/time, serialization, map/struct conversion, collections, network/IP, file & compression, UUID/random, concurrency-safe structures, and validation. All are package-level exported functions, imported as needed, e.g.:

```go
s  := utils.MD5Encode("hello")                    // "5d41402abc4b2a76b9719d911017c592"
id := utils.NewUUIDString()                       // "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
ok := utils.IsChinaMobileString("13800138000")    // true
```

> 📌 Convention: crypto functions usually come in both a "byte-slice" form (`[]byte`/error) and a "string" form (ignores errors, returns empty string or a hint on failure); pick per your scenario.

### 16.1 Crypto
#### AES (CBC / ECB)
| Function | Description |
| --- | --- |
| `AESBase64Encrypt(origin, key string, iv []byte) (string, error)` | CBC mode, outputs a standard Base64 string |
| `AESBase64Decrypt(encrypt, key string, iv []byte) (string, error)` | corresponding decrypt |
| `AesEncrypt(origData, key []byte) ([]byte, error)` | CBC, IV = `key[:16]` |
| `AesDecrypt(encrypted, key []byte) ([]byte, error)` | corresponding decrypt |
| `AESEncrypt(data, key string) string` / `AESDecrypt(data, key string) string` | string wrappers, return `""` on error |
| `AesEncryptEcb(src, key string) (string, error)` / `AesDecryptEcb(crypted, key string) (string, error)` | ECB mode |
| `NewECBEncrypter(b)` / `NewECBDecrypter(b)` | custom ECB BlockMode |
| `PKCS5Padding` / `PKCS5UnPadding` | padding / unpadding helpers |

> Key length must be 16 / 24 / 32 bytes; `AesEncrypt` family uses the first 16 bytes of `key` as IV.

#### DES / 3DES
| Function | Description |
| --- | --- |
| `DesEncrypt / DesDecrypt(origData, key []byte) ([]byte, error)` | DES CBC, key must be 8 bytes |
| `DesEcbEncrypt / DesEcbDecrypt(src, key []byte) ([]byte, error)` | DES ECB |
| `DESEncrypt / DESDecrypt(data, key string) string` | string wrappers |
| `TripleDesEncrypt / TripleDesDecrypt(origData, key []byte, pcks5 bool) ([]byte, error)` | 3DES CBC, IV = `key[:8]` |
| `TripleEcbDesEncrypt / TripleEcbDesDecrypt(origData, key []byte) ([]byte, error)` | 3DES ECB (k1/k2/k3, 8 bytes each) |

#### RSA
- Convenience functions (auto Base64 encode/decode): `PublicEncrypt(data, publicKey string) (string, error)`, `PriKeyEncrypt`, `PublicDecrypt`, `PriKeyDecrypt`.
- Instance style: package-global `var RSA = &RSASecurity{}`.
  - `SetPublicKey(str)` / `SetPrivateKey(str)` load PEM (PKIX / PKCS1 / PKCS8 supported);
  - `PubKeyENCTYPT / PriKeyENCTYPT / PubKeyDECRYPT / PriKeyDECRYPT(input []byte)` for encrypt/decrypt;
  - `SignSha1WithRsa / SignSha256WithRsa(data string) (string, error)` → Base64 signature;
  - `SignSha256WithRsaHex` (Hex) / `SignSha256WithRsaUrlSafe` (URL-safe Base64);
  - `VerifySignSha1WithRsa / VerifySignSha256WithRsa(data, sign string) error` verify signatures.

#### Digest & HMAC
| Function | Description |
| --- | --- |
| `MD5Encode(content string) string` | lowercase Hex, 32 chars |
| `FileMD5(filename string) (string, error)` | file MD5 |
| `MapMD5(m map[string]string) string` | concatenate `k=v&` in key order (excludes `sign` and empty values) then MD5; common for signing |
| `Sha1(src)` / `Sha256(src) string` | lowercase Hex |
| `FileSha256(filename string) (string, error)` | file SHA256 |
| `HmacSHA1 / HmacSHA256(key, data []byte) []byte` | raw bytes |
| `HmacSHA1Hex / HmacSHA256Hex(key, data string) string` | Hex |
| `HmacSHA1Base64 / HmacSHA256Base64(key, data string) string` | standard Base64 |

#### Base64 / JWT
- `Base64Encode(str) string` / `Base64Decode(str) string`: standard `StdEncoding` (padded).
- `GenerateToken(claims jwt.MapClaims) (string, error)`: HS256 signature, fixed `exp = now+24h`, key from `config.Config.Jwt.Secret`.
- `ValidateToken(tokenStr string) (*jwt.Token, error)`: validate token.

### 16.2 String & Chinese
| Function | Description |
| --- | --- |
| `IsEmpty / IsNotEmpty / IsBlank / IsNotBlank(text)` | empty / blank checks |
| `Left / Right(src, size)`, `LeftJustin / RightJustin / CenterJustin` | truncate / pad |
| `Length(text) int` | length by rune (merges combining marks) |
| `Lowercase / Uppercase(s)`, `UpperFirst / LowerFirst(s)`, `UpperWord(s)` | case |
| `IsNumeric / IsAlphabet / IsAlphaNum` | character classes |
| `StrPos / BytePos / RunePos`, `IsStartOf / IsEndOf` | search |
| `ReplacePunctuation(src, rep)`, `AddSpaceBetweenCharsAndNumbers` | punctuation handling |
| `AnyToString(i any) (string, error)`, `StringToAny[T](src) (T, error)` | generic type conversion |
| `SubChineseString(str, begin, length)`, `ChineseLength(str)`, `IsChinese(str)` | Chinese safe substring by rune |
| `DBCtoSBC(s)` | full-width → half-width |
| `UnicodeEmojiDecode / UnicodeEmojiCode / TrimEmoji(s)` | Emoji decode / encode / strip |

### 16.3 Date & Time
| Function | Description |
| --- | --- |
| `ToDateTimeString / ToDateString / ToTimeString(t)` | `2006-01-02 15:04:05` / date / time |
| `GetNowDateTime() / GetDate()` | current time string in **CST (+08:00)** |
| `DateFormat(t, format)`, `ConvertDateFormat(str, format)` | custom / parse formatting (`yyyy/MM/dd` etc.) |
| `GormTimeFormat(t string)` | GORM time-string normalization (strips `T` and `+08:00`) |
| `TimeSubDays(t1, t2) int`, `WeekByDate(t) int` | days diff / week-of-year |
| `Get0Hour / Get0Yesterday / Get0Tomorrow / Get0Week / Get0Month ...` | zeroed time points per granularity |
| `GroupByWeekDate(start, end) []WeekDate` | slice by week |
| `WaitNextMinute()` | block until the next minute's 0 second |

### 16.4 Serialization (JSON / XML / YAML)
| Function | Description |
| --- | --- |
| `ToJSON(o any) string` | serialize (restores `<` `>` `&` escaping) |
| `ToJSONPretty(o any) string` | pretty (2-space indent) |
| `FromJSON(j string, o any) *any` | deserialize (logs on error, returns nil) |
| `JSONPretty / CompactJSON(in string) string` | pretty / compact |
| `DecodeXMLToMap(r)` / `EncodeXMLFromMap(w, m, root)` | XML ↔ map |
| `LoadYaml(filename, cfg)` / `StoreYaml(filename, cfg)` | YAML read/write (auto-mkdir on write) |

### 16.5 Map & Struct Conversion
| Function | Description |
| --- | --- |
| `MapItoS / MapStoI`, `Exists / Existi` | map conversion, key-exists checks |
| `Map2Struct(input, output)` | `mapstructure` weak typing (with duration hook) |
| `MapGet(input, field) interface{}` | dotted nested get (`a.b.c`) |
| `Struct2StringMap(obj)` / `AnyToMap(obj)` | struct → `map[string]string` |
| `Struct2Map(obj)` / `Struct2MapString(obj)` | struct → map (recursive) |
| `Clone(src, dst)` / `CopyStruct(src, dst)` | JSON deep copy / reflect field copy |
| `DeepCopy[D any](src any) D` | generic deep copy |
| `GetStructFields / GetStructJsonTags(obj)` | reflect field names / json tags |
| `SortMapByValue / SortMapByValueDesc(src)` | sort by value (value must be float64) |

### 16.6 Collections & Slices
| Function | Description |
| --- | --- |
| `StringArrayContains / IntArrayContains / Float64ArrayContains` | contains |
| `StringArrayDelete / IntArrayDelete / ...` | delete by index |
| `SliceContains / SliceContainsInt / SliceContainsInt64 / SliceContainsString` | generic contains |
| `SliceMerge(Int/Int64/String)`, `SliceUnique(Int/Int64/String)` | merge / unique |
| `SliceSumInt / SliceSumInt64 / SliceSumFloat64` | sum |
| `ArrayStr2Int / ArrayInt2Str`, `UnSplitString(src, sep)` | type conversion / join |
| `UnionStringSlice / IntersectStringSlice / DifferenceStringSlice` | union / intersection / difference |
| `StringUnique / Int64Unique`, `TrimSpaceStrInArray` | unique / whitespace-trim check |
| `NewHashSet() *HashSet` (`Add / Exists / Remove / Members`) | string set |

### 16.7 Network & IP
| Function | Description |
| --- | --- |
| `GetLocalIpAddress() string` | first non-loopback, non-`169.254.x` IPv4, else `127.0.0.1` |
| `LocalIPv4s() ([]string, error)`, `GetIPv4ByInterface(name)` | all / per-interface IPv4 |
| `IsIntranetIP(ip) bool` | intranet check (`10.*` / `192.168.*` / `172.16-31.*`) |
| `IsPortUse(port int) bool` | port-in-use check (note: returns `true` when output is non-empty) |
| `UrlEncode(raw) string` / `UrlDecode(encoded) (string, error)` | RFC3986-style encode/decode |

### 16.8 File & Compression
| Function | Description |
| --- | --- |
| `SelfPath() / SelfDir()`, `Basename / Dir / Ext(file)` | path parsing |
| `InsureDir(path)`, `IsFile / IsDir / IsExist(path)` | ensure dir / existence checks |
| `ReadFileToBytes / ReadFileToString`, `WriteBytesToFile / WriteStringToFile` | read/write (auto-mkdir on write) |
| `FileSize / FileMTime`, `DirsUnder / FilesUnder(dir)` | file info / listing |
| `SearchFile(filename, paths...)`, `RealPath(file)` | search / real path |
| `DownloadFile(fileUrl, localPath) (string, error)` | HTTP download (`grequests`) |
| `SftpConnect / SftpUploadFile / SftpClose` | SFTP upload (password auth) |
| `ZipFiles(filename, files, srcpath, aliasnames)` | multi-file ZIP |
| `Compress / Decompress(data []byte)` | Gzip compress / decompress |
| `Utf8ToGbk / GbkToUtf8`, `ClearUtf8BOM(str)` | encoding conversion / strip BOM |

### 16.9 UUID & Random
| Function | Description |
| --- | --- |
| `NewUUID() (UUID, error)` / `MustNewUUID() UUID` | generate v4 UUID |
| `UUIDFromString(s)`, `IsValidUUIDString(s)` | parse / validate (RFC4122) |
| `UUID.String() / Simple()`, `NewUUIDString() / SimpleUUID()` | standard / no-separator string |
| `GetRandomString(l)` / `GetRandomCaseString(l)` / `GetRandomHexString(l)` / `GetRandomIntString(l)` | random string (digits / mixed-case+symbols / hex / digits-only) |
| `GenerateRandString(source, l)` | random from a custom charset |
| `GetUUIDString()` | v4 string based on `gofrs/uuid` |

### 16.10 Concurrency & Data Structures
| Function / Type | Description |
| --- | --- |
| `NewSafeGo(fn)` (`SetGoBeforeHandler / SetCallBeforeHandler / Run`) | panic-safe goroutine (recover + colored stack) |
| `GetGoroutineID() uint64` | current goroutine ID (debugging) |
| `Map[T any]` (`Load / Store / Range / Delete / LoadAndStore / LoadAndDelete / Len / Clear`) | generic concurrency-safe map |
| `LinkList[T any]` (`Add / Push / Pop / Enqueue / Dequeue / Get / GetAll / Walk / Size`) | generic doubly-linked list |
| `RingBuffer[T any]` (`Write / Read / Latest / Oldest / Overwrite`) | generic ring buffer (overwriting) |
| `HashSet` (`Add / Exists / Remove / Members`) | string set (see 16.6) |
| `Values` (`Put / Get / GetAll / Merge / Clear`) | concurrency-safe key-value container |
| `ExpireCache` (`Store / Load / Delete`, `Timeout` seconds) | in-memory cache with expiry |
| `NewLimitQueue()` + `LimitFreqSingle(queue, count, window) bool` | single-node sliding-window rate limiter |

### 16.11 Validation & Protection
| Function | Description |
| --- | --- |
| `IsChinaMobile / Mail / UserName / Nickname / ChineseName(...)` | mobile / email / username / nickname / Chinese-name (both `...String` and `[]byte` forms) |
| `IsChineseNameEx(s) (string, bool)` | normalize irregular separators to `·` and return the corrected value |
| `IsIdCard(cardNo string) bool` | 15 / 18-digit ID card (last char may be X) |
| `CheckSqlValidate(content string) (bool, string)` | SQL-injection keyword blacklist (returns suspected string on hit) |
| `AddPortsToFirewall(ports []int)` | linux `firewall-cmd` port open (linux only) |

### 16.12 Misc
| Function | Description |
| --- | --- |
| `AppName() / AppDir()` | executable name / directory |
| `GinParamMap(c *gin.Context) map[string]string` | merge GET Query and POST form params |
| `GinHeaders(c *gin.Context) map[string]string` | headers → map |
| `CmdExec(name, arg...) (string, error)`, `CmdRunWithTimeout(cmd, timeout)` | run system commands |
| `DisplaySize(raw float64) string` | bytes → human readable (`B/K/M/G/T/P/E`) |
| `IfThen / IfThenElse / DefaultIfNil / FirstNonNil` | conditional / nil-value helpers |

> ⚠️ **Implementation notes (for production)**: source review found known edge issues in some `utils` functions — `LinkList.Get` panics on out-of-range; `ExpireCache.checkExpire` mistakenly uses value as the delete key; `sqlvalidate` is a keyword blacklist and misses a leading-character hit; `SftpConnect` always returns nil from `HostKeyCallback` (MITM risk); `IsPortUse` is named opposite to its behavior (returns `true` when output is non-empty). Review the source before production use.


















---

**Part VII — Reference & Branches**　（第 17–20 章）

## 17. Best Practices

1. **Unified response**: all Controllers return `models.Result[T]` (or `i18n.Error/ErrorT`); frontends judge success by `status`, avoiding scattered HTTP status codes.
2. **Parameter validation**: prefer `i18n.CheckParametersLost(params, "a","b")` for non-empty checks, combined with `utils` phone/email validators.
3. **Prefer DAO over raw SQL**: use `dao.MySQLDao[T]` and friends; multi-DB routing via the `Tag` closure reduces boilerplate.
4. **Return MongoDB sessions**: `db.Mongo.GetConnection()` returns a session Copy — **always `defer db.Mongo.ReturnConnection(db)`**.
5. **Redis cache layer**: when enabling MySQL L2 cache, also enable Redis for higher hit rate and multi-instance sharing.
6. **Distributed tracing**: cross-service calls must go through `client.Call/CallT`, which auto-propagates trace headers (`X-Request-ID`); combined with `postlog` this correlates the full chain.
7. **Graceful shutdown**: don't manage `os.Signal` yourself; let `app.Run()` do `SafeExit()` then `Shutdown`.
8. **Multi-environment config**: separate environments with `go.config.env` + `<prefix>-<env>.yml`; sensitive info via Nacos/Polaris config center.
9. **Middleware order**: auth middleware (jwt/casbin) should be after `postlog` and before business routes; whitelist paths (swagger/doc) are pre-set in jwt/casbin.
10. **XSS**: endpoints receiving rich text/user content should mount `xss.RemoveXss()` with `UGCPolicy`.



---

## 18. FAQ

**Q: Component enabled but cannot connect?**
A: Check `go.config.used` includes the component name and that `<prefix>-<env>.yml` exists with correct naming; a failed load prints `加载{}失败，配置文件中未使用` or `{}配置错误`.

**Q: MongoDB session exhausted?**
A: You didn't return the session after `GetConnection()`. Always `defer db.Mongo.ReturnConnection(db)`.

**Q: Microservice call reports "failed to get host:port"?**
A: Target service isn't registered to the registry, or `go.discovery.registry` type mismatch; verify service name and group.

**Q: i18n text not applied?**
A: `NewApp`'s 4th arg must be `true`, and the `x-lang` microservice must be configured; built-in Chinese falls back to `errcode` constants.

**Q: How to switch multi-DB?**
A: Configure `multidb:true` + `dbNames`; at runtime `GetConnection("test2")` or the DAO's `Tag` closure returns the DB name. `IsMultiDB()` / `ListConnNames()` help detect.

**Q: How to switch Gin to production mode?**
A: Set `go.application.debug=false` (or remove it); the framework auto-uses `gin.ReleaseMode`.



---

## 19. Version and Upgrade Notes

- **v1.20.3**: built-in system module, enabled via yml only, auto table creation, ships Swagger docs.
- **v1.20.1**: new `App` object, greatly simplifies creating a new MGin app.
- **v1.19.42**: persistent cache switched to bitcask, standardized with in-memory cache via `ICache` interface.
- **v1.19.38**: added Redis reconnecting `PSubscribe`; Kafka consumers gain reconnect.
- **v1.19.36**: Redis supports cluster, sentinel, and standalone modes.
- **v1.19.21**: DAO layer returns no error when querying a single missing record (returns nil).
- **v1.19.19**: added mongo/mysql DAOs, plus `CopyStruct`.
- **v1.19.10**: mysql/mongo/redis multi-DB added `IsMultiDB` and `ListConnNames`.
- **v1.19.9**: postlog supports multi-DB switching by header param.
- **v1.19.8**: `client.Options` params changed to `any` type.
- **v1.19.7**: added `Struct2Map` and `AnyToMap`.
- **v1.19.5**: Nacos subscription unified management and unified unsubscribe.
- **v1.19.3**: added custom CORS header support.
- **v1.19.1**: `Result` implements any <-> generic T conversion.
- **v1.19.0**: Go 1.19 support, `Result` becomes generic; `client.Call` refactored with generic `CallT[T]` return.


> This document is based on the source (`mgin.go / app.go / config / db / client / cache / middleware / i18n / errcode / logs / registry / models / utils`) and covers MGin's core capabilities and common APIs. For deeper detail, read the corresponding package source directly.



---

## 20. jh Branch (Jihai Edition) Notes

> This chapter provides supplementary notes specific to the **`jh` branch (Jihai edition)**. It is a **slimmed-down variant** of MGin that, relative to `master-1.25`, makes two kinds of changes: "remove modules + add capabilities". All conclusions below are verified against the `jh` branch source (latest tag `v1.25.11-jh`).

### 20.1 Purpose and Origin

- The branch name `jh` comes from the pinyin initials of 「寄海」(Jihai).
- Design goal: keep MGin's core microservice capabilities while **removing the built-in system-management module and Swagger docs** to **significantly shrink the compiled binary** (~16k lines of `sys`-related code removed).
- Best for: services that don't need the built-in user/permission system, or that want a smaller image / a custom admin backend.

### 20.2 Difference Overview vs master-1.25

| Dimension | master-1.25 | jh branch |
|---|---|---|
| Built-in `sys` module | ✅ yes | ❌ removed |
| Swagger docs | ✅ bundled `docs/` | ❌ removed |
| MySQL/SQLite/ClickHouse L2 cache | ❌ | ✅ `UseCache()` |
| PostgreSQL support | ❌ | ✅ `db.Pg` + `PostgresDao` |
| ClickHouse support | ❌ | ✅ `db.Clickhouse` + `ClickhouseDao` |
| `NewApp` `-v` version flag | ❌ | ✅ |
| GIN mode | hardcoded `debug` | controlled by `go.application.debug` |
| MongoDB driver | `maczh/mgo` | official `mongo-driver` |
| etcd registration | plain register | lease + keepalive renewal |
| Nacos registration | Alibaba Cloud SDK | community `nacos-sdk-go` |
| CORS | fixed headers | external config supported |

### 20.3 Removed Capabilities (Important)

On the `jh` branch, the following directories/capabilities have been **deleted entirely** — do not depend on them:

- The whole built-in system management tree: `sys/controller`, `sys/dao`, `sys/service`, `sys/route`, `sys/middle` (users, roles, departments, dictionaries, posts, APIs, resources, role-API, role-resource, online users, third-party login, dynamic config, captcha, etc.).
- Swagger: `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`, and the Swagger route registration inside `app.go`.
- `app.go`'s `baseRouter()` no longer registers the sys routes or the `JwtAuthorize`/`ApiAuth` middlewares.

Consequently:

- The config keys `go.sys.*` (enabled / initdb / baseUri / casbin / swagger) have **no effect** on `jh`;
- Casbin can still be used as a standalone middleware `middleware/casbin` + `go.casbin.enabled`;
- If you need the built-in user/permission system, switch to `master-1.25` (or merge the `sys` module back in).

### 20.4 New: GORM L2 Cache Layer (UseCache)

The `jh` branch adds a read-cache for relational DB clients based on [go-gorm/caches/v4](https://github.com/go-gorm/caches):

- Implementation: new `db/mysql/redisCacher.go` defines `RedisCacher` (implements `Get/Store/Invalidate`, backed by Redis).
- Switch & TTL: each client exposes `UseCache() bool`, which mounts the cache plugin based on config:

| Client | Switch key | TTL key | Default TTL | Without Redis |
|---|---|---|---|---|
| MySQL | `go.data.mysql_cache` | `go.data.mysql_cache_expired` (sec) | 300s(5min) | in-process memory cache |
| SQLite | always (if enabled) | fixed 300s | 300s | in-process memory cache |
| PostgreSQL | `go.data.postgres.cache.enabled` | `go.data.postgres.cache.expired` (sec) | 300s(5min) | in-process memory cache |
| ClickHouse | `go.data.clickhouse_cache` | — | see 20.8 | in-process memory cache |

- Usage (call once after `mgin.Init`, before serving requests):

  ```go
  db.Mysql.UseCache()      // MySQL
  db.Pg.UseCache()         // PostgreSQL
  db.Sqlite.UseCache()     // SQLite
  db.Clickhouse.UseCache() // ClickHouse
  ```

- Note: this is a **read-query cache**; writes/updates trigger `Invalidate` (scans and deletes by the `caches.IdentifierPrefix` prefix), so it fits read-heavy, write-light workloads.

### 20.5 New: PostgreSQL Support

- Global client: `db.Pg = &postgres.PostgresClient{}` (package `github.com/maczh/mgin/db/postgres`).
- Config prefix: `go.config.prefix.postgres` (optional; component-level keys under `go.data.postgres.*`).
- Generic DAO: `PostgresDao[E schema.Tabler]`, API identical to `MySQLDao`: `Create / MultiCreate / Delete / Updates / Save / All / One / Exists / Count / Pager / Where / Debug / WithContext`.

  ```go
  dao := &dao.PostgresDao[User]{}
  dao.Tag = func() string { return "testdb" } // multi-DB: specify DB name
  _ = dao.Create(&user)
  ```

- Connection config `postgres-test.yml` — see [5.2](#52-postgresql).

### 20.6 New: ClickHouse Support

- Global client: `db.Clickhouse = &clickhouse.ClickhouseClient{}` (package `github.com/maczh/mgin/db/clickhouse`).
- Generic DAO: `ClickhouseDao[E schema.Tabler]`, API identical to `MySQLDao`.
- Config prefix: `go.config.prefix.clickhouse`; component DSN key `go.data.clickhouse` (single) or `go.data.clickhouse.<dbName>` (multi).

  ```go
  dao := &dao.ClickhouseDao[Event]{}
  _ = dao.Create(&event)
  rows, page, _ := dao.Pager(db.Clickhouse.GetConnection(), 1, 20)
  ```

- L2 cache: `UseCache()` reads `go.data.clickhouse_cache`; uses `RedisCacher` when Redis is present, else in-memory.

### 20.7 Other Improvements (jh branch)

1. **Version flag `-v` and `GetVersion()`**: `NewApp(configFile, appName, version string, xlang bool)` adds a `-v` CLI flag that prints `appName, 版本号: version` and exits; at runtime `app.GetVersion()` returns the version.
2. **GIN run mode**: no longer hardcoded to `debug`; uses `gin.DebugMode` when `go.application.debug=true`, otherwise `gin.ReleaseMode` (recommended to disable debug in production).
3. **etcd registration**: added lease + keepalive auto-renewal (v1.21.17-jh) to avoid stale registrations after an instance goes offline.
4. **Nacos registration**: switched to community `nacos-sdk-go`, dropping the Alibaba Cloud SDK (v1.21.15-jh) to reduce dependency size and conflict risk.
5. **External CORS config**: CORS rules can be injected via config instead of only built-in fixed headers (v1.21.15-jh).
6. **MongoDB driver swap**: `maczh/mgo` fully replaced by the official `go.mongodb.org/mongo-driver` (v1.25.2-jh).

### 20.8 Known Issues and Cautions

- **⚠️ ClickHouse `ClickhouseDao.MultiCreate` defect**: as of `v1.25.11-jh`, the bulk-insert method mistakenly calls `db.Mysql.GetConnection(receiver.Tag())`, writing data to **MySQL** instead of ClickHouse. Until it is fixed upstream, avoid `MultiCreate` for ClickHouse — use single `Create` or fetch `db.Clickhouse.GetConnection()` directly.
- **ClickHouse cache TTL**: `UseCache()` constructs `RedisCacher` **without setting `Expiration`** (unlike MySQL/Postgres' 5-minute default), which is equivalent to a Redis `SET` with no expiry — the cache keys **never expire automatically**. To add a TTL, set `Expiration` in `redisCacher.go` yourself.
- **PostgreSQL cache key nesting**: the cache switch is `go.data.postgres.cache.enabled` (nested under `cache`), not the top-level `go.data.postgres.cache`.
- **Dependency changes**: the `jh` branch `go.mod` adds `github.com/go-gorm/caches/v4`, `gorm.io/driver/postgres`, `gorm.io/driver/clickhouse`, etc., and the Redis client is `github.com/go-redis/redis/v7` (matching `RedisCacher`). Mind the minimum Go version and dependency conflicts when upgrading.

### 20.9 Upgrade / Migration Advice

- Migrating `master-1.25` → `jh`: if your project depends on the built-in `sys` module (users/permissions/captcha) or Swagger, **do not switch directly** — build your own admin backend first, or stay on `master-1.25`. The rest (Web / config / registry / cache / DAO usage) is largely compatible.
- Using the L2 cache: call the relevant `UseCache()` after `mgin.Init`; ensure `go.config.used` includes `redis` (to enable the Redis cache layer; otherwise it falls back to in-process memory).
- Enabling PostgreSQL/ClickHouse: add `postgres` / `clickhouse` to `go.config.used` and provide the corresponding `<prefix>-<env>.yml`.

---

## 21. New: Singleton Rate-Limit Middleware (ratelimit)

The `jh` branch ships a **config-driven, singleton-managed** rate-limit middleware `middleware/ratelimit` that supports multiple algorithms and dimensions, protecting endpoints from traffic spikes.

### 21.1 Features

- **Multiple algorithms**: token bucket (`token_bucket`), sliding log window (`sliding_window`), max concurrency (`concurrency`).
- **Multiple dimensions**: global, by IP, by path, by IP+path, by request header.
- **Rule-based**: each rule independently configures its algorithm, dimension, thresholds, limit HTTP status and message.
- **Whitelist**: bypass by IP / path prefix.
- **Idle GC**: unused limiters are reclaimed by a background goroutine to avoid unbounded memory growth when limiting per IP/path.
- **Programmatic limiting**: besides the HTTP middleware, `Allow(key, rule)` lets non-HTTP code (queue consumers, jobs) reuse the same limiting logic.

### 21.2 Configuration (application.yml)

Add `ratelimit` to `go.config.used` and configure under the `go.ratelimit` node:

```yaml
go:
  config:
    used: "...,ratelimit"
    prefix:
      ratelimit: "go.ratelimit"
  ratelimit:
    enabled: true
    idleTimeout: 600
    whitelist:
      - "/health"
    whiteIps:
      - "127.0.0.1"
    rules:
      - name: "global token bucket"
        algorithm: "token_bucket"
        dimension: "global"
        rate: 100
        burst: 20
        httpStatus: 429
        code: 1011
        message: "Too many requests, please retry later"
      - name: "login per IP"
        path: "/api/login/*"
        methods: ["POST"]
        algorithm: "sliding_window"
        dimension: "ip"
        rate: 5
        window: 60
        httpStatus: 429
        code: 1011
        message: "Too many login attempts"
      - name: "upload concurrency"
        path: "/api/upload"
        algorithm: "concurrency"
        dimension: "global"
        maxConcurrent: 10
```

### 21.3 Usage

```go
import "github.com/maczh/mgin/middleware/ratelimit"

// Option 1: from yml config (effective when go.ratelimit.enabled is true)
app.Router.Use(ratelimit.RateLimit())

// Option 2: code-only rules
app.Router.Use(ratelimit.RateLimitWith(ratelimit.Rule{
    Name:         "global concurrency",
    Algorithm:    ratelimit.AlgoConcurrency,
    Dimension:    ratelimit.DimGlobal,
    MaxConcurrent: 50,
}))
```

When a request is limited, the middleware writes `X-RateLimit-Limit` / `X-RateLimit-Remaining` / `X-RateLimit-Retry-After` headers and returns the configured `httpStatus` with `code/message`.

Programmatic limiting:

```go
ok, release := ratelimit.Allow("my-key", ratelimit.Rule{
    Algorithm: ratelimit.AlgoTokenBucket, Rate: 10, Burst: 5,
})
if !ok {
    return errors.New("rate limited")
}
defer release()
```

---

## 22. New: Job Scheduler (job)

The `jh` branch ships an **xxl-job-like** job scheduler `job`. Job definitions and execution logs are persisted in the current GORM database (auto-selected by priority **MySQL → PostgreSQL → SQLite**), with pure single-instance scheduling.

### 22.1 Enable

Add `job` to `go.config.used`. The scheduler auto-starts during `mgin.Init` (when `go.job.enabled: true`); register all handlers via `job.Register` beforehand:

```yaml
go:
  config:
    used: "mysql,job"
  job:
    enabled: true
    dbName: ""
    initdb: true
    tablePrefix: "mgin_"
    scanInterval: 1
    refreshInterval: 30
    logRetainDays: 30
    maxConcurrent: 50
    maxSerialQueue: 10
    timezone: "Asia/Shanghai"
```

Tables: `mgin_job_info` (definitions), `mgin_job_log` (execution logs).

### 22.2 Register a Handler

A handler is `func(*job.Context) error`, registered by `job.Register(name, handler)`; `name` must match the DB `handler_name` column:

```go
import "github.com/maczh/mgin/job"

func init() {
    job.Register("syncUserJob", func(ctx *job.Context) error {
        ctx.Log("start sync, param=%s", ctx.Param)
        if err := userService.Sync(ctx.Ctx()); err != nil {
            return err // returning error triggers retry
        }
        return nil
    })
}
```

`job.Context` provides `Ctx()` (cancellable context), `Log(format, ...)` (writes to log detail), `ParamMap()`, `Done()` / `Err()`.

### 22.3 Schedule Types & Strategies

| Field | Values | Description |
|---|---|---|
| scheduleType | `cron` | cron expression (5/6 fields, `@every 5m`, `@daily`) |
| | `fixed_rate` | interval seconds, counted from last **start** |
| | `fixed_delay` | interval seconds, counted from last **finish** |
| | `once` | one-shot, time `2006-01-02 15:04:05` |
| blockStrategy | `serial` / `discard` / `concurrent` / `cover` | behavior when previous run is still active |
| misfireStrategy | `do_nothing` / `fire_now` | compensate a missed schedule |
| timeout / retryCount / retryInterval | seconds / count / seconds | timeout, retries |

### 22.4 Admin API (Gin router group)

Mount `job.RouterGroup(r)` onto any router group (e.g. `/job`):

```go
job.GetManager()
job.RouterGroup(app.Router.Group("/job"))
```

| Method | Path | Description |
|---|---|---|
| GET | `/job/list?group=&keyword=&status=&index=&size=` | list (paged) |
| GET | `/job/:id` | detail |
| POST | `/job` | create |
| PUT | `/job` | update |
| DELETE | `/job/:id` | delete |
| POST | `/job/:id/start` | start |
| POST | `/job/:id/stop` | stop |
| POST | `/job/:id/trigger` | trigger once (`{"param":"optional override"}`) |
| GET | `/job/handlers` | registered handlers |
| GET | `/job/log?jobId=&jobName=&status=&index=&size=` | execution logs (paged) |

All endpoints return `models.Result` (`status:1` on success).

### 22.5 Health & Shutdown

`mgin` pings the running scheduler in `checkAll` (every 5 min) and calls `job.Stop()` on `SafeExit` to gracefully drain in-flight jobs. Manual control:

```go
job.Start()
job.Stop()
job.GetManager().IsRunning()
logID, err := job.GetManager().Trigger(id, "")
```

---

## 23. New: S3 Object Storage Plugin (storage/s3)

The `jh` branch ships an S3 object-storage plugin `storage/s3` built on **aws-sdk-go-v2**, compatible with AWS S3 and MinIO-like services, supporting multiple buckets, multipart upload and presigned URLs.

### 23.1 Enable

Add `s3` to `go.config.used`. The plugin reads the `go.s3` node during `mgin.Init` and closes on `SafeExit`:

```yaml
go:
  config:
    used: "...,s3"
  s3:
    enabled: true
    endpoint: "https://s3.amazonaws.com"   # MinIO example: http://localhost:9000
    region: "cn-north-1"
    accessKey: "AKIDEXAMPLE"
    secretKey: "SECRET"
    pathStyle: true                         # true for MinIO / self-hosted
    ssl: true
    maxRetries: 3
    uploadPartSize: 16777216
    downloadPartSize: 16777216
    maxUploadParts: 10
    maxDownloadParts: 10
    presignExpiry: 3600
    singleBucket: "my-bucket"
    buckets:
      - name: "public-assets"
        public: true
        defaultContentType: "image/png"
      - name: "private-files"
        public: false
```

### 23.2 Basic Usage

```go
import "github.com/maczh/mgin/storage/s3"

b := s3.GetS3().Default()
b = s3.GetS3().Get("public-assets")

_ = b.Upload(ctx, "avatars/1.png", bytes.NewReader(data), "image/png")
buf := manager.NewWriteAtBuffer([]byte{})
_ = b.Download(ctx, "avatars/1.png", buf)
_ = b.Delete(ctx, "avatars/1.png")
exists, _ := b.Exists(ctx, "avatars/1.png")
list, _ := b.List(ctx, "avatars/", 100)
url, _ := b.Presign(ctx, "avatars/1.png")
upUrl, _ := b.PresignUpload(ctx, "avatars/2.png")
etag, _ := b.UploadMultipart(ctx, "big.iso", "", bytes.NewReader(huge), 16*1024*1024)
```

> `Upload` auto-switches to multipart for a `*bytes.Reader` larger than the part size; large downloads are auto-chunked by `manager.Downloader`. ContentType defaults to an extension guess, then `application/octet-stream`.

---

## 24. New Capabilities Summary (jh branch)

Summary of capabilities added in the `jh` branch beyond `master-1.25` (chapters 21–23 above):

| Capability | Package / Entry | Switch | Notes |
|---|---|---|---|
| GORM L2 cache | `db.Mysql.UseCache()` etc. | `go.data.*.cache.enabled` | see 20.4 |
| PostgreSQL | `db.Pg` / `PostgresDao` | `postgres` | see 20.5 |
| ClickHouse | `db.Clickhouse` / `ClickhouseDao` | `clickhouse` | see 20.6 |
| Singleton rate-limit | `middleware/ratelimit` | `ratelimit` | see 21 |
| Job scheduler | `job` | `job` | see 22 |
| S3 storage | `storage/s3` | `s3` | see 23 |

All new capabilities are opt-in via switches in `go.config.used`; when disabled they have no connection overhead or side effects.
