package job

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/maczh/mgin/v2/pkg/logs"
)

// HandlerFunc 定时任务执行体。
// 返回 error 即视为本次执行失败，会按任务配置的重试次数进行重试。
// 实现中应当监听 ctx.Done() 以支持超时中断与 cover 策略取消。
type HandlerFunc func(ctx *Context) error

// Context 定时任务执行上下文，作为参数传递给 HandlerFunc
type Context struct {
	ctx context.Context

	// JobId 任务 ID
	JobId int64
	// JobName 任务名称
	JobName string
	// JobGroup 任务分组
	JobGroup string
	// HandlerName 执行器名称
	HandlerName string
	// Param 任务参数，来自任务配置的 JobParam 或手动触发时传入的参数
	Param map[string]interface{}
	// LogId 本次执行的日志 ID
	LogId int64
	// TriggerType 触发类型：cron / manual / retry / misfire
	TriggerType string
	// RetryNum 当前重试次数，0 表示首次执行
	RetryNum int

	mu  sync.Mutex
	buf strings.Builder
}

// Done 返回上下文完成通道，用于感知超时或被取消
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err 返回上下文错误
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Deadline 返回上下文截止时间
func (c *Context) Deadline() (time.Time, bool) {
	return c.ctx.Deadline()
}

// Ctx 返回标准库 context，便于传递给下游数据库/HTTP 调用
func (c *Context) Ctx() context.Context {
	return c.ctx
}

// Log 记录一行执行明细，最终会写入执行日志表的 log_detail 字段。
// 支持 fmt 风格的格式化参数。
func (c *Context) Log(format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	c.mu.Lock()
	c.buf.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
	c.buf.WriteString(" ")
	c.buf.WriteString(msg)
	c.buf.WriteString("\n")
	c.mu.Unlock()
	logs.Debug("[Job:{}] {}", c.JobName, msg)
}

// detail 取出已记录的执行明细
func (c *Context) detail() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// ParamMap 将形如 a=1&b=2 的任务参数解析为 map，便于快速取值
// func (c *Context) ParamMap() map[string]string {
// 	m := make(map[string]string)
// 	for _, kv := range strings.Split(c.Param, "&") {
// 		if kv == "" {
// 			continue
// 		}
// 		parts := strings.SplitN(kv, "=", 2)
// 		if len(parts) == 2 {
// 			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
// 		} else {
// 			m[strings.TrimSpace(parts[0])] = ""
// 		}
// 	}
// 	return m
// }

// handlerRegistry 执行器注册表，全局唯一
type handlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

var registry = &handlerRegistry{
	handlers: make(map[string]HandlerFunc),
}

// Register 注册一个定时任务执行器。
// name 需与数据库任务表中的 handler_name 一致，重复注册会覆盖并打印警告。
// 建议在 app 启动、Job 模块 Start 之前完成所有执行器注册。
//
//	job.Register("syncUserJob", func(ctx *job.Context) error {
//	    ctx.Log("开始同步用户, 参数=%s", ctx.Param)
//	    return userService.Sync(ctx.Ctx())
//	})
func Register(name string, h HandlerFunc) {
	if name == "" || h == nil {
		logs.Error("[Job] 注册执行器失败: 名称或执行体为空")
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.handlers[name]; exists {
		logs.Warn("[Job] 执行器{}重复注册，已覆盖", name)
	}
	registry.handlers[name] = h
	logs.Debug("[Job] 已注册执行器: {}", name)
}

// Unregister 注销执行器
func Unregister(name string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.handlers, name)
}

// GetHandler 获取已注册的执行器，不存在返回 nil
func GetHandler(name string) HandlerFunc {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.handlers[name]
}

// ListHandlers 返回所有已注册的执行器名称（升序）
func ListHandlers() []string {
	registry.mu.RLock()
	names := make([]string, 0, len(registry.handlers))
	for name := range registry.handlers {
		names = append(names, name)
	}
	registry.mu.RUnlock()
	sort.Strings(names)
	return names
}
