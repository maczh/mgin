# MGin v2 数据结构与存储设计

| 项 | 内容 |
|----|------|
| 文档版本 | v2.0 |
| 对应代码 | `pkg/models`、`pkg/db`、`pkg/job`、`pkg/casbin`、`pkg/i18n`、`pkg/cache` |
| 说明 | 本文描述**框架自身使用**的存储结构；业务表结构由业务项目自行设计 |

---

## 1. 存储全景

| 存储 | 使用方 | 是否框架建表 | 说明 |
|------|--------|--------------|------|
| MySQL | 业务 DAO、job、casbin、i18n、sys 模型 | job/i18n/casbin 会建表；sys 不建 | 主数据源 |
| SQLite | 单机/测试、job、i18n | job/i18n 会建表 | 文件库 |
| MongoDB | 接口日志（postlog）、MgoDao | 集合由驱动隐式创建 | 文档存储 |
| Redis | DAO 查询缓存、限流/会话（业务自用） | 无 | 键值缓存 |
| 本地磁盘 | diskcache（bitcask）、日志文件 | 自动创建 | `<exeDir>/cache.db`、`<file>.YYYY-MM-DD.log` |

**没有** PostgreSQL / ClickHouse / Elasticsearch / 对象存储的数据结构。

---

## 2. 数据源与多库模型

### 2.1 启用方式

```yaml
go:
  config:
    used: "mysql,redis"          # 逗号分隔；插件层用子串匹配
    prefix:
      mysql: mysql
      redis: redis
      sqlite: ./data/app.db      # sqlite 的值是文件路径，不是前缀
    env: test
```

子配置文件 `mysql-test.yml` / `redis-test.yml` 由 `go.config.server_type` 决定从本地或配置中心加载。

### 2.2 多库（multidb）

```yaml
go:
  data:
    mysql:
      multidb: true
      dbNames: "order_db,user_db"
      order_db: "user:pwd@tcp(127.0.0.1:3306)/order?charset=utf8mb4&parseTime=True&loc=Local"
      user_db:  "user:pwd@tcp(127.0.0.1:3306)/user?charset=utf8mb4&parseTime=True&loc=Local"
    mysql_pool:
      max: 100
      total: 50
      timeout: 30
      life: 3600
```

| 客户端 | 取连接 | 多库时未指定库名 |
|--------|--------|------------------|
| MySQL | `db.Mysql.GetConnection("order_db")` | 报错 |
| Redis | `db.Redis.GetConnection("0")` | 报错 |
| MongoDB | `db.Mongo.GetConnection()` | 默认第一个连接成功的库 |
| SQLite | `db.Sqlite.GetConnection()` | 不适用 |

DAO 通过 `Tag` 选择库：

```go
var orderDao = &dao.MySQLDao[Order]{Tag: func() string { return "order_db" }}
```

`Tag == nil` 时返回空串，多库 MySQL 会报错。事务内 `WithTx` 直接使用 `tx`，忽略 `Tag`。

### 2.3 主键与通用约定

- MySQL 表推荐 `InnoDB` + `utf8mb4`（casbin 错误提示明确依赖 InnoDB）。
- 时间字段统一 `datetime`（MySQL）/ `DATETIME`（SQLite），GORM 自动维护 `create_at`/`update_at`。
- 逻辑删除用 `del_flag`（`sys_*`），**不使用** GORM `DeletedAt` 软删除。

---

## 3. 框架自动建表

### 3.1 定时任务表（`pkg/job`）

建表：`store.autoMigrate()` → `AutoMigrate(&JobInfo{}, &JobLog{})`，
受 `go.job.initdb` 控制（默认 `true`），表名前缀 `go.job.tablePrefix`（默认 `mgin_`）。

#### `<prefix>job_info`

