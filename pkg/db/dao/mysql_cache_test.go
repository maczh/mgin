package dao

import (
	"testing"
	"time"

	cacheStore "github.com/maczh/mgin/v2/pkg/cache"
)

type cachedUser struct {
	ID   int
	Name string
}

func (cachedUser) TableName() string { return "users" }

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
		t.Fatal("expected a stable key for equivalent non-empty values")
	}
}

func TestMySQLDaoClearCacheOnlyClearsTable(t *testing.T) {
	store := cacheStore.OnMemCache("v2-dao-cache-invalidation-test")
	userDAO := (&MySQLDao[cachedUser]{}).WithCache(CacheConfig{Enabled: true, Store: store})
	otherDAO := (&MySQLDao[otherCachedEntity]{}).WithCache(CacheConfig{Enabled: true, Store: store})

	userKey, _ := userDAO.cacheKey("all", cachedUser{ID: 1})
	otherKey, _ := otherDAO.cacheKey("all", otherCachedEntity{ID: 1})
	store.Set(userKey, []byte(`[]`), time.Minute)
	store.Set(otherKey, []byte(`[]`), time.Minute)

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
