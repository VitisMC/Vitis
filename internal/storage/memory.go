package storage

import "sync"

// MemoryKV is a thread-safe in-memory key-value store.
type MemoryKV struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryKV creates a new in-memory KV store.
func NewMemoryKV() *MemoryKV {
	return &MemoryKV{data: make(map[string][]byte)}
}

func (m *MemoryKV) Get(key string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, true
}

func (m *MemoryKV) Set(key string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[key] = cp
}

func (m *MemoryKV) Delete(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[key]
	delete(m.data, key)
	return ok
}

func (m *MemoryKV) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok
}

func (m *MemoryKV) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

func (m *MemoryKV) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *MemoryKV) Close() error { return nil }
