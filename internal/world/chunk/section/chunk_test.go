package section

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/nbt"
)

func TestNewChunk(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	if len(c.Sections) != OverworldSections {
		t.Fatalf("expected %d sections, got %d", OverworldSections, len(c.Sections))
	}
	if got := c.GetBlock(0, 0, 0); got != 0 {
		t.Fatalf("expected air at (0,0,0), got %d", got)
	}
}

// TestCloneRawDoesNotShareSectionData protects cached terrain from later writes.
func TestCloneRawDoesNotShareSectionData(t *testing.T) {
	original := NewChunk(0, 0, OverworldSections, 1)
	original.SetBlock(1, 64, 1, genblock.GrassBlockDefaultState)
	original.Sections[8].SetBiome(0, 0, 0, 3)
	clone := original.CloneRaw()
	clone.SetBlock(1, 64, 1, genblock.DirtDefaultState)
	clone.Sections[8].SetBiome(0, 0, 0, 4)
	if original.GetBlock(1, 64, 1) != genblock.GrassBlockDefaultState || original.Sections[8].GetBiome(0, 0, 0) != 3 {
		t.Fatal("clone changed cached section data")
	}
}

func TestChunkHeightmapsEncodeSurfaceAndMotion(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(2, 64, 3, genblock.StoneDefaultState)
	c.SetBlock(2, 70, 3, genblock.WaterDefaultState)

	data := c.encodeHeightmaps(nil)
	root, err := nbt.NewDecoder(data).ReadRootCompound()
	if err != nil {
		t.Fatalf("decode heightmaps: %v", err)
	}
	if root == nil {
		t.Fatal("missing heightmap compound")
	}
	worldSurface := heightmapValues(t, root, "WORLD_SURFACE")
	motionBlocking := heightmapValues(t, root, "MOTION_BLOCKING")
	index := 3*16 + 2
	want := uint16(70 - OverworldMinY + 1)
	if worldSurface[index] != want {
		t.Fatalf("world surface = %d, want %d", worldSurface[index], want)
	}
	if motionBlocking[index] != want {
		t.Fatalf("motion blocking = %d, want %d", motionBlocking[index], want)
	}
}

func TestChunkHeightmapCacheInvalidatesChangedColumn(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(4, 64, 7, genblock.StoneDefaultState)
	c.EnableHeightmapCache()
	if got, ok := c.Heightmap(4, 7, HeightmapMotionBlocking); !ok || got != 65 {
		t.Fatalf("initial height = %d, %v; want 65, true", got, ok)
	}
	c.SetBlock(4, 80, 7, genblock.WaterDefaultState)
	if got, ok := c.Heightmap(4, 7, HeightmapMotionBlocking); !ok || got != 81 {
		t.Fatalf("updated height = %d, %v; want 81, true", got, ok)
	}
	if got, ok := c.Heightmap(5, 7, HeightmapMotionBlocking); ok || got != OverworldMinY {
		t.Fatalf("empty height = %d, %v; want %d, false", got, ok, OverworldMinY)
	}
}

func TestEncodedHeightmapsWithUniformAndMixedSections(t *testing.T) {
	for _, cached := range []bool{false, true} {
		c := NewChunk(0, 0, OverworldSections, 0)
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				for y := 0; y < 16; y++ {
					c.SetBlock(x, y, z, genblock.StoneDefaultState)
				}
			}
		}
		c.SetBlock(3, 20, 4, genblock.ShortGrassDefaultState)
		if cached {
			c.EnableHeightmapCache()
		}
		for pass := 0; pass < 2; pass++ {
			root, err := nbt.NewDecoder(c.encodeHeightmaps(nil)).ReadRootCompound()
			if err != nil {
				t.Fatal(err)
			}
			surface := heightmapValues(t, root, "WORLD_SURFACE")
			motion := heightmapValues(t, root, "MOTION_BLOCKING")
			wantSurface := uint16(21 - OverworldMinY)
			if pass == 1 {
				wantSurface = uint16(16 - OverworldMinY)
			}
			if got, want := surface[4*16+3], wantSurface; got != want {
				t.Fatalf("cached=%v pass=%d surface=%d, want %d", cached, pass, got, want)
			}
			if got, want := motion[4*16+3], uint16(16-OverworldMinY); got != want {
				t.Fatalf("cached=%v pass=%d motion=%d, want %d", cached, pass, got, want)
			}
			if got, want := surface[5*16+3], uint16(16-OverworldMinY); got != want {
				t.Fatalf("cached=%v pass=%d adjacent surface=%d, want %d", cached, pass, got, want)
			}
			c.SetBlock(3, 20, 4, 0)
		}
	}
}

func heightmapValues(t *testing.T, root *nbt.Compound, name string) [16 * 16]uint16 {
	t.Helper()
	raw, tagID, ok := root.Get(name)
	if !ok || tagID != nbt.TagLongArray {
		t.Fatalf("missing %s heightmap", name)
	}
	packed, ok := raw.([]int64)
	if !ok || len(packed) != 37 {
		t.Fatalf("invalid %s heightmap storage", name)
	}

	var values [16 * 16]uint16
	for index := range values {
		word := index / 7
		shift := uint((index % 7) * 9)
		values[index] = uint16((uint64(packed[word]) >> shift) & 0x1FF)
	}
	return values
}

