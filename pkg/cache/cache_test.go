package cache

import "testing"

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
