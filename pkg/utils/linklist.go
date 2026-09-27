package utils

import (
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

// Add 在指定下标处插入节点。
// num < 0 或 num > Size() 时不做插入（不再 panic）。
// num == Size() 时追加到末尾。
func (l *LinkList[T]) Add(v T, num int) {
	if num < 0 {
		return
	}

	// 必须先加锁再读 l.size，否则与 Push/Pop/Remove 中的 l.size 写入产生数据竞争
	l.lock.Lock()
	defer l.lock.Unlock()

	// 超出链表长度时保持原有语义：不做插入
	if num > l.size {
		return
	}
	// 追加到末尾：使用已持锁的 pushLocked，避免调用 Push 造成二次加锁自锁
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
	// 链表实际长度不足时退化为追加，避免后续空指针解引用
	if next == nil {
		l.pushLocked(v)
		l.size--
		return
	}

	node.prev = next
	node.next = next.next
	// num <= 0 且链表只有 1 个节点时 next.next 为 nil，必须判空后再赋值（否则空指针崩溃）
	if next.next != nil {
		next.next.prev = &node
	}
	next.next = &node

	if node.next == nil {
		l.tail = &node
	}
}

// Remove 移除指定下标的节点。
// 下标越界（index < 0 或 index >= Size()）时不做任何操作，不会 panic 也不会误删其他节点。
// 注意：index == 0 沿用历史行为，等价于 Pop（移除尾节点）。
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

	// 越界（含原实现中 index == size 走 Dequeue 的情况）时不做任何删除，
	// 避免误删头节点；原实现在 index == 1 时会因 ptr.prev 为 nil 触发空指针崩溃。
	if index >= l.size {
		return
	}

	// 定位待删除节点的前驱，随后摘除前驱的后继节点
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

	node := &linkNode[T]{value: v}
	if l.head == nil {
		l.head = node
		l.tail = l.head
	} else {
		node.prev = l.tail
		l.tail.next = node
		l.tail = node
	}
	l.size++
}

func (l *LinkList[T]) Pop() {
	// 必须先加锁再读 l.tail，否则与 Push/pushLocked 中的 l.tail 写入产生数据竞争（与 jh 保持一致）
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
	// 必须先加锁再读 l.head，否则与 pushLocked 中的 l.head 写入产生数据竞争（与 jh 保持一致）
	l.lock.Lock()
	defer l.lock.Unlock()
	l.dequeueLocked()
}

func (l *LinkList[T]) Size() int {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.size
}

// Get 返回指定下标的节点值。
// 下标越界（index < 0 或 index >= Size()）时不再 panic，而是返回 T 的零值。
// 如需区分「越界」与「元素值恰好是零值」，请改用 TryGet。
func (l *LinkList[T]) Get(index int) T {
	value, _ := l.TryGet(index)
	return value
}

// TryGet 尝试返回指定下标的节点值。
// 第二个返回值表示下标是否有效：false 表示越界，此时第一个返回值为 T 的零值。
// 该函数不会 panic，调用方可以显式处理越界场景。
func (l *LinkList[T]) TryGet(index int) (T, bool) {
	var zero T
	if index < 0 {
		return zero, false
	}

	l.lock.RLock()
	defer l.lock.RUnlock()

	if index >= l.size || l.head == nil {
		return zero, false
	}

	ptr := l.head
	for i := 0; i < index; i++ {
		if ptr.next == nil {
			return zero, false
		}
		ptr = ptr.next
	}
	if ptr == nil {
		return zero, false
	}

	return ptr.value, true
}

func (l *LinkList[T]) GetAll() []T {
	// 必须先加锁再读 l.size，否则与 Push/Remove 中的 l.size 写入产生数据竞争（与 jh 保持一致）
	l.lock.RLock()
	defer l.lock.RUnlock()

	if l.size == 0 || l.head == nil {
		return make([]T, 0)
	}

	// 使用 append 而非按索引写入，避免链表实际长度小于 l.size 时下标越界panic
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

	// 必须先加锁再读 l.size，否则与 Push/Remove 中的 l.size 写入产生数据竞争（与 jh 保持一致）
	l.lock.RLock()
	defer l.lock.RUnlock()

	for node := l.head; node != nil; node = node.next {
		if !fn(node.value) {
			return
		}
	}
}
