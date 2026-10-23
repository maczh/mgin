# 变量、常量与配置键

本文覆盖三类「变量」：包级全局对象、编译期常量、运行时配置键。所有默认值来自代码；标注「已验证」的条目经过实际启动验证。

---

## 1. 包级全局变量

| 名称 | 类型 | 初始值 | 作用 | 约束 |
|------|------|--------|------|------|
| `config.Config` | `*config`（未导出类型） | `&config{}` | 全局配置单例 | 配置加载失败时 `Cnf == nil`，读取返回零值；重载期间会被重置字段 |
| `db.Mysql` | `*mysql.MysqlClient` | `&mysql.MysqlClient{}` | MySQL 门面 | 仅 `used` 含 `mysql` 时可用；多库需传库名 |
| `db.Mongo` | `*mongo.Mongodb` | `&mongo.Mongodb{}` | MongoDB 门面 | `GetConnection` 必须与 `ReturnConnection` 成对 |
| `db.Redis` | `redis.Redis` | 零值 | Redis 门面 | `Check()` 恒返回 nil |
| `db.Sqlite` | `*sqlite.Sqlite` | `&sqlite.Sqlite{}` | SQLite 门面 | 未初始化时 `GetConnection` 返回 `SQLite not opened` |
| `registry.Registry` | `registry.RegistryClient` | `noopRegistryClient` | 服务发现客户端 | 未配置 `go.discovery.registry` 时为空实现，调用直接报错 |
| `casbin.Casbin` | `*casbinService` | `&casbinService{}` | Casbin Enforcer 持有者 | 依赖 MySQL；模型文件必须存在 |
| `metrics.` 默认注册器 | Prometheus 内部 | 包初始化时注册 | 指标单例 | 重复注册会 panic（用框架 API 设置） |

> 这些变量是**进程级单例**。测试中修改后需要恢复；生产不要重新赋值（会绕过生命周期管理）。

### 读取示例

```go
// config
name := config.Config.App.Name
used := config.Config.Config.Used
raw := config.Config.GetConfigString("go.custom.key")

// 数据源
conn, err := db.Mysql.GetConnection()        // 单库
if err != nil { logs.Error("mysql: {}", err) }

// 服务发现
host, scheme := registry.Registry.GetServiceURL("user-service")
logs.Info("target={}://{}", scheme, host)
```

---

## 2. 常量

### 2.1 业务错误码（`errcode`）

| 常量 | 值 | 含义 |
|------|----|------|
| `URI_NOT_FOUND` | 1000 | 路由不存在（404 处理器使用） |
| `SYSTEM_ERROR` | 1001 | 未捕获异常 |
| `DB_CONNECT_ERROR` | 1002 | 数据库连接/查询失败 |
| `REQUEST_PARAMETER_LOST` | 1003 | 参数缺失（`ParamLostError`） |
| `DATA_NOT_FOUND` | 1004 | 数据不存在 |
| `USER_NOT_FOUND` | 1005 | 用户不存在 |
| `PASSWORD_ERROR` | 1006 | 密码错误 |
| `VERIFY_CODE_ERROR` | 1007 | 验证码错误 |
| `TOKEN_ERROR` | 1008 | Token 无效 |
| `AUTHENTICATION_FAILURE` | 1009 | 认证失败 |
| `SERVICE_UNAVAILABLE` | 1010 | 服务不可用 |
| `TOO_MANY_REQUESTS` | 1011 | 限流 |
| `JOB_ERROR` | 1012 | 任务执行失败 |
| `STORAGE_ERROR` | 1013 | 存储错误（无对象存储实现） |
| `PARAM_ERROR` | 1014 | 参数格式错误（`ParamError`） |

配套 messageId 常量：`Success`、`UrlNotFound`、`SystemError`、`DbConnectError`、`ParamLost`、`DataNotFound`、`UserNotFound`、`PasswordError`、`VerifyCodeError`、`TokenError`、`AuthenticationFailure`、`ServiceUnavailable`、`TooManyRequests`、`JobError`、`StorageError`、`ParamError`。

