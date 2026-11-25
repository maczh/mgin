package utils_test

import (
	"sync"
	"testing"

	"github.com/maczh/mgin/v2/pkg/utils"
)

func TestValuesGetAllReturnsIndependentSnapshot(t *testing.T) {
	values := &utils.Values{}
	values.Put("one", 1)
	snapshot, ok := values.GetAll().(map[string]any)
	if !ok {
		t.Fatalf("GetAll type=%T, want map[string]any", values.GetAll())
	}
	snapshot["one"] = 2
	snapshot["two"] = 2
	if values.Get("one") != 1 || values.Get("two") != nil {
		t.Fatal("mutating GetAll result changed Values")
	}
}

func TestValuesConcurrentAccess(t *testing.T) {
	values := &utils.Values{}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				values.Put("key", i)
				values.Merge(map[string]any{"worker": worker})
				_ = values.Get("key")
				_ = values.GetAll()
				if i%20 == 0 {
					values.Clear()
				}
			}
		})
	}
	wg.Wait()
}
