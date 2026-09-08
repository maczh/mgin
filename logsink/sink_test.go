package logsink

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maczh/mgin/models"
)

type memHandler struct {
	name    string
	mu      sync.Mutex
	got     []*Entry
	fail    bool
	panic   bool
	closeN  int
	writeCh chan struct{}
}

func (h *memHandler) Name() string { return h.name }

func (h *memHandler) Write(e *Entry) error {
	if h.writeCh != nil {
		h.writeCh <- struct{}{}
	}
	if h.panic {
		panic("boom")
	}
	if h.fail {
		return errors.New("write failed")
	}
	h.mu.Lock()
	h.got = append(h.got, e)
	h.mu.Unlock()
	return nil
}

func (h *memHandler) Close() error { h.closeN++; return nil }

func (h *memHandler) len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.got)
}

func newEntry(uri string) *Entry {
	return &models.PostLog{Uri: uri, Method: "GET"}
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", msg)
}

func TestRegisterAndEmit(t *testing.T) {
	Reset()
	defer Reset()

	h := &memHandler{name: "mem"}
	if err := Register(h); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if !HasHandlers() {
		t.Fatal("HasHandlers 应为 true")
	}
	if names := HandlerNames(); len(names) != 1 || names[0] != "mem" {
		t.Fatalf("HandlerNames 异常: %v", names)
	}
	if err := Register(&memHandler{name: "mem"}); !errors.Is(err, ErrDuplicateHandler) {
		t.Fatalf("重复注册应返回 ErrDuplicateHandler, 实际 %v", err)
	}
	if err := Register(nil); !errors.Is(err, ErrInvalidHandler) {
		t.Fatalf("空 handler 应返回 ErrInvalidHandler, 实际 %v", err)
	}

	if !Emit(newEntry("/a")) {
		t.Fatal("Emit 应投递成功")
	}
	waitFor(t, func() bool { return h.len() == 1 }, "handler 收到日志")

	if Emit(nil) {
		t.Fatal("空日志不应投递成功")
	}
}

func TestEmitWithoutHandler(t *testing.T) {
	Reset()
	defer Reset()
	if Emit(newEntry("/a")) {
		t.Fatal("无 handler 时 Emit 应返回 false")
	}
}

func TestDropWhenQueueFull(t *testing.T) {
	Reset()
	defer Reset()

	var dropped atomic.Int64
	SetDropLogger(func(name string, n uint64) { dropped.Add(1) })
	defer SetDropLogger(nil)

	h := &memHandler{name: "slow", writeCh: make(chan struct{}, 1)}
	if err := Register(h, WithQueueSize(1)); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 第一条被 worker 取走并卡在 writeCh, 队列塞满后开始丢弃
	for i := 0; i < 20; i++ {
		Emit(newEntry("/x"))
	}
	waitFor(t, func() bool { return Stats()[0].Dropped > 0 }, "出现丢弃计数")
	if Stats()[0].Name != "slow" {
		t.Fatalf("stats 名称异常: %+v", Stats()[0])
	}
	Unregister("slow", 200*time.Millisecond)
}

func TestPanicAndErrorAreIsolated(t *testing.T) {
	Reset()
	defer Reset()

	var errN atomic.Int64
	SetErrorLogger(func(name string, err error) { errN.Add(1) })
	defer SetErrorLogger(nil)

	if err := Register(&memHandler{name: "bad", fail: true}); err != nil {
		t.Fatal(err)
	}
	if err := Register(&memHandler{name: "boom", panic: true}); err != nil {
		t.Fatal(err)
	}
	good := &memHandler{name: "good"}
	if err := Register(good); err != nil {
		t.Fatal(err)
	}

	Emit(newEntry("/a"))
	waitFor(t, func() bool { return good.len() == 1 }, "正常 handler 不受影响")
	waitFor(t, func() bool { return errN.Load() >= 2 }, "失败与 panic 均被捕获")
}

func TestCloseDrainsAndCallsCloser(t *testing.T) {
	Reset()
	defer Reset()

	h := &memHandler{name: "drain"}
	if err := Register(h); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		Emit(newEntry("/a"))
	}
	Close(2 * time.Second)
	if h.closeN != 1 {
		t.Fatalf("Close 应调用一次 Closer, 实际 %d", h.closeN)
	}
	if h.len() != 5 {
		t.Fatalf("Close 应排空队列, 实际收到 %d 条", h.len())
	}
	if HasHandlers() {
		t.Fatal("Close 后不应再有 handler")
	}
	if Emit(newEntry("/a")) {
		t.Fatal("Close 后 Emit 应返回 false")
	}
}

func TestHandlerFuncAdapter(t *testing.T) {
	Reset()
	defer Reset()

	var n atomic.Int64
	closed := false
	h := NewHandlerFunc("fn", func(e *Entry) error {
		n.Add(1)
		return nil
	}).WithClose(func() error { closed = true; return nil })

	if err := Register(h); err != nil {
		t.Fatal(err)
	}
	Emit(newEntry("/a"))
	waitFor(t, func() bool { return n.Load() == 1 }, "HandlerFunc 被调用")
	Unregister("fn", time.Second)
	if !closed {
		t.Fatal("Unregister 应调用 WithClose")
	}
}

func TestConcurrentEmit(t *testing.T) {
	Reset()
	defer Reset()

	h := &memHandler{name: "conc"}
	if err := Register(h, WithQueueSize(4096), WithWorkers(2)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				Emit(newEntry("/c"))
			}
		}()
	}
	wg.Wait()
	waitFor(t, func() bool { return h.len()+int(droppedOf(h.name)) == 1000 }, "全部日志被处理或丢弃")
	Close(3 * time.Second)
}

func droppedOf(name string) uint64 {
	for _, s := range Stats() {
		if s.Name == name {
			return s.Dropped
		}
	}
	return 0
}
