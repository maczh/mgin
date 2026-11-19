package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maczh/mgin/registry"
)

func resetResponseCache() {
	responseCache.Lock()
	responseCache.values = make(map[string]cacheEntry)
	responseCache.Unlock()
}

func cacheSize() int {
	responseCache.Lock()
	defer responseCache.Unlock()
	return len(responseCache.values)
}

func useTestServer(t *testing.T, newClient func(host string) registry.RegistryClient, handler http.HandlerFunc) {
	t.Helper()
	old := registry.Registry
	server := httptest.NewServer(handler)
	registry.Registry = newClient(server.URL)
	circuits.Lock()
	circuits.values = make(map[string]*circuitState)
	circuits.Unlock()
	t.Cleanup(func() {
		registry.Registry = old
		server.Close()
	})
}

func plainRegistry(host string) registry.RegistryClient { return testRegistry{host: host} }

func groupedRegistry(host string) registry.RegistryClient {
	return groupRegistry{testRegistry{host: host}}
}

type groupRegistry struct {
	testRegistry
}

func (r groupRegistry) GetServiceURL(_ string, group ...string) (string, string) {
	if len(group) > 0 && group[0] != "" {
		return r.host, group[0]
	}
	return r.host, "default"
}

func cachedGet(token string) *Options {
	return &Options{
		Method:   http.MethodGet,
		Protocol: CONTENT_TYPE_JSON,
		Cache:    true,
		Header:   map[string]string{"Authorization": token},
	}
}

// A cached success must never be replayed to a different principal.
func TestCachedFallbackIsolatesIdentity(t *testing.T) {
	resetResponseCache()
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer A" {
			_, _ = w.Write([]byte("private-of-A"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})

	if got, err := Call("svc", "/private", cachedGet("Bearer A")); err != nil || got != "private-of-A" {
		t.Fatalf("seed call got=%q err=%v", got, err)
	}
	if size := cacheSize(); size != 1 {
		t.Fatalf("expected exactly one cached entry, got %d", size)
	}

	degraded := false
	opB := cachedGet("Bearer B")
	opB.OnFallback = func(error) { degraded = true }
	got, err := Call("svc", "/private", opB)
	if err == nil {
		t.Fatalf("expected failure for a different principal, got %q", got)
	}
	if got != "" || degraded {
		t.Fatalf("cross-identity fallback leaked data: value=%q degraded=%v", got, degraded)
	}
	if size := cacheSize(); size != 1 {
		t.Fatalf("failed call must not add cache entries, got %d", size)
	}
}

// A 403 must fail even when the caller supplied a fallback.
func TestAuthRejectionSkipsFallback(t *testing.T) {
	resetResponseCache()
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	called := false
	op := &Options{Method: http.MethodGet, Protocol: CONTENT_TYPE_JSON, Header: map[string]string{"Authorization": "Bearer A"}}
	op.Fallback = func(error) (string, bool) { called = true; return "stale", true }
	got, err := Call("svc", "/private", op)
	if err == nil || got != "" {
		t.Fatalf("expected failure, got=%q err=%v", got, err)
	}
	if called {
		t.Fatal("fallback must not run on authorization rejection")
	}
}

// Authorization rejections must not open the breaker for other callers.
func TestAuthRejectionDoesNotTripCircuit(t *testing.T) {
	resetResponseCache()
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer B" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	for i := 0; i < circuitFailureThreshold; i++ {
		if _, err := Call("svc", "/item", cachedGet("Bearer A")); err == nil {
			t.Fatalf("attempt %d should fail", i)
		}
	}
	got, err := Call("svc", "/item", cachedGet("Bearer B"))
	if err != nil || got != "ok" {
		t.Fatalf("breaker opened for a different caller: got=%q err=%v", got, err)
	}
}

