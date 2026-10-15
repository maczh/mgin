package logs_test

import (
	"encoding/json"
	"testing"
	"time"

	legacy "github.com/maczh/mgin/v2/logs"
	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/maczh/mgin/v2/pkg/logsink"
)

func TestCompatibilityEntrySharesConfiguredOutputs(t *testing.T) {
	oldLogger, oldDebug := config.Config.Logger, config.Config.App.Debug
	t.Cleanup(func() {
		logsink.Close(time.Second)
		config.Config.Logger, config.Config.App.Debug = oldLogger, oldDebug
	})
	config.Config.App.Debug = false
	config.Config.Logger.Level = "info"
	config.Config.Logger.Outputs = []config.LogOutput{{Target: "compat", Format: "json", Fields: []string{"message"}}}
	records := make(chan *logsink.Record, 2)
	if err := legacy.RegisterHandler(logsink.NewRecordHandlerFunc("compat", func(r *logsink.Record) error {
		records <- r
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	legacy.Info("legacy {}", "entry")
	logs.Info("pkg {}", "entry")
	logsink.Close(time.Second)
	for _, want := range []string{"legacy entry", "pkg entry"} {
		select {
		case r := <-records:
			var data map[string]string
			if err := json.Unmarshal(r.Data, &data); err != nil {
				t.Fatal(err)
			}
			if data["message"] != want || r.File != "alias_test.go" {
				t.Fatalf("unexpected record: %+v", r)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing output: %s", want)
		}
	}
}