| 列 | 类型 | 约束/索引 | 说明 |
|----|------|-----------|------|
| `id` | bigint | PK, auto increment | 任务 ID |
| `job_name` | varchar(100) | unique, not null | 任务名（全局唯一） |
| `job_group` | varchar(50) | index, default `DEFAULT` | 分组 |
| `description` | varchar(255) | — | 描述 |
| `schedule_type` | varchar(20) | not null, default `cron` | `cron`/`fixed_rate`/`fixed_delay`/`once` |
| `schedule_conf` | varchar(128) | not null | cron 表达式 / 间隔秒 / 执行时间 |
| `handler_name` | varchar(100) | not null, index | 对应 `job.Register` 的名字 |
| `job_param` | text (JSON) | — | `map[string]interface{}` |
| `timeout` | int | default 0 | 执行超时（秒），0 不限 |
| `retry_count` | int | default 0 | 失败重试次数 |
| `retry_interval` | int | default 0 | 重试间隔（秒） |
| `block_strategy` | varchar(20) | default `serial` | `serial`/`discard`/`concurrent`/`cover` |
| `misfire_strategy` | varchar(20) | default `do_nothing` | `do_nothing`/`fire_now` |
| `status` | int | index, default 0 | 1 运行中 / 0 停止 |
| `last_fire_time` / `next_fire_time` | datetime | 可空 | 上次/下次调度 |
| `trigger_count` / `success_count` / `fail_count` | bigint | default 0 | 累计统计 |
| `remark` | varchar(255) | — | 备注 |
| `create_at` / `update_at` | datetime | 自动维护 | — |

#### `<prefix>job_log`

| 列 | 类型 | 约束/索引 | 说明 |
|----|------|-----------|------|
| `id` | bigint | PK, auto increment | 日志 ID |
| `job_id` | bigint | index, not null | 任务 ID |
| `job_name` | varchar(100) | index | 任务名（冗余） |
| `job_group` | varchar(50) | index | 分组（冗余） |
| `handler_name` | varchar(100) | — | 执行器名 |
| `job_param` | text (JSON) | — | 本次参数 |
| `trigger_type` | varchar(20) | — | `cron`/`manual`/`retry`/`misfire` |
| `status` | int | index, default 0 | 0 执行中 / 1 成功 / 2 失败 / 3 超时 / 4 阻塞丢弃 / 5 被取消 |
| `start_at` | datetime | index | 开始时间 |
| `end_at` | datetime | 可空 | 结束时间 |
| `cost_ms` | bigint | default 0 | 耗时（毫秒） |
| `retry_num` | int | default 0 | 第几次重试 |
| `message` | text | — | 结果/错误 |
| `log_detail` | text | — | `ctx.Log` 明细 |
| `hostname` | varchar(100) | — | 执行实例主机名 |
| `create_at` | datetime | autoCreateTime, **index** | 用于按天清理 |

保留策略：`go.job.logRetainDays`（默认 30，0 不清理），由 `cleanLoop` 定期删除。

### 3.2 i18n 字典表（`pkg/i18n`）

表名为 `<appName>_i18n`（直接拼接，不做大小写转换；应用名须匹配 1–59 位 ASCII
字母/数字/点/下划线/连字符），主键 `(message_id, language)`：

```sql
CREATE TABLE IF NOT EXISTS `<appName>_i18n` (
  message_id VARCHAR(191) NOT NULL,
  language   VARCHAR(35)  NOT NULL,
  text       TEXT         NOT NULL,
  created_at DATETIME(3)  NOT NULL,
  updated_at DATETIME(3)  NOT NULL,
  PRIMARY KEY (message_id, language)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
```

SQLite 版本将 `DATETIME(3)` 换成 `DATETIME`，去掉 `ENGINE` 后缀。

- 语言标签正则：`^[a-z]{2,8}(-[a-z0-9]{1,8})*$`，最长 35。
- `message_id` 1–191 字符，`text` 1–65535 字节。
- 缺目标语言时回退默认语言（`go.i18n.default`，默认 `zh-cn`）。
- 选库顺序：`used` 含 mysql → MySQL；含 sqlite → 共享 SQLite；否则独立 SQLite 文件（`go.i18n.sqlite`）。

### 3.3 Casbin 策略表（`casbin_rule`）

由 `gorm-adapter/v3` 自动创建，本仓库不写 DDL：

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | uint | PK, auto increment |
| `ptype` | varchar(100) | 策略类型，本模型为 `p` |
| `v0` | varchar(100) | `sub` = 角色 ID |
| `v1` | varchar(100) | `obj` = 请求路径（支持 `keyMatch2`） |
| `v2` | varchar(100) | `act` = HTTP 方法 |
| `v3`–`v5` | varchar(100) | 未使用 |

唯一索引：`idx_casbin_rule (ptype,v0,v1,v2,v3,v4,v5)`。

模型（`casbin.conf`）：

```ini
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act
```

