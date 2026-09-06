package utils

import (
	"fmt"
	"sync"
)

type linkNode[T any] struct {
	prev, next *linkNode[T]
	value      T
}

type LinkList[T any] struct {
	head, tail *linkNode[T]
	size       int
	lock       sync.RWMutex
}

func (l *LinkList[T]) Add(v T, num int) {
	if num < 0 {
		return
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	// 超出链表长度时保持原有语义：不做插入
	if num > l.size {
		return
	}
	// 追加到末尾
	if num == l.size {
		l.pushLocked(v)
		return
	}

	node := linkNode[T]{value: v, prev: nil, next: nil}
	l.size++

	if num == 0 {
		node.next = l.head
		if l.head != nil {
			l.head.prev = &node
		}
		l.head = &node
		if l.tail == nil {
			l.tail = &node
		}
		return
	}

	next := l.head
	for i := 1; i < num && next != nil; i++ {
		next = next.next
	}
	if next == nil {
		l.pushLocked(v)
		l.size--
		return
	}

	node.prev = next
	node.next = next.next
	if next.next != nil {
		next.next.prev = &node
	}
	next.next = &node

	if node.next == nil {
		l.tail = &node
	}
}

func (l *LinkList[T]) Remove(index int) {
	if index < 0 {
		return
	}

	if index == 0 {
		l.Pop()
		return
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	// 越界时保持原有语义：退化为移除头节点
	if index >= l.size {
		l.dequeueLocked()
		return
	}

	ptr := l.head
	for i := 1; i < index && ptr != nil; i++ {
		ptr = ptr.next
	}
	if ptr == nil || ptr.next == nil {
		return
	}

	target := ptr.next
	if target.next != nil {
		target.next.prev = ptr
	}
	ptr.next = target.next
	if target == l.tail {
		l.tail = ptr
	}
	l.size--
}

// pushLocked 在已持有写锁的前提下插入尾部节点
func (l *LinkList[T]) pushLocked(v T) {
	node := &linkNode[T]{value: v}
	if l.head == nil {
		l.head = node
		l.tail = node
	} else {
		node.prev = l.tail
		l.tail.next = node
		l.tail = node
	}
	l.size++
}

// dequeueLocked 在已持有写锁的前提下移除头部节点
func (l *LinkList[T]) dequeueLocked() {
	if l.head == nil {
		return
	}
	if l.head == l.tail {
		l.head = nil
		l.tail = nil
	} else {
		l.head = l.head.next
		l.head.prev = nil
	}
	l.size--
}

func (l *LinkList[T]) Push(v T) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.pushLocked(v)
}

func (l *LinkList[T]) Pop() {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.tail == nil {
		return
	}

	if l.tail.prev == nil {
		l.tail = nil
		l.head = nil
	} else {
		l.tail = l.tail.prev
		l.tail.next = nil
	}
	l.size--
}

func (l *LinkList[T]) Enqueue(v T) {
	l.Push(v)
}

func (l *LinkList[T]) Dequeue() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.dequeueLocked()
}

func (l *LinkList[T]) Size() int {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.size
}

func (l *LinkList[T]) Get(index int) T {
	var zero T
	if index < 0 {
		panic(fmt.Errorf("链表越界访问 index:%d", index))
	}

	l.lock.RLock()
	defer l.lock.RUnlock()

	if l.size <= index {
		panic(fmt.Errorf("链表越界访问 index:%d size:%d", index, l.size))
	}

	ptr := l.head
	for i := 1; i <= index && ptr != nil; i++ {
		ptr = ptr.next
	}
	if ptr == nil {
		return zero
	}

	return ptr.value
}

func (l *LinkList[T]) GetAll() []T {
	l.lock.RLock()
	defer l.lock.RUnlock()

	if l.size == 0 || l.head == nil {
		return make([]T, 0)
	}

	items := make([]T, 0, l.size)
	for node := l.head; node != nil; node = node.next {
		items = append(items, node.value)
	}

	return items
}

func (l *LinkList[T]) Walk(fn func(v T) bool) {
	if fn == nil {
		return
	}

	l.lock.RLock()
	defer l.lock.RUnlock()

	for node := l.head; node != nil; node = node.next {
		if !fn(node.value) {
			return
		}
	}
}
