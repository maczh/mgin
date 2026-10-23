# 定时任务

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 任务注册、调度器、阻塞策略与任务管理 API。

## `github.com/maczh/mgin/v2/pkg/job`

```go
import "github.com/maczh/mgin/v2/pkg/job"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `ScheduleCron` | = "cron" | 调度类型 |
| `ScheduleFixedRate` | = "fixed_rate" | 调度类型 |
| `ScheduleFixedDelay` | = "fixed_delay" | 调度类型 |
| `ScheduleOnce` | = "once" | 调度类型 |
| `BlockSerial` | = "serial" | 阻塞处理策略：上一次调度尚未执行完毕时，本次调度的处理方式 |
| `BlockDiscard` | = "discard" | 阻塞处理策略：上一次调度尚未执行完毕时，本次调度的处理方式 |
| `BlockConcurrent` | = "concurrent" | 阻塞处理策略：上一次调度尚未执行完毕时，本次调度的处理方式 |
| `BlockCover` | = "cover" | 阻塞处理策略：上一次调度尚未执行完毕时，本次调度的处理方式 |
| `MisfireDoNothing` | = "do_nothing" | 调度过期策略：服务停机等原因导致错过了调度时刻 |
| `MisfireFireNow` | = "fire_now" | 调度过期策略：服务停机等原因导致错过了调度时刻 |
| `StatusStopped` | = 0 | 任务状态 |
| `StatusRunning` | = 1 | 任务状态 |
| `TriggerCron` | = "cron" | 触发类型 |
| `TriggerManual` | = "manual" | 触发类型 |
| `TriggerRetry` | = "retry" | 触发类型 |
| `TriggerMisfire` | = "misfire" | 触发类型 |
| `LogRunning` | = 0 | 执行结果状态 |
| `LogSuccess` | = 1 | 执行结果状态 |
| `LogFailed` | = 2 | 执行结果状态 |
| `LogTimeout` | = 3 | 执行结果状态 |
| `LogBlocked` | = 4 | 执行结果状态 |
| `LogCanceled` | = 5 | 执行结果状态 |
| `DriverMysql` | = "mysql" | 支持的存储驱动 |
| `DriverSqlite` | = "sqlite" | 支持的存储驱动 |

### 类型

#### `HandlerFunc`

```go
type HandlerFunc func(ctx *Context) error
```

HandlerFunc 定时任务执行体。
返回 error 即视为本次执行失败，会按任务配置的重试次数进行重试。
实现中应当监听 ctx.Done() 以支持超时中断与 cover 策略取消。

#### `Context`

```go
type Context struct
```

Context 定时任务执行上下文，作为参数传递给 HandlerFunc

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `JobId` | int64 |  | JobId 任务 ID |
| `JobName` | string |  | JobName 任务名称 |
| `JobGroup` | string |  | JobGroup 任务分组 |
| `HandlerName` | string |  | HandlerName 执行器名称 |
| `Param` | map[string]interface{} |  | Param 任务参数，来自任务配置的 JobParam 或手动触发时传入的参数 |
| `LogId` | int64 |  | LogId 本次执行的日志 ID |
| `TriggerType` | string |  | TriggerType 触发类型：cron / manual / retry / misfire |
| `RetryNum` | int |  | RetryNum 当前重试次数，0 表示首次执行 |

**方法**

```go
func (c *Context) Done() <-chan struct{}
```

Done 返回上下文完成通道，用于感知超时或被取消

```go
func (c *Context) Err() error
```

Err 返回上下文错误

```go
func (c *Context) Deadline() (time.Time, bool)
```

Deadline 返回上下文截止时间

```go
func (c *Context) Ctx() context.Context
```

Ctx 返回标准库 context，便于传递给下游数据库/HTTP 调用

```go
func (c *Context) Log(format string, args ...any)
```

Log 记录一行执行明细，最终会写入执行日志表的 log_detail 字段。
支持 fmt 风格的格式化参数。

#### `JobInfo`

```go
type JobInfo struct
```

JobInfo 定时任务配置表。

注意：本表的字段刻意不使用 MySQL 方言专属类型（如 tinyint / datetime），
全部交由 GORM 按当前数据库方言自动推导，以保证同一套模型可在
MySQL / PostgreSQL / SQLite 三种数据库上正确建表。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"primaryKey;autoIncrement;comment:任务ID" json:"id" form:"id" |  |
| `JobName` | string | gorm:"column:job_name;size:100;uniqueIndex;not null;comment:任务名称(全局唯一)" json:"jobName" form:"jobName" binding:"required" |  |
| `JobGroup` | string | gorm:"column:job_group;size:50;index;default:DEFAULT;comment:任务分组" json:"jobGroup" form:"jobGroup" |  |
| `Description` | string | gorm:"column:description;size:255;comment:任务描述" json:"description" form:"description" |  |
| `ScheduleType` | string | gorm:"column:schedule_type;size:20;not null;default:cron;comment:调度类型 cron\|fixed_rate\|fixed_delay\|once" json:"scheduleType" form:"scheduleType" |  |
| `ScheduleConf` | string | gorm:"column:schedule_conf;size:128;not null;comment:调度配置 cron表达式/间隔秒数/执行时间" json:"scheduleConf" form:"scheduleConf" binding:"required" |  |
| `HandlerName` | string | gorm:"column:handler_name;size:100;not null;index;comment:执行器名称，对应代码中 job.Register 注册的名字" json:"handlerName" form:"handlerName" binding:"required" |  |
| `JobParam` | map[string]interface{} | gorm:"column:job_param;type:text;serializer:json;comment:任务执行参数" json:"jobParam" form:"jobParam" |  |
| `Timeout` | int | gorm:"column:timeout;default:0;comment:执行超时时间(秒)，0为不限制" json:"timeout" form:"timeout" |  |
| `RetryCount` | int | gorm:"column:retry_count;default:0;comment:失败重试次数" json:"retryCount" form:"retryCount" |  |
| `RetryInterval` | int | gorm:"column:retry_interval;default:0;comment:失败重试间隔(秒)" json:"retryInterval" form:"retryInterval" |  |
| `BlockStrategy` | string | gorm:"column:block_strategy;size:20;default:serial;comment:阻塞策略 serial\|discard\|concurrent\|cover" json:"blockStrategy" form:"blockStrategy" |  |
| `MisfireStrategy` | string | gorm:"column:misfire_strategy;size:20;default:do_nothing;comment:调度过期策略 do_nothing\|fire_now" json:"misfireStrategy" form:"misfireStrategy" |  |
| `Status` | int | gorm:"column:status;default:0;index;comment:任务状态 1运行中 0已停止" json:"status" form:"status" |  |
| `LastFireTime` | *time.Time | gorm:"column:last_fire_time;comment:上次调度时间" json:"lastFireTime" |  |
| `NextFireTime` | *time.Time | gorm:"column:next_fire_time;comment:下次调度时间" json:"nextFireTime" |  |
| `TriggerCount` | int64 | gorm:"column:trigger_count;default:0;comment:累计调度次数" json:"triggerCount" |  |
| `SuccessCount` | int64 | gorm:"column:success_count;default:0;comment:累计成功次数" json:"successCount" |  |
| `FailCount` | int64 | gorm:"column:fail_count;default:0;comment:累计失败次数" json:"failCount" |  |
| `Remark` | string | gorm:"column:remark;size:255;comment:备注" json:"remark" form:"remark" |  |
| `CreateAt` | time.Time | gorm:"column:create_at;autoCreateTime;comment:创建时间" json:"createAt" |  |
| `UpdateAt` | time.Time | gorm:"column:update_at;autoUpdateTime;comment:更新时间" json:"updateAt" |  |

**方法**

```go
func (r JobInfo) TableName() string
```

#### `JobLog`

```go
type JobLog struct
```

JobLog 定时任务执行日志表

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"primaryKey;autoIncrement;comment:日志ID" json:"id" form:"id" |  |
| `JobId` | int64 | gorm:"column:job_id;index;not null;comment:任务ID" json:"jobId" form:"jobId" |  |
| `JobName` | string | gorm:"column:job_name;size:100;index;comment:任务名称" json:"jobName" form:"jobName" |  |
| `JobGroup` | string | gorm:"column:job_group;size:50;index;comment:任务分组" json:"jobGroup" form:"jobGroup" |  |
| `HandlerName` | string | gorm:"column:handler_name;size:100;comment:执行器名称" json:"handlerName" |  |
| `JobParam` | map[string]interface{} | gorm:"column:job_param;type:text;serializer:json;comment:本次执行参数" json:"jobParam" |  |
| `TriggerType` | string | gorm:"column:trigger_type;size:20;comment:触发类型 cron\|manual\|retry\|misfire" json:"triggerType" |  |
| `Status` | int | gorm:"column:status;index;default:0;comment:执行状态 0执行中 1成功 2失败 3超时 4阻塞丢弃 5被取消" json:"status" form:"status" |  |
| `StartAt` | time.Time | gorm:"column:start_at;index;comment:开始执行时间" json:"startAt" |  |
| `EndAt` | *time.Time | gorm:"column:end_at;comment:执行结束时间" json:"endAt" |  |
| `CostMs` | int64 | gorm:"column:cost_ms;default:0;comment:执行耗时(毫秒)" json:"costMs" |  |
| `RetryNum` | int | gorm:"column:retry_num;default:0;comment:当前为第几次重试，0表示首次执行" json:"retryNum" |  |
| `Message` | string | gorm:"column:message;type:text;comment:执行结果信息或错误原因" json:"message" |  |
| `LogDetail` | string | gorm:"column:log_detail;type:text;comment:任务内通过 ctx.Log 输出的执行明细" json:"logDetail" |  |
| `Hostname` | string | gorm:"column:hostname;size:100;comment:执行该任务的实例主机名" json:"hostname" |  |
| `CreateAt` | time.Time | gorm:"column:create_at;autoCreateTime;index;comment:创建时间" json:"createAt" |  |

**方法**

```go
func (r JobLog) TableName() string
```

#### `Config`

```go
type Config struct
```

Config 定时任务管理器配置，对应 application.yml 中 go.job 节点

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Enabled` | bool |  | Enabled 是否启用定时任务模块 |
| `DbName` | string |  | DbName 多库(multidb)模式下指定使用的库名，单库模式留空 |
| `Initdb` | bool |  | Initdb 是否自动建表（AutoMigrate），默认 true |
| `TablePrefix` | string |  | TablePrefix 表名前缀，默认 mgin_ |
| `ScanInterval` | int |  | ScanInterval 调度扫描间隔（秒），默认 1 |
| `RefreshInterval` | int |  | RefreshInterval 从数据库同步任务配置的间隔（秒），默认 30 |
| `LogRetainDays` | int |  | LogRetainDays 执行日志保留天数，0 表示不清理，默认 30 |
| `MaxConcurrent` | int |  | MaxConcurrent 全局最大并发执行任务数，默认 50 |
| `MaxSerialQueue` | int |  | MaxSerialQueue serial 阻塞策略下的最大排队数，超出则丢弃，默认 10 |
| `Timezone` | string |  | Timezone 调度时区，默认 Local，例如 Asia/Shanghai |

