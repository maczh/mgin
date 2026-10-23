# 可观测性

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 健康探针、Prometheus 指标、OpenTelemetry、日志与日志投递。

## `github.com/maczh/mgin/v2/pkg/health`

```go
import "github.com/maczh/mgin/v2/pkg/health"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `StatusOK` | = "ok" | 响应体 status 字段的取值。 |
| `StatusFail` | = "fail" | 响应体 status 字段的取值。 |
| `StatusUP` | = "UP" | 响应体 status 字段的取值。 |

### 函数

#### `CheckDependencies`

```go
func CheckDependencies() (bool, map[string]string)
```

CheckDependencies 按顺序检查所有已启用的数据源。
返回值:
- bool: 全部通过为 true，任一失败为 false；没有启用任何数据源时返回 true。
- map[string]string: 每个数据源的健康状态，值为 StatusOK 或 StatusFail。

说明：这里直接复用各数据源的 Check()，其语义与 mgin.go 中 checkAll() 完全一致
（部分数据源的 Check() 内部失败时会尝试重连），因此探针具备“自愈”能力。
注意 Check() 内部可能发起重连，耗时不可控，调用方应自行评估 K8s 探针超时时间。

#### `HealthzHandler`

```go
func HealthzHandler(c *gin.Context)
```

HealthzHandler 存活探针的别名端点（根路径 /healthz）。

它与 /health/live 调用完全相同的处理逻辑（liveHandler），因此两者返回结果
永远一致，用于兼容同时要求 /healthz 与 /health/live 两种路径的探针规范。

#### `IsStarted`

```go
func IsStarted() bool
```

IsStarted 返回应用是否已完成启动。

#### `MarkStarted`

```go
func MarkStarted()
```

MarkStarted 标记应用已完成启动。
业务应在所有初始化工作（数据源、缓存、定时任务、本地预热等）完成之后调用；
未调用之前 /startup 一律返回 503。
也可以通过配置 go.health.autoStarted=true 让框架在监听启动后自动标记。

#### `ResetStarted`

```go
func ResetStarted()
```

ResetStarted 清除启动完成标记，主要用于测试场景。

#### `Router`

```go
func Router(g *gin.RouterGroup)
```

Router 在给定的路由组上挂载三个探针端点。
传入 app.Router.Group("/health") 可把端点挂到 /health/live、/health/ready、/health/startup 下。

#### `SetVersion`

```go
func SetVersion(v string)
```

SetVersion 注入应用版本号，供 /healthz、/health/live 的标准健康模板使用。
建议在 NewApp() 中创建 App 后调用一次。

#### `StartedAt`

```go
func StartedAt() time.Time
```

StartedAt 返回应用完成启动的时间；未完成启动时返回零值。

## `github.com/maczh/mgin/v2/pkg/metrics`

```go
import "github.com/maczh/mgin/v2/pkg/metrics"
```

### 函数

#### `Handler`

```go
func Handler() http.Handler
```

Handler 返回 promhttp 的 /metrics handler，可直接挂到 gin 路由。

#### `Middleware`

```go
func Middleware() gin.HandlerFunc
```

Middleware 返回一个 gin handler，用于自动记录 HTTP 指标。
用法：r.Use(metrics.Middleware())

注意：path 用 c.FullPath()，避免把 URL 参数（/users/123）当成不同的 label，
否则高基数会撑爆 Prometheus。

#### `SetBuildInfo`

```go
func SetBuildInfo(version, commit, goVersion string)
```

SetBuildInfo 在启动时填入版本信息。version/commit/goVersion 任一为空会被填为 "unknown"。
建议在 mgin.NewApp 之后、app.Run 之前调用一次。

#### `SetDependencyUp`

```go
func SetDependencyUp(name string, up bool)
```

SetDependencyUp 一次性更新某个外部依赖的可用性。

#### `SetPluginHealth`

```go
func SetPluginHealth(name string, healthy bool)
```

SetPluginHealth 一次性更新某个 plugin 的健康状态。name 为空时 no-op。

## `github.com/maczh/mgin/v2/pkg/otel`

```go
import "github.com/maczh/mgin/v2/pkg/otel"
```

### 函数

#### `Disable`

```go
func Disable()
```

Disable 显式禁用 OTel，Tracer 退化到 noop。
重复调用安全。

#### `IsEnabled`

```go
func IsEnabled() bool
```

IsEnabled 报告 OTel 是否已启用（供 health/metrics 等模块做"已启用才上报"判断）。

#### `SetTracerProvider`

```go
func SetTracerProvider(tp trace.TracerProvider)
```

SetTracerProvider 注入业务已构造好的 TracerProvider。
同时把 otel 全局也设上，让 gin / gorm 的 OTel 扩展能识别到。

重复设置会以最后一次为准（用于测试场景切换）。

#### `StartSpan`

```go
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
```

StartSpan 是 Tracer().Start 的便捷封装，自动从 ctx 透传 parent。
业务侧可用：

ctx, span := mginotel.StartSpan(ctx, "user-service.GetUser")
defer span.End()

#### `Tracer`

```go
func Tracer() trace.Tracer
```

Tracer 取 mgin 命名空间下的 Tracer。
未启用时返 noop Tracer，所有 span 不会真正产生开销。

## `github.com/maczh/mgin/v2/pkg/logs`

```go
import "github.com/maczh/mgin/v2/pkg/logs"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `Reset` | Attribute = iota | Base attributes |
| `Bold` | = | Base attributes |
| `Faint` | = | Base attributes |
| `Italic` | = | Base attributes |
| `Underline` | = | Base attributes |
| `BlinkSlow` | = | Base attributes |
| `BlinkRapid` | = | Base attributes |
| `ReverseVideo` | = | Base attributes |
| `Concealed` | = | Base attributes |
| `CrossedOut` | = | Base attributes |
| `FgBlack` | Attribute = iota + 30 | Foreground text colors |
| `FgRed` | = | Foreground text colors |
| `FgGreen` | = | Foreground text colors |
| `FgYellow` | = | Foreground text colors |
| `FgBlue` | = | Foreground text colors |
| `FgMagenta` | = | Foreground text colors |
| `FgCyan` | = | Foreground text colors |
| `FgWhite` | = | Foreground text colors |
| `BgBlack` | Attribute = iota + 40 | Background text colors |
| `BgRed` | = | Background text colors |
| `BgGreen` | = | Background text colors |
| `BgYellow` | = | Background text colors |
| `BgBlue` | = | Background text colors |
| `BgMagenta` | = | Background text colors |
| `BgCyan` | = | Background text colors |
| `BgWhite` | = | Background text colors |
| `CONSOLE` | string = "console" |  |
| `FILE` | string = "file" |  |
| `ELASTICSEARCH` | string = "es" |  |
| `SimpleLog` | string = "simple" |  |
| `ColoredLog` | string = "color" |  |
| `VictoriaLogsTarget` | = "victorialogs" |  |
| `VictoriaLogsClientTarget` | = "victorialogs-client" |  |

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `NoColor` | = !isatty.IsTerminal(os.Stdout.Fd()) | NoColor defines if the output is colorized or not. It's dynamically set to false or true based on the stdout's file descriptor referring to a terminal or not. This is a global option and affects all colors. For more cont… |
| `Output` | = ansicolor.NewAnsiColorWriter(os.Stdout) | Output defines the standard output of the print functions. By default os.Stdout is used. |

