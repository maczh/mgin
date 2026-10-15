package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoggerOutputsConfig(t *testing.T) {
	name := filepath.Join(t.TempDir(), "application.yml")
	data := []byte(`go:
  logger:
    level: info
    out: console
    outputs:
      - target: kafka
        format: json
        fields: [message, goroutine_id]
      - target: console
        format: text
        logtext: "{level}: {message}"
`)
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config{}
	cfg.Init(name)
	if len(cfg.Logger.Outputs) != 2 {
		t.Fatalf("outputs: %+v", cfg.Logger.Outputs)
	}
	if cfg.Logger.Outputs[0].Target != "kafka" || cfg.Logger.Outputs[0].Format != "json" || len(cfg.Logger.Outputs[0].Fields) != 2 || cfg.Logger.Outputs[1].LogText != "{level}: {message}" {
		t.Fatalf("incorrect output config: %+v", cfg.Logger.Outputs)
	}
	if err := os.WriteFile(name, []byte("go:\n  logger:\n    out: file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Init(name)
	if len(cfg.Logger.Outputs) != 0 || cfg.Logger.Out != "file" {
		t.Fatal("reinitialization retained old outputs")
	}
}
