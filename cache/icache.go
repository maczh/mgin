package cache

import "time"

type ICache interface {
	Add(key any, value any, lifeSpan time.Duration)
	Value(key any) (any, bool)
	IsExist(key any) bool
	Clear() bool
	Get(key any) (any, bool)
	Set(key any, value any, duration time.Duration)
	Range(f func(key, value any) bool)
	Delete(key any)
	Close()
}