### 类型

#### `Color`

```go
type Color struct
```

Color defines a custom color object which is defined by SGR parameters.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (c *Color) Set() *Color
```

Set sets the SGR sequence.

```go
func (c *Color) Add(value ...Attribute) *Color
```

Add is used to chain SGR parameters. Use as many as parameters to combine
and create custom color objects. Example: Add(color.FgRed, color.Underline).

```go
func (c *Color) Print(a ...any) (n int, err error)
```

Print formats using the default formats for its operands and writes to
standard output. Spaces are added between operands when neither is a
string. It returns the number of bytes written and any write error
encountered. This is the standard fmt.Print() method wrapped with the given
color.

```go
func (c *Color) Printf(format string, a ...any) (n int, err error)
```

Printf formats according to a format specifier and writes to standard output.
It returns the number of bytes written and any write error encountered.
This is the standard fmt.Printf() method wrapped with the given color.

```go
func (c *Color) Println(a ...any) (n int, err error)
```

Println formats using the default formats for its operands and writes to
standard output. Spaces are always added between operands and a newline is
appended. It returns the number of bytes written and any write error
encountered. This is the standard fmt.Print() method wrapped with the given
color.

```go
func (c *Color) PrintFunc() func(a ...any)
```

PrintFunc returns a new function that prints the passed arguments as
colorized with color.Print().

```go
func (c *Color) PrintfFunc() func(format string, a ...any)
```

PrintfFunc returns a new function that prints the passed arguments as
colorized with color.Printf().

```go
func (c *Color) PrintlnFunc() func(a ...any)
```

PrintlnFunc returns a new function that prints the passed arguments as
colorized with color.Println().

```go
func (c *Color) SprintFunc() func(a ...any) string
```

SprintFunc returns a new function that returns colorized strings for the
given arguments with fmt.Sprint(). Useful to put into or mix into other
string. Windows users should use this in conjuction with color.Output, example:

put := New(FgYellow).SprintFunc()
fmt.Fprintf(color.Output, "This is a %s", put("warning"))

```go
func (c *Color) SprintfFunc() func(format string, a ...any) string
```

SprintfFunc returns a new function that returns colorized strings for the
given arguments with fmt.Sprintf(). Useful to put into or mix into other
string. Windows users should use this in conjuction with color.Output.

```go
func (c *Color) SprintlnFunc() func(a ...any) string
```

SprintlnFunc returns a new function that returns colorized strings for the
given arguments with fmt.Sprintln(). Useful to put into or mix into other
string. Windows users should use this in conjuction with color.Output.

```go
func (c *Color) DisableColor()
```

DisableColor disables the color output. Useful to not change any existing
code and still being able to output. Can be used for flags like
"--no-color". To enable back use EnableColor() method.

```go
func (c *Color) EnableColor()
```

EnableColor enables the color output. Use it in conjuction with
DisableColor(). Otherwise this method has no side effects.

```go
func (c *Color) Equals(c2 *Color) bool
```

Equals returns a boolean value indicating whether two colors are equal.

#### `Attribute`

```go
type Attribute int
```

Attribute defines a single SGR Code

#### `GoLogger`

```go
type GoLogger struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Loggers` | []Logger |  |  |

**方法**

```go
func (log GoLogger) Log(message string)
```

```go
func (log GoLogger) Message(message string)
```

```go
func (log GoLogger) Info(message string)
```

```go
func (log GoLogger) Warn(message string)
```

```go
func (log GoLogger) Debug(message string)
```

```go
func (log GoLogger) Error(message string)
```

```go
func (log GoLogger) Fatal(message string)
```

```go
func (log GoLogger) ReplaceMessage(message string)
```

#### `GORMLogger`

```go
type GORMLogger struct
```

GORMLogger routes GORM diagnostics through the application's logs configuration.
LogMode returns a copy so per-session Debug does not mutate shared state.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Level` | logger.LogLevel |  |  |
| `SlowThreshold` | time.Duration |  |  |
| `IgnoreRecordNotFoundError` | bool |  |  |

