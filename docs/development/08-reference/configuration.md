# 配置项参考

> 穷举当前代码读取的配置键。**“读取”表示代码中确实有消费方**；
> 标 ❌ 的键仅为兼容保留或脚手架模板生成，配置后不生效。
>
> 类型：`string` / `int` / `bool` / `[]string` / `object`。
> 默认值以代码中的 normalize 逻辑为准。

---

## 1. go.application

| 键 | 类型 | 默认 | 读取 | 说明 |
|----|------|------|------|------|
| `name` | string | 空 | ✅ | 服务名；注册中心、i18n 表名、日志 app 字段 |
| `project` | string | `DEFAULT` | ✅ | 配置中心 group/命名空间 |
| `port` | int | 0 | ✅ | HTTP 端口；0 不监听 |
| `port_ssl` | int | 0 | ✅ | HTTPS 端口；需 cert+key |
| `cert` | string | 空 | ✅ | 证书路径，相对可执行文件目录 |
| `key` | string | 空 | ✅ | 私钥路径 |
| `debug` | bool | false | ✅ | gin debug + 日志 debug |
| `ip` | string | 空 | ✅ | 注册地址；空则自动探测 |
| `shutdownTimeout` | int（秒） | 5 | ✅ | 优雅关闭超时；<=0 回退 5 |

---

## 2. go.config

| 键 | 类型 | 默认 | 读取 | 说明 |
|----|------|------|------|------|
| `server` | string | 空 | ✅ | 配置中心地址 |
| `server_type` | string | 空 | ✅ | `file`/`nacos`/`consul`/`etcd`/`polaris`/`springconfig` |
| `token` | string | 空 | ✅ | Polaris 等需要的 token |
| `path` | string | 空 | ✅ | file 模式目录 |
| `env` | string | 空 | ✅ | 子配置环境后缀 |
| `used` | string | 空 | ✅ | 逗号分隔的启用列表（子串匹配） |
| `prefix.mysql` | string | 空 | ✅ | MySQL 子配置前缀 |
| `prefix.redis` | string | 空 | ✅ | Redis 前缀 |
| `prefix.mongodb` | string | 空 | ✅ | Mongo 前缀 |
| `prefix.sqlite` | string | 空 | ✅ | **SQLite 文件路径** |
| `prefix.nacos` | string | 空 | ✅ | Nacos 子配置前缀 |
| `prefix.etcd` | string | 空 | ✅ | etcd 前缀 |
| `prefix.consul` | string | 空 | ✅ | Consul 前缀 |
| `prefix.postgres` | string | 空 | ❌ | **未读取**（脚手架会写，框架忽略） |
| `prefix.clickhouse` | string | 空 | ❌ | 读取但无实现 |
| `prefix.elasticsearch` | string | 空 | ❌ | 读取但无实现 |
| `prefix.kafka` | string | 空 | ❌ | 读取但无实现 |

---

## 3. go.discovery

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `registry` | string | 空 | 注册中心类型：`nacos`/`etcd`/`consul`/`polaris`（空则不启用） |
| `callType` | string | `x-form` | client 默认协议 |
| `cacheTTL` | int（秒） | 60 | 发现结果缓存 |
| `cacheRefresh` | int（秒） | 30 | 后台刷新 |

---

## 4. 数据源子配置（mysql-test.yml 等）

### 4.1 MySQL (`go.data.*`)

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `mysql` | string | 空 | 单库 DSN |
| `multidb` | bool | false | 多库开关 |
| `dbNames` | string | 空 | 多库列表，逗号分隔 |
| `mysql.<dbName>` | string | 空 | 多库时每库 DSN |
| `mysql_debug` | bool | false | 打印 SQL |
| `mysql_cache` | bool | false | ❌ 当前无消费方 |
| `mysql_pool.max` | int | 0 | 最大连接；<=1 不启用池配置 |
| `mysql_pool.total` | int | 0 | 最大空闲；0/<max 时 5×max |
| `mysql_pool.timeout` | int（秒） | 60 | 空闲超时 |
| `mysql_pool.life` | int（分钟） | 60 | 连接寿命 |

