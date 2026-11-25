package mgin

import (
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestWaitForHealthChecksIsBounded(t *testing.T) {
	done := make(chan struct{})
	started := time.Now()
	if waitForHealthChecks(done, 20*time.Millisecond) {
		t.Fatal("expected blocked health check wait to time out")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("shutdown wait exceeded bound: %s", elapsed)
	}
}

func TestWaitForHealthChecksReturnsWhenDone(t *testing.T) {
	done := make(chan struct{})
	started, release := make(chan struct{}), make(chan struct{})
	utils.GoSafe(func() {
		defer close(done)
		close(started)
		<-release
	})
	<-started
	result := make(chan bool, 1)
	utils.GoSafe(func() { result <- waitForHealthChecks(done, time.Second) })
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

func TestWaitForAppShutdownReturnsListenerFailure(t *testing.T) {
	serverErrors := make(chan error, 1)
	want := errors.New("address already in use")
	serverErrors <- want
	sig, err := waitForAppShutdown(make(chan os.Signal), serverErrors)
	if sig != nil || !errors.Is(err, want) {
		t.Fatalf("got signal=%v, err=%v; want listener error %v", sig, err, want)
	}
}

func TestShutdownServersForceClosesAndReportsUndrainedHandler(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered, release, handlerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		close(handlerDone)
		w.WriteHeader(http.StatusOK)
	})}
	utils.GoSafe(func() { _ = server.Serve(listener) })
	requestDone := make(chan struct{})
	utils.GoSafe(func() {
		defer close(requestDone)
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr == nil {
			_ = response.Body.Close()
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("request handler did not start")
	}
	if shutdownServers([]*http.Server{server}, 20*time.Millisecond) {
		close(release)
		t.Fatal("shutdown should report the blocked handler as undrained")
	}
	select {
	case <-handlerDone:
		t.Fatal("handler unexpectedly exited before its release")
	default:
	}
	close(release)
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not exit after release")
	}
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("client request did not finish after forced close")
	}
}
