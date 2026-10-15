package logs

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"context"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logsink"
	"github.com/maczh/mgin/v2/pkg/middleware/trace"
	"gorm.io/gorm/logger"
)

func TestFormatRecord(t *testing.T) {
	r := &logsink.Record{Level: "info", Message: "hello {level}\n世界", GoroutineID: 42, RequestID: "request"}
	for _, tc := range []struct {
		name string
		out  config.LogOutput
		want string
		bad  bool
	}{
		{"text fields", config.LogOutput{Format: "text", Fields: []string{"level", "goroutine_id"}}, "info 42", false},
		{"template", config.LogOutput{Fields: []string{"message", "goroutine_id"}, LogText: "[{goroutine_id}] {message} {request_id}"}, "[42] hello {level}\n世界 ", false},
		{"unknown field", config.LogOutput{Fields: []string{"missing"}}, "", true},
		{"unknown format", config.LogOutput{Format: "xml"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := FormatRecord(r, tc.out)
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v", err)
			}
			if !tc.bad && string(data) != tc.want {
				t.Fatalf("got %q want %q", data, tc.want)
			}
		})
	}
	data, err := FormatRecord(r, config.LogOutput{Format: "json", Fields: []string{"message", "goroutine_id"}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["message"] != r.Message || got["goroutine_id"] != float64(42) {
		t.Fatalf("unexpected JSON: %s", data)
	}
}

func TestConfiguredOutputsAndConcurrentLogging(t *testing.T) {
	old := config.Config.Logger
	oldDebug := config.Config.App.Debug
	defer func() { config.Config.Logger = old; config.Config.App.Debug = oldDebug; logsink.Reset() }()
	config.Config.App.Debug = false
	config.Config.Logger.Level = "info"
	config.Config.Logger.Outputs = []config.LogOutput{
		{Target: "json", Format: "json", Fields: []string{"message", "goroutine_id", "file"}},
		{Target: "text", Format: "text", LogText: "{level}: {message}", Fields: []string{"level", "message"}},
	}
	var mu sync.Mutex
	got := map[string][]*logsink.Record{}
	for _, name := range []string{"json", "text"} {
		name := name
		if err := logsink.RegisterRecord(logsink.RecordHandlerFunc{ID: name, WriteFunc: func(r *logsink.Record) error { mu.Lock(); got[name] = append(got[name], r); mu.Unlock(); return nil }}, logsink.WithWorkers(2)); err != nil {
			t.Fatal(err)
		}
	}
	Debug("filtered")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); Info("hello {}", "world") }()
	}
	wg.Wait()
	logsink.Close(time.Second)
	for _, name := range []string{"json", "text"} {
		if len(got[name]) != 20 {
			t.Fatalf("%s got %d", name, len(got[name]))
		}
	}
	r := got["json"][0]
	if r.GoroutineID == 0 || r.File != "output_test.go" || !strings.Contains(string(r.Data), `"message":"hello world"`) {
		t.Fatalf("bad record: %+v", r)
	}
	if string(got["text"][0].Data) != "info: hello world" {
		t.Fatalf("bad text: %s", got["text"][0].Data)
	}
}

func TestCaptureGoroutine(t *testing.T) {
	r := captureRecord("info", "hello")
	if r.GoroutineID != trace.GetGoroutineID() {
		t.Fatalf("wrong goroutine id %d", r.GoroutineID)
	}
}

func TestGORMLogModeAndLazyTrace(t *testing.T) {
	l := NewGORMLogger()
	other := l.LogMode(logger.Info).(*GORMLogger)
	if l.Level != logger.Warn || other.Level != logger.Info {
		t.Fatal("LogMode mutated original")
	}
	l.LogMode(logger.Silent).Trace(context.Background(), time.Now(), func() (string, int64) { t.Fatal("silent SQL evaluated"); return "", 0 }, nil)
	l.Trace(context.Background(), time.Now(), func() (string, int64) { t.Fatal("normal SQL evaluated at warn"); return "", 0 }, nil)
}

func TestGORMTraceRoutesThroughOutputs(t *testing.T) {
	old := config.Config.Logger
	oldDebug := config.Config.App.Debug
	defer func() { config.Config.Logger = old; config.Config.App.Debug = oldDebug; logsink.Reset() }()
	config.Config.App.Debug = false
	config.Config.Logger.Level = "info"
	config.Config.Logger.Outputs = []config.LogOutput{{Target: "gorm", Format: "json", Fields: []string{"level", "message", "goroutine_id"}}}
	var got []*logsink.Record
	if err := RegisterHandler(logsink.NewRecordHandlerFunc("gorm", func(r *logsink.Record) error { got = append(got, r); return nil })); err != nil {
		t.Fatal(err)
	}
	l := NewGORMLogger()
	l.Trace(context.Background(), time.Now().Add(-time.Second), func() (string, int64) { return "select 1", 1 }, nil)
	l.Error(context.Background(), "failure %s", "example")
	l.LogMode(logger.Info).Trace(context.Background(), time.Now(), func() (string, int64) { return "select 2", 2 }, nil)
	logsink.Close(time.Second)
	if len(got) != 3 {
		t.Fatalf("got %d logs", len(got))
	}
	for i, level := range []string{"warn", "error", "info"} {
		if got[i].Level != level || got[i].GoroutineID != trace.GetGoroutineID() {
			t.Fatalf("unexpected record %+v", got[i])
		}
	}
	if !strings.Contains(got[0].Message, "select 1") || !strings.Contains(got[1].Message, "failure example") {
		t.Fatalf("wrong messages: %+v", got)
	}
}
