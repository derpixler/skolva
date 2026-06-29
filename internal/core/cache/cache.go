// Package cache provides a key/value cache seam. The default implementation is
// in-memory with optional TTL; a Redis backend can implement Cache later.
package cache

import (
	"context"
	"sync"
	"time"
)

// Cache is a byte-oriented key/value store with optional per-entry TTL.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type entry struct {
	val []byte
	exp time.Time // zero => no expiry
}

// Memory is the default in-memory Cache. Safe for concurrent use.
type Memory struct {
	mu    sync.Mutex
	items map[string]entry
	now   func() time.Time
}

// NewMemory returns an empty in-memory cache.
func NewMemory() *Memory {
	return &Memory{items: make(map[string]entry), now: time.Now}
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[key]
	if !ok {
		return nil, false, nil
	}
	if !e.exp.IsZero() && m.now().After(e.exp) {
		delete(m.items, key)
		return nil, false, nil
	}
	out := make([]byte, len(e.val))
	copy(out, e.val)
	return out, true, nil
}

func (m *Memory) Set(_ context.Context, key string, val []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = m.now().Add(ttl)
	}
	cp := make([]byte, len(val))
	copy(cp, val)
	m.items[key] = entry{val: cp, exp: exp}
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}
