# 配置方式与核心配置语义

本章解释“怎么配、配了影响什么、配错会怎样”。
穷举键表见 [08 · 配置项参考](../08-reference/configuration.md)。

---

## 1. 三种配置方式

### 1.1 本地 YAML（默认）

```yaml
# conf/application.yml
go:
  application:
    name: order
    port: 8080
  config:
    env: test
    server_type: file
    path: conf
    used: "mysql,redis"
    prefix:
      mysql: mysql
      redis: redis
```

```yaml
# conf/mysql-test.yml
go:
  data:
    mysql: "user:pwd@tcp(127.0.0.1:3306)/order?charset=utf8mb4&parseTime=True&loc=Local"
    mysql_debug: false
    mysql_pool:
      max: 100
      total: 50
      timeout: 30
      life: 60
```

适用：单机、开发、配置变化少的服务。

### 1.2 配置中心

```yaml
go:
  config:
    server_type: nacos       # nacos|consul|etcd|polaris|springconfig
    server: "http://127.0.0.1:8848/"
    token: ""                # polaris 需要
    env: prod
    prefix:
      mysql: mysql
      nacos: nacos           # 注册中心自身配置也从配置中心取
```

配置中心上的文件名必须是 `{prefix}-{env}.yml`，内容与本地子配置文件完全一致。
适用：多环境、多副本、需要集中审计。

### 1.3 代码中读取

```go
import "github.com/maczh/mgin/v2/pkg/config"

port := config.Config.GetConfigInt("go.application.port")
enabled := config.Config.GetConfigBool("go.my.feature")
list := config.Config.GetConfigStringArray("go.my.list")
```

规则：

- 只能读 `application.yml` 中的键（`Config.Cnf` 不包含子配置文件）；
- 缺失返回零值，不 panic；
- 启动后修改配置文件不会生效（除日志级别）。

---

## 2. 应用与 HTTP

```yaml
go:
  application:
    name: order          # 必需；注册中心服务名、i18n 表名前缀都用它
    project: shop        # 默认 DEFAULT；配置中心 group
    port: 8080           # HTTP 端口；0 表示不监听
    port_ssl: 8443       # 可选；与 cert/key 同时配置才启动 HTTPS
    cert: server.crt     # 相对可执行文件目录
    key: server.key
    debug: false         # true 时 gin debug + 日志 debug
    ip: ""               # 注册到注册中心的地址；空则自动探测
    shutdownTimeout: 10  # 优雅关闭秒数；默认 5
```

约束：

- `port` 为 0 且 `port_ssl` 为 0 时，`Run()` 只等待信号，不提供服务；
- `cert/key` 是相对**可执行文件目录**拼接（`filepath.Dir(os.Args[0])`）；
- `shutdownTimeout` 只接受秒；非法/0 回退 5。

---

## 3. 数据源配置

### 3.1 MySQL

```yaml
go:
  data:
    mysql: "user:pwd@tcp(127.0.0.1:3306)/order?charset=utf8mb4&parseTime=True&loc=Local"
    multidb: false
    dbNames: "order_db,user_db"          # multidb=true 时必填
    order_db: "user:pwd@tcp(...)/order?..."   # 多库时每库一项
    mysql_debug: false                   # true 打印 SQL
    mysql_pool:
      max: 100          # 最大打开连接数；<=1 时不启用连接池配置
      total: 50         # 最大空闲；0 或 < max 时取 5*max
      timeout: 30       # 空闲超时（秒），默认 60
      life: 60          # 连接生命周期（分钟），默认 60
```

获取连接：

```go
conn, err := db.Mysql.GetConnection()          // 单库
conn, err := db.Mysql.GetConnection("order_db") // 多库，必须指定
```

约束：

- 多库时 `GetConnection()` 不传库名会报错；
- `mysql_pool.max <= 1` 时整段池参数被忽略（使用驱动默认）；
- DSN 必须带 `parseTime=True`，否则 `time.Time` 字段扫描失败。

### 3.2 Redis

```yaml
go:
  data:
    redis:
      uri: "127.0.0.1:6379"     # 地址列表，逗号分隔；也可写 host/port
      host: "127.0.0.1"
      port: "6379"
      password: ""
      database: 0
      master: ""                # 哨兵模式主节点名
      multidb: false
      dbNames: "0,1"
      0: { uri: "127.0.0.1:6379", database: 0 }
    redis_pool:
      max: 100                  # >1 才启用
      min: 2                    # 默认 2
      idleTimeout: 5            # 分钟，默认 5
      timeout: 60               # 拨号超时（秒），默认 60
```

获取：

```go
rc, err := db.Redis.GetConnection()      // 单库
rc, err := db.Redis.GetConnection("0")   // 多库
```

### 3.3 MongoDB

```yaml
go:
  data:
    mongodb:
      uri: "mongodb://127.0.0.1:27017"
      db: "order"
      multidb: false
      dbNames: "order,log"
      order: "mongodb://127.0.0.1:27017"   # 多库时每库一项
    mongo_pool:
      max: 100
```

