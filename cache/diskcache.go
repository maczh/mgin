package cache

import (
	"bytes"
	"encoding/binary"
	"git.mills.io/prologic/bitcask"
	"time"
)

type DiskCache struct {
	db *bitcask.Bitcask
}

func (d *DiskCache) Add(key any, value any, lifeSpan time.Duration) {
	keyBuf, valueBuf := new(bytes.Buffer), new(bytes.Buffer)
	binary.Write(keyBuf, binary.LittleEndian, key)
	binary.Write(valueBuf, binary.LittleEndian, value)
	if lifeSpan <= 0 {
		d.db.Put(keyBuf.Bytes(), valueBuf.Bytes())
	} else {
		d.db.PutWithTTL(keyBuf.Bytes(), valueBuf.Bytes(), lifeSpan)
	}
}

func (d *DiskCache) Value(key any) (any, bool) {
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.LittleEndian, key)
	if d.db.Has(keyBuf.Bytes()) {
		v, err := d.db.Get(keyBuf.Bytes())
		if err != nil {
			return nil, false
		}
		var value any
		binary.Read(bytes.NewReader(v), binary.LittleEndian, &value)
		return value, true
	}
	return nil, false
}

func (d *DiskCache) IsExist(key any) bool {
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.LittleEndian, key)
	return d.db.Has(keyBuf.Bytes())
}

func (d *DiskCache) Clear() bool {
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
	return
}

func (d *DiskCache) Delete(key any) {
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.LittleEndian, key)
	d.db.Delete(keyBuf.Bytes())
}

func (d *DiskCache) Close() {
	d.db.Sync()
	d.db.Close()
}
