package logsink

import (
	"sync"
	"time"
)

// Record is an application log captured before asynchronous dispatch.
type Record struct {
	// Data is the destination-specific formatted payload (without a trailing newline).
	Data        []byte `json:"-"`
	Time        string `json:"time"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	App         string `json:"app"`
	RequestID   string `json:"request_id"`
	GoroutineID uint64 `json:"goroutine_id"`
	Package     string `json:"package"`
	File        string `json:"file"`
	Function    string `json:"function"`
	Line        int    `json:"line"`
}

// RecordHandler receives application logs, independently of PostLog handlers.
// Write must be concurrency safe and must not log back into the same sink.
type RecordHandler interface {
	Name() string
	Write(*Record) error
}

var recordMu sync.RWMutex
var recordRegistry = make(map[string]*sink[Record])
var recordOrder []string

// RecordHandlerFunc adapts a function to RecordHandler.
type RecordHandlerFunc struct {
	closeFn   func() error
	ID        string
	WriteFunc func(*Record) error
}

// Name returns the destination name.
func (h RecordHandlerFunc) Name() string { return h.ID }

// Write invokes the configured application log callback.
func (h RecordHandlerFunc) Write(r *Record) error {
	if h.WriteFunc == nil {
		return nil
	}
	return h.WriteFunc(r)
}

// RegisterRecord 注册一个日志处理器并立即启动其消费协程。
// 同名 Handler 重复注册返回 ErrDuplicateHandler，原 Handler 保持不变。
func RegisterRecord(h RecordHandler, opts ...Option) error {
	if h == nil || h.Name() == "" {
		return ErrInvalidHandler
	}
	var o Options
	for _, apply := range opts {
		if apply != nil {
			apply(&o)
		}
	}
	o = o.normalize()

	recordMu.Lock()
	defer recordMu.Unlock()
	if _, exists := recordRegistry[h.Name()]; exists {
		return ErrDuplicateHandler
	}
	s := newSink(h, o)
	recordRegistry[s.name] = s
	recordOrder = append(recordOrder, s.name)
	s.start()
	return nil
}

// MustRegisterRecord 同 RegisterRecord，注册失败时 panic。适用于启动阶段装配。
func MustRegisterRecord(h RecordHandler, opts ...Option) {
	if err := RegisterRecord(h, opts...); err != nil {
		panic(err)
	}
}

// UnregisterRecord 注销并停止指定 Handler，等待其队列排空（受 timeout 限制），
// 若 Handler 实现了 Closer 则调用 Close。
func UnregisterRecord(name string, timeout time.Duration) bool {
	recordMu.Lock()
	s, ok := recordRegistry[name]
	if ok {
		delete(recordRegistry, name)
		for i, n := range recordOrder {
			if n == name {
				recordOrder = append(recordOrder[:i], recordOrder[i+1:]...)
				break
			}
		}
	}
	recordMu.Unlock()
	if !ok {
		return false
	}
	s.stop(timeout)
	return true
}

// HasRecordHandlers 是否已注册任何 Handler。用于快速跳过日志投递。
func HasRecordHandlers() bool {
	recordMu.RLock()
	defer recordMu.RUnlock()
	return len(recordOrder) > 0
}

// RecordHandlerNames 返回已注册 Handler 的名称，按注册顺序排列。
func RecordHandlerNames() []string {
	recordMu.RLock()
	defer recordMu.RUnlock()
	names := make([]string, len(recordOrder))
	copy(names, recordOrder)
	return names
}

// EmitRecord 异步投递一条日志到所有 Handler。
// 非阻塞：队列满时该条日志对该 Handler 丢弃并计数。返回是否至少投递成功一个 Handler。
func EmitRecord(entry *Record) bool {
	if entry == nil {
		return false
	}
	recordMu.RLock()
	defer recordMu.RUnlock()
	if len(recordOrder) == 0 {
		return false
	}
	delivered := false
	for _, name := range recordOrder {
		if recordRegistry[name].deliver(entry) {
			delivered = true
		}
	}
	return delivered
}

// closeRecords 停止全部 Handler：停止接收新日志、尽力排空队列、调用 Closer。
// timeout 为等待每个 Handler 排空的时间预算，超时则放弃剩余日志。
func closeRecords(timeout time.Duration) {

	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	recordMu.Lock()
	names := make([]string, len(recordOrder))
	copy(names, recordOrder)
	sinks := make([]*sink[Record], 0, len(names))
	for _, name := range names {
		sinks = append(sinks, recordRegistry[name])
	}
	recordRegistry = make(map[string]*sink[Record])
	recordOrder = nil
	recordMu.Unlock()

	for _, s := range sinks {
		s.stop(timeout)
	}
}

// resetRecords 清空全部 Handler（不等待排空、不调用 Closer），仅用于测试。
func resetRecords() {
	recordMu.Lock()
	defer recordMu.Unlock()
	for _, s := range recordRegistry {
		s.halt()
	}
	recordRegistry = make(map[string]*sink[Record])
	recordOrder = nil
}

// RecordStats 返回全部 Handler 的运行状态，按注册顺序排列。
func RecordStats() []HandlerStats {
	recordMu.RLock()
	defer recordMu.RUnlock()
	stats := make([]HandlerStats, 0, len(recordOrder))
	for _, name := range recordOrder {
		stats = append(stats, recordRegistry[name].stats())
	}
	return stats
}

// EmitRecordTo sends a record only to the named application log handler.
// The record and its Data must remain immutable after dispatch.
func EmitRecordTo(name string, entry *Record) bool {
	if entry == nil {
		return false
	}
	recordMu.RLock()
	defer recordMu.RUnlock()
	s := recordRegistry[name]
	return s != nil && s.deliver(entry)
}

// NewRecordHandlerFunc adapts a write callback to an application log handler.
func NewRecordHandlerFunc(name string, write func(*Record) error) *RecordHandlerFunc {
	return &RecordHandlerFunc{ID: name, WriteFunc: write}
}

// WithClose attaches a shutdown callback.
func (h *RecordHandlerFunc) WithClose(closeFn func() error) *RecordHandlerFunc {
	h.closeFn = closeFn
	return h
}

// Close closes the destination after its queue drains.
func (h RecordHandlerFunc) Close() error {
	if h.closeFn != nil {
		return h.closeFn()
	}
	return nil
}