```go
dbConn, err := db.Mongo.GetConnection()      // 多库默认第一个
defer db.Mongo.ReturnConnection(dbConn)      // 必须归还
```

### 3.4 SQLite

```yaml
go:
  config:
    used: "sqlite"
    prefix:
      sqlite: ./data/order.db     # 相对可执行文件目录
```

- 需要 CGO（`gorm.io/driver/sqlite` 基于 mattn/go-sqlite3）；
- 无连接池配置；
- `Check()` 恒返回 nil（探针不会发现文件损坏）。

### 3.5 启停与顺序

`used` 是唯一开关：

```yaml
go:
  config:
    used: "mysql,redis,mongodb,job,nacos"
```

初始化顺序：DB(10) → Redis(15) → Registry(30) → Job(90)。
关闭逆序。单个组件初始化失败不影响其他组件。

---

## 4. 注册中心

```yaml
go:
  discovery:
    registry: nacos      # 决定使用哪个客户端
    callType: json       # client 默认协议 x-form|json|restful
    cacheTTL: 60         # 发现结果缓存秒数，默认 60
    cacheRefresh: 30     # 后台刷新秒数，默认 30
```

子配置（`conf/nacos-test.yml` 等）：

| 后端 | 键 |
|------|----|
| Nacos | `go.nacos.server`、`port`、`group`、`clusterName`、`lan`、`lanNet`、`apiversion` |
| etcd | `go.etcd.server`、`port`、`group`、`prefix`、`clusterName`、`lan`、`lanNet` |
| Consul | `go.consul.server`、`port`、`group`、`clusterName`、`lan`、`lanNet` |
| Polaris | `go.polaris.server`、`port`、`namespace`、`token`、`lan`、`lanNet` |

约束：

- `used` 必须包含对应名称，否则注册插件不启用；
- Polaris 只做发现，**不会自动注册**；
- `lan/lanNet` 用于多网卡环境选择注册 IP。

---

## 5. 日志配置

```yaml
go:
  logger:
    level: info              # debug|info|warn|error；实时生效
    out: console             # console|file，可逗号分隔
    file: logs/order         # out 含 file 时使用，按天追加
    outputs:                 # 可选：多目标
      - target: console
        format: text
      - target: file
        file: logs/order
        format: json
        fields: [time, level, traceId, message]
        logtext: false
  log:
    get: full                # 访问日志响应体：full|line|off
    req: orderReqLog         # MongoDB 集合名（非空才注册 Mongo 日志 Handler）
    db: mongodb              # 日志使用的数据库类型
    dbName: ""               # 多库时指定库
    api: orderApiLog
    call: orderCallLog
    kafka:
      use: false
      topic: order-log
    sink:
      queue: 100             # 有效范围 1–100，超出按 100
      workers: 1             # 恒为 1（保证 FIFO）
      shutdown: 3000         # 关闭排空超时（毫秒）
    victorialogs:
      client: false
      url: ""
      # 其余字段见 08 配置参考
```

要点：

- `go.log.get` 只控制**响应体**记录：`full` 全文、`line` 单行截断、其他值等同 `full`；
- `go.log.req` 同时被当作 MongoDB 集合名与开关：为空则不注册 Mongo Handler；
- 日志级别每次输出实时读取，可用于临时排障；
- 生产建议 `get: line` 或 `off`，避免记录敏感响应。

---

## 6. 探针与指标

```yaml
go:
  health:
    enabled: true         # 在 baseRouter 中挂载 /health/*
    autoStarted: true     # Run() 启动监听后自动 MarkStarted；默认 false
  metrics:
    enabled: true         # 仅表示“允许”，仍需 app.EnableMetrics()
```

行为：

- `go.health.enabled=true` → 自动挂载 `/healthz`、`/health/live`、`/health/ready`、`/health/startup`；
- `go.metrics.enabled=true` 单独不会挂载 `/metrics`，必须调用 `app.EnableMetrics()`；
- `/health/ready` 会实时 `Check()` 已启用的 mysql/mongodb/redis，失败返 503。

---

## 7. 定时任务

```yaml
go:
  job:
    enabled: true
    initdb: true             # 自动建表，默认 true
    tablePrefix: "mgin_"     # 表名 mgin_job_info / mgin_job_log
    scanInterval: 1          # 扫描间隔（秒），默认 1
    refreshInterval: 30      # 配置刷新（秒），默认 30
    logRetainDays: 30        # 日志保留天数，0 不清理
    maxConcurrent: 50        # 全局并发上限
    maxSerialQueue: 10       # serial 策略排队上限
    timezone: Asia/Shanghai  # 默认 Local
    dbName: ""               # 多库时指定
```

启用条件：`go.config.used` 含 `job` **且** `go.job.enabled=true`。

---

## 8. 国际化

```yaml
go:
  i18n:
    enabled: true
    default: zh-cn
    dbName: ""               # 多库时指定
    sqlite: ""               # 无 MySQL 时独立 SQLite 文件，默认 <app>_i18n.db
    api:
      enabled: true          # 暴露 /i18n CRUD
```

