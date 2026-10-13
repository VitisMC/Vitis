package section

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestFeatureRegionRadiusRetainsFarWrites checks negative and distant chunk ownership.
func TestFeatureRegionRadiusRetainsFarWrites(t *testing.T) {
	chunk := NewChunkWithBounds(-2, 3, -64, 128, 0)
	read := func(int, int, int) int32 { return 0 }
	height := func(int, int, HeightmapKind) (int, bool) { return -64, false }
	if err := chunk.UseFeatureRegionRadius(read, height, make([]uint32, len(chunk.Sections)*BlocksPerSection), 5); err != nil {
		t.Fatal(err)
	}
	chunk.SetFeatureStep(8, 4)
	chunk.SetFeatureWriteRadius(5)
	stone := block.DefaultStateID("minecraft:stone")
	positions := [...]BorderPosition{{X: -112, Y: 19, Z: 56}, {X: 48, Y: 27, Z: 56}, {X: -24, Y: 35, Z: -32}}
	for _, position := range positions {
		chunk.SetFeatureBlock(position.X, position.Y, position.Z, stone)
		if got, ok := chunk.GetBorderBlock(position.X, position.Y, position.Z); !ok || got != stone {
			t.Fatalf("far write at %+v = (%d,%v)", position, got, ok)
		}
		if y, ok := chunk.FeatureHeightmap(position.X, position.Z, HeightmapWorldSurface); !ok || y != position.Y+1 {
			t.Fatalf("far height at %+v = (%d,%v)", position, y, ok)
		}
		count := 0
		chunk.FeatureBorderWrites().ForChunk(int32(position.X>>4), int32(position.Z>>4), func(found BorderPosition, write FeatureBorderWrite) {
			if found != position || write.State != stone {
				t.Fatalf("far buffer entry = %+v %+v, want %+v", found, write, position)
			}
			count++
		})
		if count != 1 {
			t.Fatalf("far buffer count for %+v = %d", position, count)
		}
	}
	chunk.SetFeatureBlock(64, 28, 56, stone)
	count := 0
	chunk.FeatureBorderWrites().ForChunk(4, 3, func(BorderPosition, FeatureBorderWrite) { count++ })
	if count != 0 {
		t.Fatal("write beyond configured feature radius was accepted")
	}
}
