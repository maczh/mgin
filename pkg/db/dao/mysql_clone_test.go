package dao

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestMySQLDaoFluentOptionsReturnIndependentCopies(t *testing.T) {
	base := &MySQLDao[cachedUser]{Tag: func() string { return "base" }}
	tx := &gorm.DB{}
	ctx := context.Background()
	cache := CacheConfig{Enabled: true}

	withTx := base.WithTx(tx)
	withContext := base.WithContext(&ctx)
	withCache := base.WithCache(cache)
	debug := base.Debug()
	if withTx == base || withContext == base || withCache == base || debug == base {
		t.Fatal("fluent option should return a DAO copy")
	}
	if base.tx != nil || base.ctx != nil || base.cache != nil || base.debug {
		t.Fatal("fluent options mutated the original DAO")
	}
	if withTx.tx != tx || withContext.ctx == nil || *withContext.ctx != ctx || withCache.cache == nil || !withCache.cache.Enabled || !debug.debug {
		t.Fatal("DAO copy did not retain requested option")
	}
	if withCache.cache == &cache {
		t.Fatal("WithCache should retain its own configuration copy")
	}
}

func TestMySQLDaoWithTxNilAndNilReceiver(t *testing.T) {
	base := &MySQLDao[cachedUser]{tx: &gorm.DB{}}
	if got := base.WithTx(nil); got == base || got.tx != nil || base.tx == nil {
		t.Fatal("WithTx(nil) should clear tx on a copy without mutating the receiver")
	}
	var missing *MySQLDao[cachedUser]
	if missing.WithTx(nil) != nil || missing.WithContext(nil) != nil || missing.WithCache(CacheConfig{}) != nil || missing.Debug() != nil {
		t.Fatal("nil receiver should safely return nil from fluent options")
	}
}

func TestRedisCacheNilReceiverAndNilRangeCallback(t *testing.T) {
	var missing *RedisCache
	missing.Add("key", "value", 0)
	if value, ok := missing.Value("key"); ok || value != nil {
		t.Fatalf("nil cache Value=(%v,%v), want (nil,false)", value, ok)
	}
	if missing.IsExist("key") || missing.Clear() {
		t.Fatal("nil cache should report no value and no successful clear")
	}
	if value, ok := missing.Get("key"); ok || value != nil {
		t.Fatalf("nil cache Get=(%v,%v), want (nil,false)", value, ok)
	}
	missing.Set("key", "value", 0)
	missing.Range(nil)
	missing.Delete("key")
	missing.Close()

	(&RedisCache{}).Range(nil)
}
