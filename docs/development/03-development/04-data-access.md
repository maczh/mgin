# 数据结构与存储设计（jh 分支）

| 项 | 内容 |
|----|------|
| 代码基线 | `jh` 分支 |
| 覆盖范围 | MySQL、SQLite、MongoDB、Redis、内存/磁盘缓存 |
| 关联 | [架构说明](../02-concepts/01-architecture.md) · [模块与接口](../08-reference/api/README.md) |

> 本分支**没有** PostgreSQL / ClickHouse / ElasticSearch / Kafka 的实现，`go.data.postgres.*`、`go.data.clickhouse.*`、`go.data.elasticsearch.*` 无消费方。数据库表结构以本文与代码中的模型为准。

---

## 1. 存储全景

| 存储 | 用途 | 建表/建库方式 | 代码入口 |
|------|------|---------------|----------|
| MySQL | 业务数据、casbin 策略、i18n 字典、job 任务 | 业务自行迁移；casbin 由 adapter 建表；job 可 `AutoMigrate` | `db.Mysql`、`db/dao` |
| SQLite | 本地开发、job 单机库、i18n 降级库 | job `AutoMigrate`；i18n 内置 DDL | `db.Sqlite`、`job`、`i18n` |
| MongoDB | 接口访问日志（`PostLog`） | 首次写入自动创建集合 | `middleware/postlog` |
| Redis | DAO 查询缓存、业务缓存 | 无需预建 | `db/dao.RedisCache`、`db.Redis` |
| 内存 | DAO 缓存、trace/语言上下文、服务发现缓存、限流计数 | — | `cache.OnMemCache`、`utils` |
| bitcask | 需要跨进程重启保留的本地缓存 | 自动创建文件 | `cache.OnDiskCache` |

---

## 2. 数据源与配置键

### 2.1 启用方式

```yaml
go:
  config:
    used: "mysql,redis,mongodb,job"
    env: test
    prefix:
      mysql: mysql        # → conf/mysql-test.yml
      redis: redis        # → conf/redis-test.yml
      mongodb: mongodb    # → conf/mongodb-test.yml
      sqlite: data/app.db # 例外：值就是文件名
```

`used` 为**子串匹配**；未声明的组件不会初始化，也不会被 `/health/ready` 检查。

### 2.2 MySQL（`db/mysql`）

| 键 | 类型 | 说明 |
|----|------|------|
| `go.data.mysql` | string | 单库 DSN |
| `go.data.mysql.multidb` | bool | 多库开关 |
| `go.data.mysql.dbNames` | string | 多库名，逗号分隔 |
| `go.data.mysql.<dbName>` | string | 各库 DSN |
| `go.data.mysql_debug` | bool | 打印 SQL |
| `go.data.mysql_pool.max` | int | 最大连接；`<=1` 不启用池 |
| `go.data.mysql_pool.total` | int | 最大空闲；缺省 `5*max` |
| `go.data.mysql_pool.timeout` | int（秒） | 空闲超时，缺省 60 |
| `go.data.mysql_pool.life` | int（分钟） | 连接生命周期，缺省 60 |

多库时必须 `db.Mysql.GetConnection("dbName")`；单库忽略参数。

### 2.3 Redis（`db/redis`）

| 键 | 说明 |
|----|------|
| `go.data.redis.multidb` / `dbNames` | 多库开关与库名 |
| `go.data.redis.<db>.uri|host|port|master|password|database` | 单库或多库连接 |
| `go.data.redis.uri|host|port|master|password|database` | 单库连接 |
| `go.data.redis_pool.min|max|idleTimeout|timeout` | 池参数（`idleTimeout` 分钟，`timeout` 秒） |

`database == -1` 或多地址时强制 DB=0。

### 2.4 MongoDB（`db/mongo`）

| 键 | 说明 |
|----|------|
| `go.data.mongodb.multidb` / `dbNames` | 多库 |
| `go.data.mongodb.<db>.uri` / `.db` | 多库连接与库名 |
| `go.data.mongodb.uri` / `.db` | 单库连接与库名 |
| `go.data.mongo_pool.max` | 连接池上限（最小 10） |

