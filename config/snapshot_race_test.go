package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func writeSnapshotConfig(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	body := "go:\n" +
		"  application:\n" +
		"    name: " + name + "\n" +
		"    project: audit\n" +
		"    port: 8080\n" +
		"  config:\n" +
		"    used: mysql,redis\n" +
		"  logger:\n" +
		"    level: info\n" +
		"  discovery:\n" +
		"    registries:\n" +
		"      - name: r1\n" +
		"        kind: nacos\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestSnapshotReadsAreRaceFree mirrors the audit probe: one goroutine reloads
// the configuration while others read every field through Current().
func TestSnapshotReadsAreRaceFree(t *testing.T) {
	dir := t.TempDir()
	a := writeSnapshotConfig(t, dir, "a.yml")
	b := writeSnapshotConfig(t, dir, "b.yml")

	const iterations = 100
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			Config.Init(a)
			Config.Init(b)
		}
		close(stop)
	}()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					s := Current()
					_ = s.App.Name
					_ = s.App.Debug
					_ = s.Config.Used
					_ = s.Logger.Outputs
					_ = s.Logger.Level
					_ = len(s.Discovery.Registries)
					_ = s.Sys.Swagger.Uri
				}
			}
		}()
	}
	wg.Wait()
}
