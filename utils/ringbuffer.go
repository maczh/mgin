package utils

import "sync/atomic"

type RingBuffer[T any] struct {
	Size   int32
	Reader int32
	Writer int32
	Buffer []T
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
	if size <= 0 {
		size = 1
	}
	rb := &RingBuffer[T]{
		Size:   int32(size),
		Buffer: make([]T, size),
	}
	return rb
}

func (r *RingBuffer[T]) Write(value T) {
	if r == nil || r.Size <= 0 || len(r.Buffer) == 0 {
		return
	}
	current := atomic.LoadInt32(&r.Writer)
	idx := current % r.Size
	if idx < 0 {
		idx = 0
	}
	r.Buffer[idx] = value
	next := (current + 1) % r.Size
	atomic.StoreInt32(&r.Writer, next)
}

func (r *RingBuffer[T]) seekReader(delta int32) {
	if r == nil || r.Size <= 0 {
		return
	}
	current := atomic.LoadInt32(&r.Reader)
	expected := (current + delta) % r.Size
	atomic.StoreInt32(&r.Reader, expected)
}

func (r *RingBuffer[T]) Read() T {
	if r == nil || r.Size <= 0 || len(r.Buffer) == 0 {
		var zero T
		return zero
	}
	defer r.seekReader(1)
	idx := atomic.LoadInt32(&r.Reader) % r.Size
	if idx < 0 {
		idx = 0
	}
	return r.Buffer[idx]
}

func (r *RingBuffer[T]) Latest() T {
	if r == nil || r.Size <= 0 || len(r.Buffer) == 0 {
		var zero T
		return zero
	}
	idx := atomic.LoadInt32(&r.Writer) - 1
	if idx < 0 {
		idx = r.Size - 1
	}
	idx = idx % r.Size
	return r.Buffer[idx]
}

func (r *RingBuffer[T]) Oldest() T {
	if r == nil || r.Size <= 0 || len(r.Buffer) == 0 {
		var zero T
		return zero
	}
	idx := atomic.LoadInt32(&r.Writer) % r.Size
	if idx < 0 {
		idx = 0
	}
	return r.Buffer[idx]
}

func (r *RingBuffer[T]) Overwrite(v T) {
	if r == nil || r.Size <= 0 || len(r.Buffer) == 0 {
		return
	}
	idx := atomic.LoadInt32(&r.Writer) % r.Size
	if idx < 0 {
		idx = 0
	}
	r.Buffer[idx] = v
}