### 4.2 Redis

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `redis.uri` | string | 空 | 地址，逗号分隔 |
| `redis.host` / `redis.port` | string | 空 | 地址（uri 优先） |
| `redis.password` | string | 空 | 密码 |
| `redis.database` | int | 0 | DB 号；集群/多地址时强制 0 |
| `redis.master` | string | 空 | 哨兵主节点名 |
| `redis.multidb` / `dbNames` | bool/string | false/空 | 多库 |
| `redis.<dbName>.uri` 等 | — | — | 多库时每库配置 |
| `redis_pool.max` | int | 0 | >1 才启用；<10 按 10 |
| `redis_pool.min` | int | 2 | 最小空闲 |
| `redis_pool.idleTimeout` | int（分钟） | 5 | 空闲超时 |
| `redis_pool.timeout` | int（秒） | 60 | 拨号超时 |

### 4.3 MongoDB

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `mongodb.uri` | string | 空 | 连接串 |
| `mongodb.db` | string | 空 | 默认库 |
| `mongodb.multidb` / `dbNames` | bool/string | false/空 | 多库 |
| `mongodb.<dbName>` | string | 空 | 多库时连接串 |
| `mongo_pool.max` | int | 0 | 连接池上限 |

### 4.4 SQLite

无子配置；`go.config.prefix.sqlite` 即文件路径（相对可执行文件目录）。

### 4.5 未实现的数据源（postgres / clickhouse / elasticsearch）

脚手架接受 `--db postgres` / `--db clickhouse` / `--db elasticsearch` 并生成子配置，
但 v2 框架没有对应实现：

| 数据源 | 脚手架生成 | 框架行为 |
|--------|-----------|----------|
| PostgreSQL | `conf/postgres-test.yml`（`go.data.postgres.dsn` 等） | 子配置不进 `used` 加载链，不建连；`dao.PostgresDao` 不存在 |
| ClickHouse | `conf/clickhouse-test.yml`（`go.data.clickhouse`） | 仅解析 `go.config.prefix.clickhouse`，不建连；`dao.ClickhouseDao` 不存在 |
| Elasticsearch | `conf/elasticsearch-test.yml`（`go.elasticsearch.*`） | 仅解析 `go.config.prefix.elasticsearch`，无客户端 |

补充：

- `pkg/config` 的 `Data.Postgres` 字段存在但从未赋值，`go.data.postgres.*` 无消费方；
- 生成的 `postgres-test.yml` 中 `cache`/`pool` 行以 **Tab** 缩进，本身不是合法 YAML
  （`found a tab character that violates indentation`）；由于该文件根本不会被加载，
  这个错误不会在启动时暴露；
- `/health/ready` 只检查 mysql/mongodb/redis，不检查上述数据源。

