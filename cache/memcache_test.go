package cache

import (
	"testing"
	"time"
)

func TestMemCacheClearPreservesCacheOperation(t *testing.T) {
	c := New(5 * time.Millisecond)
	t.Cleanup(c.Close)
	c.Set("old", "value", 0)
	if !c.Clear() {
		t.Fatal("Clear returned false")
	}
	if _, ok := c.Get("old"); ok {
		t.Fatal("Clear left old value readable")
	}
	c.Set("new", "value", 0)
	if got, ok := c.Get("new"); !ok || got != "value" {
		t.Fatalf("Set after Clear = %v, %v", got, ok)
	}
	c.Set("expires", "value", 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("expires"); ok {
		t.Fatal("expired value remained readable")
	}
	if _, ok := c.items.Load("expires"); ok {
		t.Fatal("cleaner did not reclaim expired value after Clear")
	}
}

func TestMemCacheCloseIsIdempotent(t *testing.T) {
	c := New(time.Hour)
	c.Close()
	c.Close()
	c.Set("after-close", "value", 0)
	if got, ok := c.Get("after-close"); !ok || got != "value" {
		t.Fatalf("Close unexpectedly disabled data access: %v %v", got, ok)
	}
}
