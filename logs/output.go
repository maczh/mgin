package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logsink"
	"github.com/maczh/mgin/middleware/trace"
)

var outputMu sync.Mutex

// FormatRecord serializes the selected fields for a destination. An empty field
// list includes all fields; text templates use {field} placeholders.
func FormatRecord(r *logsink.Record, output config.LogOutput) ([]byte, error) {
	values := map[string]any{"time": r.Time, "level": r.Level, "message": r.Message, "app": r.App, "request_id": r.RequestID, "goroutine_id": r.GoroutineID, "package": r.Package, "file": r.File, "function": r.Function, "line": r.Line}
	fields := output.Fields
	if len(fields) == 0 {
		fields = []string{"time", "level", "app", "request_id", "goroutine_id", "package", "file", "function", "line", "message"}
	}
	selected := make(map[string]any, len(fields))
	for _, key := range fields {
		value, ok := values[key]
		if !ok {
			return nil, fmt.Errorf("unknown log field %q", key)
		}
		selected[key] = value
	}
	switch output.Format {
	case "json":
		return json.Marshal(selected)
	case "", "text":
		if output.LogText != "" {
			pairs := make([]string, 0, len(values)*2)
			for key, value := range values {
				text := ""
				if _, ok := selected[key]; ok {
					text = fmt.Sprint(value)
				}
				pairs = append(pairs, "{"+key+"}", text)
			}
			return []byte(strings.NewReplacer(pairs...).Replace(output.LogText)), nil
		}
		parts := make([]string, 0, len(fields))
		for _, key := range fields {
			parts = append(parts, fmt.Sprint(selected[key]))
		}
		return []byte(strings.Join(parts, " ")), nil
	default:
		return nil, fmt.Errorf("unknown log format %q", output.Format)
	}
}

func captureRecord(level, message string) *logsink.Record {
	r := &logsink.Record{Time: time.Now().Format(time.RFC3339Nano), Level: level, Message: message, App: config.Config.App.Name, RequestID: trace.GetRequestId(), GoroutineID: trace.GetGoroutineID()}
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "github.com/maczh/mgin/logs.") || strings.HasSuffix(frame.File, "_test.go") {
			r.File = filepath.Base(frame.File)
			r.Function = frame.Function
			r.Line = frame.Line
			if i := strings.LastIndex(frame.Function, "."); i >= 0 {
				r.Package = frame.Function[:i]
			}
			break
		}
		if !more {
			break
		}
	}
	return r
}

func writeRecord(r *logsink.Record, outputs []config.LogOutput) {
	for _, out := range outputs {
		data, err := FormatRecord(r, out)
		if err != nil {
			reportOutputError(err)
			continue
		}
		switch out.Target {
		case "console":
			outputMu.Lock()
			_, err = fmt.Fprintln(Output, string(data))
			outputMu.Unlock()
		case "file":
			name := out.File
			if name == "" {
				name = config.Config.Logger.File
			}
			if name == "" {
				reportOutputError(fmt.Errorf("file log target requires a file"))
				continue
			}
			name += "." + time.Now().Format("2006-01-02") + ".log"
			outputMu.Lock()
			err = os.MkdirAll(filepath.Dir(name), 0755)
			if err == nil {
				var f *os.File
				f, err = os.OpenFile(name, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
				if err == nil {
					_, err = f.Write(append(data, '\n'))
					closeErr := f.Close()
					if err == nil {
						err = closeErr
					}
				}
			}
			outputMu.Unlock()
		default:
			copyRecord := *r
			copyRecord.Data = append([]byte(nil), data...)
			logsink.EmitRecordTo(out.Target, &copyRecord)
		}
		if err != nil {
			reportOutputError(err)
		}
	}
}

// reportOutputError bypasses logsink so downstream failures cannot recurse.
func reportOutputError(err error) { fmt.Fprintln(os.Stderr, "mgin logs: "+err.Error()) }

func enabled(level string) bool {
	threshold := strings.ToLower(config.Config.Logger.Level)
	if threshold == "" {
		threshold = "error"
	}
	if config.Config.App.Debug {
		threshold = "debug"
	}
	ranks := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3}
	min, ok := ranks[threshold]
	if !ok {
		return false
	}
	return ranks[level] >= min
}

func emit(level, format string, args []any) {
	if !enabled(level) {
		return
	}
	message := formatMessage(format, args)
	if len(config.Config.Logger.Outputs) > 0 {
		writeRecord(captureRecord(level, message), config.Config.Logger.Outputs)
		return
	}
	// Keep legacy text formatting and destination defaults.
	l := GetLogger(strings.FieldsFunc(config.Config.Logger.Out, func(r rune) bool { return r == ',' })...)
	if id := trace.GetRequestId(); id != "" {
		message = "[" + id + "] " + message
	}
	switch level {
	case "debug":
		l.Debug(message)
	case "info":
		l.Info(message)
	case "warn":
		l.Warn(message)
	case "error":
		l.Error(message)
	}
}

// RegisterHandler attaches an external application log destination. Configure its
// name in go.logger.outputs.target. Queue defaults follow go.log.sink.
func RegisterHandler(h logsink.RecordHandler, opts ...logsink.Option) error {
	defaults := []logsink.Option{logsink.WithQueueSize(config.Config.Log.Sink.Queue), logsink.WithWorkers(config.Config.Log.Sink.Workers)}
	return logsink.RegisterRecord(h, append(defaults, opts...)...)
}
