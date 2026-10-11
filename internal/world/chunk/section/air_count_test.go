package section

import (
	"encoding/binary"
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestSectionAirVariantsDoNotCountAsBlocks checks uniform and mixed protocol section counts.
func TestSectionAirVariantsDoNotCountAsBlocks(t *testing.T) {
	for _, name := range []string{"minecraft:air", "minecraft:cave_air", "minecraft:void_air"} {
		air := block.DefaultStateID(name)
		s := NewSection(0, air, 0)
		if s.NonAirCount() != 0 {
			t.Fatalf("uniform %s counted as non-air", name)
		}
		s.SetBlock(0, 0, 0, block.DefaultStateID("minecraft:stone"))
		s.SetBlock(1, 0, 0, block.DefaultStateID("minecraft:cave_air"))
		s.SetBlock(2, 0, 0, block.DefaultStateID("minecraft:void_air"))
		if s.NonAirCount() != 1 {
			t.Fatalf("mixed %s counted incorrectly", name)
		}
		encoded := EncodeSectionData(nil, s)
		if len(encoded) < 2 || binary.BigEndian.Uint16(encoded[:2]) != 1 {
			t.Fatal("protocol non-air count includes air variants")
		}
	}
}
