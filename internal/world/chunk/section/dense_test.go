package section

import "testing"

// TestDenseGenerationStoragePreservesPalettesAndHeightmaps verifies ownership and cached writes.
func TestDenseGenerationStoragePreservesPalettesAndHeightmaps(t *testing.T) {
	c := NewChunkWithBounds(-2, -3, -64, 384, 0)
	blocks := make([]int32, len(c.Sections)*BlocksPerSection)
	if err := c.UseDenseBlocks(blocks); err != nil {
		t.Fatal(err)
	}
	if err := c.UseDenseBlocks(blocks[:1]); err == nil {
		t.Fatal("invalid dense buffer accepted")
	}
	c.SetBlock(-31, -64, -47, 1)
	c.SetBlock(-31, 319, -47, 1)
	if c.GetBlock(-31, -64, -47) != 1 || c.GetBlock(-31, 319, -47) != 1 || c.GetBlock(-31, 320, -47) != 0 {
		t.Fatal("dense indexing changed")
	}
	c.EnableHeightmapCache()
	before, _ := c.Heightmap(1, 1, HeightmapWorldSurface)
	copy := c.CloneRaw()
	c.SetBlock(1, 319, 1, 0)
	after, _ := c.Heightmap(1, 1, HeightmapWorldSurface)
	if before != 320 || after != -63 || copy.GetBlock(1, 319, 1) != 1 {
		t.Fatal("dense heightmap or clone changed")
	}
	c.PackDenseBlocks()
	clear(blocks)
	if c.DenseBlocks() != nil || c.GetBlock(1, -64, 1) != 1 {
		t.Fatal("packed palettes alias borrowed buffer")
	}
}
