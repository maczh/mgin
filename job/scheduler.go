package job

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/araddon/dateparse"
	goerr "github.com/go-errors/errors"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"github.com/robfig/cron/v3"
)

// Config 定时任务管理器配置，对应 application.yml 中 go.job 节点
type Config struct {
	// Enabled 是否启用定时任务模块
	Enabled bool
	// DbName 多库(multidb)模式下指定使用的库名，单库模式留空
	DbName string
	// Initdb 是否自动建表（AutoMigrate），默认 true
	Initdb bool
	// TablePrefix 表名前缀，默认 mgin_
	TablePrefix string
	// ScanInterval 调度扫描间隔（秒），默认 1
	ScanInterval int
	// RefreshInterval 从数据库同步任务配置的间隔（秒），默认 30
	RefreshInterval int
	// LogRetainDays 执行日志保留天数，0 表示不清理，默认 30
	LogRetainDays int
	// MaxConcurrent 全局最大并发执行任务数，默认 50
	MaxConcurrent int
	// MaxSerialQueue serial 阻塞策略下的最大排队数，超出则丢弃，默认 10
	MaxSerialQueue int
	// Timezone 调度时区，默认 Local，例如 Asia/Shanghai
	Timezone string
}

// loadConfig 从 application.yml 的 go.job 节点加载配置
func loadConfig() *Config {
	c := &Config{
		Enabled:         config.Config.GetConfigBool("go.job.enabled"),
		DbName:          config.Config.GetConfigString("go.job.dbName"),
		TablePrefix:     config.Config.GetConfigString("go.job.tablePrefix"),
		ScanInterval:    config.Config.GetConfigInt("go.job.scanInterval"),
		RefreshInterval: config.Config.GetConfigInt("go.job.refreshInterval"),
		LogRetainDays:   config.Config.GetConfigInt("go.job.logRetainDays"),
		MaxConcurrent:   config.Config.GetConfigInt("go.job.maxConcurrent"),
		MaxSerialQueue:  config.Config.GetConfigInt("go.job.maxSerialQueue"),
		Timezone:        config.Config.GetConfigString("go.job.timezone"),
	}
	// initdb 与 logRetainDays 默认值特殊处理：未显式配置时取默认值
	c.Initdb = true
	if config.Config.Exists("go.job.initdb") {
		c.Initdb = config.Config.GetConfigBool("go.job.initdb")
	}
	if !config.Config.Exists("go.job.logRetainDays") {
		c.LogRetainDays = 30
	}
	c.normalize()
	return c
}

func (c *Config) normalize() {
	if c.TablePrefix == "" {
		c.TablePrefix = "mgin_"
	}
	if c.ScanInterval <= 0 {
		c.ScanInterval = 1
	}
	if c.RefreshInterval <= 0 {
		c.RefreshInterval = 30
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 50
	}
	if c.MaxSerialQueue <= 0 {
		c.MaxSerialQueue = 10
	}
}

// jobRuntime 任务运行时状态
type jobRuntime struct {
	mu   sync.RWMutex
	info JobInfo

	schedule cron.Schedule  // cron 类型的调度计划
	interval time.Duration  // fixed_rate / fixed_delay 的间隔
	onceAt   time.Time      // once 类型的执行时刻
	next     time.Time      // 下次触发时间，零值表示不再调度

	active   int32 // 正在执行的实例数
	queued   int32 // serial 策略下的排队数
	serialMu sync.Mutex

	cancelMu sync.Mutex
	cancels  map[int64]context.CancelFunc // logId -> cancel，用于 cover 策略
}

func (jr *jobRuntime) snapshot() JobInfo {
	jr.mu.RLock()
	defer jr.mu.RUnlock()
	return jr.info
}

// computeNext 按调度类型计算 from 之后的下一次触发时间
func (jr *jobRuntime) computeNext(from time.Time) time.Time {
	jr.mu.RLock()
	defer jr.mu.RUnlock()
	switch jr.info.ScheduleType {
	case ScheduleCron:
		if jr.schedule == nil {
			return time.Time{}
		}
		return jr.schedule.Next(from)
	case ScheduleFixedRate, ScheduleFixedDelay:
		if jr.interval <= 0 {
			return time.Time{}
		}
		return from.Add(jr.interval)
	default:
		// once 类型执行后不再调度
		return time.Time{}
	}
}

