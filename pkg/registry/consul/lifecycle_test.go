package consul

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestTTLUpdatePanicIsReportedAndDoesNotEscapeLoopBoundary(t *testing.T) {
	if err := safeUpdateTTL(func() error { panic("injected TTL panic") }); err == nil {
		t.Fatal("expected recovered TTL panic to be reported")
	}
	called := false
	if err := safeUpdateTTL(func() error { called = true; return nil }); err != nil {
		t.Fatalf("subsequent TTL update: %v", err)
	}
	if !called {
		t.Fatal("subsequent TTL operation was not executed")
	}
}

func TestDeRegisterWaitsForInflightTTLUpdate(t *testing.T) {
	updateStarted := make(chan struct{})
	allowUpdateReturn := make(chan struct{})
	updateFinished := make(chan struct{})
	deregistered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agent/check/update/mgin:test-instance":
			close(updateStarted)
			<-allowUpdateReturn
			close(updateFinished)
			w.WriteHeader(http.StatusOK)
		case "/v1/agent/service/deregister/test-instance":
			select {
			case <-updateFinished:
			default:
				t.Error("service deregistered before the in-flight TTL update completed")
			}
			close(deregistered)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	clientConfig := api.DefaultConfig()
	clientConfig.Address = strings.TrimPrefix(server.URL, "http://")
	client, err := newConsulAPIClient(clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	if clientConfig.HttpClient.Timeout != consulRequestTimeout {
		t.Fatalf("Consul HTTP timeout=%s, want %s", clientConfig.HttpClient.Timeout, consulRequestTimeout)
	}

	ttlCtx, cancelTTL := context.WithCancel(context.Background())
	consulClient := &ConsulClient{
		client:     client,
		instanceID: "test-instance",
		cancel:     cancelTTL,
	}
	consulClient.ttlWG.Add(1)
	utils.GoSafe(func() {
		defer consulClient.ttlWG.Done()
		consulClient.passTTLLoop(ttlCtx, "test-instance", client, time.Millisecond)
	})
	select {
	case <-updateStarted:
	case <-time.After(time.Second):
		t.Fatal("TTL worker did not start an update")
	}

	deregisterDone := make(chan struct{})
	utils.GoSafe(func() {
		consulClient.DeRegister()
		close(deregisterDone)
	})
	select {
	case <-deregisterDone:
		t.Fatal("DeRegister returned while TTL update was still in flight")
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-deregistered:
		t.Fatal("service deregistration started before the TTL update was released")
	default:
	}
	close(allowUpdateReturn)
	select {
	case <-deregisterDone:
	case <-time.After(time.Second):
		t.Fatal("DeRegister did not finish after the TTL update returned")
	}
	select {
	case <-deregistered:
	default:
		t.Fatal("service deregistration was not issued after the TTL worker exited")
	}
}