### 2.2 调用响应码（`models/vo`）

| 常量 | 值 | 含义 |
|------|----|------|
| `vo.SUCCESS` | 1 | 成功 |
| `vo.ERROR` | 500 | 失败 |
| `vo.UnLogin` | 999 | 未登录 |

### 2.3 调用协议（`client`）

| 常量 | 值 |
|------|----|
| `client.CONTENT_TYPE_FORM` | `x-form` |
| `client.JSON` | `json` |
| `client.RESTFUL` | `restful` |
| `client.FILE` | `file` |

### 2.4 任务枚举（`job`）

| 类别 | 取值 |
|------|------|
| `ScheduleType` | `cron`、`fixed_rate`、`fixed_delay`、`once` |
| `BlockStrategy` | `serial`、`discard`、`concurrent`、`cover` |
| `MisfireStrategy` | `do_nothing`、`fire_now` |
| 状态 | `0` 已停止、`1` 运行中 |
| 触发类型 | 定时、手动（`TriggerType` 字段） |

### 2.5 限流枚举（`middleware/ratelimit`）

| 类别 | 取值 |
|------|------|
| `Algorithm` | `token_bucket`、`sliding_window`、`concurrency` |
| `Dimension` | `global`、`ip`、`path`、`ip_path`、`header` |

### 2.6 日志等级与目标

| 类别 | 取值 |
|------|------|
| Level | `debug`、`info`、`warn`、`error` |
| `outputs[].target` | `console`、`file`、`victorialogs`、已注册的自定义 handler 名 |
| `outputs[].format` | `text`、`json` |
| 可选字段 | `time,level,message,app,request_id,goroutine_id,package,file,function,line` |

---

## 3. 配置键

### 3.1 应用（`go.application`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `name` | string | 空 | 应用名 | 注册实例名、SQLite 默认文件、i18n 表前缀；i18n 要求 `^[A-Za-z0-9_.-]{1,59}$` |
| `project` | string | `DEFAULT` | 配置中心项目/分组 | 影响配置中心路径 |
| `port` | int | 0 | HTTP 端口 | `>0` 才监听 |
| `port_ssl` | int | 0 | HTTPS 端口 | 需同时配 `cert`/`key` |
| `cert` / `key` | string | 空 | 证书与私钥路径 | 非空才启用 TLS |
| `debug` | bool | false | gin debug + 强制 debug 日志 | 生产关闭 |
| `ip` | string | 空（自动探测） | 注册到注册中心的地址 | 多网卡/NAT 必填 |

### 3.2 配置源（`go.config`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `used` | string | 空 | 启用组件，逗号分隔 | **子串匹配**；未知组件被忽略 |
| `env` | string | 空 | 子配置后缀 | 文件名为 `<prefix>-<env>.yml` |
| `prefix.mysql` 等 | string | 空 | 子配置前缀 | `sqlite` 例外，值为文件名；`prefix.postgres` **未被读取** |
| `server` | string | 空 | 配置中心地址 | `file` 模式可省略 |
| `server_type` | string | 空 | `file`/`nacos`/`consul`/`etcd`/`polaris`/`springconfig` | 未知值走默认 HTTP 拼接 |
| `path` | string | 空 | 本地子配置目录 | 不改变 `WorkDir` |
| `token` | string | 空 | Polaris token | 仅 polaris |
| `reload.token` | string | 空 | 重载令牌 | 非空才注册 `PUT /cfg/reload`；请求头 `X-MGin-Reload-Token`/Bearer |