// Manager 定时任务管理器（类 xxl-job）。
//
// 特性：
//   - 任务配置与执行日志持久化在当前 GORM 数据库中（MySQL → PostgreSQL → SQLite 优先级自动选择）
//   - 支持 cron 表达式、固定频率、固定延迟、一次性四种调度类型
//   - 支持超时中断、失败重试、阻塞策略、调度过期补偿
//   - 纯单机调度：调度器只在本进程内工作，不做多实例协调
type Manager struct {
	mu       sync.RWMutex
	cfg      *Config
	st       *store
	parser   cron.Parser
	loc      *time.Location
	jobs     map[int64]*jobRuntime
	sem      chan struct{}
	hostname string

	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

var (
	manager  *Manager
	initOnce sync.Once
)

// GetManager 获取单例定时任务管理器
func GetManager() *Manager {
	initOnce.Do(func() {
		manager = &Manager{
			jobs: make(map[int64]*jobRuntime),
			// 支持 5 段(分 时 日 月 周)与 6 段(秒 分 时 日 月 周)cron，以及 @every/@daily 等描述符
			parser: cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour |
				cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		}
		manager.hostname, _ = os.Hostname()
	})
	return manager
}

// Start 启动定时任务调度器。
// 会依次完成：加载配置 → 选择数据库 → 自动建表 → 载入任务 → 启动调度/同步/清理协程。
// 应在所有 job.Register 执行器注册完成之后调用。
func Start() error {
	return GetManager().Start()
}

// Stop 停止调度器并等待正在执行的任务结束
func Stop() {
	GetManager().Stop()
}

// Start 启动调度器
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return errors.New("定时任务调度器已在运行中")
	}
	cfg := loadConfig()
	if !cfg.Enabled {
		m.mu.Unlock()
		logs.Info("[Job] go.job.enabled 未开启，定时任务模块不启动")
		return nil
	}
	m.cfg = cfg
	tablePrefix = cfg.TablePrefix

	// 解析时区
	m.loc = time.Local
	if cfg.Timezone != "" {
		if loc, err := time.LoadLocation(cfg.Timezone); err == nil {
			m.loc = loc
		} else {
			logs.Error("[Job] 时区{}加载失败，使用系统默认时区: {}", cfg.Timezone, err.Error())
		}
	}

	// 按 MySQL → PostgreSQL → SQLite 优先级选择存储
	st, err := newStore(cfg.DbName)
	if err != nil {
		m.mu.Unlock()
		logs.Error("[Job] {}", err.Error())
		return err
	}
	m.st = st

	if cfg.Initdb {
		if err = st.autoMigrate(); err != nil {
			m.mu.Unlock()
			logs.Error("[Job] 定时任务表创建失败: {}", err.Error())
			return err
		}
		logs.Info("[Job] 定时任务表已就绪: {}job_info, {}job_log", cfg.TablePrefix, cfg.TablePrefix)
	}

	m.sem = make(chan struct{}, cfg.MaxConcurrent)
	m.stopChan = make(chan struct{})
	m.running = true
	m.mu.Unlock()

	// 清理上次非正常退出残留的「执行中」日志
	if n, err := st.resetRunningLogs(); err == nil && n > 0 {
		logs.Warn("[Job] 已清理{}条残留的执行中日志", n)
	}

	if err = m.reload(); err != nil {
		logs.Error("[Job] 载入任务列表失败: {}", err.Error())
	}

	m.wg.Add(1)
	go m.scheduleLoop()
	m.wg.Add(1)
	go m.refreshLoop()
	if cfg.LogRetainDays > 0 {
		m.wg.Add(1)
		go m.cleanLoop()
	}

	logs.Info("[Job] 定时任务调度器已启动, 存储={}, 任务数={}, 已注册执行器={}",
		st.Driver(), len(m.jobs), strings.Join(ListHandlers(), ","))
	return nil
}

// Stop 停止调度器，等待运行中的任务执行完毕
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopChan)
	m.mu.Unlock()

	logs.Info("[Job] 正在停止定时任务调度器，等待运行中的任务结束...")
	m.wg.Wait()
	logs.Info("[Job] 定时任务调度器已停止")
}