// Server failures may degrade to the same caller's cached response.
func TestCachedFallbackServesSameIdentity(t *testing.T) {
	resetResponseCache()
	var failing atomic.Bool
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("cached-A"))
	})

	if got, err := Call("svc", "/item", cachedGet("Bearer A")); err != nil || got != "cached-A" {
		t.Fatalf("seed got=%q err=%v", got, err)
	}
	failing.Store(true)

	var degradedErr error
	op := cachedGet("Bearer A")
	op.OnFallback = func(err error) { degradedErr = err }
	got, err := Call("svc", "/item", op)
	if err != nil || got != "cached-A" {
		t.Fatalf("degraded call got=%q err=%v", got, err)
	}
	if degradedErr == nil {
		t.Fatal("OnFallback must report the degraded call")
	}
}

// A registry outage may degrade to the cached response for the same caller.
func TestCachedFallbackOnDiscoveryFailure(t *testing.T) {
	resetResponseCache()
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("cached"))
	})
	if got, err := Call("svc", "/item", cachedGet("Bearer A")); err != nil || got != "cached" {
		t.Fatalf("seed got=%q err=%v", got, err)
	}
	registry.Registry = testRegistry{} // simulate a registry outage

	degraded := false
	op := cachedGet("Bearer A")
	op.OnFallback = func(error) { degraded = true }
	got, err := Call("svc", "/item", op)
	if err != nil || got != "cached" || !degraded {
		t.Fatalf("got=%q err=%v degraded=%v", got, err, degraded)
	}
}

// Caching is opt-in: without Cache or config no entry is stored.
func TestCacheIsOptIn(t *testing.T) {
	resetResponseCache()
	var hits atomic.Int32
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("ok"))
	})
	op := func() *Options { return &Options{Method: http.MethodGet, Protocol: CONTENT_TYPE_JSON} }
	for i := 0; i < 2; i++ {
		if _, err := Call("svc", "/item", op()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if hits.Load() != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", hits.Load())
	}
}

// Expired entries must not be replayed.
func TestCacheTTLExpires(t *testing.T) {
	resetResponseCache()
	var failing atomic.Bool
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("cached"))
	})
	seed := &Options{Method: http.MethodGet, Protocol: CONTENT_TYPE_JSON, Cache: true, CacheTTL: 20 * time.Millisecond}
	if _, err := Call("svc", "/item", seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	failing.Store(true)
	op := &Options{Method: http.MethodGet, Protocol: CONTENT_TYPE_JSON, Cache: true, CacheTTL: 20 * time.Millisecond}
	op.OnFallback = func(error) { t.Fatal("expired entry must not be replayed") }
	if _, err := Call("svc", "/item", op); err == nil {
		t.Fatal("expected failure after ttl expiry")
	}
}

// The cache must stay bounded and evict the oldest entry.
func TestCacheCapacityEviction(t *testing.T) {
	resetResponseCache()
	old := responseCacheCapacity
	responseCacheCapacity = 2
	t.Cleanup(func() { responseCacheCapacity = old; resetResponseCache() })
	for _, key := range []string{"a", "b", "c"} {
		cachePut(key, key, time.Minute)
	}
	responseCache.Lock()
	size := len(responseCache.values)
	_, hasA := responseCache.values["a"]
	_, hasC := responseCache.values["c"]
	responseCache.Unlock()
	if size != 2 || hasA || !hasC {
		t.Fatalf("size=%d hasA=%v hasC=%v", size, hasA, hasC)
	}
}

// Method and group are part of the cache key.
func TestCachedFallbackVariesByMethodAndGroup(t *testing.T) {
	resetResponseCache()
	var failing atomic.Bool
	useTestServer(t, groupedRegistry, func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("cached"))
	})
	seed := &Options{Method: http.MethodGet, Protocol: CONTENT_TYPE_JSON, Cache: true, Group: "g1"}
	if _, err := Call("svc", "/item", seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	failing.Store(true)

	for _, op := range []*Options{
		{Method: http.MethodPost, Protocol: CONTENT_TYPE_JSON, Cache: true, Group: "g1"},
		{Method: http.MethodGet, Protocol: CONTENT_TYPE_JSON, Cache: true, Group: "g2"},
	} {
		current := op
		current.OnFallback = func(error) { t.Fatalf("fallback must miss for %s/%s", current.Method, current.Group) }
		if _, err := Call("svc", "/item", current); err == nil {
			t.Fatalf("expected failure for %s/%s", current.Method, current.Group)
		}
	}
}

