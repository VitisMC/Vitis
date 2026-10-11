package section

import (
	"strconv"
	"testing"
)

// TestFillBlocksMatchesIncrementalWrites verifies uniform, indirect and direct palettes.
func TestFillBlocksMatchesIncrementalWrites(t *testing.T) {
	for _, diversity := range []int{1, 2, 17, 257} {
		t.Run(strconv.Itoa(diversity), func(t *testing.T) {
			var values [BlocksPerSection]int32
			want, got := NewSection(0, 0, 0), NewSection(0, 0, 0)
			for index := range values {
				values[index] = int32((index*13 + index/19) % diversity)
				want.Blocks.Set(index, values[index])
			}
			got.FillBlocks(&values)
			clear(values[:])
			for index := 0; index < BlocksPerSection; index++ {
				if got.Blocks.Get(index) != want.Blocks.Get(index) {
					t.Fatalf("block %d differs", index)
				}
			}
			got.SetBlock(3, 8, 2, 314)
			if got.GetBlock(3, 8, 2) != 314 {
				t.Fatal("batch palette cannot be updated")
			}
		})
	}
}
