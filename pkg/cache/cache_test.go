package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/maczh/mgin/v2/internal/safego"
)

func TestMemCacheHandlesInvalidLifecycleInputs(t *testing.T) {
	cache := New(0)
	cache.Set("key", "value", 0)
	if value, ok := cache.Get("key"); !ok || value != "value" {
		t.Fatalf("Get() = (%v, %v), want (value, true)", value, ok)
	}
	cache.Range(nil)
	cache.Close()
	cache.Close()
}

func TestMemCacheHandlesCorruptStoredValue(t *testing.T) {
	cache := &MemCache{}
	cache.items.Store("key", "invalid")
	if value, ok := cache.Get("key"); ok || value != nil {
		t.Fatalf("Get() = (%v, %v), want (nil, false)", value, ok)
	}
}

func TestConcurrentAddValueClearAndClose(t *testing.T) {
	c := New(time.Hour)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		wg.Add(1)
		safego.Run(func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := worker*100 + i
				c.Add(key, i, 0)
				_, _ = c.Value(key)
				if i%10 == 0 {
					c.Clear()
				}
			}
		})
	}
	c.Close()
	wg.Wait()
	c.Close()
}
