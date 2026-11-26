package logs

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logsink"
)

func TestVictoriaLogsOTLPAndBasicAuth(t *testing.T) {
	oldLog, oldLogger, oldName := config.Config.Log, config.Config.Logger, config.Config.App.Name
	defer func() {
		config.Config.Log, config.Config.Logger, config.Config.App.Name = oldLog, oldLogger, oldName
		logsink.Reset()
	}()
	logsink.Reset()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret"))
		if r.URL.Path != "/insert/opentelemetry/v1/logs" || r.Header.Get("Authorization") != want || r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Error("incorrect request")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			t.Errorf("invalid protobuf body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	config.Config.App.Name = "test-app"
	config.Config.Logger.Outputs = []config.LogOutput{{Target: VictoriaLogsTarget}}
	config.Config.Log.VictoriaLogs = config.VictoriaLogsConfig{URL: server.URL, HTTPUser: "alice", HTTPPass: "secret"}
	if err := InitVictoriaLogs(); err != nil {
		t.Fatal(err)
	}
	logsink.EmitRecordTo(VictoriaLogsTarget, &logsink.Record{Level: "info", Message: "basic auth"})
	logsink.Close(3 * time.Second)
}
