package logs

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logsink"
)

func TestVictoriaLogsTLSAndRouting(t *testing.T) {
	oldLog, oldLogger, oldName := config.Config.Log, config.Config.Logger, config.Config.App.Name
	defer func() {
		config.Config.Log = oldLog
		config.Config.Logger = oldLogger
		config.Config.App.Name = oldName
		logsink.Reset()
	}()
	logsink.Reset()
	var mu sync.Mutex
	events := []victoriaEvent{}
	retries := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/logs" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect request headers or URL")
		}
		var e victoriaEvent
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			t.Error(err)
		}
		mu.Lock()
		defer mu.Unlock()
		if retries == 0 {
			retries++
			w.WriteHeader(503)
			return
		}
		events = append(events, e)
		w.WriteHeader(202)
	}))
	defer server.Close()
	config.Config.App.Name = "test-app"
	config.Config.Logger.Outputs = []config.LogOutput{{Target: VictoriaLogsTarget}}
	config.Config.Log.VictoriaLogs = config.VictoriaLogsConfig{URL: server.URL, APIToken: "test-token",
		APICert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})), Request: true, Client: true}
	if err := InitVictoriaLogs(); err != nil {
		t.Fatal(err)
	}
	logsink.Emit(&logsink.Entry{Method: "GET", Uri: "/hello", RequestId: "request-1"})
	logsink.EmitRecordTo(VictoriaLogsTarget, &logsink.Record{Level: "warn", Message: "console hello"})
	logsink.EmitRecordTo(VictoriaLogsClientTarget, &logsink.Record{Level: "info", Message: "client hello"})
	logsink.Close(3 * time.Second)
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 3 {
		t.Fatalf("got %d events", len(events))
	}
	kinds := map[string]string{}
	for _, e := range events {
		kinds[e.Message] = e.Type
		if e.App != "test-app" {
			t.Errorf("app=%s", e.App)
		}
	}
	if kinds["GET /hello"] != "request" || kinds["console hello"] != "console" || kinds["client hello"] != "request" {
		t.Fatalf("routing=%v", kinds)
	}
	config.Config.Log.VictoriaLogs.APICert = "invalid"
	if err := InitVictoriaLogs(); err == nil {
		t.Fatal("invalid certificate accepted")
	}
}
