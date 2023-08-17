package cache

import (
	"crypto/md5"
	"fmt"
	"git.mills.io/prologic/bitcask"
	"github.com/sadlil/gologger"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Cache struct {
	cache     sync.Map
	cacheType sync.Map
	db        sync.Map
}

// An item represents arbitrary data with expiration time.
type item struct {
	data    any
	expires int64
}

var mc = &Cache{}
var logger = gologger.GetLogger()

func OnDiskCache(cachePath string) ICache {
	mc.cacheType.Store(cachePath, "disk")
	key := md5Encode(cachePath)
	//if mc == nil {
	//	mc = new(Cache)
	//	mc.cache = make(map[string]*MemCache)
	//	mc.cacheType = make(map[string]string)
	//	mc.db = make(map[string]*DiskCache)
	//}
	if db, ok := mc.db.Load(key); ok {
		return db.(ICache)
	}
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if cachePath == "" {
		cachePath = fmt.Sprintf("%s/cache.db", path)
	} else {
		if !(strings.HasPrefix(cachePath, "/") || cachePath[1:2] == ":") {
			cachePath = fmt.Sprintf("%s/%s", path, cachePath)
		}
	}
	db, err := bitcask.Open(cachePath, bitcask.WithSync(true), bitcask.WithAutoRecovery(true))
	if err != nil {
		logger.Error(fmt.Sprintf("file %s open failed: %s", cachePath, err.Error()))
		return nil
	}
	diskCache := &DiskCache{db: db}
	mc.db.Store(key, diskCache)
	return diskCache
}

/*
初始化一个cache
cachename 缓存名字
*/
func OnGetCache(cachename string, persistent ...bool) ICache {
	if len(persistent) > 0 && persistent[0] {
		return OnDiskCache(cachename)
	} else {
		return OnMemCache(cachename)
	}
}

func OnMemCache(cachename string) ICache {
	mc.cacheType.Store(cachename, "mem")
	//if mc == nil {
	//	mc = new(Cache)
	//	mc.cache = make(map[string]*MemCache)
	//	mc.cacheType = make(map[string]string)
	//	mc.db = make(map[string]*DiskCache)
	//}
	if c, ok := mc.cache.Load(cachename); ok {
		return c.(ICache)
	} else {
		c = new(MemCache)
		mc.cache.Store(cachename, c)
		return c.(ICache)
	}
}

func CloseCache() {
	if mc != nil {
		mc.db.Range(func(key, value any) bool {
			value.(ICache).Close()
			mc.db.Delete(key)
			return true
		})
		mc.cache.Range(func(key, value any) bool {
			value.(ICache).Close()
			mc.cache.Delete(key)
			return true
		})
	}
}

func md5Encode(content string) (md string) {
	h := md5.New()
	_, _ = io.WriteString(h, content)
	md = fmt.Sprintf("%x", h.Sum(nil))
	return
}
