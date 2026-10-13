package worldgen

import "sync"

// GenerationDiagnostics reports actual source work rather than completed target chunks.
type GenerationDiagnostics struct {
	RawGenerations          uint64
	RepeatedRawGenerations  uint64
	BaseGenerations         uint64
	RepeatedBaseGenerations uint64
	BaseCacheHits           uint64
	ClimateSamples          uint64
	ClimateColumns          uint64
	// ClimateColumnComputations counts full vectors, including repeated horizontal evaluations.
	ClimateColumnComputations uint64
}

// terrainDiagnostics stores optional benchmark counters outside the generation hot path.
type terrainDiagnostics struct {
	mu       sync.Mutex
	values   GenerationDiagnostics
	seen     map[rawChunkKey]struct{}
	seenBase map[rawChunkKey]struct{}
}

// EnableDiagnostics enables bounded-workload profiling before generation starts.
// Coordinate history is retained until cache reclamation; production leaves it disabled.
func (g *TerrainGenerator) EnableDiagnostics() {
	g.diagnostics.CompareAndSwap(nil, &terrainDiagnostics{seen: make(map[rawChunkKey]struct{}), seenBase: make(map[rawChunkKey]struct{})})
}

// recordBaseHit counts dependency reuse, including requests coalesced while terrain was running.
func (d *terrainDiagnostics) recordBaseHit() { d.mu.Lock(); d.values.BaseCacheHits++; d.mu.Unlock() }

// recordBase separates physical terrain work from independently requested decoration results.
func (d *terrainDiagnostics) recordBase(key rawChunkKey, biomes *cachedBiomeSource) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values.BaseGenerations++
	if _, seen := d.seenBase[key]; seen {
		d.values.RepeatedBaseGenerations++
	} else {
		d.seenBase[key] = struct{}{}
	}
	d.values.ClimateSamples += biomes.climateSamples
	d.values.ClimateColumns += uint64(len(biomes.climateColumns))
	d.values.ClimateColumnComputations += biomes.climateComputations
}

// Diagnostics returns an independent snapshot of enabled generation counters.
func (g *TerrainGenerator) Diagnostics() GenerationDiagnostics {
	d := g.diagnostics.Load()
	if d == nil {
		return GenerationDiagnostics{}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.values
}

// record publishes one independently generated source chunk and its climate work.
func (d *terrainDiagnostics) record(key rawChunkKey, biomes *cachedBiomeSource) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values.RawGenerations++
	if _, exists := d.seen[key]; exists {
		d.values.RepeatedRawGenerations++
	} else {
		d.seen[key] = struct{}{}
	}
	d.values.ClimateSamples += biomes.climateSamples
	d.values.ClimateColumns += uint64(len(biomes.climateColumns))
	d.values.ClimateColumnComputations += biomes.climateComputations
}
