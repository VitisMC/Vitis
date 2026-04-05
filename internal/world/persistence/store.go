package persistence

import (
	"fmt"
	"os"
	"sync"

	"github.com/vitismc/vitis/internal/world/region"
)

// ChunkStore manages reading/writing chunks to Anvil region files on disk.
type ChunkStore struct {
	worldDir string
	mu       sync.Mutex
	regions  map[uint64]*region.Region
}

// NewChunkStore creates a chunk store for the given world directory.
func NewChunkStore(worldDir string) *ChunkStore {
	return &ChunkStore{
		worldDir: worldDir,
		regions:  make(map[uint64]*region.Region),
	}
}

// ReadChunkNBT reads raw NBT data for a chunk at absolute chunk coordinates.
func (cs *ChunkStore) ReadChunkNBT(cx, cz int) ([]byte, error) {
	r, err := cs.getOrOpenRegion(cx, cz)
	if err != nil {
		return nil, err
	}
	lx, lz := region.ChunkInRegion(cx, cz)
	return r.ReadChunkNBT(lx, lz)
}

// WriteChunkNBT writes raw NBT data for a chunk at absolute chunk coordinates.
func (cs *ChunkStore) WriteChunkNBT(cx, cz int, data []byte) error {
	r, err := cs.getOrCreateRegion(cx, cz)
	if err != nil {
		return err
	}
	lx, lz := region.ChunkInRegion(cx, cz)
	return r.WriteChunkNBT(lx, lz, data)
}

// HasChunk checks if a chunk exists on disk.
func (cs *ChunkStore) HasChunk(cx, cz int) bool {
	r, err := cs.getOrOpenRegion(cx, cz)
	if err != nil {
		return false
	}
	lx, lz := region.ChunkInRegion(cx, cz)
	return r.HasChunk(lx, lz)
}

// Close closes all open region files.
func (cs *ChunkStore) Close() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	var lastErr error
	for _, r := range cs.regions {
		if err := r.Close(); err != nil {
			lastErr = err
		}
	}
	cs.regions = make(map[uint64]*region.Region)
	return lastErr
}

func (cs *ChunkStore) getOrOpenRegion(cx, cz int) (*region.Region, error) {
	rx, rz := region.ChunkToRegion(cx, cz)
	key := regionKey(rx, rz)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if r, ok := cs.regions[key]; ok {
		return r, nil
	}

	path := region.RegionPath(cs.regionDir(), rx, rz)
	r, err := region.Open(path)
	if err != nil {
		return nil, err
	}
	cs.regions[key] = r
	return r, nil
}

func (cs *ChunkStore) getOrCreateRegion(cx, cz int) (*region.Region, error) {
	rx, rz := region.ChunkToRegion(cx, cz)
	key := regionKey(rx, rz)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if r, ok := cs.regions[key]; ok {
		return r, nil
	}

	dir := cs.regionDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("persistence: mkdir %s: %w", dir, err)
	}

	path := region.RegionPath(dir, rx, rz)
	r, err := region.Open(path)
	if err != nil {
		r, err = region.Create(path)
		if err != nil {
			return nil, err
		}
	}
	cs.regions[key] = r
	return r, nil
}

func (cs *ChunkStore) regionDir() string {
	return cs.worldDir + "/region"
}

func regionKey(rx, rz int) uint64 {
	return uint64(uint32(rx)) | uint64(uint32(rz))<<32
}
