package section

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestFeatureBorderCompaction preserves overwrites, explicit air and resumed writes.
func TestFeatureBorderCompaction(t *testing.T) {
	c := featureRegionTestChunk(t, -3, -4)
	b := c.FeatureBorderWrites()
	want := make(map[BorderPosition]FeatureBorderWrite)
	for i := 0; i < 2048; i++ {
		position := BorderPosition{X: -64 + i%16, Y: i / 32, Z: -80 + (i/16)%16}
		write := FeatureBorderWrite{State: int32(i % 3), Priority: uint32(i + 1)}
		b.set(position.X, position.Y, position.Z, write)
		want[position] = write
	}
	b.Compact()
	b.Compact()
	for _, bucket := range b.chunks {
		if bucket != nil {
			t.Fatal("mutable table retained")
		}
	}
	position := BorderPosition{X: -64, Y: 0, Z: -80}
	want[position] = FeatureBorderWrite{Priority: 9999}
	b.set(position.X, position.Y, position.Z, want[position])
	b.Compact()
	for position, expected := range want {
		actual, ok := b.get(position.X, position.Y, position.Z)
		if !ok || actual != expected {
			t.Fatalf("write at %+v = %+v, %v; want %+v", position, actual, ok, expected)
		}
	}
	count := 0
	b.Range(func(position BorderPosition, write FeatureBorderWrite) {
		count++
		if expected, ok := want[position]; !ok || write != expected {
			t.Fatalf("unexpected write at %+v: %+v", position, write)
		}
	})
	if count != len(want) {
		t.Fatalf("visited %d writes, want %d", count, len(want))
	}
}

// featureRegionTestChunk attaches uniform terrain readers to owned stamp storage.
func featureRegionTestChunk(t *testing.T, x, z int32) *Chunk {
	t.Helper()
	c := NewChunkWithBounds(x, z, 0, 64, 0)
	stone := block.DefaultStateID("minecraft:stone")
	if err := c.UseFeatureRegion(func(x, y, z int) int32 {
		if y <= 15 {
			return stone
		}
		return 0
	}, func(x, z int, kind HeightmapKind) (int, bool) { return 16, true }, make([]uint32, len(c.Sections)*BlocksPerSection)); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestFeatureBorderBufferKeepsCoordinatesAndOwnership validates compact storage across all neighbors.
func TestFeatureBorderBufferKeepsCoordinatesAndOwnership(t *testing.T) {
	c := featureRegionTestChunk(t, -3, -4)
	c.SetFeatureStep(7, 9)
	positions := make(map[BorderPosition]int32)
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			for _, y := range []int{0, 63} {
				p := BorderPosition{X: (int(c.X)+dx)*16 + 15, Y: y, Z: (int(c.Z) + dz) * 16}
				state := int32(0)
				if y == 63 {
					state = block.DefaultStateID("minecraft:stone")
				}
				positions[p] = state
				c.SetFeatureBlock(p.X, p.Y, p.Z, state)
				if actual, ok := c.GetBorderBlock(p.X, p.Y, p.Z); !ok || actual != state {
					t.Fatal("explicit air or solid write lost")
				}
			}
		}
	}
	buffer := c.FeatureBorderWrites()
	count := 0
	buffer.Range(func(p BorderPosition, write FeatureBorderWrite) {
		count++
		state, ok := positions[p]
		if !ok || state != write.State || write.Priority != (7<<16)|11 {
			t.Fatalf("invalid compact write: %v %+v", p, write)
		}
	})
	if count != len(positions) {
		t.Fatal("compact write count differs")
	}
	count = 0
	buffer.ForChunk(-2, -5, func(p BorderPosition, write FeatureBorderWrite) {
		count++
		if p.X>>4 != -2 || p.Z>>4 != -5 {
			t.Fatal("wrong destination")
		}
	})
	if count != 2 {
		t.Fatal("destination filtering lost writes")
	}
	c.FinishFeatureReads()
	c.ReleaseFeatureRegion()
	c.DropBorderBlocks()
	count = 0
	buffer.Range(func(BorderPosition, FeatureBorderWrite) { count++ })
	if count != len(positions) {
		t.Fatal("detaching chunk erased published border writes")
	}
}