### 2.5 SQLite（`db/sqlite`）

`go.config.prefix.sqlite` 即文件名；为空时用 `<可执行目录>/<go.application.name>.db`。相对路径基于可执行文件目录。

---

## 3. MySQL 表设计

### 3.1 定时任务（`job`）

前缀由 `go.job.tablePrefix` 控制，默认 `mgin_`；`go.job.initdb: true` 时 `AutoMigrate`。

**`mgin_job_info`**

| 字段 | 类型（GORM 推导） | 说明 |
|------|------------------|------|
| `id` | bigint PK auto | 任务 ID |
| `job_name` | varchar，唯一 | 任务名 |
| `job_group` | varchar | 分组 |
| `description` | varchar | 描述 |
| `schedule_type` | varchar | `cron/fixed_rate/fixed_delay/once` |
| `schedule_conf` | varchar | cron 表达式或间隔 |
| `handler_name` | varchar | 对应 `job.Register` 的名称 |
| `job_param` | text | 传给执行器的参数（常为 JSON） |
| `timeout` | int | 超时秒数 |
| `retry_count` / `retry_interval` | int | 失败重试 |
| `block_strategy` | varchar | `serial/discard/concurrent/cover` |
| `misfire_strategy` | varchar | `do_nothing/fire_now` |
| `status` | int | 启停状态 |
| `last_fire_time` / `next_fire_time` | datetime | 调度时间 |
| `trigger_count` / `success_count` / `fail_count` | int | 统计 |
| `remark` | varchar | 备注 |
| `create_at` / `update_at` | datetime | 时间戳 |

**`mgin_job_log`**

| 字段 | 说明 |
|------|------|
| `id` | 主键 |
| `job_id` / `job_name` / `job_group` / `handler_name` | 任务标识 |
| `job_param` | 本次参数 |
| `trigger_type` | 触发方式（定时/手动） |
| `status` | 执行结果 |
| `start_at` / `end_at` | 起止时间 |
| `cost_ms` | 耗时 |
| `retry_num` | 重试次数 |
| `message` | 摘要 |
| `log_detail` | 执行器 `ctx.Log` 写入的明细 |
| `hostname` | 执行实例 |
| `create_at` | 创建时间 |

日志清理：`go.job.logRetainDays`（默认 30，0 不清理）。

### 3.2 国际化字典（`i18n`）

表名 `{go.application.name}_i18n`（应用名需匹配 `^[A-Za-z0-9_.-]{1,59}$`），MySQL 使用 `InnoDB` + `utf8mb4_bin`：

```sql
CREATE TABLE IF NOT EXISTS `myapp_i18n` (
  `message_id` VARCHAR(191) NOT NULL,
  `language`   VARCHAR(35)  NOT NULL,
  `text`       TEXT         NOT NULL,
  `created_at` DATETIME(3)  NOT NULL,
  `updated_at` DATETIME(3)  NOT NULL,
  PRIMARY KEY (`message_id`, `language`)
);
```

SQLite 版本时间列为 `DATETIME`，其余相同。语言值统一小写、`_` 归一化为 `-`（如 `zh-cn`、`en-us`）。

### 3.3 Casbin 策略（`casbin_rule`）

由 `gorm-adapter` 自动建表/迁移，字段为 `ptype, v0, v1, v2, v3, v4, v5`。本框架写入的规则形如：

| ptype | v0 | v1 | v2 | 含义 |
|-------|----|----|----|------|
| `p` | 用户 ID 或角色 ID | 路径 | 方法 | 允许访问 |

模型文件由 `go.casbin.model_file` 指定（仓库提供 `casbin.conf`），匹配器为 `keyMatch2(r.obj, p.obj)`。

### 3.4 内置系统管理模型（`models/sys`，仅模型）

这些模型**不会**被框架自动迁移；`go.sys.initdb` 仅被解析，没有建表逻辑。业务如启用，需要自行 `AutoMigrate` 或写 SQL。

公共字段（`BaseModel`）：`del_flag`、`create_at`、`update_at`、`update_by`、`create_by`、`tenant_id`。

