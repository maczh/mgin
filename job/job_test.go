package job

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newMemStore 构造一个内存 SQLite 存储用于测试（纯 Go 驱动，无需 cgo）
func newMemStore(t *testing.T) *store {
	tablePrefix = "mgin_"
	db, err := gorm.Open(sqlite.Open("file:jobtest?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	st := &store{db: db, driver: DriverSqlite}
	if err = st.autoMigrate(); err != nil {
		t.Fatalf("autoMigrate: %v", err)
	}
	return st
}

func TestStoreCRUD(t *testing.T) {
	st := newMemStore(t)

	job := &JobInfo{
		JobName:      "testJob",
		JobGroup:     "default",
		ScheduleType: ScheduleCron,
		ScheduleConf: "0 */1 * * * ?",
		HandlerName:  "testHandler",
		Status:       StatusRunning,
		JobParam:     map[string]interface{}{"amount": 17820, "orderId": "DO202609011451071788245467286109", "enabled": true},
	}
	if err := st.create(job); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Logf("入库成功")
	if job.ID <= 0 {
		t.Fatalf("期望自增 ID > 0, got %d", job.ID)
	}

	got, err := st.getById(job.ID)
	if err != nil {
		t.Fatalf("getById: %v", err)
	}
	jobJson, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	t.Logf("查询结果: %v", string(jobJson))
	if got.JobName != "testJob" {
		t.Fatalf("JobName mismatch: %s", got.JobName)
	}

	got.Timeout = 30
	got.RetryCount = 2
	if err = st.update(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := st.getById(job.ID)
	if got2.Timeout != 30 || got2.RetryCount != 2 {
		t.Fatalf("update 未生效: %+v", got2)
	}

	// 列表分页
	list, total, err := st.page("", "", -1, 1, 10)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("page 期望 total=1, got %d list=%d", total, len(list))
	}

	// 启用中列表
	enabled, err := st.listEnabled()
	if err != nil {
		t.Fatalf("listEnabled: %v", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("listEnabled 期望 1 条, got %d", len(enabled))
	}

	if err = st.remove(job.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err = st.getById(job.ID); err == nil {
		t.Fatalf("remove 后期望查不到")
	}
}

func TestStoreUpdatePreservesRuntimeFields(t *testing.T) {
	st := newMemStore(t)
	j := &JobInfo{JobName: "editable", JobGroup: "g", ScheduleType: ScheduleCron, ScheduleConf: "@every 1h", HandlerName: "unitHandler", Status: StatusRunning, TriggerCount: 7, SuccessCount: 4, FailCount: 2}
	if err := st.create(j); err != nil {
		t.Fatal(err)
	}
	if err := st.db.Model(&JobInfo{}).Where("id = ?", j.ID).Updates(map[string]any{"status": StatusStopped, "trigger_count": 8, "success_count": 5}).Error; err != nil {
		t.Fatal(err)
	}
	j.Description = "edited"
	if err := st.update(j); err != nil {
		t.Fatal(err)
	}
	got, err := st.getById(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusStopped || got.TriggerCount != 8 || got.SuccessCount != 5 || got.FailCount != 2 || got.Description != "edited" {
		t.Fatalf("runtime fields overwritten: %+v", got)
	}
}

func TestManualTriggerUsesOneExecutionLog(t *testing.T) {
	st := newMemStore(t)
	Register("manual-single-log", func(*Context) error { return nil })
	defer Unregister("manual-single-log")
	j := &JobInfo{JobName: "manual", JobGroup: "g", ScheduleType: ScheduleCron, ScheduleConf: "@every 1h", HandlerName: "manual-single-log", Status: StatusRunning, BlockStrategy: BlockConcurrent}
	if err := st.create(j); err != nil {
		t.Fatal(err)
	}
	m := &Manager{cfg: &Config{MaxConcurrent: 1, MaxSerialQueue: 1}, st: st, jobs: map[int64]*jobRuntime{j.ID: {info: *j}}, sem: make(chan struct{}, 1), running: true, hostname: "test"}
	gotID, err := m.Trigger(j.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	logs, _, err := st.pageLog(j.ID, "", -1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ID != gotID || logs[0].Status != LogSuccess {
		t.Fatalf("manual log mismatch: id=%d logs=%+v", gotID, logs)
	}
}

func TestFixedDelayMissingHandlerReschedules(t *testing.T) {
	st := newMemStore(t)
	m := &Manager{cfg: &Config{MaxConcurrent: 1, MaxSerialQueue: 1}, st: st, jobs: map[int64]*jobRuntime{}, sem: make(chan struct{}, 1), running: true, stopChan: make(chan struct{}), loc: time.Local}
	j := JobInfo{ID: 77, JobName: "missing", HandlerName: "not-registered", ScheduleType: ScheduleFixedDelay}
	jr := &jobRuntime{info: j, interval: time.Second, next: time.Now().Add(-time.Second)}
	m.fire(jr, TriggerCron, nil)
	jr.mu.RLock()
	next := jr.next
	jr.mu.RUnlock()
	if next.IsZero() || !next.After(time.Now()) {
		t.Fatalf("fixed_delay was not rescheduled: %v", next)
	}
}

func TestFixedDelayAdmissionTimeoutReschedules(t *testing.T) {
	st := newMemStore(t)
	Register("fixed-delay-timeout", func(*Context) error { return nil })
	defer Unregister("fixed-delay-timeout")
	j := JobInfo{ID: 91, JobName: "fixed-timeout", HandlerName: "fixed-delay-timeout", ScheduleType: ScheduleFixedDelay, BlockStrategy: BlockConcurrent, Status: StatusRunning}
	jr := &jobRuntime{info: j, interval: time.Second, next: time.Now().Add(-time.Second)}
	m := &Manager{cfg: &Config{MaxConcurrent: 1, MaxSerialQueue: 1}, st: st, jobs: map[int64]*jobRuntime{j.ID: jr}, sem: make(chan struct{}, 1), running: true, stopChan: make(chan struct{}), stopDone: make(chan struct{}), loc: time.Local, admissionTimeout: 20 * time.Millisecond, hostname: "test"}
	m.sem <- struct{}{}
	m.tick(time.Now().Add(time.Second))
	// tick clears the fixed-delay next time before fire submits the attempt.
	time.Sleep(60 * time.Millisecond)
	jr.mu.RLock()
	next := jr.next
	jr.mu.RUnlock()
	if next.IsZero() || !next.After(time.Now()) {
		t.Fatalf("fixed_delay was not scheduled after admission timeout: %v", next)
	}
	logs, _, err := st.pageLog(j.ID, "", -1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Status != LogBlocked {
		t.Fatalf("expected blocked admission log, got %+v", logs)
	}
}

func TestStopCancelsCooperativeHandler(t *testing.T) {
	st := newMemStore(t)
	started := make(chan struct{}, 2)
	exited := make(chan struct{})
	Register("stop-cooperative", func(ctx *Context) error { close(started); <-ctx.Done(); close(exited); return ctx.Err() })
	defer Unregister("stop-cooperative")
	j := &JobInfo{ID: 88, JobName: "cooperative", HandlerName: "stop-cooperative", ScheduleType: ScheduleCron, Timeout: 0}
	m := &Manager{cfg: &Config{MaxConcurrent: 1, MaxSerialQueue: 1}, st: st, jobs: map[int64]*jobRuntime{}, sem: make(chan struct{}, 1), running: true, stopChan: make(chan struct{}), hostname: "test"}
	jr := &jobRuntime{info: *j, cancels: map[int64]context.CancelFunc{}}
	m.jobs[j.ID] = jr
	m.mu.Lock()
	m.wg.Add(1)
	m.mu.Unlock()
	go func() { defer m.wg.Done(); m.execute(jr, GetHandler(j.HandlerName), TriggerManual, nil) }()
	<-started
	done := make(chan struct{})
	go func() { m.Stop(); close(done) }()
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel cooperative handler")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not wait for execution completion")
	}
}

func TestTimeoutWaitsForUncooperativeHandlerBeforeReturning(t *testing.T) {
	st := newMemStore(t)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	returned := make(chan struct{})
	var attempts atomic.Int32
	Register("timeout-uncooperative", func(*Context) error {
		if attempts.Add(1) == 1 {
			started <- struct{}{}
			<-release
			close(returned)
			return nil
		}
		return nil
	})
	defer Unregister("timeout-uncooperative")
	j := JobInfo{ID: 89, JobName: "uncooperative", HandlerName: "timeout-uncooperative", ScheduleType: ScheduleCron, Timeout: 1, RetryCount: 1}
	m := &Manager{cfg: &Config{MaxConcurrent: 1, MaxSerialQueue: 1}, st: st, jobs: map[int64]*jobRuntime{}, sem: make(chan struct{}, 1), running: true, stopChan: make(chan struct{}), hostname: "test", loc: time.Local}
	jr := &jobRuntime{info: j}
	done := make(chan struct{})
	go func() { m.execute(jr, GetHandler(j.HandlerName), TriggerManual, nil); close(done) }()
	<-started
	timer := time.NewTimer(1200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-done:
		t.Fatal("execute returned while handler still running")
	case <-timer.C:
	}
	select {
	case <-started:
		t.Fatal("retry began while first handler was still running")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("execute did not finish after handler returned")
	}
	logs, total, err := st.pageLog(j.ID, "", -1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(logs) != 2 || logs[1].Status != LogTimeout || logs[1].RetryNum != 0 {
		t.Fatalf("timeout lifecycle logs: total=%d logs=%+v", total, logs)
	}
}

func TestManualSerialWaitIsCanceledByStop(t *testing.T) {
	st := newMemStore(t)
	Register("manual-serial-stop", func(*Context) error { return nil })
	defer Unregister("manual-serial-stop")
	j := &JobInfo{ID: 90, JobName: "serial", HandlerName: "manual-serial-stop", ScheduleType: ScheduleCron, BlockStrategy: BlockSerial}
	jr := &jobRuntime{info: *j, serialMu: make(chan struct{}, 1)}
	jr.serialMu <- struct{}{} // Occupy the serial slot so Trigger must wait.
	m := &Manager{cfg: &Config{MaxConcurrent: 1, MaxSerialQueue: 2}, st: st, jobs: map[int64]*jobRuntime{j.ID: jr}, sem: make(chan struct{}, 1), running: true, stopChan: make(chan struct{}), hostname: "test"}
	triggerDone := make(chan struct{})
	go func() { _, _ = m.Trigger(j.ID, nil); close(triggerDone) }()
	time.Sleep(10 * time.Millisecond)
	stopDone := make(chan struct{})
	go func() { m.Stop(); close(stopDone) }()
	select {
	case <-triggerDone:
	case <-time.After(time.Second):
		t.Fatal("Trigger did not leave serial wait after Stop")
	}
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("Stop did not complete after queued Trigger left")
	}
}

func TestConcurrentStopCallsWaitForSameShutdown(t *testing.T) {
	m := &Manager{running: true, stopChan: make(chan struct{}), stopDone: make(chan struct{})}
	started := make(chan struct{})
	release := make(chan struct{})
	m.wg.Add(1)
	go func() { defer m.wg.Done(); close(started); <-release }()
	first := make(chan struct{})
	go func() { m.Stop(); close(first) }()
	<-started
	second := make(chan struct{})
	go func() { m.Stop(); close(second) }()
	select {
	case <-second:
		t.Fatal("concurrent Stop returned before shutdown completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("first Stop did not finish")
	}
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("second Stop did not wait for shutdown")
	}
}

func TestStoreLog(t *testing.T) {
	st := newMemStore(t)

	now := time.Now()
	jl := &JobLog{
		JobId:       1,
		JobName:     "testJob",
		HandlerName: "testHandler",
		TriggerType: TriggerManual,
		Status:      LogRunning,
		StartAt:     now,
	}
	if err := st.createLog(jl); err != nil {
		t.Fatalf("createLog: %v", err)
	}
	if jl.ID <= 0 {
		t.Fatalf("log ID 未自增")
	}

	end := now.Add(100 * time.Millisecond)
	jl.Status = LogSuccess
	jl.EndAt = &end
	jl.CostMs = 100
	jl.Message = "ok"
	if err := st.finishLog(jl); err != nil {
		t.Fatalf("finishLog: %v", err)
	}

	list, total, err := st.pageLog(1, "", -1, 1, 10)
	if err != nil {
		t.Fatalf("pageLog: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("pageLog 期望 1 条, got %d", total)
	}

	// 残留日志清理
	n, err := st.resetRunningLogs()
	if err != nil {
		t.Fatalf("resetRunningLogs: %v", err)
	}
	_ = n
}

func TestHandlerRegistry(t *testing.T) {
	called := false
	Register("unitHandler", func(ctx *Context) error {
		called = true
		t.Logf("hello %s", ctx.Param["value"].(string))
		return nil
	})
	defer Unregister("unitHandler")

	h := GetHandler("unitHandler")
	if h == nil {
		t.Fatalf("注册后 GetHandler 应非空")
	}
	names := ListHandlers()
	found := false
	for _, n := range names {
		if n == "unitHandler" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListHandlers 应包含 unitHandler")
	}

	ctx := &Context{Param: map[string]interface{}{"value": "world"}, JobName: "u"}
	if err := h(ctx); err != nil {
		t.Fatalf("handler 执行失败: %v", err)
	}
	if !called {
		t.Fatalf("handler 未被调用")
	}
}
