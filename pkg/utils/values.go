package utils

import "sync"

type Values struct {
	data map[string]any
	lock sync.RWMutex
}

func (p *Values) Put(id string, val any) {
	if p == nil {
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.data == nil {
		p.data = make(map[string]any)
	}
	p.data[id] = val
}

func (p *Values) Get(id string) any {
	if p == nil {
		return nil
	}
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.data[id]
}

func (p *Values) GetAll() any {
	if p == nil {
		return map[string]any{}
	}
	p.lock.RLock()
	defer p.lock.RUnlock()

	data := make(map[string]any, len(p.data))
	for key, value := range p.data {
		data[key] = value
	}
	return data
}

func (p *Values) Merge(props map[string]any) {
	if p == nil {
		return
	}
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
	if p == nil {
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.data = make(map[string]any)
}
