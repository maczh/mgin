# 配置分域指南

本文解释「为什么这样配、配了会影响什么」。完整键表与默认值见 [变量与配置](../08-reference/variables.md)。

## 1. 应用与端口

```yaml
go:
  application:
    name: order-service
    port: 8080
    port_ssl: 8443
    cert: server.crt
    key: server.key
    project: order
    debug: true
    ip: 10.0.0.12
```

- `name` 同时用于：注册中心实例名、SQLite 默认文件名、i18n 表名前缀。改名会影响这三处。
- `port > 0` 启动 HTTP；`cert`/`key` 非空时额外启动 HTTPS（`port_ssl`）。
- `debug: true` 会把 gin 设为 debug 并强制日志级别为 debug，**生产必须关闭**。
- `ip` 留空时自动探测；多网卡/NAT 环境建议显式指定，否则注册的地址可能不可达。

## 2. 日志

```yaml
go:
  logger:
    level: info
    outputs:
      - target: console
        format: text
        logtext: "[{time}] [{level}] [{request_id}] {message}"
      - target: file
        file: ./logs/order
        format: json
      - target: victorialogs
        format: json
        fields: [time, level, app, request_id, message]
  log:
    get: line
    sink: { queue: 100, workers: 1, shutdown: 3000 }
    victorialogs:
      url: https://vl.example.com:9428
      api_token: ""
      timeout: 5000
```

建议：

- 生产用 `outputs`（多目标 + 结构化），`out/file` 仅用于兼容老配置。
- `file` 目标按天切分，不自动清理，需配合 logrotate 或定时任务。
- `queue` 上限 100（代码默认 1024）；队列满会丢最旧日志，用 `Logsink` 统计观察。
- `go.log.get: line` 可避免把大响应体写入日志；`off` 关闭 GET 响应记录。

## 3. 服务发现

```yaml
go:
  discovery:
    registry: nacos
    callType: json
    cacheTTL: 60
    cacheRefresh: 60
```

- `registry` 为空表示不注册、不发现；调用方会直接报「注册中心未初始化」。
- `cacheTTL` 越大，实例下线感知越慢；越小，注册中心压力越大。30–60s 是常用区间。
- `cacheRefresh` 控制热服务的后台刷新，`<=0` 时使用内部默认值。
- Nacos/etcd/Consul 需要同时在 `used` 中声明，并配置对应的 `go.nacos.*`/`go.etcd.*`/`go.consul.*`。
- Nacos 配置中心和注册中心的 `apiversion` 均支持 `v1`、`v2`、`v3`。配置中心 v3 使用 `/nacos/v3/client/cs/config` 并解析 `data.content`；鉴权可配置 `accessToken`，或由 `username/password` 自动登录换取 token。
- Polaris **不会**被自动注册，需要在代码中调用 `registry.Registry.Register(...)`。

## 4. 数据源

### 4.1 连接池

```yaml
go:
  data:
    mysql_pool: { max: 200, total: 1000, timeout: 30, life: 5 }
```

- `max` 必须 `> 1` 才启用池；`max < 10` 会被抬到 10。
- `total` 是最大空闲连接，缺省 `5*max`；不要设成大于 `max`。
- 池大小经验值：`max ≈ 峰值并发 SQL 数`，不超过数据库 `max_connections / 实例数` 的 70%。
- `life` 单位为分钟，配合数据库侧的连接回收，避免长连接被服务端静默断开。

### 4.2 多库

```yaml
go:
  data:
    mysql:
      multidb: true
      dbNames: order,user
      order: dsn1
      user: dsn2
```

代价：所有 `GetConnection()` 必须传库名；casbin 内部不传库名，**多库模式下不可用**；`/health/ready` 会逐库检查。

### 4.3 Redis 与缓存

- `go.data.redis_pool.idleTimeout` 单位为分钟，`timeout` 为秒。
- `database: -1` 或多地址（集群）时强制使用 DB 0。
- DAO 缓存 TTL 由代码 `CacheConfig.TTL` 决定，**0 表示永不过期**，生产必须显式设置。
- 不要与业务共用同一个 Redis DB：`RedisCache.Clear()` 会执行 `FLUSHDB`。

## 5. 安全

```yaml
go:
  jwt:
    secret: change-me
  casbin:
    enabled: true
    model_file: casbin.conf
  cors:
    headers: "X-Tenant-Id"
```

- `jwt.secret` 不能为空，否则 `JwtAuthorize` 对所有请求返回 401。
- `casbin.model_file` 相对 `WorkDir`；仓库只提供 `casbin.conf`，默认值 `rbac_model.conf` 会找不到。
- `cors.headers` 只追加允许头，允许方法固定，不能通过配置放开。
- 生产禁止把密钥写入仓库；用 Secret 挂载或配置中心加密字段。

## 6. 限流

```yaml
go:
  ratelimit:
    enabled: true
    idleTimeout: 600
    whitelist: ["/health", "/actuator"]
    whiteIps: ["127.0.0.1"]
    rules:
      - name: "登录"
        path: "/api/login/*"
        methods: ["POST"]
        algorithm: "sliding_window"
        dimension: "ip"
        rate: 5
        window: 60
        httpStatus: 429
        code: 1011
```

- 规则按声明顺序匹配，未命中回落全局规则。
- `dimension=header` 时需指定 `headerKey`；缺失会回退 `global`。
- 默认返回 HTTP 200 + `status=1011`；对客户端不友好时显式设 `httpStatus: 429`。
- `whitelist` 是路径前缀匹配，`whiteIps` 精确匹配。

## 7. 任务

- `job` 需要 MySQL 或 SQLite；`initdb: true` 会自动建表。
- `maxConcurrent` 控制全局并发，`maxSerialQueue` 控制 `serial` 策略的排队上限。
- `logRetainDays=0` 表示不清理，长期运行会持续增长。
- 单机调度：多副本部署会重复执行，必须让业务幂等或仅单实例开启。

## 8. 国际化

- `dbName` 用于 MySQL 多库时选库；留空取第一个。
- `sqlite` 用于没有 MySQL 时的独立字典文件（相对 `WorkDir`）。
- 表名由 `application.name` 决定，改名等于换表，需要迁移数据。
- `api.enabled: true` 会暴露 `/i18n` 的增删改查，**生产需加鉴权**。

## 9. 常见取值建议

| 场景 | 建议 |
|------|------|
| 开发环境 | `debug: true`、`logger.level: debug`、`used: ""` 起步 |
| 预发/生产 | `debug: false`、`level: info`、显式 `ip`、`httpStatus: 429` |
| 高并发读 | DAO 缓存 + Redis，TTL 30–300s |
| 多实例 | job 只在一个实例开启；配置中心统一管理 `used` 与数据源 |
| 弱网/跨机房 | 增大 `cacheTTL`，调用方自行加超时与熔断 |
