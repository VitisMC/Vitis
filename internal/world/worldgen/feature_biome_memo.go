package worldgen

const featureBiomeMemoSize = 8192

// featureBiomeMemoEntry stores one sampled quart position for the current epoch.
type featureBiomeMemoEntry struct {
	position biomeQuartPosition
	biome    int32
	epoch    uint32
}

// featureBiomeMemo reuses fixed storage for placement biome checks within one chunk.
type featureBiomeMemo struct {
	entries [featureBiomeMemoSize]featureBiomeMemoEntry
	epoch   uint32
}

// reset discards the previous chunk's entries without clearing the table normally.
func (m *featureBiomeMemo) reset() {
	m.epoch++
	if m.epoch == 0 {
		clear(m.entries[:])
		m.epoch = 1
	}
}

// sample returns the cached quart biome or computes and retains it.
func (m *featureBiomeMemo) sample(source biomeSource, position biomeQuartPosition) int32 {
	if m == nil {
		return source.sample(position.x, position.y, position.z)
	}
	index := featureBiomeMemoHash(position) & (featureBiomeMemoSize - 1)
	for range featureBiomeMemoSize {
		entry := &m.entries[index]
		if entry.epoch != m.epoch {
			biome := source.sample(position.x, position.y, position.z)
			*entry = featureBiomeMemoEntry{position: position, biome: biome, epoch: m.epoch}
			return biome
		}
		if entry.position == position {
			return entry.biome
		}
		index = (index + 1) & (featureBiomeMemoSize - 1)
	}
	return source.sample(position.x, position.y, position.z)
}

// featureBiomeMemoHash mixes signed quart coordinates into a table index.
func featureBiomeMemoHash(position biomeQuartPosition) int {
	hash := uint64(position.x)*0x9e3779b185ebca87 ^ uint64(position.y)*0xc2b2ae3d27d4eb4f ^ uint64(position.z)*0x165667b19e3779f9
	hash ^= hash >> 32
	return int(hash)
}