#### `Manager`

```go
type Manager struct
```

Manager 定时任务管理器（类 xxl-job）。

特性：
- 任务配置与执行日志持久化在当前 GORM 数据库中（MySQL → PostgreSQL → SQLite 优先级自动选择）
- 支持 cron 表达式、固定频率、固定延迟、一次性四种调度类型
- 支持超时中断、失败重试、阻塞策略、调度过期补偿
- 纯单机调度：调度器只在本进程内工作，不做多实例协调

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (m *Manager) Start() error
```

Start 启动调度器

```go
func (m *Manager) Stop()
```

Stop 停止调度器，等待运行中的任务执行完毕

```go
func (m *Manager) Check() error
```

Check 健康检查，供 mgin 定时自检调用

```go
func (m *Manager) IsRunning() bool
```

IsRunning 调度器是否运行中

```go
func (m *Manager) Store() *store
```

Store 返回底层存储，便于业务侧扩展查询

```go
func (m *Manager) Trigger(id int64, param map[string]interface{}) (int64, error)
```

Trigger 手动触发一次任务执行（等价于管理界面「执行一次」）。
param 可临时覆盖任务配置中的 JobParam，留空则使用任务配置的参数。
返回本次执行产生的日志 ID；若调度器未运行或任务不存在则返回错误。

### 函数

#### `GetHandler`

```go
func GetHandler(name string) HandlerFunc
```

GetHandler 获取已注册的执行器，不存在返回 nil

#### `GetManager`

```go
func GetManager() *Manager
```

GetManager 获取单例定时任务管理器

#### `ListHandlers`

```go
func ListHandlers() []string
```

ListHandlers 返回所有已注册的执行器名称（升序）

#### `Register`

```go
func Register(name string, h HandlerFunc)
```

Register 注册一个定时任务执行器。
name 需与数据库任务表中的 handler_name 一致，重复注册会覆盖并打印警告。
建议在 app 启动、Job 模块 Start 之前完成所有执行器注册。

job.Register("syncUserJob", func(ctx *job.Context) error {
ctx.Log("开始同步用户, 参数=%s", ctx.Param)
return userService.Sync(ctx.Ctx())
})

#### `RouterGroup`

```go
func RouterGroup(g *gin.RouterGroup)
```

RouterGroup 返回可挂载到任意 Gin 路由组下的定时任务管理接口。

提供的接口（以 group 的前缀为 /job 为例）：

GET    /job/list          任务列表（分页，支持 group/keyword/status 过滤）
GET    /job/:id           任务详情
POST   /job               新增任务
PUT    /job               更新任务
DELETE /job/:id           删除任务
POST   /job/:id/start     启动任务
POST   /job/:id/stop      停止任务
POST   /job/:id/trigger   手动触发一次
GET    /job/handlers      已注册执行器列表
GET    /job/log           执行日志列表（分页）

用法：

job.GetManager()            // 确保已初始化
r := router.Group("/job")
job.RouterGroup(r)

#### `Start`

```go
func Start() error
```

Start 启动定时任务调度器。
会依次完成：加载配置 → 选择数据库 → 自动建表 → 载入任务 → 启动调度/同步/清理协程。
应在所有 job.Register 执行器注册完成之后调用。

#### `Stop`

```go
func Stop()
```

Stop 停止调度器并等待正在执行的任务结束

#### `Unregister`

```go
func Unregister(name string)
```

Unregister 注销执行器