| 表 | 关键字段与约束 |
|----|----------------|
| `sys_user` | `login_name`（唯一）、`nick_name`、`user_type`、`email`、`mobile`、`sex`、`avatar`、`password`、`status`、`login_ip`、`login_date`、`remark` |
| `sys_user_ext` | `user_id`（唯一）、`department_id`、`position_id`、`role_id`（无 BaseModel） |
| `sys_dept` | `parent_id`、`ancestors`、`name`（唯一）、`sort`、`leader`、`mobile`、`dept_type`、`status` |
| `sys_post` | `post_code`（唯一）、`post_name`（唯一）、`dept_id`、`sort`、`status` |
| `sys_role` | `role_name`（唯一）、`role_ident`（唯一）、`is_enable`、`description` |
| `sys_role_api` | `role_id`、`api_id` |
| `sys_role_resource` | `role_id`、`resource_id` |
| `sys_resource` | `type`、`menu_level`、`parent_id`、`path`、`name`、`component`、`redirect`、`icon`、`title`、`sort`、`hidden`、`keep_alive`、`always_show`、`breadcrumb`、`affix`、`active_resource`、`status`、`api_id` |
| `sys_api` | `name`（唯一）、`api_path` + `method`（复合唯一 `api_unique`）、`api_group`、`enabled`、`need_auth`、`need_log` |
| `sys_dict` | `type`、`parent_id`、`name`、`key`、`value`、`sort`、`remark` |
| `sys_config` | `module`、`name`、`key`（唯一）、`value`（text）、`description`（无 BaseModel） |
| `sys_social` | `user_id`、`auth_id`、`source`（`idx_auth` 复合索引）、`access_token`、`expire_in`、`refresh_token`、`open_id`、`union_id`、`scope`、`token_type`、`id_token` |
| `sys_user_online` | `token_id`（`md5(jwt)`，业务主键）、`user_id`、`user_name`、`client_key`、`device_type`、`ip_addr`、`login_location`、`browser`、`os`、`login_time`（无 BaseModel） |

> `sys_user_online` 的 `id` 与 `token_id` 都标了 primary 标记，实际以 `token_id` 为业务主键；建表时建议只保留一个物理主键。

---

## 4. MongoDB 设计

### 4.1 接口访问日志（`models.PostLog`）

集合名来自 `go.log.req`（例如 `myappRequestLog`）；**仅当** `used` 含 `mongodb` 时写入；多库模式需配置 `go.log.dbName`，未配置时 handler 直接返回错误（日志丢弃并计数）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `time` | string | 请求时间（格式化） |
| `requestId` | string | `X-Request-ID` |
| `responseTime` | string | 耗时（毫秒字符串） |
| `ttl` | int | 业务自定义 |
| `appName` | string | 应用名 |
| `apiName` | string | 接口名（可由业务设置） |
| `method` | string | HTTP 方法 |
| `contentType` | string | 请求类型 |
| `uri` | string | 路径 |
| `clientIP` | string | 客户端 IP |
| `requestHeader` | map[string]string | 请求头 |
| `requestParam` / `requestBody` | any | 请求内容 |
| `responseStr` | string | 响应文本 |
| `responseMap` | any | 响应映射 |

代码未创建索引。生产建议至少为 `time`（TTL 索引）与 `requestId` 建索引：

```js
db.myappRequestLog.createIndex({ time: 1 }, { expireAfterSeconds: 30*24*3600 })
db.myappRequestLog.createIndex({ requestId: 1 })
```

### 4.2 其它集合

`go.log.call`（调用日志）与 `go.log.api` 在当前代码中没有独立写入实现，仅保留配置字段；调用日志通过 VictoriaLogs（`go.log.victorialogs.client`）或自定义 `logsink` handler 输出。

---

## 5. Redis 设计

### 5.1 DAO 查询缓存键

`MySQLDao.WithCache(CacheConfig)` 生成的键：

```
<Prefix><Tag>:<TableName>:<op>:<md5(json(query))>
```

