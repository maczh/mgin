package dao

import (
	"testing"
	"time"

	cacheStore "github.com/maczh/mgin/cache"
)

type cachedUser struct {
	ID   int
	Name string
}

func (cachedUser) TableName() string { return "users" }

func TestMySQLDaoCacheKeyIncludesQueryCondition(t *testing.T) {
	dao := (&MySQLDao[cachedUser]{}).WithCache(CacheConfig{
		Enabled: true,
		Store:   cacheStore.OnMemCache("dao-cache-key-test"),
		TTL:     time.Minute,
	})

	first, ok := dao.cacheKey("one", cachedUser{ID: 1})
	if !ok {
		t.Fatal("expected a cache key")
	}
	second, ok := dao.cacheKey("one", cachedUser{ID: 2})
	if !ok {
		t.Fatal("expected a cache key")
	}
	if first == second {
		t.Fatal("different query conditions must produce different cache keys")
	}
}

func TestMySQLDaoCacheKeyIsStableForMapOrder(t *testing.T) {
	dao := (&MySQLDao[cachedUser]{})
	first, ok := dao.cacheKey("where", map[string]interface{}{
		"name": "alice",
		"age":  18,
	})
	if !ok {
		t.Fatal("expected a cache key")
	}
	second, ok := dao.cacheKey("where", map[string]interface{}{
		"age":  18,
		"name": "alice",
	})
	if !ok {
		t.Fatal("expected a cache key")
	}
	if first != second {
		t.Fatal("equivalent maps must produce the same cache key")
	}
}

func TestMySQLDaoCacheKeySortsAndOmitsEmptyValues(t *testing.T) {
	dao := &MySQLDao[cachedUser]{}
	first, ok := dao.cacheKey("where", map[string]interface{}{
		"name":  "alice",
		"age":   18,
		"empty": "",
	})
	if !ok {
		t.Fatal("expected a cache key")
	}
	second, ok := dao.cacheKey("where", map[string]interface{}{
		"age":  18,
		"name": "alice",
	})
	if !ok || first != second {
		t.Fatal("empty values must be excluded from the stable cache key")
	}
}

func TestMySQLDaoClearCacheOnlyClearsTable(t *testing.T) {
	store := cacheStore.OnMemCache("dao-cache-invalidation-test")
	userDAO := (&MySQLDao[cachedUser]{}).WithCache(CacheConfig{
		Enabled: true,
		Store:   store,
	})
	otherDAO := (&MySQLDao[otherCachedEntity]{}).WithCache(CacheConfig{
		Enabled: true,
		Store:   store,
	})

	userKey, _ := userDAO.cacheKey("all", cachedUser{ID: 1})
	otherKey, _ := otherDAO.cacheKey("all", otherCachedEntity{ID: 1})
	store.Set(userKey, []byte(`[]`), 0)
	store.Set(otherKey, []byte(`[]`), 0)

	userDAO.clearCache()
	if store.IsExist(userKey) {
		t.Fatal("expected user cache to be invalidated")
	}
	if !store.IsExist(otherKey) {
		t.Fatal("expected unrelated table cache to remain")
	}
}

type otherCachedEntity struct {
	ID int
}

func (otherCachedEntity) TableName() string { return "orders" }
