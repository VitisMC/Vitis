package chunk

import "sync"

const defaultStorageCapacity = 2048

// Storage is a world-thread-owned chunk map optimized for fast lookups.
type Storage struct {
	mu     sync.RWMutex
	chunks map[int64]*Chunk
}

// NewStorage creates chunk storage with optional initial capacity.
func NewStorage(initialCapacity int) *Storage {
	if initialCapacity <= 0 {
		initialCapacity = defaultStorageCapacity
	}
	return &Storage{chunks: make(map[int64]*Chunk, initialCapacity)}
}

// Get returns a chunk by coordinates.
func (s *Storage) Get(x, z int32) (*Chunk, bool) {
	if s == nil {
		return nil, false
	}
	return s.GetByKey(ChunkKey(x, z))
}

// GetByKey returns a chunk by packed key.
func (s *Storage) GetByKey(key int64) (*Chunk, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	chunk, ok := s.chunks[key]
	s.mu.RUnlock()
	return chunk, ok
}

// Set stores a chunk by its coordinates.
func (s *Storage) Set(chunk *Chunk) {
	if s == nil || chunk == nil {
		return
	}
	s.mu.Lock()
	s.chunks[chunk.Key()] = chunk
	s.mu.Unlock()
}

// SetByKey stores a chunk by packed key.
func (s *Storage) SetByKey(key int64, chunk *Chunk) {
	if s == nil || chunk == nil {
		return
	}
	s.mu.Lock()
	s.chunks[key] = chunk
	s.mu.Unlock()
}

// Delete removes a chunk by coordinates.
func (s *Storage) Delete(x, z int32) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.chunks, ChunkKey(x, z))
	s.mu.Unlock()
}

// DeleteByKey removes a chunk by packed key.
func (s *Storage) DeleteByKey(key int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.chunks, key)
	s.mu.Unlock()
}

// Len returns currently stored chunk count.
func (s *Storage) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	length := len(s.chunks)
	s.mu.RUnlock()
	return length
}

// Keys appends all stored keys into dst and returns the resulting slice.
func (s *Storage) Keys(dst []int64) []int64 {
	if s == nil {
		return dst
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	dst = dst[:0]
	for key := range s.chunks {
		dst = append(dst, key)
	}
	return dst
}
