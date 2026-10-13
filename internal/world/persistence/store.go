package persistence

import (
	"fmt"
	"os"
	"sync"

	"github.com/vitismc/vitis/internal/world/region"
)

type regionNamespace string

const (
	chunkNamespace  regionNamespace = "region"
	entityNamespace regionNamespace = "entities"
	poiNamespace    regionNamespace = "poi"
)

// ChunkStore manages chunk, entity, and POI Anvil region files on disk.
type ChunkStore struct {
	worldDir  string
	lifecycle sync.RWMutex
	mu        sync.Mutex
	regions   map[regionNamespace]map[uint64]*region.Region
}

// NewChunkStore creates a chunk store for the given world directory.
func NewChunkStore(worldDir string) *ChunkStore {
	return &ChunkStore{
		worldDir: worldDir,
		regions: map[regionNamespace]map[uint64]*region.Region{
			chunkNamespace:  make(map[uint64]*region.Region),
			entityNamespace: make(map[uint64]*region.Region),
			poiNamespace:    make(map[uint64]*region.Region),
		},
	}
}

// ReadChunkNBT reads raw NBT data for a chunk at absolute chunk coordinates.
func (cs *ChunkStore) ReadChunkNBT(cx, cz int) ([]byte, error) {
	return cs.readNBT(chunkNamespace, cx, cz)
}

// ReadEntityNBT reads the entity-region payload for a chunk.
func (cs *ChunkStore) ReadEntityNBT(cx, cz int) ([]byte, error) {
	return cs.readNBT(entityNamespace, cx, cz)
}

// ReadPOINBT reads the point-of-interest region payload for a chunk.
func (cs *ChunkStore) ReadPOINBT(cx, cz int) ([]byte, error) {
	return cs.readNBT(poiNamespace, cx, cz)
}

func (cs *ChunkStore) readNBT(namespace regionNamespace, cx, cz int) ([]byte, error) {
	cs.lifecycle.RLock()
	defer cs.lifecycle.RUnlock()
	r, err := cs.getOrOpenRegion(namespace, cx, cz)
	if err != nil {
		return nil, err
	}
	lx, lz := region.ChunkInRegion(cx, cz)
	return r.ReadChunkNBT(lx, lz)
}

// WriteChunkNBT writes raw NBT data for a chunk at absolute chunk coordinates.
func (cs *ChunkStore) WriteChunkNBT(cx, cz int, data []byte) error {
	return cs.writeNBT(chunkNamespace, cx, cz, data)
}

// WriteEntityNBT writes the entity-region payload for a chunk.
func (cs *ChunkStore) WriteEntityNBT(cx, cz int, data []byte) error {
	return cs.writeNBT(entityNamespace, cx, cz, data)
}

// WritePOINBT writes the point-of-interest region payload for a chunk.
func (cs *ChunkStore) WritePOINBT(cx, cz int, data []byte) error {
	return cs.writeNBT(poiNamespace, cx, cz, data)
}

func (cs *ChunkStore) writeNBT(namespace regionNamespace, cx, cz int, data []byte) error {
	cs.lifecycle.RLock()
	defer cs.lifecycle.RUnlock()
	r, err := cs.getOrCreateRegion(namespace, cx, cz)
	if err != nil {
		return err
	}
	lx, lz := region.ChunkInRegion(cx, cz)
	return r.WriteChunkNBT(lx, lz, data)
}

// HasChunk checks if a chunk exists on disk.
func (cs *ChunkStore) HasChunk(cx, cz int) bool {
	return cs.hasNBT(chunkNamespace, cx, cz)
}

// HasEntityChunk reports whether an entity-region payload exists for a chunk.
func (cs *ChunkStore) HasEntityChunk(cx, cz int) bool {
	return cs.hasNBT(entityNamespace, cx, cz)
}

// HasPOIChunk reports whether a point-of-interest payload exists for a chunk.
func (cs *ChunkStore) HasPOIChunk(cx, cz int) bool {
	return cs.hasNBT(poiNamespace, cx, cz)
}

func (cs *ChunkStore) hasNBT(namespace regionNamespace, cx, cz int) bool {
	cs.lifecycle.RLock()
	defer cs.lifecycle.RUnlock()
	r, err := cs.getOrOpenRegion(namespace, cx, cz)
	if err != nil {
		return false
	}
	lx, lz := region.ChunkInRegion(cx, cz)
	return r.HasChunk(lx, lz)
}

// Close closes all open region files.
func (cs *ChunkStore) Close() error {
	cs.lifecycle.Lock()
	defer cs.lifecycle.Unlock()
	cs.mu.Lock()
	defer cs.mu.Unlock()
	var lastErr error
	for _, namespaceRegions := range cs.regions {
		for _, r := range namespaceRegions {
			if err := r.Close(); err != nil {
				lastErr = err
			}
		}
	}
	cs.regions = map[regionNamespace]map[uint64]*region.Region{
		chunkNamespace:  make(map[uint64]*region.Region),
		entityNamespace: make(map[uint64]*region.Region),
		poiNamespace:    make(map[uint64]*region.Region),
	}
	return lastErr
}

func (cs *ChunkStore) getOrOpenRegion(namespace regionNamespace, cx, cz int) (*region.Region, error) {
	rx, rz := region.ChunkToRegion(cx, cz)
	key := regionKey(rx, rz)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if r, ok := cs.regions[namespace][key]; ok {
		return r, nil
	}

	path := region.RegionPath(cs.regionDir(namespace), rx, rz)
	r, err := region.Open(path)
	if err != nil {
		return nil, err
	}
	cs.regions[namespace][key] = r
	return r, nil
}

func (cs *ChunkStore) getOrCreateRegion(namespace regionNamespace, cx, cz int) (*region.Region, error) {
	rx, rz := region.ChunkToRegion(cx, cz)
	key := regionKey(rx, rz)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if r, ok := cs.regions[namespace][key]; ok {
		return r, nil
	}

	dir := cs.regionDir(namespace)
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
	cs.regions[namespace][key] = r
	return r, nil
}

func (cs *ChunkStore) regionDir(namespace regionNamespace) string {
	return cs.worldDir + "/" + string(namespace)
}

func regionKey(rx, rz int) uint64 {
	return uint64(uint32(rx)) | uint64(uint32(rz))<<32
}
