// Package logsink 提供 MGIN 接口访问日志(PostLog)的异步分发能力。
//
// 设计目标：
//  1. 中间件 middleware/postlog 只负责产生日志并调用 Emit，不关心日志落到哪里；
//  2. 落库、发 Kafka、写 Elasticsearch 等动作由注册的 Handler 完成；
//  3. 外部插件（mgkafka、elasticsearch 插件等）只需实现 Handler 并在启动时注册，
//     即可接收全量接口日志，框架无需为每种目标硬编码分支。
//
// 处理模型：每个 Handler 拥有独立的缓冲队列与消费协程，慢 Handler 互不影响；
// 投递是非阻塞的，队列满时丢弃并计数，绝不会拖慢接口响应。
package logsink

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maczh/mgin/models"
)

// Entry 一条接口访问日志。
type Entry = models.PostLog

// Handler 日志处理器。由内置实现或外部插件实现，注册后即可收到全量接口日志。
//
// Name 用于注册去重、日志标识以及运行期注销，必须唯一且非空。
// Write 在独立协程中被调用，实现必须是并发安全的；返回 error 仅用于统计与告警。
type Handler interface {
	Name() string
	Write(entry *Entry) error
}

// Closer 可选接口。实现了它的 Handler 在注销或全局关闭时会被调用一次。
type Closer interface {
	Close() error
}

const (
	// DefaultQueueSize 每个 Handler 默认的缓冲队列长度。
	DefaultQueueSize = 1024
	// DefaultWorkers 每个 Handler 默认的消费协程数，1 保证日志按到达顺序处理。
	DefaultWorkers = 1
	// DefaultShutdownTimeout 关闭时等待队列排空的默认超时。
	DefaultShutdownTimeout = 3 * time.Second

	// 丢弃与错误告警的最小间隔，避免故障时被日志刷屏。
	notifyInterval = 60 * time.Second
)

// Options 注册参数。
type Options struct {
	// QueueSize 缓冲队列长度，<=0 使用 DefaultQueueSize。
	QueueSize int
	// Workers 消费协程数，<=0 使用 DefaultWorkers。大于 1 时不保证处理顺序。
	Workers int
}

// Option 注册时的可选配置。
type Option func(*Options)

// WithQueueSize 设置该 Handler 的缓冲队列长度。
func WithQueueSize(n int) Option { return func(o *Options) { o.QueueSize = n } }

// WithWorkers 设置该 Handler 的消费协程数。
func WithWorkers(n int) Option { return func(o *Options) { o.Workers = n } }

func (o *Options) normalize() Options {
	opt := Options{QueueSize: o.QueueSize, Workers: o.Workers}
	if opt.QueueSize <= 0 {
		opt.QueueSize = DefaultQueueSize
	}
	if opt.Workers <= 0 {
		opt.Workers = DefaultWorkers
	}
	return opt
}

// ErrDuplicateHandler 同名 Handler 重复注册。
var ErrDuplicateHandler = errors.New("logsink: 同名 handler 已注册")

// ErrInvalidHandler Handler 为空或 Name 为空。
var ErrInvalidHandler = errors.New("logsink: handler 不能为空且 Name 不能为空")

var (
	mu       sync.RWMutex
	registry = make(map[string]*sink)
	order    []string
)

var (
	dropLogger  atomic.Pointer[func(string, uint64)]
	errorLogger atomic.Pointer[func(string, error)]
)

// SetDropLogger 设置日志丢弃时的回调，参数为 handler 名称与该 handler 累计丢弃条数。
// 回调已做 60 秒限流。默认不输出任何内容，由 MGIN 内部接管为 logs 包输出。
func SetDropLogger(fn func(handler string, dropped uint64)) {
	if fn == nil {
		dropLogger.Store(nil)
		return
	}
	dropLogger.Store(&fn)
}

// SetErrorLogger 设置 Handler 写失败或 panic 时的回调。
// 回调已做 60 秒限流。默认不输出任何内容。
func SetErrorLogger(fn func(handler string, err error)) {
	if fn == nil {
		errorLogger.Store(nil)
		return
	}
	errorLogger.Store(&fn)
}

// Register 注册一个日志处理器并立即启动其消费协程。
// 同名 Handler 重复注册返回 ErrDuplicateHandler，原 Handler 保持不变。
func Register(h Handler, opts ...Option) error {
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

	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[h.Name()]; exists {
		return ErrDuplicateHandler
	}
	s := newSink(h, o)
	registry[s.name] = s
	order = append(order, s.name)
	s.start()
	return nil
}

// MustRegister 同 Register，注册失败时 panic。适用于启动阶段装配。
func MustRegister(h Handler, opts ...Option) {
	if err := Register(h, opts...); err != nil {
		panic(err)
	}
}

// Unregister 注销并停止指定 Handler，等待其队列排空（受 timeout 限制），
// 若 Handler 实现了 Closer 则调用 Close。
func Unregister(name string, timeout time.Duration) bool {
	mu.Lock()
	s, ok := registry[name]
	if ok {
		delete(registry, name)
		for i, n := range order {
			if n == name {
				order = append(order[:i], order[i+1:]...)
				break
			}
		}
	}
	mu.Unlock()
	if !ok {
		return false
	}
	s.stop(timeout)
	return true
}

// HasHandlers 是否已注册任何 Handler。用于快速跳过日志投递。
func HasHandlers() bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(order) > 0
}

