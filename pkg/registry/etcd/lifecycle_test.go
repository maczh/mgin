package etcd

import (
	"context"
	"io"
	"net"
	"strconv"
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

func TestDeregisterWaitsForConcurrentRegisterLifecycle(t *testing.T) {
	client := &EtcdClient{}
	client.lifecycleMu.Lock() // simulates Register holding the lifecycle gate
	started, done := make(chan struct{}), make(chan struct{})
	utils.GoSafe(func() {
		close(started)
		client.DeRegister()
		close(done)
	})
	<-started
	select {
	case <-done:
		t.Fatal("DeRegister passed the lifecycle gate while Register was active")
	case <-time.After(20 * time.Millisecond):
	}
	client.lifecycleMu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("DeRegister did not continue after Register released lifecycle gate")
	}
}

func TestDeregisterCancelsNetworkRegistrationWithoutWaitingForTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	utils.GoSafe(func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer conn.Close()
			close(accepted)
			_, _ = io.Copy(io.Discard, conn)
		}
	})
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	client := &EtcdClient{}
	utils.GoSafe(func() {
		client.Register([]byte("go:\n  etcd:\n    server: 127.0.0.1\n    port: " + port + "\n    group: test\n"))
	})
	select {
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("Etcd registration did not reach the fake endpoint")
	}
	deregistered := make(chan struct{})
	utils.GoSafe(func() {
		client.DeRegister()
		close(deregistered)
	})
	select {
	case <-deregistered:
	case <-time.After(3 * time.Second):
		t.Fatal("DeRegister did not cancel in-flight registration")
	}
	if client.client != nil || client.conf != nil {
		t.Fatal("failed registration state was not reset after deregistration")
	}
}
