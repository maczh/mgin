package job

import (
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
		JobName:     "testJob",
		JobGroup:    "default",
		ScheduleType: ScheduleCron,
		ScheduleConf: "0 */1 * * * ?",
		HandlerName: "testHandler",
		Status:      StatusRunning,
	}
	if err := st.create(job); err != nil {
		t.Fatalf("create: %v", err)
	}
	if job.ID <= 0 {
		t.Fatalf("期望自增 ID > 0, got %d", job.ID)
	}

	got, err := st.getById(job.ID)
	if err != nil {
		t.Fatalf("getById: %v", err)
	}
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

func TestStoreLog(t *testing.T) {
	st := newMemStore(t)

	now := time.Now()
	jl := &JobLog{
		JobId:       1,
		JobName:    "testJob",
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
		ctx.Log("hello %s", ctx.Param)
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

	ctx := &Context{Param: "world", JobName: "u"}
	if err := h(ctx); err != nil {
		t.Fatalf("handler 执行失败: %v", err)
	}
	if !called {
		t.Fatalf("handler 未被调用")
	}
}