> matcher 未使用 `g(...)`，因此 `g` 定义不参与鉴权；实际为"角色 ID 直连路径"的平铺 ACL。

策略写入示例（`Casbin.UpdateCasbin` 内部格式）：

| ptype | v0 | v1 | v2 |
|-------|----|----|----|
| p | 1 | /api/v1/users | GET |
| p | 1 | /api/v1/users/:id | PUT |

---

## 4. 内置系统模型（`pkg/models/sys`）

### 4.1 重要说明

- **框架不创建这 13 张表**：无 `AutoMigrate`、无 DDL、无迁移命令。
- 无外键约束，关联全部靠应用层拼接（`gorm:"-"` 字段）。
- 建议业务侧在初始化时显式迁移，例如：

```go
conn, _ := db.Mysql.GetConnection()
err := conn.AutoMigrate(
    &sys.SysUser{}, &sys.SysUserExt{}, &sys.SysRole{}, &sys.SysApi{},
    &sys.SysResource{}, &sys.SysRoleApi{}, &sys.SysRoleResource{},
    &sys.SysDept{}, &sys.SysPost{}, &sys.SysDict{}, &sys.SysConfig{},
    &sys.SysSocial{}, &sys.SysUserOnline{},
)
```

> 迁移前请先修正 §4.3 的类型问题，否则 `tenant_id` 会被建成 `varchar(32)`。

### 4.2 公共基类 `BaseModel`

| 列 | Go 类型 | GORM | 说明 |
|----|---------|------|------|
| `del_flag` | `uint` | `tinyint;default:0;index` | 逻辑删除 |
| `create_at` | `time.Time` | `datetime;autoCreateTime` | 创建时间 |
| `update_at` | `time.Time` | `datetime;autoUpdateTime` | 更新时间 |
| `update_by` | `string` | `varchar(32)` | 更新人 |
| `create_by` | `string` | `varchar(32)` | 创建人 |
| `tenant_id` | `int64` | `type:string;size:32` | 租户 ID（类型声明冲突） |

未嵌入 `BaseModel` 的表：`sys_user_ext`、`sys_config`、`sys_user_online`。

### 4.3 表清单

#### `sys_user`（用户）

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | bigint(20) | PK, auto increment |
| `login_name` | varchar(30) | **unique**, not null |
| `nick_name` | varchar(30) | — |
| `user_type` | varchar(10) | index |
| `email` | varchar(50) | index（注释称唯一，实际非唯一） |
| `mobile` | varchar(11) | index（同上） |
| `sex` | tinyint | 1 男 / 2 女 / 3 未知 |
| `avatar` | varchar(100) | — |
| `password` | varchar(50) | — |
| `status` | tinyint | index, default 1 |
| `login_ip` | varchar(50) | — |
| `login_date` | datetime | 可空 |
| `remark` | varchar(500) | — |
| + BaseModel | | |

#### `sys_user_ext`（用户扩展，1:1）

`id` bigint PK auto increment；`user_id` bigint **unique** not null；
`department_id` / `position_id` / `role_id` bigint index。
**无 BaseModel**；`role_id` 为单值，与多对多表并存（设计不一致）。

#### `sys_role`（角色）

`id` uint PK；`role_name` varchar(50) not null unique；
`role_ident` varchar(50) not null unique；`is_enable` tinyint default 1 index；
`description` varchar(255)；+ BaseModel。

#### `sys_api`（接口权限）

| 列 | 类型 | 约束 |
|----|------|------|
| `id` | uint | PK |
| `name` | varchar(100) | unique, not null |
| `api_path` | varchar(255) | not null, 复合唯一 `api_unique` |
| `method` | varchar(10) | not null default 'GET', 复合唯一 `api_unique` |
| `description` | varchar(255) | — |
| `api_group` | varchar(50) | index |
| `enabled` | tinyint | default 1, index |
| `need_auth` | tinyint | 是否需 JWT |
| `need_log` | tinyint | default 1 |
| + BaseModel | | |

#### `sys_resource`（前端菜单/资源）

`id` PK；`type` tinyint default 1（1 菜单 2 按钮）；`menu_level` tinyint default 1；
`parent_id` index（0 根）；`path` varchar(255) index；`name` varchar(50) index；
`component` varchar(255)；`redirect` varchar(255)；`icon` varchar(50)；
`title` varchar(50) not null；`sort` int default 0 index；`hidden` tinyint default 0；
`keep_alive` tinyint default 1；`always_show` tinyint default 0；`breadcrumb` tinyint default 1；
`affix` tinyint default 0；`active_resource` varchar(255)；`status` tinyint default 1 index；
`api_id` `*uint` index；+ BaseModel。

