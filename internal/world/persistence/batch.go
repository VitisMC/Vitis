package persistence

import "github.com/vitismc/vitis/internal/world/region"

// ChunkWrite carries an immutable NBT snapshot at absolute chunk coordinates.
type ChunkWrite struct {
	X, Z int
	NBT  []byte
}

// chunkWriteGroup preserves encounter order within one independently committed region.
type chunkWriteGroup struct {
	x, z    int
	indices []int
	chunks  []region.ChunkData
}

// WriteChunkBatch returns one result per snapshot after its region's durability barriers.
func (cs *ChunkStore) WriteChunkBatch(chunks []ChunkWrite) []error {
	return cs.writeBatch(chunkNamespace, chunks)
}

// WriteEntityBatch durably writes immutable entity-region snapshots in encounter order.
func (cs *ChunkStore) WriteEntityBatch(chunks []ChunkWrite) []error {
	return cs.writeBatch(entityNamespace, chunks)
}

// writeBatch groups snapshots by region within one storage namespace.
func (cs *ChunkStore) writeBatch(namespace regionNamespace, chunks []ChunkWrite) []error {
	cs.lifecycle.RLock()
	defer cs.lifecycle.RUnlock()
	results := make([]error, len(chunks))
	groups := make([]chunkWriteGroup, 0, 4)
	indices := make(map[uint64]int, 4)
	for index, chunk := range chunks {
		rx, rz := region.ChunkToRegion(chunk.X, chunk.Z)
		key := regionKey(rx, rz)
		groupIndex, ok := indices[key]
		if !ok {
			groupIndex = len(groups)
			indices[key] = groupIndex
			groups = append(groups, chunkWriteGroup{x: chunk.X, z: chunk.Z})
		}
		group := &groups[groupIndex]
		lx, lz := region.ChunkInRegion(chunk.X, chunk.Z)
		group.indices = append(group.indices, index)
		group.chunks = append(group.chunks, region.ChunkData{X: lx, Z: lz, Data: chunk.NBT})
	}
	for _, group := range groups {
		r, err := cs.getOrCreateRegion(namespace, group.x, group.z)
		if err == nil {
			err = r.WriteChunksNBT(group.chunks)
		}
		for _, index := range group.indices {
			results[index] = err
		}
	}
	return results
}
