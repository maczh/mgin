package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestStopWithContextCancelsAndWaitsForJobGoroutines(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	manager := &Manager{running: true, stopChan: make(chan struct{}), runCtx: runCtx, runCancel: cancel}
	manager.wg.Add(1)
	utils.GoSafe(func() {
		defer manager.wg.Done()
		<-runCtx.Done()
	})
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := manager.StopWithContext(ctx); err != nil {
		t.Fatalf("StopWithContext: %v", err)
	}
	if manager.IsRunning() {
		t.Fatal("manager reports running after successful stop")
	}
}

func TestStopWithContextTimeoutLeavesManagerStoppingUntilWorkExits(t *testing.T) {
	manager := &Manager{running: true, stopChan: make(chan struct{})}
	release := make(chan struct{})
	manager.handlerWG.Add(1)
	utils.GoSafe(func() {
		defer manager.handlerWG.Done()
		<-release
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := manager.StopWithContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("StopWithContext error=%v, want deadline exceeded", err)
	}
	if !manager.stopping || !manager.running {
		t.Fatal("manager must remain in stopping state while work is still active")
	}
	close(release)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if err := manager.StopWithContext(ctx2); err != nil {
		t.Fatalf("second StopWithContext: %v", err)
	}
	if manager.running || manager.stopping {
		t.Fatal("manager state was not finalized after work exited")
	}
}

func TestStoreInitializationDoesNotHoldManagerStateLock(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	manager := &Manager{
		cfg: &Config{},
		storeFactory: func(string) (*store, error) {
			close(started)
			<-release
			return &store{}, nil
		},
	}
	result := make(chan error, 1)
	utils.GoSafe(func() {
		_, err := manager.StoreWithError()
		result <- err
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("store factory did not start")
	}
	stateRead := make(chan struct{})
	utils.GoSafe(func() {
		_ = manager.IsRunning()
		close(stateRead)
	})
	select {
	case <-stateRead:
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("manager state read blocked behind store initialization")
	}
	close(release)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("StoreWithError did not finish")
	}
}

func TestJobLoopPanicIsReportedAsUnhealthy(t *testing.T) {
	manager := &Manager{running: true}
	manager.runLoopTask("schedule", func() { panic("scheduler invariant failed") })
	if err := manager.Check(); err == nil {
		t.Fatal("Check should report a recovered critical-loop panic")
	}
	manager.runLoopTask("schedule", func() {})
	if len(manager.loopErrors) != 0 {
		t.Fatal("successful subsequent loop iteration should clear prior loop panic")
	}
}
