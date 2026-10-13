package chunk

import (
	"bytes"
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

func TestChunkKeyPackingUniqueness(t *testing.T) {
	testCases := []struct {
		x int32
		z int32
	}{
		{x: 0, z: 0},
		{x: 1, z: 0},
		{x: 0, z: 1},
		{x: -1, z: -1},
		{x: 12345, z: -54321},
	}

	seen := make(map[int64]struct{}, len(testCases))
	for _, testCase := range testCases {
		key := ChunkKey(testCase.x, testCase.z)
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate key for x=%d z=%d", testCase.x, testCase.z)
		}
		seen[key] = struct{}{}
	}
}

func TestChunkStateAndAccessors(t *testing.T) {
	loaded := New(2, 3)
	if loaded.X() != 2 || loaded.Z() != 3 {
		t.Fatalf("unexpected coordinates x=%d z=%d", loaded.X(), loaded.Z())
	}
	if loaded.State() != StateLoaded {
		t.Fatalf("expected loaded state, got %d", loaded.State())
	}

	loaded.Touch(42)
	if loaded.LastAccessTick() != 42 {
		t.Fatalf("unexpected last access tick %d", loaded.LastAccessTick())
	}

	loaded.SetState(StateUnloading)
	if loaded.State() != StateUnloading {
		t.Fatalf("expected unloading state, got %d", loaded.State())
	}

	loading := NewLoading(-7, 9)
	if loading.State() != StateLoading {
		t.Fatalf("expected loading state, got %d", loading.State())
	}
	if loading.Key() != ChunkKey(-7, 9) {
		t.Fatalf("unexpected key %d", loading.Key())
	}
}

func TestSetBlockRecomputesChunkLight(t *testing.T) {
	c := New(0, 0)
	data := section.NewChunk(0, 0, section.OverworldSections, 0)
	data.ComputeLight()
	c.SetData(data)

	c.SetBlock(8, 64, 8, genblock.StoneDefaultState)
	c.EnsureLight()
	if data.LightEngine == nil {
		t.Fatal("block mutation did not recompute lighting")
	}
	lightSection := data.LightEngine.GetSkyLightSection((64-section.OverworldMinY)/16 + 1)
	if got := lightSection.GetXYZ(8, 64, 8); got != 0 {
		t.Fatalf("opaque block skylight = %d, want 0", got)
	}
}

// TestChunkPacketPayloadCache checks reuse and invalidation after world mutations.
func TestChunkPacketPayloadCache(t *testing.T) {
	c := New(0, 0)
	first := c.EncodePacketPayload()
	second := c.EncodePacketPayload()
	if len(first) == 0 || &first[0] != &second[0] {
		t.Fatal("unchanged chunk payload was encoded again")
	}

	c.SetBlock(1, 64, 1, genblock.StoneDefaultState)
	updated := c.EncodePacketPayload()
	if bytes.Equal(first, updated) {
		t.Fatal("block mutation did not invalidate cached payload")
	}
	if next := c.EncodePacketPayload(); &updated[0] != &next[0] {
		t.Fatal("updated chunk payload was encoded again")
	}

	replacement := section.NewChunk(0, 0, section.OverworldSections, 0)
	replacement.SetBlock(2, 64, 2, genblock.DirtDefaultState)
	c.SetData(replacement)
	replaced := c.EncodePacketPayload()
	if bytes.Equal(updated, replaced) {
		t.Fatal("replacement chunk data did not invalidate cached payload")
	}
}

// BenchmarkCachedChunkPacketPayload measures repeated sends of a ready chunk.
func BenchmarkCachedChunkPacketPayload(b *testing.B) {
	c := New(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, -64, z, genblock.BedrockDefaultState)
			for y := -63; y < 64; y++ {
				c.SetBlock(x, y, z, genblock.DirtDefaultState)
			}
			c.SetBlock(x, 64, z, genblock.GrassBlockDefaultState)
		}
	}
	c.EncodePacketPayload()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.EncodePacketPayload()
	}
}