表名 = `go.application.name + "_i18n"`；应用名须为 1–59 位 ASCII 字母/数字/`.`/`_`/`-`。

---

## 9. 限流

```yaml
go:
  ratelimit:
    enabled: true
    whitelist: ["/health/*", "/metrics"]
    whiteIps: ["10.0.0.0/8"]
    idleTimeout: 600          # 空闲限流器回收（秒）
    headers: true             # 输出 X-RateLimit-*
    # 全局规则（与 Config 同层）
    algorithm: token_bucket
    dimension: ip
    rate: 100
    burst: 200
    # 独立规则，按顺序优先匹配，命中即不再叠加全局规则
    rules:
      - name: login
        path: /api/v1/login
        methods: [POST]
        algorithm: sliding_window
        dimension: ip
        window: 60
        rate: 5
        code: 1011
        httpStatus: 429
      - name: export
        path: /api/v1/export/*
        algorithm: concurrency
        dimension: global
        maxConcurrent: 2
        wait: true
        waitTimeoutMs: 1000
```

默认值：

| 字段 | 默认 |
|------|------|
| algorithm | `token_bucket` |
| dimension | `global` |
| window | 1 秒 |
| burst | `ceil(rate)`，至少 1 |
| code | 1011 |
| httpStatus | **200**（框架统一用业务码表达错误，建议改 429） |

---

## 10. 安全

```yaml
go:
  jwt:
    secret: "change-me"
  casbin:
    enabled: true
    model_file: casbin.conf    # 相对 application.yml 目录
  sys:
    enabled: true
    initdb: false
    baseUri: /api/v1           # 默认 /api/v1
    swagger:
      enabled: true
      uri: /swagger/sys        # 默认 /swagger/sys
    casbin: true
```

---

## 11. 加载流程

```
NewApp
  ├─ config.Init(application.yml)
  │    ├─ koanf.Load(file) → Cnf
  │    └─ 解析 App/Config/Log/Logger/Discovery/Jwt/Sys/Casbin/Framework
  ├─ plugin.InitAll
  │    ├─ Mysql.Init(GetConfigData(prefix.mysql))
  │    │     └─ 按 server_type 取 mysql-<env>.yml → koanf → gorm.Open
  │    ├─ Redis.Init(...)
  │    ├─ Registry.Register(GetConfigData(prefix.nacos))
  │    └─ Job.Start() → 读 go.job.* + 自动建表
  └─ baseRouter → 读 go.health / go.metrics / go.i18n / go.ratelimit
```

失败点：

- `application.yml` 读失败：`Cnf=nil`，后续组件拿不到配置，通常表现为“全部未启用”；
- 子配置文件缺失：对应组件 Init 报错，但进程继续；
- VictoriaLogs 配置非法：panic（启动失败）。

---

## 12. 配置错误诊断

| 症状 | 可能原因 | 确认方式 |
|------|----------|----------|
| 组件没初始化 | `used` 没写/拼写不一致 | 打印 `config.Config.Config.Used` |
| `GetConfigString` 返回空 | 键在子配置文件里，不在 application.yml | 用组件的 `conf` 查，或改放主配置 |
| 改了配置不生效 | 该键非热更新 | 重启；查是否被代码覆盖（`Enable*`） |
| MySQL 连接失败但配置看着对 | DSN 缺 `parseTime=True`、库不存在、网络 | 看启动日志；用 mysql client 直连 |
| 探针 404 | 未启用或 `EnableHealth` 调用在旧 `App` 上 | 查 `go.health.enabled` 与启动日志 |
| `/metrics` 404 | 只配了 `go.metrics.enabled` | 加 `app.EnableMetrics()` |
| 配置中心拉不到 | `prefix`/`env` 拼出的文件名不存在 | 按 §3.1 规则核对 dataId |
| 限流不生效 | `used` 未含 `ratelimit`？不需要；查 `go.ratelimit.enabled` 与规则路径 | 打日志/看响应头 `X-RateLimit-*` |
| 定时任务不跑 | 少 `job` 或 `enabled=false` | 查启动日志 `[Job]` |

---

## 13. 本地中间件（Docker）

```bash
docker run -d --name mgin-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=order mysql:8.0

docker run -d --name mgin-redis -p 6379:6379 redis:7-alpine

docker run -d --name mgin-mongo -p 27017:27017 mongo:6

docker run -d --name mgin-nacos -p 8848:8848 -p 9848:9848 \
  -e MODE=standalone nacos/nacos-server:v2.2.0

docker run -d --name mgin-etcd -p 2379:2379 \
  quay.io/coreos/etcd:v3.5.4 \
  /usr/local/bin/etcd --advertise-client-urls http://0.0.0.0:2379 \
  --listen-client-urls http://0.0.0.0:2379

docker run -d --name mgin-consul -p 8500:8500 consul:1.15 agent -dev -client=0.0.0.0
```
