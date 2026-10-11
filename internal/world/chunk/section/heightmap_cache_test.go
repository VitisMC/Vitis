package section

import "testing"

// TestHeightmapCacheKeepsChangesBelowTop verifies incremental validity for ore replacement.
func TestHeightmapCacheKeepsChangesBelowTop(t *testing.T) {
	c := NewChunkWithBounds(0, 0, -64, 384, 0)
	c.SetBlock(4, 64, 7, 1)
	c.EnableHeightmapCache()
	for kind := HeightmapKind(0); kind < heightmapKindCount; kind++ {
		if got, ok := c.Heightmap(4, 7, kind); !ok || got != 65 {
			t.Fatalf("kind %d initial height %d, %v", kind, got, ok)
		}
	}
	c.SetBlock(4, 32, 7, 1)
	index := 7*16 + 4
	for kind := HeightmapKind(0); kind < heightmapKindCount; kind++ {
		if c.heightmapValid[kind][index>>6]&(uint64(1)<<uint(index&63)) == 0 {
			t.Fatalf("kind %d invalidated below its top", kind)
		}
	}
	c.SetBlock(4, 64, 7, 0)
	for kind := HeightmapKind(0); kind < heightmapKindCount; kind++ {
		if got, ok := c.Heightmap(4, 7, kind); !ok || got != 33 {
			t.Fatalf("kind %d removed top height %d, %v", kind, got, ok)
		}
	}
	c.SetBlock(4, 80, 7, 1)
	for kind := HeightmapKind(0); kind < heightmapKindCount; kind++ {
		if got, ok := c.Heightmap(4, 7, kind); !ok || got != 81 {
			t.Fatalf("kind %d raised top height %d, %v", kind, got, ok)
		}
	}
}
