package utils

import "sync"

type RingBuffer[T any] struct {
	Size   int32
	Reader int32
	Writer int32
	Buffer []T
	mu     sync.Mutex
	count  int
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
	if size < 1 {
		size = 1
	}
	rb := &RingBuffer[T]{
		Size:   int32(size),
		Buffer: make([]T, size),
	}
	return rb
}

func (r *RingBuffer[T]) Write(value T) {
	if r == nil || len(r.Buffer) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.normalizeIndexesLocked()
	r.Buffer[r.Writer] = value
	r.Writer = (r.Writer + 1) % int32(len(r.Buffer))
	if r.count < len(r.Buffer) {
		r.count++
	} else {
		r.Reader = (r.Reader + 1) % int32(len(r.Buffer))
	}
}

func (r *RingBuffer[T]) seekReader(delta int32) {
	if len(r.Buffer) == 0 {
		return
	}
	r.Reader = (r.Reader + delta) % int32(len(r.Buffer))
}

func (r *RingBuffer[T]) Read() T {
	if r == nil || len(r.Buffer) == 0 {
		var zero T
		return zero
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.normalizeIndexesLocked()
	if r.count == 0 {
		var zero T
		return zero
	}
	value := r.Buffer[r.Reader]
	r.seekReader(1)
	r.count--
	return value
}

func (r *RingBuffer[T]) Latest() T {
	if r == nil || len(r.Buffer) == 0 {
		var zero T
		return zero
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.normalizeIndexesLocked()
	if r.count == 0 {
		var zero T
		return zero
	}
	idx := r.Writer - 1
	if idx < 0 {
		idx = int32(len(r.Buffer)) - 1
	}
	return r.Buffer[idx]
}

func (r *RingBuffer[T]) Oldest() T {
	if r == nil || len(r.Buffer) == 0 {
		var zero T
		return zero
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.normalizeIndexesLocked()
	if r.count == 0 {
		var zero T
		return zero
	}
	return r.Buffer[r.Reader]
}

func (r *RingBuffer[T]) Overwrite(v T) {
	if r == nil || len(r.Buffer) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.normalizeIndexesLocked()
	r.Buffer[r.Writer] = v
	if r.count < len(r.Buffer) {
		r.count++
	}
	r.Writer = (r.Writer + 1) % int32(len(r.Buffer))
}

func (r *RingBuffer[T]) normalizeIndexesLocked() {
	size := int32(len(r.Buffer))
	if size == 0 {
		r.Reader, r.Writer = 0, 0
		return
	}
	r.Reader %= size
	r.Writer %= size
	if r.Reader < 0 {
		r.Reader += size
	}
	if r.Writer < 0 {
		r.Writer += size
	}
}
