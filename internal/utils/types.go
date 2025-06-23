package utils

import (
	"sync"
	"time"

	"github.com/samber/lo"
)

// Int Map
type SafeIntMap struct {
	mu    sync.Mutex
	Value map[string]int
}

func NewSafeIntMap() *SafeIntMap {
	return &SafeIntMap{Value: make(map[string]int)}
}
func (rx *SafeIntMap) Inc(key string) int {
	rx.mu.Lock()
	rx.Value[key]++
	val := rx.Value[key]
	rx.mu.Unlock()
	return val
}
func (rx *SafeIntMap) Val(key string) int {
	rx.mu.Lock()
	val := rx.Value[key]
	rx.mu.Unlock()
	return val
}

func (rx *SafeIntMap) Sum() int {
	rx.mu.Lock()
	sum := lo.Sum(lo.Values(rx.Value))
	rx.mu.Unlock()
	return sum
}

// Time Map
type SafeTimeMap struct {
	mu    sync.Mutex
	Value map[string]time.Time
}

func NewSafeTimeMap() *SafeTimeMap {
	return &SafeTimeMap{Value: make(map[string]time.Time)}
}
func (rx *SafeTimeMap) Set(key string, t time.Time) {
	rx.mu.Lock()
	rx.Value[key] = t
	rx.mu.Unlock()
}
