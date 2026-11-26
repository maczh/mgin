package cache

import "testing"

func TestNilAndZeroValueMemCacheAreSafe(t *testing.T) {
	var nilCache *MemCache
	if _, ok := nilCache.Get("x"); ok {
		t.Fatal("nil cache unexpectedly hit")
	}
	nilCache.Add("x", 1, 0)
	nilCache.Set("x", 1, 0)
	nilCache.Delete("x")
	nilCache.Range(nil)
	nilCache.Close()

	var zeroValue MemCache
	zeroValue.Close()
	zeroValue.Close()
}