- `Prefix` 为空时默认 `mgin:mysql:`；`Tag` 为 nil 时为空串。
- query 的 map 键会排序、空值字段被剔除，保证同条件同键。
- 支持的读操作：`WhereAll/WhereOne/WhereCount/WhereExists/All/Exists/Count`。
- 写操作（`Create/MultiCreate/Updates/Save/Delete`）成功后按键前缀删除缓存。
- `TTL <= 0` 时**永不过期**（`RedisCache.Set` 与 `MemCache.Set` 对 0 都按永久处理）。

建议：为业务缓存统一加前缀（如 `app:module:`），不要与框架默认前缀混用。

### 5.2 其它 Redis 使用

- `RedisCache.Clear()` 执行 `FLUSHDB`、`Range` 执行全库 `SCAN`：**禁止与业务共用同一 DB 编号**。
- 服务发现缓存、trace/语言上下文、限流计数均在**进程内存**，不写 Redis。
- `PSubscribe` 用于多库订阅，内部带无限重连。

---

## 6. 内存与磁盘缓存

| 项 | 内存（`MemCache`） | 磁盘（`DiskCache`，bitcask） |
|----|--------------------|------------------------------|
| 过期 | 每条独立 TTL + 每分钟清理协程 | `Put`（永久）/`PutWithTTL` |
| 容量 | **无上限、无淘汰** | 受磁盘限制 |
| 值类型 | 任意（写入/读取用 `clone.Clone` 深拷贝） | 统一序列化为 string，读取返回 string |
| 生命周期 | 进程内 | 跨进程（文件在可执行目录或指定路径） |
| 适用 | 热点字典、临时状态 | 需要重启后保留的小型缓存 |

⚠️ 内存缓存没有容量保护，不要把用户请求级的大对象长期写入。

---

## 7. 数据生命周期

| 数据 | TTL/保留策略 | 配置 |
|------|--------------|------|
| DAO 查询缓存 | `CacheConfig.TTL`；0 = 永久 | 代码 |
| 内存缓存项 | 写入时 `lifeSpan` | 代码 |
| 服务发现缓存 | 默认 60s + 后台热刷新 60s | `go.discovery.cacheTTL/cacheRefresh` |
| trace/语言上下文 | 5 分钟（按 goroutine ID） | 代码 |
| 任务执行日志 | 默认 30 天 | `go.job.logRetainDays` |
| 访问日志（Mongo） | 代码不清理，建议 TTL 索引 | 手工 |
| 日志文件 | 按天切分，不自动清理 | 运维 |

---

## 8. 初始化、迁移与备份

| 对象 | 初始化方 | 说明 |
|------|----------|------|
| `mgin_job_info` / `mgin_job_log` | 框架（`AutoMigrate`） | `go.job.initdb: true`；生产建议改为受控迁移 |
| `{app}_i18n` | 框架（DDL） | 首次初始化时 `CREATE TABLE IF NOT EXISTS` |
| `casbin_rule` | gorm-adapter | 首次 `GetEnforcer` 时创建 |
| `sys_*` | **无** | 业务自行迁移 |
| 业务表 | **无** | 业务自行迁移 |
| Mongo 集合/索引 | 写入时自动建集合；索引需手工 | 建议见 §4.1 |

备份建议：

- MySQL：`mysqldump --single-transaction` 按库备份；`casbin_rule` 与 i18n 字典一并纳入。
- MongoDB：`mongodump` 备份日志库；日志量大时按时间范围导出后清理。
- SQLite：停机或 `VACUUM INTO` 备份，避免直接复制正在写入的文件。
- Redis：缓存可丢；若业务把 Redis 当主存储，需单独评估持久化策略。

---

## 9. 一致性与事务边界

- 事务 API 只覆盖 MySQL/SQLite（`db.MySQLTx`、`db.TransactionOn`）；MongoDB/Redis 无跨库事务。
- `MySQLDao.WithTx(tx)` 返回绑定事务的副本；事务内**不清缓存**，提交后需自行失效或等待 TTL。
- 缓存与数据库是「先写库、再删缓存」，没有分布式锁；高并发下的短暂不一致由 TTL 兜底。
- `job` 的调度状态存库，但调度器无分布式锁：多实例部署会重复触发，需业务幂等或改用单实例。
