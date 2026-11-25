package mgin

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestLegacyPluginCheckPanicDoesNotStopOtherChecks(t *testing.T) {
	var checked atomic.Int32
	app := &mgin{}
	app.registerLegacyPlugin("a-panic", legacyPlugin{CheckFunc: func() error { panic("check failed") }})
	app.registerLegacyPlugin("b-healthy", legacyPlugin{CheckFunc: func() error { checked.Add(1); return nil }})
	app.checkAll()
	if got := checked.Load(); got != 1 {
		t.Fatalf("healthy plugin checks=%d, want 1", got)
	}
}

func TestLegacyPluginRegistryConcurrentUseAndChecks(t *testing.T) {
	app := &mgin{}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				app.registerLegacyPlugin(string(rune('a'+worker))+string(rune('A'+i%26)), legacyPlugin{
					CheckFunc: func() error { return nil },
				})
				app.checkAll()
			}
		})
	}
	wg.Wait()
}
