package worldgen

// TryReleaseGenerationCaches drops idle generation state without waiting for workers.
// Resources and seed remain available; the next generation rebuilds its snapshot.
func (g *TerrainGenerator) TryReleaseGenerationCaches() bool {
	if g == nil {
		return true
	}
	if !g.generationMu.TryLock() {
		return false
	}
	defer g.generationMu.Unlock()
	if !g.mu.TryLock() {
		return false
	}
	defer g.mu.Unlock()
	if !g.rawMu.TryLock() {
		return false
	}
	defer g.rawMu.Unlock()
	if !g.baseMu.TryLock() {
		return false
	}
	defer g.baseMu.Unlock()
	g.rawCache = nil
	g.rawClock, g.rawTerrains, g.rawBorders = 0, 0, 0
	g.terrainLRU.Init()
	g.borderLRU.Init()
	g.borderSeamLRU.Init()
	g.baseCache = nil
	g.baseActive = nil
	g.baseLRU.Init()
	g.baseSeamLRU.Init()
	g.compiled, g.resource = nil, nil
	g.baseBandZ.Store(1<<31 - 1)
	if diagnostics := g.diagnostics.Load(); diagnostics != nil {
		diagnostics.mu.Lock()
		diagnostics.seen = make(map[rawChunkKey]struct{})
		diagnostics.seenBase = make(map[rawChunkKey]struct{})
		diagnostics.mu.Unlock()
	}
	return true
}
