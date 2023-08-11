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
	"time"
)

type Cache struct {
	cache     map[string]*MemCache
	cacheType map[string]string
	db        map[string]*DiskCache
}

// An item represents arbitrary data with expiration time.
type item struct {
	data    any
	expires int64
}

var mc = &Cache{
	cache:     make(map[string]*MemCache),
	cacheType: make(map[string]string),
	db:        make(map[string]*DiskCache),
}
var logger = gologger.GetLogger()

func OnDiskCache(cachePath string) ICache {
	mc.cacheType[cachePath] = "disk"
	key := md5Encode(cachePath)
	//if mc == nil {
	//	mc = new(Cache)
	//	mc.cache = make(map[string]*MemCache)
	//	mc.cacheType = make(map[string]string)
	//	mc.db = make(map[string]*DiskCache)
	//}
	if db, ok := mc.db[key]; ok {
		return db
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
	mc.db[key] = diskCache
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
	mc.cacheType[cachename] = "mem"
	//if mc == nil {
	//	mc = new(Cache)
	//	mc.cache = make(map[string]*MemCache)
	//	mc.cacheType = make(map[string]string)
	//	mc.db = make(map[string]*DiskCache)
	//}
	if mc.cache[cachename] == nil {
		mc.cache[cachename] = New(time.Minute)
	}
	return mc.cache[cachename]
}

func CloseCache() {
	if mc != nil && len(mc.db) > 0 {
		for k, db := range mc.db {
			db.Close()
			delete(mc.db, k)
		}
	}
}

func md5Encode(content string) (md string) {
	h := md5.New()
	_, _ = io.WriteString(h, content)
	md = fmt.Sprintf("%x", h.Sum(nil))
	return
}
