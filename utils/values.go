package utils

import "sync"

type Values struct {
	data map[string]any
	lock sync.RWMutex
}

func (p *Values) Put(id string, val any) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.data == nil {
		p.data = make(map[string]any)
	}
	p.data[id] = val
}

func (p *Values) Get(id string) any {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.data[id]
}

// GetAll 返回内部 map 的浅拷贝，避免外部并发写入导致 concurrent map read and map write
func (p *Values) GetAll() any {
	p.lock.RLock()
	defer p.lock.RUnlock()
	cp := make(map[string]any, len(p.data))
	for k, v := range p.data {
		cp[k] = v
	}
	return cp
}

func (p *Values) Merge(props map[string]any) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.data == nil {
		p.data = make(map[string]any)
	}

	for k, v := range props {
		p.data[k] = v
	}
}

func (p *Values) Clear() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.data = make(map[string]any)
}