### 3.3 日志（`go.logger`、`go.log`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `logger.level` | string | `error` | 日志级别 | `debug=true` 时强制 debug |
| `logger.out` | string | 空 | `console`/`file`（旧式） | 与 `outputs` 可并存 |
| `logger.file` | string | 空 | 文件前缀 | 实际文件 `<file>.YYYY-MM-DD.log` |
| `logger.outputs` | array | 空 | 多目标配置 | 每项 `target/format/fields/logtext/file` |
| `log.req` | string | 空 | 接口日志集合名 | 需 `used` 含 `mongodb` 才落库 |
| `log.dbName` | string | 空 | 接口日志库名 | 多库必填，否则丢弃 |
| `log.db` | string | 空 | 日志库（旧） | 仅解析 |
| `log.get` | string | 空（按 full） | GET 响应记录策略 | `full`/`line`/`off` |
| `log.call` / `log.api` | string | 空 | 调用/接口日志表名 | **仅解析，无写入实现** |
| `log.sink.queue` | int | 100 | 每 handler 队列长度 | 上限 100；代码默认 1024 |
| `log.sink.workers` | int | 1 | 消费协程 | 强制 1 |
| `log.sink.shutdown` | int | 3000 | 退出排空超时（ms） | — |
| `log.victorialogs.url` | string | 空 | VL 地址 | 配置错误会在启动时 panic |
| `log.victorialogs.api_token` | string | 空 | VL token | — |
| `log.victorialogs.timeout` | int | 0 | 超时（ms） | 0 使用默认 |
| `log.victorialogs.request` | bool | false | 上报接口日志 | — |
| `log.victorialogs.client` | bool | false | 上报调用日志 | — |
| `log.kafka.use` / `topic` | bool/string | false/空 | Kafka 投递 | **无实现** |

### 3.4 发现与调用（`go.discovery`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `registry` | string | 空 | `nacos`/`consul`/`etcd`/`polaris` | 空=不注册不发现 |
| `callType` | string | `x-form` | 调用默认协议 | 可被 `Options.Protocol` 覆盖 |
| `cacheTTL` | int（秒） | 60 | 发现缓存 TTL | 影响下线感知 |
| `cacheRefresh` | int（秒） | 60 | 热刷新间隔 | `<=0` 用内部默认 |

子配置键：`go.nacos.{server,port,group,apiversion,clusterName,lan,lanNet}`、`go.consul.{server,port,group,clusterName,lan,lanNet}`、`go.etcd.{server,port,prefix,group,clusterName,lan,lanNet}`、`go.polaris.{server,port,token,namespace}`。

### 3.5 数据源（`go.data`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `mysql` | string | 空 | 单库 DSN | — |
| `mysql.multidb` / `.dbNames` | bool/string | false/空 | 多库开关与库名 | 多库必须 `GetConnection(name)` |
| `mysql_debug` | bool | false | 打印 SQL | 生产慎用 |
| `mysql_pool.max` | int | 0（不开池） | 最大连接 | `<=1` 不启用；`<10` 抬到 10 |
| `mysql_pool.total` | int | `5*max` | 最大空闲 | — |
| `mysql_pool.timeout` | int（秒） | 60 | 空闲超时 | — |
| `mysql_pool.life` | int（分） | 60 | 连接寿命 | — |
| `redis.uri/host/port/master/password/database` | — | — | 单库连接 | `database=-1` 强制 DB0 |
| `redis_pool.min/max/idleTimeout/timeout` | int | 0 | 池参数 | `idleTimeout` 分钟、`timeout` 秒 |
| `mongodb.uri` / `.db` | string | 空 | 单库连接 | — |
| `mongodb.multidb/dbNames` | bool/string | — | 多库 | — |
| `mongo_pool.max` | int | 10 | 连接池上限 | 最小 10 |
| `postgres.*` | — | — | **未读取** | 无实现 |
| `clickhouse.*` | — | — | **未读取** | 无实现 |
| `elasticsearch.*` | — | — | **未读取** | 无实现 |

### 3.6 安全（`go.jwt`、`go.casbin`、`go.cors`、`go.sys`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `jwt.secret` | string | 空 | HMAC 密钥 | 空=全部 401 |
| `casbin.enabled` | bool | false | 启用鉴权 | 需 MySQL |
| `casbin.model_file` | string | `rbac_model.conf` | 模型路径 | 相对 `WorkDir`；仓库只有 `casbin.conf` |
| `cors.headers` | string | 空 | 追加允许头 | 方法固定，不可配 |
| `sys.name` / `sys.version` / `sys.copyright` | string | 空 | 解析 | 无 HTTP 实现 |
| `sys.initdb` | bool | false | 解析 | 不建表 |