非持久化：`Children []SysResource`、`Roles []SysRole`、`Parent *SysResource`（均 `gorm:"-"`）。

#### `sys_role_api`（角色-接口）

`id` PK；`role_id` not null index；`api_id` not null（**无索引**）；
`Role`/`Api` 指针 `gorm:"-"`；+ BaseModel。**缺 `(role_id, api_id)` 唯一约束**。

#### `sys_role_resource`（角色-资源）

`id` PK；`role_id` not null index；`resource_id` not null（无索引）；
`Role`/`Resource` 指针 `gorm:"-"`；+ BaseModel。同样缺复合唯一约束。

#### `sys_dept`（部门）

`id` uint PK auto increment；`parent_id` uint index；`ancestors` varchar(100) index（逗号分隔祖级）；
`name` varchar(30) unique not null；`sort` int；`leader` varchar(20)；`mobile` varchar(11)；
`dept_type` varchar(50) index default 0；`status` tinyint default 1；+ BaseModel。

#### `sys_post`（岗位）

`id` bigint PK；`post_code` varchar(64) unique；`post_name` varchar(50) unique；
`dept_id` bigint index；`sort` int；`status` tinyint；`remark` varchar(500)；+ BaseModel。

#### `sys_dict`（字典）

`id` int PK auto increment；`type` varchar(50) not null index；`parent_id` int default 0；
`name` varchar(100) not null index；`key` varchar(50) not null index；
`value` varchar(100) not null；`sort` int default 0；`remark` varchar(255)；+ BaseModel。

#### `sys_config`（动态配置）

`id` int64 PK auto increment；`module` varchar(100) index；`name` varchar(50) not null index；
`key` varchar(100) unique not null；`value` text；`description` varchar(255)。**无 BaseModel**。

#### `sys_social`（第三方账号）

`id` bigint unsigned PK auto increment；`user_id` bigint unsigned index；
`auth_id` varchar(255) not null（复合索引 `idx_auth`）；`source` varchar(50) not null（`idx_auth`）；
`access_token` varchar(1024) not null；`expire_in` int default 0；`refresh_token` varchar(1024)；
`open_id`/`user_name`/`nick_name`/`email`/`union_id` varchar(255)；
`avatar` varchar(1024)；`access_code` varchar(512)；`scope` varchar(255)；`token_type` varchar(50)；
`id_token` varchar(1024)；+ BaseModel。

#### `sys_user_online`（在线会话）

`id` bigint `primary_key` auto increment **且** `token_id` varchar(255) `primaryKey`
→ GORM 会生成复合主键 `(id, token_id)`；其余：`user_id` index、`user_name` index、
`client_key` index、`device_type`、`ip_addr`、`login_location`、`browser`、`os`、
`login_time` bigint。**无 BaseModel**。

### 4.4 已确认不存在但常被误认为存在的表

`sys_menu`（实际为 `sys_resource`）、`sys_tenant`、`sys_login_log`、`sys_oper_log`、
`sys_notice`、`sys_job`（旧 xxl-job 表）。

### 4.5 设计缺陷（迁移前须知）

| 编号 | 问题 | 影响 |
|------|------|------|
| DS-01 | `BaseModel.TenantId int64` 配 `type:string;size:32` | 生成 varchar，与 Go 类型不符 |
| DS-02 | `sys_user.email/mobile` 注释"全局唯一"，实际普通索引 | 可能重复 |
| DS-03 | `sys_user_online` 双 primaryKey | 复合主键，与自增语义冲突 |
| DS-04 | `sys_role_api`/`sys_role_resource` 无复合唯一 | 重复绑定需应用层去重 |
| DS-05 | 13 张表无迁移 | 接入成本、易漂移 |
| DS-06 | 无外键/无 `Preload` 关联 tag | 无法用 GORM 关联预加载 |
| DS-07 | `sys_resource` 的 JSON 字段混用 snake_case 与 camelCase | 前端契约不统一 |

---

## 5. MongoDB

### 5.1 接口日志集合