详见 [项目结构 §1.3](../03-development/01-project-layout.md#13-数据层支持状态实测)。

---

## 5. 注册中心子配置

| 后端 | 键 | 类型 | 默认 |
|------|----|------|------|
| Nacos | `go.nacos.server` | string | 空 |
| | `go.nacos.port` | string/int | 8848（实现默认） |
| | `go.nacos.group` | string | `DEFAULT_GROUP` |
| | `go.nacos.clusterName` | string | `DEFAULT` |
| | `go.nacos.lan` / `lanNet` | string | 空 |
| | `go.nacos.apiversion` | string | 实现默认 |
| etcd | `go.etcd.server` / `port` | string | 空 |
| | `go.etcd.group` / `prefix` | string | 空 |
| | `go.etcd.clusterName` / `lan` / `lanNet` | string | 空 |
| Consul | `go.consul.server` / `port` | string | 空 |
| | `go.consul.group` / `clusterName` / `lan` / `lanNet` | string | 空 |
| Polaris | `go.polaris.server` / `port` | string | 空 |
| | `go.polaris.namespace` | string | `default` |
| | `go.polaris.token` | string | 空 |
| | `go.polaris.lan` / `lanNet` | string | 空 |

---

## 6. 日志

### 6.1 go.logger

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `level` | string | 空（info） | `debug/info/warn/error`；实时生效 |
| `out` | string | 空 | `console`/`file`，逗号分隔 |
| `file` | string | 空 | 文件日志前缀，生成 `<file>.YYYY-MM-DD.log` |
| `outputs[]` | object[] | 空 | 多目标，字段见下 |
| `outputs[].target` | string | — | `console/file/kafka/mongo/victorialogs` |
| `outputs[].format` | string | text | `text`/`json` |
| `outputs[].fields` | []string | 空 | 保留字段 |
| `outputs[].logtext` | string | 空 | 是否保留正文（实现按非空判定） |
| `outputs[].file` | string | 空 | target=file 时的路径 |

### 6.2 go.log

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `get` | string | 空 | 响应体记录：`full`/`line`/`off`（其他值按 full） |
| `req` | string | 空 | 访问日志 Mongo 集合名；空则不注册 |
| `api` | string | 空 | 预留 |
| `call` | string | 空 | 预留 |
| `db` | string | 空 | 日志库类型（`mongodb`） |
| `dbName` | string | 空 | 多库时指定 |
| `kafka.use` | bool | false | 是否投递 Kafka |
| `kafka.topic` | string | app name | 主题 |
| `sink.queue` | int | 100 | 每 handler 队列；有效 1–100 |
| `sink.workers` | int | 1 | 恒为 1 |
| `sink.shutdown` | int（ms） | 3000 | 排空等待 |

### 6.3 go.log.victorialogs

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `url` | string | 空 | 采集端地址 |
| `api_token` | string | 空 | 认证 token |
| `api_crt` | string | 空 | CA 证书 |
| `request` | bool | false | 是否采集请求 |
| `client` | bool | false | 是否采集下游调用 |
| `timeout` | int | 0（实现默认） | 超时（秒） |

---

## 7. 健康与指标

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `go.health.enabled` | bool | false | 挂载 `/health/*` |
| `go.health.autoStarted` | bool | false | Run 后自动 MarkStarted |
| `go.metrics.enabled` | bool | false | 仅“允许”，仍需 `EnableMetrics()` |

---

## 8. 定时任务

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `go.job.enabled` | bool | false | 总开关（还需 used 含 job） |
| `go.job.initdb` | bool | true | 自动建表 |
| `go.job.tablePrefix` | string | `mgin_` | 表前缀 |
| `go.job.scanInterval` | int（秒） | 1 | 调度扫描 |
| `go.job.refreshInterval` | int（秒） | 30 | 配置刷新 |
| `go.job.logRetainDays` | int（天） | 30 | 0 不清理 |
| `go.job.maxConcurrent` | int | 50 | 全局并发 |
| `go.job.maxSerialQueue` | int | 10 | serial 排队 |
| `go.job.timezone` | string | Local | 时区 |
| `go.job.dbName` | string | 空 | 多库指定 |

---

## 9. 国际化

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `go.i18n.enabled` | bool | false | 启用中间件与 API |
| `go.i18n.default` | string | `zh-cn` | 默认语言 |
| `go.i18n.dbName` | string | 空 | 多库指定 |
| `go.i18n.sqlite` | string | `<app>_i18n.db` | 独立 SQLite 文件（相对配置目录） |
| `go.i18n.api.enabled` | bool | false | 暴露 `/i18n` CRUD |

---

## 10. 限流

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `go.ratelimit.enabled` | bool | false | 总开关 |
| `go.ratelimit.whitelist` | []string | 空 | 路径前缀白名单 |
| `go.ratelimit.whiteIps` | []string | 空 | IP 白名单 |
| `go.ratelimit.idleTimeout` | int（秒） | 600 | 空闲回收 |
| `go.ratelimit.headers` | bool | false | 输出限流响应头 |
| `go.ratelimit.rules[]` | object[] | 空 | 独立规则 |
| 全局规则 | 与 `go.ratelimit` 同层 | — | `algorithm`/`dimension`/`rate`/`burst`/`window`/`windowMs`/`maxConcurrent`/`wait`/`waitTimeoutMs`/`code`/`message`/`httpStatus` |
| `rules[].path` | string | 空 | 精确 / `/*` 前缀 / `:id` |
| `rules[].methods` | []string | 空 | 空表示不限 |
| `rules[].algorithm` | string | `token_bucket` | `sliding_window`/`concurrency` |
| `rules[].dimension` | string | `global` | `ip`/`path`/`ip_path`/`header` |
| `rules[].headerKey` | string | 空 | dimension=header 时 |
| `rules[].rate` | float | 0 | 令牌/秒 或 窗口内上限 |
| `rules[].burst` | int | ceil(rate) | 令牌桶容量 |
| `rules[].window` | int（秒） | 1 | 滑动窗口 |
| `rules[].windowMs` | int（ms） | 0 | 优先于 window |
| `rules[].maxConcurrent` | int | 0 | 并发算法 |
| `rules[].wait` / `waitTimeoutMs` | bool/int | false/1000 | 排队 |
| `rules[].code` | int | 1011 | 业务码 |
| `rules[].httpStatus` | int | 200 | HTTP 状态（建议 429） |

---

## 11. 安全

| 键 | 类型 | 默认 | 说明 |
|----|------|------|------|
| `go.jwt.secret` | string | 空 | HS 签名密钥；空则 JWT 中间件全 401 |
| `go.casbin.enabled` | bool | false | 启用 Casbin 中间件检查 |
| `go.casbin.model_file` | string | 空 | 相对 application.yml 目录 |
| `go.sys.enabled` | bool | false | 系统模块开关（框架仅读取，无内置实现） |
| `go.sys.initdb` | bool | false | 同上 |
| `go.sys.baseUri` | string | `/api/v1` | 系统接口前缀 |
| `go.sys.swagger.enabled` | bool | false | Swagger 开关（需业务集成） |
| `go.sys.swagger.uri` | string | `/swagger/sys` | 用于日志/JWT 跳过判断 |
| `go.sys.casbin` | bool | false | 系统模块是否启用 Casbin |
| `go.cors.headers` | []string | 空 | 允许的额外请求头 |

---

## 12. 框架元配置（多数未生效）

| 键 | 类型 | 默认 | 读取 | 说明 |
|----|------|------|------|------|
| `go.framework.healthEnabled` | bool | false | ❌ | 改用 `go.health.enabled` |
| `go.framework.i18nEnabled` | bool | false | ✅ | `NewApp` 的 xlang 或 `go.i18n.enabled` 亦可 |
| `go.framework.ratelimitEnabled` | bool | false | ❌ | 改用 `go.ratelimit.enabled` |
| `go.framework.recoverEnabled` | bool | false | ❌ | Recovery 恒挂载 |
| `go.framework.logRequests` | bool | false | ❌ | postlog 恒挂载 |
| `go.framework.shutdownTimeout` | int | — | ❌ | 不存在该键；用 `go.application.shutdownTimeout` |
| `go.framework.loadBalancer` | string | — | ❌ | 不存在该键；用调用参数 `Options.LoadBalancer` |

---

## 13. 运行期（只读）

| 键/字段 | 说明 |
|---------|------|
| `go.runtime.startedAt` | 进程启动时间 |
| `go.runtime.pid` | 进程 ID |
| `go.runtime.goVersion` | Go 版本 |
| `go.runtime.commitHash` / `buildTime` | 需业务注入，框架不自动填 |

---

## 14. 生效路径速查

```
used 含 mysql → prefix.mysql → mysql-<env>.yml → go.data.mysql*
used 含 redis → prefix.redis → redis-<env>.yml → go.data.redis*
used 含 mongodb → prefix.mongodb → mongodb-<env>.yml → go.data.mongodb*
used 含 sqlite → prefix.sqlite（文件路径）
used 含 job 且 go.job.enabled → go.job.* → 建表
go.discovery.registry + used 含对应后端 → <backend>-<env>.yml
go.logger.* / go.log.* → 日志初始化
```
