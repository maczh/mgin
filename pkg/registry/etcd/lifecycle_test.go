package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestStopKeepAliveWaitsForWorkerBeforeDeregistration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &EtcdClient{cancel: cancel}
	started, canceled, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	client.keepAliveWG.Add(1)
	utils.GoSafe(func() {
		defer client.keepAliveWG.Done()
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
	})
	<-started
	utils.GoSafe(func() {
		client.stopKeepAlive()
		close(stopped)
	})
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("keepalive worker did not receive cancellation")
	}
	select {
	case <-stopped:
		t.Fatal("stopKeepAlive returned before the worker exited")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stopKeepAlive did not return after worker exit")
	}
}

func TestEtcdKeepAliveSupervisorRestartsAfterPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	calls := 0
	utils.GoSafe(func() {
		defer close(done)
		superviseEtcdKeepAlive(ctx, time.Millisecond, func(ctx context.Context) {
			calls++
			if calls == 1 {
				panic("injected keepalive panic")
			}
			cancel()
		})
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("keepalive supervisor did not exit after cancellation")
	}
	if calls != 2 {
		t.Fatalf("keepalive sessions=%d, want 2 after panic restart", calls)
	}
}
