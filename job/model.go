package job

import (
	"time"
)

// 调度类型
const (
	// ScheduleCron cron 表达式调度，ScheduleConf 为 cron 表达式。
	// 支持 5 段（分 时 日 月 周）、6 段（秒 分 时 日 月 周）以及 @every 5m / @daily 等描述符
	ScheduleCron = "cron"
	// ScheduleFixedRate 固定频率调度，ScheduleConf 为间隔秒数，从上次「开始执行」时刻起算
	ScheduleFixedRate = "fixed_rate"
	// ScheduleFixedDelay 固定延迟调度，ScheduleConf 为间隔秒数，从上次「执行结束」时刻起算
	ScheduleFixedDelay = "fixed_delay"
	// ScheduleOnce 一次性调度，ScheduleConf 为执行时间（2006-01-02 15:04:05），执行后自动停止
	ScheduleOnce = "once"
)

// 阻塞处理策略：上一次调度尚未执行完毕时，本次调度的处理方式
const (
	// BlockSerial 串行排队：等待上一次执行完成后依次执行
	BlockSerial = "serial"
	// BlockDiscard 丢弃后续：上次仍在执行则直接丢弃本次调度
	BlockDiscard = "discard"
	// BlockConcurrent 并发执行：不做任何限制，允许多实例同时执行
	BlockConcurrent = "concurrent"
	// BlockCover 覆盖之前：取消正在执行的任务（触发其 context 取消），立即执行本次
	BlockCover = "cover"
)

// 调度过期策略：服务停机等原因导致错过了调度时刻
const (
	// MisfireDoNothing 忽略错过的调度，等待下一个调度时刻
	MisfireDoNothing = "do_nothing"
	// MisfireFireNow 立即补偿执行一次
	MisfireFireNow = "fire_now"
)

// 任务状态
const (
	// StatusStopped 已停止
	StatusStopped = 0
	// StatusRunning 运行中
	StatusRunning = 1
)

// 触发类型
const (
	// TriggerCron 调度器自动触发
	TriggerCron = "cron"
	// TriggerManual 人工手动触发
	TriggerManual = "manual"
	// TriggerRetry 失败重试触发
	TriggerRetry = "retry"
	// TriggerMisfire 调度过期补偿触发
	TriggerMisfire = "misfire"
)

// 执行结果状态
const (
	// LogRunning 执行中
	LogRunning = 0
	// LogSuccess 执行成功
	LogSuccess = 1
	// LogFailed 执行失败
	LogFailed = 2
	// LogTimeout 执行超时
	LogTimeout = 3
	// LogBlocked 因阻塞策略被丢弃
	LogBlocked = 4
	// LogCanceled 被覆盖策略取消
	LogCanceled = 5
)

// tablePrefix 表名前缀，可通过 go.job.tablePrefix 配置
var tablePrefix = "mgin_"

