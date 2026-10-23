package cache

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.mills.io/prologic/bitcask"
	"github.com/sadlil/gologger"
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
	path, err := diskPath(cachePath)
	if err != nil {
		logger.Error(fmt.Sprintf("invalid cache path: %s", err.Error()))
		return nil
	}
	key := md5Encode(path)
	//if mc == nil {
	//	mc = new(Cache)
	//	mc.cache = make(map[string]*MemCache)
	//	mc.cacheType = make(map[string]string)
	//	mc.db = make(map[string]*DiskCache)
	//}
	if db, ok := mc.db.Load(key); ok {
		return db.(ICache)
	}
	db, err := bitcask.Open(path, bitcask.WithSync(true), bitcask.WithAutoRecovery(true))
	if err != nil {
		logger.Error(fmt.Sprintf("file %s open failed: %s", cachePath, err.Error()))
		return nil
	}
	diskCache := &DiskCache{db: db}
	actual, loaded := mc.db.LoadOrStore(key, diskCache)
	if loaded {
		_ = db.Close()
		return actual.(ICache)
	}
	mc.cacheType.Store(path, "disk")
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
	}
	c := ICache(New(time.Minute))
	actual, _ := mc.cache.LoadOrStore(cachename, c)
	if actual != c {
		c.Close()
	}
	return actual.(ICache)
}

func diskPath(cachePath string) (string, error) {
	base, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", err
	}
	if cachePath == "" {
		return filepath.Join(base, "cache.db"), nil
	}
	if filepath.IsAbs(cachePath) || strings.HasPrefix(cachePath, "/") || cachePath[1:2] == ":" {
		return filepath.Clean(cachePath), nil
	}
	return filepath.Join(base, cachePath), nil
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