**方法**

```go
func (l *GORMLogger) LogMode(level logger.LogLevel) logger.Interface
```

LogMode implements logger.Interface.

```go
func (l *GORMLogger) Info(ctx context.Context, msg string, args ...interface{})
```

Info implements logger.Interface. GORM uses printf placeholders.

```go
func (l *GORMLogger) Warn(ctx context.Context, msg string, args ...interface{})
```

Warn implements logger.Interface.

```go
func (l *GORMLogger) Error(ctx context.Context, msg string, args ...interface{})
```

Error implements logger.Interface.

```go
func (l *GORMLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error)
```

Trace implements logger.Interface, evaluating SQL only when it will be logged.

#### `Logger`

```go
type Logger struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `PrinterType` | string |  |  |
| `Location` | string |  |  |

#### `LogInstance`

```go
type LogInstance struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `LogType` | string |  |  |
| `Message` | string |  |  |
| `LoggerInit` | Logger |  |  |

### 函数

#### `Black`

```go
func Black(format string, a ...any)
```

Black is an convenient helper function to print with black foreground. A
newline is appended to format by default.

#### `BlackString`

```go
func BlackString(format string, a ...any) string
```

BlackString is an convenient helper function to return a string with black
foreground.

#### `Blue`

```go
func Blue(format string, a ...any)
```

Blue is an convenient helper function to print with blue foreground. A
newline is appended to format by default.

#### `BlueString`

```go
func BlueString(format string, a ...any) string
```

BlueString is an convenient helper function to return a string with blue
foreground.

#### `ConsolePrinter`

```go
func ConsolePrinter(log LogInstance, packageName string, fileName string, lineNumber int, funcName string, time time.Time)
```