// The key must differ for every isolation dimension and stay stable otherwise.
func TestCacheKeyIsolationDimensions(t *testing.T) {
	base := func() *Options {
		return &Options{
			Method:   http.MethodGet,
			Protocol: CONTENT_TYPE_JSON,
			Group:    "g1",
			Cache:    true,
			Header:   map[string]string{"Authorization": "Bearer A", "X-Tenant-Id": "t1"},
		}
	}
	key := func(op *Options) string { return op.responseCacheKey("svc", "/item") }

	baseline := key(base())
	if again := key(base()); again != baseline {
		t.Fatal("identical requests must produce identical keys")
	}
	if strings.Contains(baseline, "Bearer") || strings.Contains(baseline, "t1") {
		t.Fatalf("cache key must not embed raw credentials: %q", baseline)
	}

	mutations := map[string]func(*Options){
		"token":  func(op *Options) { op.Header = map[string]string{"Authorization": "Bearer B", "X-Tenant-Id": "t1"} },
		"tenant": func(op *Options) { op.Header = map[string]string{"Authorization": "Bearer A", "X-Tenant-Id": "t2"} },
		"user":   func(op *Options) { op.Header = map[string]string{"Authorization": "Bearer A", "X-User-Id": "u9"} },
		"scope":  func(op *Options) { op.CacheScope = "tenant=t1;perm=v2" },
		"group":  func(op *Options) { op.Group = "g2" },
		"method": func(op *Options) { op.Method = http.MethodPost },
		"query":  func(op *Options) { op.Query = map[string]string{"page": "2"} },
		"path":   func(op *Options) { op.Path = map[string]string{"id": "9"} },
	}
	for name, mutate := range mutations {
		op := base()
		mutate(op)
		if key(op) == baseline {
			t.Fatalf("%s must change the key", name)
		}
	}
	if base().responseCacheKey("svc", "/other") == baseline {
		t.Fatal("uri must change the key")
	}
	if base().responseCacheKey("other-svc", "/item") == baseline {
		t.Fatal("service must change the key")
	}
}

// Header casing must not fragment or confuse the identity.
func TestCacheKeyHeaderCasing(t *testing.T) {
	upper := &Options{Method: http.MethodGet, Group: "g1", Header: map[string]string{"Authorization": "Bearer A"}}
	lower := &Options{Method: http.MethodGet, Group: "g1", Header: map[string]string{"authorization": "Bearer A"}}
	if upper.responseCacheKey("svc", "/item") != lower.responseCacheKey("svc", "/item") {
		t.Fatal("header casing must not change the cache key")
	}
}

// Two tenants sharing a token must stay isolated through CacheScope.
func TestCachedFallbackIsolatesCacheScope(t *testing.T) {
	resetResponseCache()
	var failing atomic.Bool
	useTestServer(t, plainRegistry, func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("tenant-t1"))
	})
	seed := cachedGet("Bearer shared")
	seed.CacheScope = "tenant=t1"
	if got, err := Call("svc", "/item", seed); err != nil || got != "tenant-t1" {
		t.Fatalf("seed got=%q err=%v", got, err)
	}
	failing.Store(true)

	degraded := false
	op := cachedGet("Bearer shared")
	op.CacheScope = "tenant=t2"
	op.OnFallback = func(error) { degraded = true }
	if got, err := Call("svc", "/item", op); err == nil || got != "" || degraded {
		t.Fatalf("cross-tenant leak: got=%q err=%v degraded=%v", got, err, degraded)
	}
}
