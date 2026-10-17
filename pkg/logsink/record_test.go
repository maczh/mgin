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

func TestRecordWorkersPreserveFIFO(t *testing.T) {
	Reset()
	defer Reset()
	got := []string{}
	if err := RegisterRecord(NewRecordHandlerFunc("workers", func(r *Record) error {
		got = append(got, r.Message)
		return nil
	}), WithWorkers(2)); err != nil {
		t.Fatal(err)
	}
	EmitRecord(&Record{Message: "first"})
	EmitRecord(&Record{Message: "second"})
	Close(time.Second)
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("order=%v", got)
	}
}