// Check 健康检查，供 mgin 定时自检调用
func (m *Manager) Check() error {
	m.mu.RLock()
	st, running := m.st, m.running
	m.mu.RUnlock()
	if !running {
		return nil
	}
	if st == nil {
		return errors.New("定时任务存储未初始化")
	}
	sqldb, err := st.DB().DB()
	if err != nil {
		return err
	}
	return sqldb.Ping()
}

// IsRunning 调度器是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Store 返回底层存储，便于业务侧扩展查询
func (m *Manager) Store() *store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.st
}

// Trigger 手动触发一次任务执行（等价于管理界面「执行一次」）。
// param 可临时覆盖任务配置中的 JobParam，留空则使用任务配置的参数。
// 返回本次执行产生的日志 ID；若调度器未运行或任务不存在则返回错误。
func (m *Manager) Trigger(id int64, param string) (int64, error) {
	m.mu.RLock()
	running := m.running
	jr, ok := m.jobs[id]
	st := m.st
	m.mu.RUnlock()
	if !running {
		return 0, errors.New("定时任务调度器未运行")
	}
	if !ok {
		return 0, fmt.Errorf("任务[%d]不存在或未启用", id)
	}
	info := jr.snapshot()
	if param == "" {
		param = info.JobParam
	}
	// 同步执行以便立即返回结果与日志 ID
	jl := &JobLog{
		JobId:       info.ID,
		JobName:     info.JobName,
		JobGroup:    info.JobGroup,
		HandlerName: info.HandlerName,
		JobParam:    param,
		TriggerType: TriggerManual,
		Status:      LogRunning,
		StartAt:     time.Now(),
		Hostname:    m.hostname,
	}
	if err := st.createLog(jl); err != nil {
		return 0, err
	}
	handler := GetHandler(info.HandlerName)
	if handler == nil {
		jl.Status = LogFailed
		now := time.Now()
		jl.EndAt = &now
		jl.Message = "执行器[" + info.HandlerName + "]未注册"
		_ = st.finishLog(jl)
		return jl.ID, fmt.Errorf("执行器[%s]未注册", info.HandlerName)
	}
	ok = m.runOnce(jr, handler, info, TriggerManual, param, 0)
	if !ok {
		return jl.ID, errors.New("任务执行失败，请查看执行日志")
	}
	return jl.ID, nil
}

// reload 从数据库全量载入启用中的任务，并与内存运行时状态做增量比对
func (m *Manager) reload() error {
	m.mu.RLock()
	st := m.st
	m.mu.RUnlock()
	if st == nil {
		return errors.New("定时任务存储未初始化")
	}
	list, err := st.listEnabled()
	if err != nil {
		return err
	}
	now := time.Now().In(m.loc)
	seen := make(map[int64]bool, len(list))

	for i := range list {
		info := list[i]
		seen[info.ID] = true
		m.mu.Lock()
		jr, exists := m.jobs[info.ID]
		m.mu.Unlock()

		if !exists {
			jr = &jobRuntime{info: info, cancels: make(map[int64]context.CancelFunc)}
			if err = m.prepare(jr); err != nil {
				logs.Error("[Job] 任务[{}]调度配置无效，已跳过: {}", info.JobName, err.Error())
				continue
			}
			m.initNext(jr, now)
			m.mu.Lock()
			m.jobs[info.ID] = jr
			m.mu.Unlock()
			logs.Debug("[Job] 已载入任务[{}], 下次触发: {}", info.JobName, fmtTime(jr.next))
			continue
		}
		// 已存在：仅当调度配置发生变化时重新计算下次触发时间
		old := jr.snapshot()
		jr.mu.Lock()
		jr.info = info
		jr.mu.Unlock()
		if old.ScheduleType != info.ScheduleType || old.ScheduleConf != info.ScheduleConf {
			if err = m.prepare(jr); err != nil {
				logs.Error("[Job] 任务[{}]调度配置无效: {}", info.JobName, err.Error())
				continue
			}
			jr.mu.Lock()
			jr.next = jr.computeNextLocked(now)
			next := jr.next
			jr.mu.Unlock()
			m.asyncUpdateNext(info.ID, next)
			logs.Info("[Job] 任务[{}]调度配置已变更, 下次触发: {}", info.JobName, fmtTime(next))
		}
	}
	// 移除已停止或已删除的任务
	m.mu.Lock()
	for id := range m.jobs {
		if !seen[id] {
			delete(m.jobs, id)
		}
	}
	m.mu.Unlock()
	return nil
}

