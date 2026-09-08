package logsink

// HandlerFunc 把普通函数适配成 Handler，方便外部插件一行挂接。
//
//	sink := logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
//	    return kafka.Send(topic, e)
//	}).WithClose(kafka.Close)
//	logsink.Register(sink)
type HandlerFunc struct {
	name    string
	write   func(entry *Entry) error
	closeFn func() error
}

// NewHandlerFunc 用名称与写函数构造 Handler。
func NewHandlerFunc(name string, write func(entry *Entry) error) *HandlerFunc {
	return &HandlerFunc{name: name, write: write}
}

// WithClose 设置关闭回调，返回自身以便链式调用。
func (f *HandlerFunc) WithClose(fn func() error) *HandlerFunc {
	f.closeFn = fn
	return f
}

// Name 返回 Handler 名称。
func (f *HandlerFunc) Name() string { return f.name }

// Write 执行日志写入。
func (f *HandlerFunc) Write(entry *Entry) error {
	if f.write == nil {
		return nil
	}
	return f.write(entry)
}

// Close 实现可选接口 Closer。
func (f *HandlerFunc) Close() error {
	if f.closeFn == nil {
		return nil
	}
	return f.closeFn()
}
