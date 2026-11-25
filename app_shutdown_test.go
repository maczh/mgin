package mgin

import (
	"sync"
	"testing"
	"time"
)

func TestWaitForHealthChecksIsBounded(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	started := time.Now()
	if waitForHealthChecks(&wg, 20*time.Millisecond) {
		t.Fatal("expected blocked health check wait to time out")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("shutdown wait exceeded bound: %s", elapsed)
	}
	wg.Done()
}

func TestWaitForHealthChecksReturnsWhenDone(t *testing.T) {
	var wg sync.WaitGroup
	if !waitForHealthChecks(&wg, time.Second) {
		t.Fatal("expected completed health checks to return immediately")
	}
}