// prepare 解析任务的调度配置
func (m *Manager) prepare(jr *jobRuntime) error {
	jr.mu.Lock()
	defer jr.mu.Unlock()
	conf := strings.TrimSpace(jr.info.ScheduleConf)
	switch jr.info.ScheduleType {
	case ScheduleCron:
		sch, err := m.parser.Parse(conf)
		if err != nil {
			return fmt.Errorf("cron表达式[%s]解析失败: %s", conf, err.Error())
		}
		jr.schedule = sch
	case ScheduleFixedRate, ScheduleFixedDelay:
		n, err := strconv.Atoi(conf)
		if err != nil || n <= 0 {
			return fmt.Errorf("间隔秒数[%s]无效，必须为正整数", conf)
		}
		jr.interval = time.Duration(n) * time.Second
	case ScheduleOnce:
		t, err := dateparse.ParseIn(conf, m.loc)
		if err != nil {
			return fmt.Errorf("执行时间[%s]解析失败，建议格式 2006-01-02 15:04:05", conf)
		}
		jr.onceAt = t
	default:
		return fmt.Errorf("不支持的调度类型[%s]", jr.info.ScheduleType)
	}
	return nil
}

// computeNextLocked 调用方已持有 jr.mu 写锁时使用
func (jr *jobRuntime) computeNextLocked(from time.Time) time.Time {
	switch jr.info.ScheduleType {
	case ScheduleCron:
		if jr.schedule == nil {
			return time.Time{}
		}
		return jr.schedule.Next(from)
	case ScheduleFixedRate, ScheduleFixedDelay:
		if jr.interval <= 0 {
			return time.Time{}
		}
		return from.Add(jr.interval)
	case ScheduleOnce:
		if jr.onceAt.After(from) {
			return jr.onceAt
		}
		return time.Time{}
	}
	return time.Time{}
}

// initNext 首次载入任务时确定下次触发时间，并处理调度过期（misfire）策略
func (m *Manager) initNext(jr *jobRuntime, now time.Time) {
	jr.mu.Lock()
	info := jr.info
	if info.ScheduleType == ScheduleOnce {
		if jr.onceAt.After(now) {
			jr.next = jr.onceAt
		} else if info.MisfireStrategy == MisfireFireNow && info.TriggerCount == 0 {
			// 一次性任务错过了执行时刻且从未执行过，立即补偿
			jr.next = now
		} else {
			jr.next = time.Time{}
		}
		next := jr.next
		jr.mu.Unlock()
		m.asyncUpdateNext(info.ID, next)
		return
	}
	// 周期任务：数据库中记录的下次触发时间已过期且配置了立即补偿
	if info.MisfireStrategy == MisfireFireNow && info.NextFireTime != nil &&
		info.NextFireTime.Before(now) {
		jr.next = now
		jr.mu.Unlock()
		logs.Info("[Job] 任务[{}]错过调度时刻，将立即补偿执行一次", info.JobName)
		return
	}
	jr.next = jr.computeNextLocked(now)
	next := jr.next
	jr.mu.Unlock()
	m.asyncUpdateNext(info.ID, next)
}

// scheduleLoop 调度主循环，按 scanInterval 扫描到期任务
func (m *Manager) scheduleLoop() {
	defer m.wg.Done()
	m.mu.RLock()
	interval := time.Duration(m.cfg.ScanInterval) * time.Second
	m.mu.RUnlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.tick(time.Now().In(m.loc))
		case <-m.stopChan:
			return
		}
	}
}