#### `Cyan`

```go
func Cyan(format string, a ...any)
```

Cyan is an convenient helper function to print with cyan foreground. A
newline is appended to format by default.

#### `CyanString`

```go
func CyanString(format string, a ...any) string
```

CyanString is an convenient helper function to return a string with cyan
foreground.

#### `Debug`

```go
func Debug(format string, v ...any)
```

#### `Error`

```go
func Error(format string, v ...any)
```

#### `FilePrinter`

```go
func FilePrinter(log LogInstance, packageName string, fileName string, lineNumber int, funcName string, time time.Time)
```

#### `FormatRecord`

```go
func FormatRecord(r *logsink.Record, output config.LogOutput) ([]byte, error)
```

FormatRecord serializes the selected fields for a destination. An empty field
list includes all fields; text templates use {field} placeholders.

#### `GetLogger`

```go
func GetLogger(selector ...string) GoLogger
```

#### `Green`

```go
func Green(format string, a ...any)
```

Green is an convenient helper function to print with green foreground. A
newline is appended to format by default.

#### `GreenString`

```go
func GreenString(format string, a ...any) string
```

GreenString is an convenient helper function to return a string with green
foreground.

#### `Info`

```go
func Info(format string, v ...any)
```

#### `InitVictoriaLogs`

```go
func InitVictoriaLogs() error
```

InitVictoriaLogs installs destinations selected by YAML. Invalid TLS or auth
configuration fails startup rather than silently losing redirected logs.

#### `Magenta`

```go
func Magenta(format string, a ...any)
```

Magenta is an convenient helper function to print with magenta foreground.
A newline is appended to format by default.

#### `MagentaString`

```go
func MagentaString(format string, a ...any) string
```

MagentaString is an convenient helper function to return a string with magenta
foreground.

#### `New`

```go
func New(value ...Attribute) *Color
```

New returns a newly created color object.

#### `NewGORMLogger`

```go
func NewGORMLogger() *GORMLogger
```

NewGORMLogger creates a GORM logger with GORM's default warning threshold.

#### `OutPrint`

```go
func OutPrint(format string, v []any) string
```

OutPrint formats placeholders and includes the current request ID.

#### `Print`

```go
func Print(log LogInstance, packageName string, fileName string, lineNumber int, funcName string, time time.Time)
```

#### `Red`

```go
func Red(format string, a ...any)
```

Red is an convenient helper function to print with red foreground. A
newline is appended to format by default.

#### `RedString`

```go
func RedString(format string, a ...any) string
```

RedString is an convenient helper function to return a string with red
foreground.

#### `RegisterHandler`

```go
func RegisterHandler(h logsink.RecordHandler, opts ...logsink.Option) error
```

RegisterHandler attaches an external application log destination. Configure its
name in go.logger.outputs.target. Queue defaults follow go.log.sink.

#### `Set`

```go
func Set(p ...Attribute) *Color
```

Set sets the given parameters immediately. It will change the color of
output with the given SGR parameters until color.Unset() is called.

#### `Unset`

```go
func Unset()
```

Unset resets all escape attributes and clears the output. Usually should
be called after Set().

#### `Warn`

```go
func Warn(format string, v ...any)
```

#### `White`

```go
func White(format string, a ...any)
```

White is an convenient helper function to print with white foreground. A
newline is appended to format by default.

#### `WhiteString`

```go
func WhiteString(format string, a ...any) string
```

WhiteString is an convenient helper function to return a string with white
foreground.

#### `Yellow`

```go
func Yellow(format string, a ...any)
```

Yellow is an convenient helper function to print with yellow foreground.
A newline is appended to format by default.

#### `YellowString`

```go
func YellowString(format string, a ...any) string
```

YellowString is an convenient helper function to return a string with yellow
foreground.

## `github.com/maczh/mgin/v2/pkg/logsink`

