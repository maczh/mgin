package mgin

import (
	"sync"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
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
	started, release := make(chan struct{}), make(chan struct{})
	wg.Add(1)
	utils.GoSafe(func() {
		defer wg.Done()
		close(started)
		<-release
	})
	<-started
	result := make(chan bool, 1)
	utils.GoSafe(func() { result <- waitForHealthChecks(&wg, time.Second) })
	select {
	case <-result:
		t.Fatal("wait returned while health check was still active")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case waited := <-result:
		if !waited {
			t.Fatal("expected wait to return after health check completed")
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not return after health check completed")
	}
}

func TestAppRunCanOnlyStartOnce(t *testing.T) {
	app := &App{}
	if !app.beginRun() {
		t.Fatal("first Run attempt should be accepted")
	}
	if app.beginRun() {
		t.Fatal("second Run attempt should be rejected")
	}
}
