package lighting

import "testing"

// TestSunlightSpreadsUnderOverhang seeds fully lit air next to opaque columns.
func TestSunlightSpreadsUnderOverhang(t *testing.T) {
	column := newMockChunk(0, 31)
	for x := 8; x < 16; x++ {
		for z := 0; z < 16; z++ {
			column.blocks[[3]int{x, 16, z}] = findOpaqueBlock()
		}
	}
	engine := NewSparseEngine(2, 0, 31)
	engine.ComputeSkyLight(column)
	if got := engine.SkyLight(8, 15, 8); got != 14 {
		t.Fatalf("sky light under overhang = %d, want 14", got)
	}
}