`models.PostLog`（只有 json/bson tag，无 gorm tag）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `time` | string | 请求时间 |
| `requestId` | string | 请求 ID |
| `responseTime` | string | 响应时间/耗时 |
| `ttl` | int | 保留时长 |
| `appName` | string | 应用名 |
| `apiName` | string | 接口名 |
| `method` | string | HTTP 方法 |
| `contentType` | string | 内容类型 |
| `uri` | string | 请求 URI |
| `clientIP` | string | 客户端 IP |
| `requestHeader` | map | 请求头 |
| `requestParam` | any | 请求参数 |
| `requestBody` | any | 请求体 |
| `responseStr` | string | 响应字符串 |
| `responseMap` | any | 响应对象 |

集合名来自 `go.log.req`；数据库来自 `go.log.db`；多库时用 `go.log.dbName`。
注册条件：`go.log.req` 非空且 `used` 含 `mongodb`（`postlog/mongo.go`）。

### 5.2 MgoDao

`MgoDao[E]` 不强制集合名：`CollectionName` 字段或 `Tag()` 提供；`Insert/Delete/Updates/Update/All/One/Pager`。
Mongo 连接需 `ReturnConnection`，否则连接不归还。

---

## 6. Redis 与缓存键

### 6.1 DAO 查询缓存

```
key = {prefix}{tag}:{TableName()}:{operation}:md5(query)
```

- `prefix` 默认 `mgin:mysql:`，`tag` 来自 `MySQLDao.Tag`（多库时为库名，否则空）。
- `query` 为按键名排序后的 `k=v&...`，空值字段剔除。
- `operation`：`all`/`one`/`exists`/`count`/`where_all`/`where_one`/`where_count`/`where_exists`。
- 失效：写操作成功后按 `{prefix}{tag}:{TableName()}:` 前缀遍历删除；事务内不清理。
- `TTL=0` 表示不过期；`TTL<0` 归 0。
- `RedisCache.Clear()` = `FLUSHDB`（危险）；`Range` 用 `SCAN *` 全库扫描。

### 6.2 进程内/磁盘缓存

| 实现 | 存储 | 键 | 过期 |
|------|------|----|------|
| `MemCache` | `sync.Map` | 名称 + md5(key) | 惰性 + 清理 ticker |
| `DiskCache` | bitcask 文件（默认 `<exeDir>/cache.db`） | 名称 + md5(key) | `PutWithTTL` |

`cache.OnGetCache(name, persistent...)` 按名称复用实例；`CloseCache()` 统一关闭。

### 6.3 框架占用的其他键

`middleware/trace` 使用 `cache.OnGetCache("Header")`，key 为 goroutine id，TTL 5 分钟，
用于在协程间传递请求头。业务不要清理该缓存。

---

## 7. 日志与文件存储

| 数据 | 位置 | 生命周期 |
|------|------|----------|
| 运行日志 | `go.logger.file` 指定目录，`<file>.YYYY-MM-DD.log` | 由外部 logrotate/采集管理，框架不清理 |
| diskcache | `<exeDir>/cache.db` | `CloseCache()`/进程退出 |
| SQLite 业务库 | `go.config.prefix.sqlite` 或 `<exeDir>/<appName>.db` | 持久 |
| i18n 独立库 | `go.i18n.sqlite` 或 `<配置目录>/<appName>_i18n.db`（`Config.WorkDir` = 配置文件所在目录） | 持久 |
| 任务日志 | `<prefix>job_log` 表 | `go.job.logRetainDays` |

---

## 8. 数据一致性建议

1. **业务写操作后清缓存**：直接使用 `MySQLDao` 的写方法（内部会清），避免绕过后写 `DB()`。
2. **事务内不读缓存**：`WithTx` 已禁用缓存读写，保持一致。
3. **多库事务**：`MySQLTx(ctx, fn, "order_db")` 必须与 DAO 的 `Tag` 一致；
   `TransactionOn` 用于已有连接（如跨库编排需自行保证）。
4. **任务表与业务表同库**：`go.job.dbName` 要与业务库一致，避免运维分库后任务数据孤立。
5. **i18n 表随应用名变化**：改 `go.application.name` 会换表名，升级前需迁移数据。
6. **casbin 策略与 `sys_api` 同步**：删除/修改接口路径时调用 `Casbin.UpdateCasbinApi`
   或 `RemoveFilteredPolicy`，否则策略残留。