```go
import "github.com/maczh/mgin/v2/pkg/logsink"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `DefaultQueueSize` | = 1024 | DefaultQueueSize 每个 Handler 默认的缓冲队列长度。 |
| `DefaultWorkers` | = 1 | DefaultWorkers 每个 Handler 默认的消费协程数，1 保证日志按到达顺序处理。 |
| `DefaultShutdownTimeout` | = 3 * time.Second | DefaultShutdownTimeout 关闭时等待队列排空的默认超时。 |

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `ErrDuplicateHandler` | = errors.New("logsink: 同名 handler 已注册") | ErrDuplicateHandler 同名 Handler 重复注册。 |
| `ErrInvalidHandler` | = errors.New("logsink: handler 不能为空且 Name 不能为空") | ErrInvalidHandler Handler 为空或 Name 为空。 |

### 类型

#### `HandlerFunc`

```go
type HandlerFunc struct
```

HandlerFunc 把普通函数适配成 Handler，方便外部插件一行挂接。

sink := logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
return kafka.Send(topic, e)
}).WithClose(kafka.Close)
logsink.Register(sink)

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (f *HandlerFunc) WithClose(fn func() error) *HandlerFunc
```

WithClose 设置关闭回调，返回自身以便链式调用。

```go
func (f *HandlerFunc) Name() string
```

Name 返回 Handler 名称。

```go
func (f *HandlerFunc) Write(entry *Entry) error
```

Write 执行日志写入。

```go
func (f *HandlerFunc) Close() error
```

Close 实现可选接口 Closer。

#### `Record`

```go
type Record struct
```

Record is an application log captured before asynchronous dispatch.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Data` | []byte | json:"-" | Data is the destination-specific formatted payload (without a trailing newline). |
| `Time` | string | json:"time" |  |
| `Level` | string | json:"level" |  |
| `Message` | string | json:"message" |  |
| `App` | string | json:"app" |  |
| `RequestID` | string | json:"request_id" |  |
| `GoroutineID` | uint64 | json:"goroutine_id" |  |
| `Package` | string | json:"package" |  |
| `File` | string | json:"file" |  |
| `Function` | string | json:"function" |  |
| `Line` | int | json:"line" |  |

#### `RecordHandler`

```go
type RecordHandler interface
```

RecordHandler receives application logs, independently of PostLog handlers.
Write must be concurrency safe and must not log back into the same sink.

| 方法/内嵌 | 签名 |
|-----------|------|
| `Name` | func() string |
| `Write` | func(*Record) error |

#### `RecordHandlerFunc`

```go
type RecordHandlerFunc struct
```

RecordHandlerFunc adapts a function to RecordHandler.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | string |  |  |
| `WriteFunc` | func(*Record) error |  |  |

**方法**

```go
func (h RecordHandlerFunc) Name() string
```

Name returns the destination name.

```go
func (h RecordHandlerFunc) Write(r *Record) error
```

Write invokes the configured application log callback.

```go
func (h *RecordHandlerFunc) WithClose(closeFn func() error) *RecordHandlerFunc
```

WithClose attaches a shutdown callback.

```go
func (h RecordHandlerFunc) Close() error
```

Close closes the destination after its queue drains.

#### `Entry`

```go
type Entry = models.PostLog
```

Entry 一条接口访问日志。

#### `Handler`

```go
type Handler interface
```

Handler 日志处理器。由内置实现或外部插件实现，注册后即可收到全量接口日志。

Name 用于注册去重、日志标识以及运行期注销，必须唯一且非空。
Write 在独立协程中被调用，实现必须是并发安全的；返回 error 仅用于统计与告警。

| 方法/内嵌 | 签名 |
|-----------|------|
| `Name` | func() string |
| `Write` | func(entry *Entry) error |

#### `Closer`

```go
type Closer interface
```

Closer 可选接口。实现了它的 Handler 在注销或全局关闭时会被调用一次。

| 方法/内嵌 | 签名 |
|-----------|------|
| `Close` | func() error |

#### `Options`

```go
type Options struct
```

Options 注册参数。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `QueueSize` | int |  | QueueSize 缓冲队列长度，<=0 使用 DefaultQueueSize。 |
| `Workers` | int |  | Workers 保留兼容字段，统一使用单协程以保证 FIFO。 |

#### `Option`

```go
type Option func(*Options)
```

Option 注册时的可选配置。

#### `HandlerStats`

```go
type HandlerStats struct
```

HandlerStats 单个 Handler 的运行状态。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Name` | string |  |  |
| `Queued` | int |  |  |
| `Dropped` | uint64 |  |  |
| `Written` | uint64 |  |  |
| `Failed` | uint64 |  |  |

### 函数

#### `Close`

```go
func Close(timeout time.Duration)
```

Close 停止全部 Handler：停止接收新日志、尽力排空队列、调用 Closer。
timeout 为等待每个 Handler 排空的时间预算，超时则放弃剩余日志。

#### `Emit`

