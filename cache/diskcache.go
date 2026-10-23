package cache

import (
	"sync"
	"time"

	"git.mills.io/prologic/bitcask"
)

type DiskCache struct {
	mu     sync.RWMutex
	db     *bitcask.Bitcask
	closed bool
}

func (d *DiskCache) Add(key any, value any, lifeSpan time.Duration) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return
	}
	k, err := anyToString(key)
	if err != nil {
		logger.Error("key to string failed: " + err.Error())
		return
	}
	v, err := anyToString(value)
	if err != nil {
		logger.Error("value to string failed: " + err.Error())
		return
	}
	if lifeSpan <= 0 {
		err := d.db.Put([]byte(k), []byte(v))
		if err != nil {
			logger.Error("保存入库错误:" + err.Error())
			return
		}
	} else {
		err = d.db.PutWithTTL([]byte(k), []byte(v), lifeSpan)
		if err != nil {
			logger.Error("保存入库错误:" + err.Error())
			return
		}
	}
}

func (d *DiskCache) Value(key any) (any, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, false
	}
	k, err := anyToString(key)
	if err != nil {
		logger.Error("key to string failed: " + err.Error())
		return nil, false
	}
	if d.db.Has([]byte(k)) {
		v, err := d.db.Get([]byte(k))
		if err != nil {
			return nil, false
		}
		value := string(v)
		return value, true
	}
	return nil, false
}

func (d *DiskCache) IsExist(key any) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return false
	}
	k, err := anyToString(key)
	if err != nil {
		logger.Error("key to string failed: " + err.Error())
		return false
	}
	return d.db.Has([]byte(k))
}

func (d *DiskCache) Clear() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return false
	}
	err := d.db.DeleteAll()
	if err != nil {
		return false
	}
	return true
}

func (d *DiskCache) Get(key any) (any, bool) {
	return d.Value(key)
}

func (d *DiskCache) Set(key any, value any, duration time.Duration) {
	d.Add(key, value, duration)
}

func (d *DiskCache) Range(f func(key, value any) bool) {
	//函数不同，暂不支持
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return
	}
	keys := make([]string, 0)
	for k := range d.db.Keys() {
		keys = append(keys, string(k))
	}
	d.mu.RUnlock()
	for _, key := range keys {
		v, ok := d.Value(key)
		if ok && !f(key, v) {
			return
		}
	}
}

func (d *DiskCache) Delete(key any) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return
	}
	k, err := anyToString(key)
	if err != nil {
		logger.Error("key to string failed: " + err.Error())
		return
	}
	d.db.Delete([]byte(k))
}

func (d *DiskCache) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.db.Sync()
	d.db.Close()
	d.closed = true
}
