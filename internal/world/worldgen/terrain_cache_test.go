package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// TestRawBorderSeamsCoverFeatureRadius protects far writes across column32 transitions.
func TestRawBorderSeamsCoverFeatureRadius(t *testing.T) {
	g := &TerrainGenerator{}
	g.baseBandZ.Store(-625)
	snapshot := &terrainSnapshot{featureRadius: 5}
	for _, row := range []int32{-6, -5, -1, 0, 4, 5, 26, 27, 31, 32, 36, 37} {
		key := rawChunkKey{snapshot: snapshot, z: -625 + row}
		entry := &rawChunkEntry{complete: true}
		g.queueRawEntry(key, entry)
		wrapped := row & 31
		want := wrapped < 5 || wrapped >= 27
		if entry.seam != want {
			t.Fatalf("row %d seam = %v, want %v", row, entry.seam, want)
		}
		g.removeRawEntry(entry)
	}
	if g.borderSeamLRU.Len() != 0 || g.borderLRU.Len() != 0 {
		t.Fatal("removed entries remained in eviction queues")
	}
}

// TestTerrainCacheBoundsFullChunksSeparatelyFromBorders retains future source terrain.
func TestTerrainCacheBoundsFullChunksSeparatelyFromBorders(t *testing.T) {
	g := &TerrainGenerator{rawCache: make(map[rawChunkKey]*rawChunkEntry)}
	for index := 0; index < rawChunkCacheLimit+4; index++ {
		key := rawChunkKey{x: int32(index)}
		g.rawCache[key] = &rawChunkEntry{chunk: &section.Chunk{}, lastUse: uint64(index + 1), complete: true}
		g.queueRawEntry(key, g.rawCache[key])
		g.rawTerrains++
	}
	for index := 0; index < rawBorderCacheLimit+4; index++ {
		key := rawChunkKey{x: -int32(index) - 1}
		g.rawCache[key] = &rawChunkEntry{lastUse: uint64(index + 1), complete: true}
		g.queueRawEntry(key, g.rawCache[key])
		g.rawBorders++
	}
	readyKey := rawChunkKey{z: 1}
	g.rawCache[readyKey] = &rawChunkEntry{ready: make(chan struct{})}
	g.evictRawChunks()
	if g.rawTerrains != rawChunkCacheLimit || g.rawBorders != rawBorderCacheLimit || len(g.rawCache) != rawChunkCacheLimit+rawBorderCacheLimit+1 {
		t.Fatal("cache budgets not enforced separately")
	}
	for x := int32(0); x < 4; x++ {
		if g.rawCache[rawChunkKey{x: x}] != nil || g.rawCache[rawChunkKey{x: -x - 1}] != nil {
			t.Fatal("oldest completed entries were not evicted")
		}
	}
	if g.rawCache[readyKey] == nil {
		t.Fatal("in-progress generation was evicted")
	}
}

// TestTerrainBorderSeamEviction keeps distant neighboring writes for the next band.
func TestTerrainBorderSeamEviction(t *testing.T) {
	g := &TerrainGenerator{rawCache: make(map[rawChunkKey]*rawChunkEntry)}
	g.baseBandZ.Store(-625)
	seamKey := rawChunkKey{x: 0, z: -595}
	seam := &rawChunkEntry{complete: true}
	g.rawCache[seamKey] = seam
	g.queueRawEntry(seamKey, seam)
	g.rawBorders++
	for x := int32(1); x <= rawBorderCacheLimit+1; x++ {
		key := rawChunkKey{x: x, z: -609}
		entry := &rawChunkEntry{complete: true}
		g.rawCache[key] = entry
		g.queueRawEntry(key, entry)
		g.rawBorders++
		g.evictRawChunks()
	}
	if g.rawBorders != rawBorderCacheLimit || g.rawCache[seamKey] != seam || g.borderSeamLRU.Len() != 1 {
		t.Fatal("border seam was evicted or cache exceeded its limit")
	}
}