// tick 扫描并触发到期任务
func (m *Manager) tick(now time.Time) {
	m.mu.RLock()
	runtimes := make([]*jobRuntime, 0, len(m.jobs))
	for _, jr := range m.jobs {
		runtimes = append(runtimes, jr)
	}
	m.mu.RUnlock()

	for _, jr := range runtimes {
		jr.mu.Lock()
		if jr.info.Status != StatusRunning || jr.next.IsZero() || jr.next.After(now) {
			jr.mu.Unlock()
			continue
		}
		info := jr.info
		triggerType := TriggerCron
		if info.NextFireTime != nil && info.NextFireTime.Before(now.Add(-2*time.Second)) &&
			info.MisfireStrategy == MisfireFireNow {
			triggerType = TriggerMisfire
		}
		// fixed_delay 的下次触发时间在执行结束后才计算，此处先置空避免重复触发
		if info.ScheduleType == ScheduleFixedDelay {
			jr.next = time.Time{}
		} else {
			jr.next = jr.computeNextLocked(now)
		}
		next := jr.next
		jr.mu.Unlock()

		m.asyncUpdateFire(info.ID, now, next)
		m.fire(jr, triggerType, info.JobParam)

		// 一次性任务执行后自动停止
		if info.ScheduleType == ScheduleOnce {
			m.stopOnceJob(jr)
		}
	}
}

// stopOnceJob 一次性任务触发后置为停止状态
func (m *Manager) stopOnceJob(jr *jobRuntime) {
	info := jr.snapshot()
	jr.mu.Lock()
	jr.info.Status = StatusStopped
	jr.next = time.Time{}
	jr.mu.Unlock()
	m.mu.Lock()
	delete(m.jobs, info.ID)
	st := m.st
	m.mu.Unlock()
	if st != nil {
		if err := st.updateStatus(info.ID, StatusStopped, nil); err != nil {
			logs.Error("[Job] 一次性任务[{}]状态更新失败: {}", info.JobName, err.Error())
		}
	}
	logs.Info("[Job] 一次性任务[{}]已执行并自动停止", info.JobName)
}

// refreshLoop 周期性从数据库同步任务配置，实现不重启即生效
func (m *Manager) refreshLoop() {
	defer m.wg.Done()
	m.mu.RLock()
	interval := time.Duration(m.cfg.RefreshInterval) * time.Second
	m.mu.RUnlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := m.reload(); err != nil {
				logs.Error("[Job] 同步任务配置失败: {}", err.Error())
			}
		case <-m.stopChan:
			return
		}
	}
}

// cleanLoop 周期性清理过期执行日志
func (m *Manager) cleanLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			st, days := m.st, m.cfg.LogRetainDays
			m.mu.RUnlock()
			if st == nil {
				continue
			}
			if n, err := st.cleanLog(days); err != nil {
				logs.Error("[Job] 清理执行日志失败: {}", err.Error())
			} else if n > 0 {
				logs.Info("[Job] 已清理{}天前的执行日志{}条", days, n)
			}
		case <-m.stopChan:
			return
		}
	}
}

// fire 触发一次任务执行，按阻塞策略决定是否真正执行
func (m *Manager) fire(jr *jobRuntime, triggerType, param string) {
	info := jr.snapshot()
	handler := GetHandler(info.HandlerName)
	if handler == nil {
		logs.Error("[Job] 任务[{}]的执行器[{}]未注册，跳过本次调度", info.JobName, info.HandlerName)
		m.writeBlockedLog(info, triggerType, param, LogFailed,
			"执行器["+info.HandlerName+"]未注册")
		return
	}

	switch info.BlockStrategy {
	case BlockDiscard:
		if atomic.LoadInt32(&jr.active) > 0 {
			logs.Warn("[Job] 任务[{}]上次执行未结束，按 discard 策略丢弃本次调度", info.JobName)
			m.writeBlockedLog(info, triggerType, param, LogBlocked, "上次执行未结束，按 discard 策略丢弃")
			return
		}
	case BlockCover:
		jr.cancelAll()
	case BlockSerial:
		m.mu.RLock()
		maxQueue := int32(m.cfg.MaxSerialQueue)
		m.mu.RUnlock()
		if atomic.LoadInt32(&jr.queued) >= maxQueue {
			logs.Warn("[Job] 任务[{}]串行排队已满({})，丢弃本次调度", info.JobName, maxQueue)
			m.writeBlockedLog(info, triggerType, param, LogBlocked, "串行队列已满，丢弃本次调度")
			return
		}
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		// serial 策略：排队等待上一次执行完成
		if info.BlockStrategy == BlockSerial {
			atomic.AddInt32(&jr.queued, 1)
			jr.serialMu.Lock()
			atomic.AddInt32(&jr.queued, -1)
			defer jr.serialMu.Unlock()
		}
		// 全局并发控制
		select {
		case m.sem <- struct{}{}:
			defer func() { <-m.sem }()
		case <-time.After(30 * time.Second):
			logs.Error("[Job] 任务[{}]等待全局并发槽位超时，放弃本次执行", info.JobName)
			m.writeBlockedLog(info, triggerType, param, LogBlocked, "全局并发已满，等待槽位超时")
			return
		case <-m.stopChan:
			return
		}
		m.execute(jr, handler, triggerType, param)
	}()
}

