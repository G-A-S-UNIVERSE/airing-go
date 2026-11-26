package olt

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]func() BaseOLTAdapter{}
)

// Register adds a vendor adapter factory. Call from each vendor package's init().
func Register(key string, factory func() BaseOLTAdapter) {
	mu.Lock()
	defer mu.Unlock()
	registry[key] = factory
}

// New instantiates an adapter by vendor key.
func New(key string) (BaseOLTAdapter, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[key]
	if !ok {
		return nil, fmt.Errorf("olt: unknown vendor key %q", key)
	}
	return f(), nil
}

// Keys returns all registered vendor keys.
func Keys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	return keys
}
