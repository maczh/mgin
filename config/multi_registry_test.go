package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitParsesNamedRegistries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.yml")
	data := []byte(`go:
  discovery:
    defaultRegistry: public
    registries:
      - name: internal
        type: etcd
        prefix: etcd-internal
      - name: public
        type: nacos
        prefix: nacos-public
        namespace: finance
        group: bank_account
        clusterName: ledger
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config{}
	cfg.Init(path)
	if cfg.Discovery.DefaultRegistry != "public" || len(cfg.Discovery.Registries) != 2 {
		t.Fatalf("discovery config = %+v", cfg.Discovery)
	}
	if got := cfg.Discovery.Registries[1]; got.Name != "public" || got.Type != "nacos" || got.Prefix != "nacos-public" || got.Namespace != "finance" || got.Group != "bank_account" || got.ClusterName != "ledger" {
		t.Fatalf("registry config = %+v", got)
	}
}