// HandlerNames 返回已注册 Handler 的名称，按注册顺序排列。
func HandlerNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, len(order))
	copy(names, order)
	return names
}

// Emit 异步投递一条日志到所有 Handler。
// 非阻塞：队列满时该条日志对该 Handler 丢弃并计数。返回是否至少投递成功一个 Handler。
func Emit(entry *Entry) bool {
	if entry == nil {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	if len(order) == 0 {
		return false
	}
	delivered := false
	for _, name := range order {
		if registry[name].deliver(entry) {
			delivered = true
		}
	}
	return delivered
}

// Close 停止全部 Handler：停止接收新日志、尽力排空队列、调用 Closer。
// timeout 为等待每个 Handler 排空的时间预算，超时则放弃剩余日志。
func Close(timeout time.Duration) {
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	mu.Lock()
	names := make([]string, len(order))
	copy(names, order)
	sinks := make([]*sink, 0, len(names))
	for _, name := range names {
		sinks = append(sinks, registry[name])
	}
	registry = make(map[string]*sink)
	order = nil
	mu.Unlock()

	for _, s := range sinks {
		s.stop(timeout)
	}
}

// Reset 清空全部 Handler（不等待排空、不调用 Closer），仅用于测试。
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	for _, s := range registry {
		s.halt()
	}
	registry = make(map[string]*sink)
	order = nil
}

// HandlerStats 单个 Handler 的运行状态。
type HandlerStats struct {
	Name    string
	Queued  int
	Dropped uint64
	Written uint64
	Failed  uint64
}

// Stats 返回全部 Handler 的运行状态，按注册顺序排列。
func Stats() []HandlerStats {
	mu.RLock()
	defer mu.RUnlock()
	stats := make([]HandlerStats, 0, len(order))
	for _, name := range order {
		stats = append(stats, registry[name].stats())
	}
	return stats
}

// sink 一个 Handler 的运行实例。
type sink struct {
	name    string
	h       Handler
	workers int
	ch      chan *Entry
	done    chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	closed  atomic.Bool
	dropped atomic.Uint64
	written atomic.Uint64
	failed  atomic.Uint64

	lastDropNotify atomic.Int64
	lastErrNotify  atomic.Int64
}

func newSink(h Handler, opt Options) *sink {
	return &sink{
		name: h.Name(),
		h:    h,
		ch:   make(chan *Entry, opt.QueueSize),
		done: make(chan struct{}),
	}
}

func (s *sink) start() {
	s.wg.Add(1)
	go s.loop()
	// 多 worker 时其余协程共用同一队列，不保证顺序。
}

func (s *sink) loop() {
	defer s.wg.Done()
	for {
		select {
		case entry := <-s.ch:
			s.handle(entry)
		case <-s.done:
			s.drain()
			return
		}
	}
}

// drain 处理关闭信号后队列中残留的日志。
func (s *sink) drain() {
	for {
		select {
		case entry := <-s.ch:
			s.handle(entry)
		default:
			return
		}
	}
}

func (s *sink) handle(entry *Entry) {
	defer func() {
		if r := recover(); r != nil {
			s.failed.Add(1)
			s.notifyError(fmt.Errorf("panic: %v", r))
		}
	}()
	if err := s.h.Write(entry); err != nil {
		s.failed.Add(1)
		s.notifyError(err)
		return
	}
	s.written.Add(1)
}

// deliver 非阻塞投递，队列满则丢弃。
func (s *sink) deliver(entry *Entry) bool {
	if s.closed.Load() {
		return false
	}
	select {
	case s.ch <- entry:
		return true
	default:
		s.notifyDrop(s.dropped.Add(1))
		return false
	}
}

// stop 停止接收并等待排空，随后调用 Closer。
func (s *sink) stop(timeout time.Duration) {
	s.closed.Store(true)
	s.once.Do(func() { close(s.done) })
	if !waitTimeout(&s.wg, timeout) {
		s.notifyError(fmt.Errorf("关闭超时，丢弃队列中的日志"))
	}
	if c, ok := s.h.(Closer); ok {
		if err := c.Close(); err != nil {
			s.notifyError(err)
		}
	}
}

// halt 立即停止，不等待排空（仅测试用）。
func (s *sink) halt() {
	s.closed.Store(true)
	s.once.Do(func() { close(s.done) })
}

func (s *sink) stats() HandlerStats {
	return HandlerStats{
		Name:    s.name,
		Queued:  len(s.ch),
		Dropped: s.dropped.Load(),
		Written: s.written.Load(),
		Failed:  s.failed.Load(),
	}
}

func (s *sink) notifyDrop(total uint64) {
	fn := dropLogger.Load()
	if fn == nil {
		return
	}
	if !throttle(&s.lastDropNotify) {
		return
	}
	(*fn)(s.name, total)
}

func (s *sink) notifyError(err error) {
	fn := errorLogger.Load()
	if fn == nil {
		return
	}
	if !throttle(&s.lastErrNotify) {
		return
	}
	(*fn)(s.name, err)
}

// throttle 保证同一 sink 的同类告警最小间隔。
func throttle(last *atomic.Int64) bool {
	now := time.Now().UnixNano()
	prev := last.Load()
	if prev != 0 && now-prev < int64(notifyInterval) {
		return false
	}
	return last.CompareAndSwap(prev, now)
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