### 3.7 可观测（`go.health`、`go.metrics`）

| 键 | 类型 | 默认 | 作用 | 约束 |
|----|------|------|------|------|
| `health.enabled` | bool | false | 挂载探针 | 已验证可单独生效 |
| `health.autoStarted` | bool | false | 监听后自动 MarkStarted | — |
| `metrics.enabled` | bool | false | 挂载 `/metrics` | 已验证可单独生效；HTTP 指标需 `metrics.Middleware()` |

### 3.8 限流（`go.ratelimit`）

| 键 | 类型 | 默认 | 作用 |
|----|------|------|------|
| `enabled` | bool | false | 启用 |
| `idleTimeout` | int（秒） | 600 | 限流器空闲回收 |
| `whitelist` | []string | 空 | 路径前缀白名单 |
| `whiteIps` | []string | 空 | IP 白名单 |
| `rules[]` | array | 空 | 规则，字段见 [types.md](types.md#ratelimitrule) |

### 3.9 任务（`go.job`）

| 键 | 类型 | 默认 | 作用 |
|----|------|------|------|
| `enabled` | bool | false | 启用调度 |
| `initdb` | bool | true | 自动建表 |
| `tablePrefix` | string | `mgin_` | 表前缀 |
| `scanInterval` | int（秒） | 1 | 新任务扫描 |
| `refreshInterval` | int（秒） | 30 | 调度刷新 |
| `logRetainDays` | int（天） | 30 | 日志保留，0 不清理 |
| `maxConcurrent` | int | 50 | 全局并发 |
| `maxSerialQueue` | int | 10 | serial 队列上限 |
| `timezone` | string | `Local` | 时区 |

### 3.10 国际化（`go.i18n`）

| 键 | 类型 | 默认 | 作用 |
|----|------|------|------|
| `enabled` | bool | false | 启用 |
| `default` | string | `zh-cn` | 默认语言 |
| `dbName` | string | 空 | 多库时字典库名 |
| `sqlite` | string | `{app}_i18n.db` | SQLite 字典文件 |
| `api.enabled` | bool | false | 暴露 `/i18n` CRUD |

---

## 4. 路径类「变量」

| 名称 | 来源 | 说明 |
|------|------|------|
| `config.Config.WorkDir` | 配置文件所在目录 | 子配置、casbin 模型、SQLite、磁盘缓存的基准 |
| 可执行文件目录 | `os.Executable()` | 不含分隔符的配置文件名与相对路径的解析基准 |
| `config.Config.Cnf` | koanf 实例 | 可直接 `String/Int/Bool` 读取任意键 |

---

## 5. 可直接运行的示例

```go
package main

import (
	"fmt"

	"github.com/maczh/mgin"
	"github.com/maczh/mgin/config"
)

func main() {
	_ = mgin.NewApp("conf/application.yml", "demo", "1.0.0", false)

	// 全局变量
	fmt.Println("app:", config.Config.App.Name)
	fmt.Println("port:", config.Config.App.Port)
	fmt.Println("used:", config.Config.Config.Used)
	fmt.Println("workdir:", config.Config.WorkDir)

	// 任意键（含未在结构体中声明的键）
	fmt.Println("registry:", config.Config.GetConfigString("go.discovery.registry"))
	fmt.Println("exists:", config.Config.Exists("go.jwt.secret"))
}
```

配套 `conf/application.yml`：

```yaml
go:
  application: { name: demo, port: 8080, project: demo, debug: true }
  config: { used: "", env: test }
```

> 断言配置是否真的被解析：不要只看 YAML，跑上面的示例打印 `config.Config` 字段；`used` 拼错时组件不会初始化，而且不会报错。
