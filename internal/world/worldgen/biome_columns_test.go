package worldgen

import (
	"github.com/vitismc/vitis/internal/datapack"
	"testing"
)

// TestClimateColumnCacheMatchesIndependentSampling verifies halo, height and worker reuse.
func TestClimateColumnCacheMatchesIndependentSampling(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	var cached *cachedBiomeSource
	for _, seed := range []int64{0, -123456789} {
		generator, err := runtime.NewTerrainGenerator("minecraft:overworld", seed, 0)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := generator.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		source := snapshot.biomes.(*climateBiomeSource)
		if !source.horizontal {
			t.Fatal("standard climate did not enable column sampling")
		}
		if cached == nil {
			cached = newCachedBiomeSource(source)
		} else {
			cached.reset(source)
		}
		for _, origin := range [][2]int{{-32, -16}, {0, 0}, {29999968, -29999968}} {
			cached.reset(source)
			cached.climateColumns = make(map[[2]int]struct{})
			cached.beginChunk(origin[0], origin[1], -64, 384)
			for x := origin[0]/4 - 2; x <= origin[0]/4+5; x++ {
				for z := origin[1]/4 - 2; z <= origin[1]/4+5; z++ {
					for _, y := range []int{-17, -16, -8, 0, 8, 40, 79, 80} {
						if got, want := cached.sample(x, y, z), source.sample(x, y, z); got != want {
							t.Fatalf("seed %d quart (%d,%d,%d): %d != %d", seed, x, y, z, got, want)
						}
					}
				}
			}
			if cached.climateSamples != 512 || len(cached.climateColumns) != 64 || cached.climateComputations != 260 {
				t.Fatalf("climate diagnostics samples=%d columns=%d evaluations=%d", cached.climateSamples, len(cached.climateColumns), cached.climateComputations)
			}
			cached.sample(origin[0]/4, 0, origin[1]/4)
			if cached.climateSamples != 512 || cached.climateComputations != 260 {
				t.Fatal("a cached biome repeated its climate evaluation")
			}
		}
	}
}
