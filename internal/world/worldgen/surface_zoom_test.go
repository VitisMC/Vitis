package worldgen

import "testing"

// patternedBiomes supplies discontinuous quart biomes independently of climate lookup.
type patternedBiomes struct{ offset int }

// sample produces a stable mixed biome field, including negative coordinates.
func (s patternedBiomes) sample(x, y, z int) int32 {
	return int32((x*31 + y*17 + z*13 + s.offset) & 7)
}

// TestSurfaceZoomMatchesOriginal checks dense lookup, negative coordinates and pool reuse.
func TestSurfaceZoomMatchesOriginal(t *testing.T) {
	var zoom surfaceZoom
	for _, seed := range []int64{0, 987654321, -42} {
		for _, origin := range [][2]int{{0, 0}, {-32, -16}, {29999984, -30000000}} {
			for _, offset := range []int{0, 3} {
				source := patternedBiomes{offset: offset}
				system := &surfaceSystem{zoomSeed: seed, biomes: source}
				zoom.reset(system, origin[0], origin[1], -64, 384, source)
				for x := origin[0]; x < origin[0]+16; x++ {
					for z := origin[1]; z < origin[1]+16; z++ {
						for y := -64; y < 320; y++ {
							if got, want := zoom.biome(x, y, z), system.biomeAtBlock(x, y, z); got != want {
								t.Fatalf("seed %d position (%d,%d,%d): %d != %d", seed, x, y, z, got, want)
							}
						}
					}
				}
			}
		}
	}
}