// execute 执行任务并处理超时、重试与日志落库
func (m *Manager) execute(jr *jobRuntime, handler HandlerFunc, triggerType, param string) {
	info := jr.snapshot()
	atomic.AddInt32(&jr.active, 1)
	defer atomic.AddInt32(&jr.active, -1)

	// fixed_delay：执行结束后按间隔重新计算下次触发时间
	if info.ScheduleType == ScheduleFixedDelay {
		defer func() {
			end := time.Now().In(m.loc)
			jr.mu.Lock()
			jr.next = end.Add(jr.interval)
			next := jr.next
			jr.mu.Unlock()
			m.asyncUpdateNext(info.ID, next)
		}()
	}

	totalTry := info.RetryCount + 1
	for try := 0; try < totalTry; try++ {
		tt := triggerType
		if try > 0 {
			tt = TriggerRetry
		}
		ok := m.runOnce(jr, handler, info, tt, param, try)
		if ok {
			return
		}
		// 失败且仍有重试机会
		if try < totalTry-1 {
			wait := time.Duration(info.RetryInterval) * time.Second
			logs.Warn("[Job] 任务[{}]第{}次执行失败，{}后重试", info.JobName, try+1, wait.String())
			if wait > 0 {
				select {
				case <-time.After(wait):
				case <-m.stopChan:
					return
				}
			}
		}
	}
}

// runOnce 单次执行，返回是否成功
func (m *Manager) runOnce(jr *jobRuntime, handler HandlerFunc, info JobInfo,
	triggerType, param string, retryNum int) bool {
	m.mu.RLock()
	st := m.st
	m.mu.RUnlock()
	if st == nil {
		return false
	}

	start := time.Now()
	jl := &JobLog{
		JobId:       info.ID,
		JobName:     info.JobName,
		JobGroup:    info.JobGroup,
		HandlerName: info.HandlerName,
		JobParam:    param,
		TriggerType: triggerType,
		Status:      LogRunning,
		StartAt:     start,
		RetryNum:    retryNum,
		Hostname:    m.hostname,
	}
	if err := st.createLog(jl); err != nil {
		logs.Error("[Job] 任务[{}]执行日志写入失败: {}", info.JobName, err.Error())
	}

	// 构造带超时的执行上下文
	baseCtx := context.Background()
	var cancel context.CancelFunc
	if info.Timeout > 0 {
		baseCtx, cancel = context.WithTimeout(baseCtx, time.Duration(info.Timeout)*time.Second)
	} else {
		baseCtx, cancel = context.WithCancel(baseCtx)
	}
	defer cancel()
	jr.addCancel(jl.ID, cancel)
	defer jr.removeCancel(jl.ID)

	jobCtx := &Context{
		ctx:         baseCtx,
		JobId:       info.ID,
		JobName:     info.JobName,
		JobGroup:    info.JobGroup,
		HandlerName: info.HandlerName,
		Param:       param,
		LogId:       jl.ID,
		TriggerType: triggerType,
		RetryNum:    retryNum,
	}

	// 在独立协程中执行，以便超时后主流程能及时返回
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				wrapped := goerr.Wrap(r, 2)
				done <- fmt.Errorf("任务执行 panic: %v\n%s", r, wrapped.Stack())
			}
		}()
		done <- handler(jobCtx)
	}()

	var (
		status  int
		message string
		runErr  error
	)
	select {
	case runErr = <-done:
		if runErr != nil {
			status = LogFailed
			message = runErr.Error()
		} else {
			status = LogSuccess
			message = "执行成功"
		}
	case <-baseCtx.Done():
		if errors.Is(baseCtx.Err(), context.DeadlineExceeded) {
			status = LogTimeout
			message = fmt.Sprintf("执行超时(%d秒)，已中断等待", info.Timeout)
		} else {
			status = LogCanceled
			message = "任务被取消(cover策略或服务停止)"
		}
	}

	end := time.Now()
	jl.Status = status
	jl.EndAt = &end
	jl.CostMs = end.Sub(start).Milliseconds()
	jl.Message = truncate(message, 4000)
	jl.LogDetail = truncate(jobCtx.detail(), 60000)
	if err := st.finishLog(jl); err != nil {
		logs.Error("[Job] 任务[{}]执行日志回填失败: {}", info.JobName, err.Error())
	}

	success := status == LogSuccess
	if err := st.incrResult(info.ID, success); err != nil {
		logs.Debug("[Job] 任务[{}]统计更新失败: {}", info.JobName, err.Error())
	}
	if success {
		logs.Info("[Job] 任务[{}]执行成功, 耗时{}ms", info.JobName, jl.CostMs)
	} else {
		logs.Error("[Job] 任务[{}]执行失败({}ms): {}", info.JobName, jl.CostMs, message)
	}
	return success
}

