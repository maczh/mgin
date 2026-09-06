package utils

import (
	"log"
	"sync"
	"time"
)

type LimitQueeMap struct {
	sync.RWMutex
	LimitQueue map[string][]int64
}

func (l *LimitQueeMap) readMap(key string) ([]int64, bool) {
	l.RLock()
	defer l.RUnlock()
	value, ok := l.LimitQueue[key]
	return value, ok
}

func (l *LimitQueeMap) writeMap(key string, value []int64) {
	l.Lock()
	defer l.Unlock()
	if l.LimitQueue == nil {
		l.LimitQueue = make(map[string][]int64)
	}
	l.LimitQueue[key] = value
}

func (l *LimitQueeMap) reset() {
	l.Lock()
	defer l.Unlock()
	l.LimitQueue = make(map[string][]int64)
}

var LimitQueue = &LimitQueeMap{
	LimitQueue: make(map[string][]int64),
}

// limitQueueStart 保证清理协程只启动一次，避免重复调用 NewLimitQueue 造成 goroutine 泄漏
var limitQueueStart sync.Once

func NewLimitQueue() {
	limitQueueStart.Do(cleanLimitQueue)
}

func cleanLimitQueue() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("cleanLimitQueue panic: %v", r)
			}
		}()
		for {
			now := time.Now()
			next := now.Add(time.Hour * 24)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			t := time.NewTimer(next.Sub(now))
			<-t.C
			LimitQueue.reset()
			log.Println("cleanLimitQueue reset done")
		}
	}()
}

// tryEnqueue 在锁内原子完成"裁剪 + 判断是否放行 + 写入"，
// 避免 read-modify-write 竞态导致的限流失效与 slice 并发写。
func (l *LimitQueeMap) tryEnqueue(key string, currTime int64, count uint, timeWindow int64) bool {
	l.Lock()
	defer l.Unlock()
	if l.LimitQueue == nil {
		l.LimitQueue = make(map[string][]int64)
	}
	q := l.LimitQueue[key]
	// count 为 0 表示不限量放行，但不允许留下无法消费的队列
	if count == 0 {
		l.LimitQueue[key] = nil
		return true
	}
	// 丢弃滑窗外已过期的时间戳
	expired := 0
	for expired < len(q) && currTime-q[expired] > timeWindow {
		expired++
	}
	if expired > 0 {
		q = append(q[:0], q[expired:]...)
	}
	if uint(len(q)) < count {
		l.LimitQueue[key] = append(q, currTime)
		return true
	}
	// 队列已满且最早访问仍在时间窗口内，拒绝
	return false
}

// 单机时间滑动窗口限流法
func LimitFreqSingle(queueName string, count uint, timeWindow int64) bool {
	if queueName == "" {
		return true
	}
	currTime := time.Now().Unix()
	return LimitQueue.tryEnqueue(queueName, currTime, count, timeWindow)
}