// JobInfo 定时任务配置表。
//
// 注意：本表的字段刻意不使用 MySQL 方言专属类型（如 tinyint / datetime），
// 全部交由 GORM 按当前数据库方言自动推导，以保证同一套模型可在
// MySQL / PostgreSQL / SQLite 三种数据库上正确建表。
type JobInfo struct {
	ID          int64  `gorm:"primaryKey;autoIncrement;comment:任务ID" json:"id" form:"id"`
	JobName     string `gorm:"size:100;uniqueIndex;not null;comment:任务名称(全局唯一)" json:"jobName" form:"jobName" binding:"required"`
	JobGroup    string `gorm:"size:50;index;default:DEFAULT;comment:任务分组" json:"jobGroup" form:"jobGroup"`
	Description string `gorm:"size:255;comment:任务描述" json:"description" form:"description"`

	ScheduleType string `gorm:"size:20;not null;default:cron;comment:调度类型 cron|fixed_rate|fixed_delay|once" json:"scheduleType" form:"scheduleType"`
	ScheduleConf string `gorm:"size:128;not null;comment:调度配置 cron表达式/间隔秒数/执行时间" json:"scheduleConf" form:"scheduleConf" binding:"required"`

	HandlerName string `gorm:"size:100;not null;index;comment:执行器名称，对应代码中 job.Register 注册的名字" json:"handlerName" form:"handlerName" binding:"required"`
	JobParam    string `gorm:"type:text;comment:任务执行参数" json:"jobParam" form:"jobParam"`

	Timeout         int    `gorm:"default:0;comment:执行超时时间(秒)，0为不限制" json:"timeout" form:"timeout"`
	RetryCount      int    `gorm:"default:0;comment:失败重试次数" json:"retryCount" form:"retryCount"`
	RetryInterval   int    `gorm:"default:0;comment:失败重试间隔(秒)" json:"retryInterval" form:"retryInterval"`
	BlockStrategy   string `gorm:"size:20;default:serial;comment:阻塞策略 serial|discard|concurrent|cover" json:"blockStrategy" form:"blockStrategy"`
	MisfireStrategy string `gorm:"size:20;default:do_nothing;comment:调度过期策略 do_nothing|fire_now" json:"misfireStrategy" form:"misfireStrategy"`

	Status int `gorm:"default:0;index;comment:任务状态 1运行中 0已停止" json:"status" form:"status"`

	LastFireTime *time.Time `gorm:"comment:上次调度时间" json:"lastFireTime"`
	NextFireTime *time.Time `gorm:"comment:下次调度时间" json:"nextFireTime"`
	TriggerCount int64      `gorm:"default:0;comment:累计调度次数" json:"triggerCount"`
	SuccessCount int64      `gorm:"default:0;comment:累计成功次数" json:"successCount"`
	FailCount    int64      `gorm:"default:0;comment:累计失败次数" json:"failCount"`

	Remark   string    `gorm:"size:255;comment:备注" json:"remark" form:"remark"`
	CreateAt time.Time `gorm:"autoCreateTime;comment:创建时间" json:"createAt"`
	UpdateAt time.Time `gorm:"autoUpdateTime;comment:更新时间" json:"updateAt"`
}

func (JobInfo) TableName() string {
	return tablePrefix + "job_info"
}

// JobLog 定时任务执行日志表
type JobLog struct {
	ID       int64  `gorm:"primaryKey;autoIncrement;comment:日志ID" json:"id" form:"id"`
	JobId    int64  `gorm:"index;not null;comment:任务ID" json:"jobId" form:"jobId"`
	JobName  string `gorm:"size:100;index;comment:任务名称" json:"jobName" form:"jobName"`
	JobGroup string `gorm:"size:50;index;comment:任务分组" json:"jobGroup" form:"jobGroup"`

	HandlerName string `gorm:"size:100;comment:执行器名称" json:"handlerName"`
	JobParam    string `gorm:"type:text;comment:本次执行参数" json:"jobParam"`
	TriggerType string `gorm:"size:20;comment:触发类型 cron|manual|retry|misfire" json:"triggerType"`

	Status   int        `gorm:"index;default:0;comment:执行状态 0执行中 1成功 2失败 3超时 4阻塞丢弃 5被取消" json:"status" form:"status"`
	StartAt  time.Time  `gorm:"index;comment:开始执行时间" json:"startAt"`
	EndAt    *time.Time `gorm:"comment:执行结束时间" json:"endAt"`
	CostMs   int64      `gorm:"default:0;comment:执行耗时(毫秒)" json:"costMs"`
	RetryNum int        `gorm:"default:0;comment:当前为第几次重试，0表示首次执行" json:"retryNum"`

	Message  string `gorm:"type:text;comment:执行结果信息或错误原因" json:"message"`
	LogDetail string `gorm:"type:text;comment:任务内通过 ctx.Log 输出的执行明细" json:"logDetail"`
	Hostname string `gorm:"size:100;comment:执行该任务的实例主机名" json:"hostname"`

	CreateAt time.Time `gorm:"autoCreateTime;index;comment:创建时间" json:"createAt"`
}

func (JobLog) TableName() string {
	return tablePrefix + "job_log"
}