func TestChunkSetGetBlock(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, 64, 0, genblock.StoneDefaultState)
	if got := c.GetBlock(0, 64, 0); got != genblock.StoneDefaultState {
		t.Fatalf("expected stone at (0,64,0), got %d", got)
	}
	if got := c.GetBlock(1, 64, 0); got != 0 {
		t.Fatalf("expected air at (1,64,0), got %d", got)
	}
}

func TestChunkSetBlockMinY(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, OverworldMinY, 0, genblock.BedrockDefaultState)
	if got := c.GetBlock(0, OverworldMinY, 0); got != genblock.BedrockDefaultState {
		t.Fatalf("expected bedrock at minY, got %d", got)
	}
}

func TestChunkSetBlockMaxY(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, OverworldMaxY, 0, genblock.StoneDefaultState)
	if got := c.GetBlock(0, OverworldMaxY, 0); got != genblock.StoneDefaultState {
		t.Fatalf("expected stone at maxY, got %d", got)
	}
}

func TestChunkOutOfBounds(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	c.SetBlock(0, OverworldMinY-1, 0, 1)
	if got := c.GetBlock(0, OverworldMinY-1, 0); got != 0 {
		t.Fatalf("expected 0 for out-of-bounds, got %d", got)
	}
	c.SetBlock(0, OverworldMaxY+1, 0, 1)
	if got := c.GetBlock(0, OverworldMaxY+1, 0); got != 0 {
		t.Fatalf("expected 0 for out-of-bounds, got %d", got)
	}
}

func TestBlockEntityKeyDoesNotOverlapCoordinates(t *testing.T) {
	positions := [][3]int32{
		{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1},
		{-1, -64, -1}, {30_000_000, 319, -30_000_000},
	}
	seen := make(map[int64]struct{}, len(positions))
	for _, position := range positions {
		key := blockEntityKey(position[0], position[1], position[2])
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate key for position %v", position)
		}
		seen[key] = struct{}{}
	}
}

func TestEncodeEmptyChunk(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	payload := c.EncodePacketPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
	if len(payload) < 100 {
		t.Fatalf("payload too small: %d bytes", len(payload))
	}
}

// TestChunkDimensionBounds verifies non-Overworld vertical indexing and lighting.
func TestChunkDimensionBounds(t *testing.T) {
	c := NewChunkWithBounds(0, 0, 0, 256, 8)
	stone := genblock.StoneDefaultState
	c.SetBlock(1, 0, 1, stone)
	c.SetBlock(1, 255, 1, stone)
	if c.MinY() != 0 || c.MaxY() != 255 || len(c.Sections) != 16 || c.GetBlock(1, 0, 1) != stone || c.GetBlock(1, 255, 1) != stone {
		t.Fatalf("dimension chunk bounds min=%d max=%d sections=%d", c.MinY(), c.MaxY(), len(c.Sections))
	}
	if c.GetBlock(1, -1, 1) != 0 || c.GetBlock(1, 256, 1) != 0 {
		t.Fatal("out-of-bounds dimension block was accessible")
	}
	c.ComputeLight()
	if c.LightEngine == nil {
		t.Fatal("dimension light engine was not initialized")
	}
}

func TestEncodeChunkWithBlocks(t *testing.T) {
	c := NewChunk(0, 0, OverworldSections, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, -64, z, genblock.BedrockDefaultState)
			c.SetBlock(x, 64, z, genblock.GrassBlockDefaultState)
			for y := -63; y < 64; y++ {
				c.SetBlock(x, y, z, genblock.DirtDefaultState)
			}
		}
	}
	payload := c.EncodePacketPayload()
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
	t.Logf("chunk payload size: %d bytes", len(payload))
}

func TestEncodeSingleSection(t *testing.T) {
	s := NewSection(0, 0, 0)
	s.SetBlock(0, 0, 0, 1)

	data := EncodeSectionData(nil, s)
	if len(data) == 0 {
		t.Fatal("expected non-empty section data")
	}
}

func TestEncodeAllAirSection(t *testing.T) {
	s := NewSection(0, 0, 0)
	data := EncodeSectionData(nil, s)
	if len(data) < 4 {
		t.Fatalf("section data too small: %d bytes", len(data))
	}
	if data[0] != 0 || data[1] != 0 {
		t.Fatalf("expected blockCount=0, got %d,%d", data[0], data[1])
	}
}

func BenchmarkEncodeChunkPayload(b *testing.B) {
	c := NewChunk(0, 0, OverworldSections, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, -64, z, genblock.BedrockDefaultState)
			for y := -63; y < 64; y++ {
				c.SetBlock(x, y, z, genblock.DirtDefaultState)
			}
			c.SetBlock(x, 64, z, genblock.GrassBlockDefaultState)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.EncodePacketPayload()
	}
}