```go
func Emit(entry *Entry) bool
```

Emit 异步投递一条日志到所有 Handler。
非阻塞：队列满时淘汰最早的待写日志并计数。返回是否至少投递成功一个 Handler。

#### `EmitRecord`

```go
func EmitRecord(entry *Record) bool
```

EmitRecord 异步投递一条日志到所有 Handler。
非阻塞：队列满时淘汰最早的待写日志并计数。返回是否至少投递成功一个 Handler。

#### `EmitRecordTo`

```go
func EmitRecordTo(name string, entry *Record) bool
```

EmitRecordTo sends a record only to the named application log handler.
The record and its Data must remain immutable after dispatch.

#### `HandlerNames`

```go
func HandlerNames() []string
```

HandlerNames 返回已注册 Handler 的名称，按注册顺序排列。

#### `HasHandlers`

```go
func HasHandlers() bool
```

HasHandlers 是否已注册任何 Handler。用于快速跳过日志投递。

#### `HasRecordHandlers`

```go
func HasRecordHandlers() bool
```

HasRecordHandlers 是否已注册任何 Handler。用于快速跳过日志投递。

#### `MustRegister`

```go
func MustRegister(h Handler, opts ...Option)
```

MustRegister 同 Register，注册失败时 panic。适用于启动阶段装配。

#### `MustRegisterRecord`

```go
func MustRegisterRecord(h RecordHandler, opts ...Option)
```

MustRegisterRecord 同 RegisterRecord，注册失败时 panic。适用于启动阶段装配。

#### `NewHandlerFunc`

```go
func NewHandlerFunc(name string, write func(entry *Entry) error) *HandlerFunc
```

NewHandlerFunc 用名称与写函数构造 Handler。

#### `NewRecordHandlerFunc`

```go
func NewRecordHandlerFunc(name string, write func(*Record) error) *RecordHandlerFunc
```

NewRecordHandlerFunc adapts a write callback to an application log handler.

#### `RecordHandlerNames`

```go
func RecordHandlerNames() []string
```

RecordHandlerNames 返回已注册 Handler 的名称，按注册顺序排列。

#### `RecordStats`

```go
func RecordStats() []HandlerStats
```

RecordStats 返回全部 Handler 的运行状态，按注册顺序排列。

#### `Register`

```go
func Register(h Handler, opts ...Option) error
```

Register 注册一个日志处理器并立即启动其消费协程。
同名 Handler 重复注册返回 ErrDuplicateHandler，原 Handler 保持不变。

#### `RegisterRecord`

```go
func RegisterRecord(h RecordHandler, opts ...Option) error
```

RegisterRecord 注册一个日志处理器并立即启动其消费协程。
同名 Handler 重复注册返回 ErrDuplicateHandler，原 Handler 保持不变。

#### `Reset`

```go
func Reset()
```

Reset 清空全部 Handler（不等待排空、不调用 Closer），仅用于测试。

#### `SetDropLogger`

```go
func SetDropLogger(fn func(handler string, dropped uint64))
```

SetDropLogger 设置日志丢弃时的回调，参数为 handler 名称与该 handler 累计丢弃条数。
回调已做 60 秒限流。默认不输出任何内容，由 MGIN 内部接管为 logs 包输出。

#### `SetErrorLogger`

```go
func SetErrorLogger(fn func(handler string, err error))
```

SetErrorLogger 设置 Handler 写失败或 panic 时的回调。
回调已做 60 秒限流。默认不输出任何内容。

#### `Stats`

```go
func Stats() []HandlerStats
```

Stats 返回全部 Handler 的运行状态，按注册顺序排列。

#### `Unregister`

```go
func Unregister(name string, timeout time.Duration) bool
```

Unregister 注销并停止指定 Handler，等待其队列排空（受 timeout 限制），
若 Handler 实现了 Closer 则调用 Close。

#### `UnregisterRecord`

```go
func UnregisterRecord(name string, timeout time.Duration) bool
```

UnregisterRecord 注销并停止指定 Handler，等待其队列排空（受 timeout 限制），
若 Handler 实现了 Closer 则调用 Close。

#### `WithQueueSize`

```go
func WithQueueSize(n int) Option
```

WithQueueSize 设置缓冲队列长度，最大 100。

#### `WithWorkers`

```go
func WithWorkers(n int) Option
```

WithWorkers 保留兼容；消费协程数固定为 1，保证 FIFO。

