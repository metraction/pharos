package utils

import (
	"sync"
	"time"
)

// Int Map
type SafeIntMap struct {
	mu    sync.Mutex
	Value map[string]int
}

func NewSafeIntMap() *SafeIntMap {
	return &SafeIntMap{Value: make(map[string]int)}
}
func (rx *SafeIntMap) Inc(key string) {
	rx.mu.Lock()
	rx.Value[key]++
	rx.mu.Unlock()
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