// TestFeatureRegionRejectsDistantBlockEntities prevents orphan NBT outside the write radius.
func TestFeatureRegionRejectsDistantBlockEntities(t *testing.T) {
	c := featureRegionTestChunk(t, 0, 0)
	for _, p := range []BorderPosition{{32, 20, 0}, {-17, 20, 0}, {16, 64, 0}} {
		root := nbt.NewCompound().PutString("id", "minecraft:chest").PutInt("x", int32(p.X)).PutInt("y", int32(p.Y)).PutInt("z", int32(p.Z))
		c.AddBlockEntity(blockentity.NewRawBlockEntity(root))
	}
	if len(c.BorderBlockEntities()) != 0 || len(c.BlockEntities) != 0 {
		t.Fatal("out-of-radius block entity accepted")
	}
}

// TestFeatureRegionReadsAndHeightmaps checks read-your-writes, clearing top blocks, and negative coordinates.
func TestFeatureRegionReadsAndHeightmaps(t *testing.T) {
	c := featureRegionTestChunk(t, -1, -2)
	x, z := 0, -24
	if state, _ := c.GetBorderBlock(x, 15, z); !block.IsSolid(state) {
		t.Fatal("neighbor terrain unavailable")
	}
	c.SetFeatureBlock(x, 25, z, block.DefaultStateID("minecraft:stone"))
	if height, _ := c.FeatureHeightmap(x, z, HeightmapWorldSurface); height != 26 {
		t.Fatal("heightmap lost neighboring placement")
	}
	c.SetFeatureBlock(x, 25, z, 0)
	c.SetFeatureBlock(x, 15, z, 0)
	if height, _ := c.FeatureHeightmap(x, z, HeightmapOceanFloor); height != 15 {
		t.Fatalf("height after clearing top = %d", height)
	}
	if c.CloneRaw().FeatureRegionEnabled() {
		t.Fatal("clone retained dependency readers")
	}
	writes := c.ReleaseFeatureRegion()
	if len(writes) != len(c.Sections)*BlocksPerSection || c.FeatureRegionEnabled() {
		t.Fatal("stamp buffer ownership not released")
	}
}

// TestFeatureRegionBounds prevents distant writes and invalid vertical storage access.
func TestFeatureRegionBounds(t *testing.T) {
	c := featureRegionTestChunk(t, 0, 0)
	for _, p := range []BorderPosition{{-17, 20, 0}, {32, 20, 0}, {0, 20, -17}, {0, 20, 32}, {16, -1, 0}, {16, 64, 0}} {
		c.SetFeatureBlock(p.X, p.Y, p.Z, 1)
	}
	if len(c.BorderBlocks()) != 0 {
		t.Fatal("out-of-radius decoration write accepted")
	}
	for _, p := range []BorderPosition{{-16, 20, 0}, {31, 20, 0}, {0, 20, -16}, {0, 20, 31}} {
		c.SetFeatureBlock(p.X, p.Y, p.Z, 1)
	}
	if len(c.BorderBlocks()) != 4 || len(c.FeatureBorderPriorities()) != 4 {
		t.Fatal("valid edge write or its stamp lost")
	}
	c.DropBorderBlocks()
	if c.FeatureRegionEnabled() {
		t.Fatal("returned chunk retained readers")
	}
}

// TestFeatureBorderOrder preserves later local features and deterministic equal-stage source order.
func TestFeatureBorderOrder(t *testing.T) {
	c := featureRegionTestChunk(t, 0, 0)
	stone := block.DefaultStateID("minecraft:stone")
	gold := block.DefaultStateID("minecraft:gold_block")
	c.SetFeatureStep(6, 5)
	c.SetFeatureBlock(8, 12, 8, stone)
	if c.ApplyFeatureBorder(1, 0, 8, 12, 8, gold, (5<<16)|6) {
		t.Fatal("earlier stage replaced later local decoration")
	}
	if c.ApplyFeatureBorder(-1, 0, 8, 12, 8, gold, (6<<16)|7) {
		t.Fatal("earlier equal-stage source beat local source")
	}
	if !c.ApplyFeatureBorder(1, 0, 8, 12, 8, gold, (6<<16)|7) {
		t.Fatal("later equal-stage source did not win")
	}
	c.SetFeatureStep(0, 0)
	c.SetFeatureBlock(9, 12, 8, stone)
	if !c.ApplyFeatureBorder(-1, 0, 9, 12, 8, gold, (6<<16)|7) || !c.ApplyFeatureBorder(-1, 1, 9, 12, 8, stone, (6<<16)|7) {
		t.Fatal("equal-stage imported sources did not follow merge order")
	}
	if c.ApplyFeatureBorder(1, 0, 9, 12, 8, gold, (5<<16)|7) {
		t.Fatal("earlier incoming stage beat a newer imported stage")
	}
}
