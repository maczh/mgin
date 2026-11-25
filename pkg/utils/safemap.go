package utils

import "sync"

type Map[T any] struct {
	container map[string]*T
	lock      sync.RWMutex
}

func (c *Map[T]) Load(name string) *T {
	if c == nil {
		return nil
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	if c.container == nil {
		return nil
	}
	return c.container[name]
}

func (c *Map[T]) Store(name string, value *T) {
	if c == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.container == nil {
		c.container = make(map[string]*T)
	}
	c.container[name] = value
}

func (c *Map[T]) Range(iterator func(name string, item *T) bool) {
	if c == nil || iterator == nil {
		return
	}
	c.lock.RLock()
	items := make(map[string]*T, len(c.container))
	for k, v := range c.container {
		items[k] = v
	}
	c.lock.RUnlock()
	for k, v := range items {
		if !iterator(k, v) {
			break
		}
	}
}

func (c *Map[T]) Delete(name string) {
	if c == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.container == nil {
		return
	}
	delete(c.container, name)
}

func (c *Map[T]) DeleteDirectly(name string) {
	if c == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.container == nil {
		return
	}
	delete(c.container, name)
}

func (c *Map[T]) LoadAndStore(name string, value *T) *T {
	if c == nil {
		return nil
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.container == nil {
		c.container = make(map[string]*T)
	}
	ret := c.container[name]
	c.container[name] = value
	return ret
}

func (c *Map[T]) LoadAndDelete(name string) *T {
	if c == nil {
		return nil
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.container == nil {
		return nil
	}
	ret := c.container[name]
	delete(c.container, name)
	return ret
}

func (c *Map[T]) Len() int {
	if c == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.container)
}

func (c *Map[T]) Map() map[string]*T {
	if c == nil {
		return map[string]*T{}
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	copy := make(map[string]*T, len(c.container))
	for key, value := range c.container {
		copy[key] = value
	}
	return copy
}

func (c *Map[T]) Clear() {
	if c == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.container = nil
}
