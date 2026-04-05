package chunk

const defaultStorageCapacity = 2048

// Storage is a world-thread-owned chunk map optimized for fast lookups.
type Storage struct {
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
	chunk, ok := s.chunks[key]
	return chunk, ok
}

// Set stores a chunk by its coordinates.
func (s *Storage) Set(chunk *Chunk) {
	if s == nil || chunk == nil {
		return
	}
	s.chunks[chunk.Key()] = chunk
}

// SetByKey stores a chunk by packed key.
func (s *Storage) SetByKey(key int64, chunk *Chunk) {
	if s == nil || chunk == nil {
		return
	}
	s.chunks[key] = chunk
}

// Delete removes a chunk by coordinates.
func (s *Storage) Delete(x, z int32) {
	if s == nil {
		return
	}
	delete(s.chunks, ChunkKey(x, z))
}

// DeleteByKey removes a chunk by packed key.
func (s *Storage) DeleteByKey(key int64) {
	if s == nil {
		return
	}
	delete(s.chunks, key)
}

// Len returns currently stored chunk count.
func (s *Storage) Len() int {
	if s == nil {
		return 0
	}
	return len(s.chunks)
}

// Keys appends all stored keys into dst and returns the resulting slice.
func (s *Storage) Keys(dst []int64) []int64 {
	if s == nil {
		return dst
	}
	dst = dst[:0]
	for key := range s.chunks {
		dst = append(dst, key)
	}
	return dst
}
