package logsink

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRecordAndPostLogIsolation(t *testing.T) {
	Reset()
	defer Reset()
	var records, requests, closed atomic.Int64
	if err := Register(NewHandlerFunc("same", func(*Entry) error { requests.Add(1); return nil })); err != nil {
		t.Fatal(err)
	}
	if err := RegisterRecord(NewRecordHandlerFunc("same", func(*Record) error { records.Add(1); return nil }).WithClose(func() error { closed.Add(1); return nil })); err != nil {
		t.Fatal(err)
	}
	Emit(newEntry("/hello"))
	EmitRecordTo("same", &Record{Message: "hello"})
	EmitRecord(&Record{Message: "world"})
	if EmitRecordTo("missing", &Record{}) || EmitRecordTo("same", nil) {
		t.Fatal("invalid target accepted")
	}
	Close(time.Second)
	if records.Load() != 2 || requests.Load() != 1 || closed.Load() != 1 {
		t.Fatalf("records=%d requests=%d closed=%d", records.Load(), requests.Load(), closed.Load())
	}
	if HasRecordHandlers() || EmitRecord(&Record{}) {
		t.Fatal("record registry remains open")
	}
}

func TestRecordWorkersRunConcurrently(t *testing.T) {
	Reset()
	defer Reset()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	if err := RegisterRecord(NewRecordHandlerFunc("workers", func(*Record) error { started <- struct{}{}; <-release; return nil }), WithWorkers(2)); err != nil {
		t.Fatal(err)
	}
	EmitRecord(&Record{})
	EmitRecord(&Record{})
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("configured workers did not run concurrently")
		}
	}
}
