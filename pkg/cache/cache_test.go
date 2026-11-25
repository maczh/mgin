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

func TestMemCacheRejectsUncomparableKeysWithoutPanic(t *testing.T) {
	cache := &MemCache{}
	if err := cache.SetWithError([]int{1}, "value", 0); err == nil {
		t.Fatal("expected uncomparable key error")
	}
	if err := cache.AddWithError(map[string]int{"x": 1}, "value", 0); err == nil {
		t.Fatal("expected AddWithError to reject an uncomparable key")
	}
	cache.Set([]int{1}, "value", 0)
	cache.Add(map[string]int{"x": 1}, "value", 0)
	cache.Delete([]int{1})
	if value, ok := cache.Get([]int{1}); ok || value != nil {
		t.Fatalf("Get(uncomparable)=(%v,%v), want (nil,false)", value, ok)
	}
	if err := cache.SetWithError("valid", "value", 0); err != nil {
		t.Fatalf("SetWithError(valid): %v", err)
	}
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