// writeBlockedLog 记录一条未真正执行的调度日志（阻塞丢弃、执行器缺失等）
func (m *Manager) writeBlockedLog(info JobInfo, triggerType, param string, status int, message string) {
	m.mu.RLock()
	st := m.st
	m.mu.RUnlock()
	if st == nil {
		return
	}
	now := time.Now()
	jl := &JobLog{
		JobId:       info.ID,
		JobName:     info.JobName,
		JobGroup:    info.JobGroup,
		HandlerName: info.HandlerName,
		JobParam:    param,
		TriggerType: triggerType,
		Status:      status,
		StartAt:     now,
		EndAt:       &now,
		Message:     truncate(message, 4000),
		Hostname:    m.hostname,
	}
	if err := st.createLog(jl); err != nil {
		logs.Debug("[Job] 调度日志写入失败: {}", err.Error())
	}
	if status != LogSuccess {
		_ = st.incrResult(info.ID, false)
	}
}

// addCancel 登记正在执行实例的取消函数
func (jr *jobRuntime) addCancel(logId int64, cancel context.CancelFunc) {
	jr.cancelMu.Lock()
	if jr.cancels == nil {
		jr.cancels = make(map[int64]context.CancelFunc)
	}
	jr.cancels[logId] = cancel
	jr.cancelMu.Unlock()
}

func (jr *jobRuntime) removeCancel(logId int64) {
	jr.cancelMu.Lock()
	delete(jr.cancels, logId)
	jr.cancelMu.Unlock()
}

// cancelAll 取消该任务当前所有正在执行的实例（cover 策略）
func (jr *jobRuntime) cancelAll() {
	jr.cancelMu.Lock()
	for id, cancel := range jr.cancels {
		cancel()
		delete(jr.cancels, id)
	}
	jr.cancelMu.Unlock()
}

// asyncUpdateFire 异步更新调度时间与累计调度次数，避免阻塞调度主循环
func (m *Manager) asyncUpdateFire(id int64, last, next time.Time) {
	m.mu.RLock()
	st := m.st
	m.mu.RUnlock()
	if st == nil {
		return
	}
	var lastPtr, nextPtr *time.Time
	if !last.IsZero() {
		l := last
		lastPtr = &l
	}
	if !next.IsZero() {
		n := next
		nextPtr = &n
	}
	go func() {
		if err := st.updateFireTime(id, lastPtr, nextPtr); err != nil {
			logs.Debug("[Job] 调度时间更新失败: {}", err.Error())
		}
	}()
}

// asyncUpdateNext 异步仅更新下次调度时间
func (m *Manager) asyncUpdateNext(id int64, next time.Time) {
	m.mu.RLock()
	st := m.st
	m.mu.RUnlock()
	if st == nil {
		return
	}
	var nextPtr *time.Time
	if !next.IsZero() {
		n := next
		nextPtr = &n
	}
	go func() {
		if err := st.updateNextTime(id, nextPtr); err != nil {
			logs.Debug("[Job] 下次调度时间更新失败: {}", err.Error())
		}
	}()
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "无"
	}
	return t.Format("2006-01-02 15:04:05")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(已截断)"
}
