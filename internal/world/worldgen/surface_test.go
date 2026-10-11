package worldgen

import (
	"sync/atomic"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
)

type countingDensity struct{ calls *atomic.Int32 }

func (f countingDensity) Sample(density.Position) float64 {
	f.calls.Add(1)
	return 1
}

func TestStandardSurfaceRulesCompile(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, settings := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		if _, err := runtime.NewTerrainGenerator(settings, 1234, generatedbiome.BiomePlains); err != nil {
			t.Fatalf("compile %s surface: %v", settings, err)
		}
	}
}

func TestBiomeZoomIsStableAcrossNegativeCoordinates(t *testing.T) {
	system := &surfaceSystem{seed: -9123456789, biomes: fixedBiomeSource{biome: generatedbiome.BiomeDesert}}
	for _, position := range [][3]int{{0, 0, 0}, {-1, -64, -1}, {-30000000, 255, 29999999}} {
		if got := system.biomeAtBlock(position[0], position[1], position[2]); got != generatedbiome.BiomeDesert {
			t.Fatalf("biome at %v = %d", position, got)
		}
	}
}

func TestPreliminarySurfaceCornersAreComputedOncePerChunk(t *testing.T) {
	var calls atomic.Int32
	system := &surfaceSystem{
		minY:       -64,
		height:     384,
		cellHeight: 8,
		initial:    countingDensity{calls: &calls},
	}
	levels := system.preliminarySurfaceCorners(-32, 48)
	for index, level := range levels {
		if level != 320 {
			t.Fatalf("corner %d = %d, want 320", index, level)
		}
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("density samples = %d, want 4", got)
	}
}

func TestSurfaceHeightsMatchBlockScan(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 64, generatedbiome.BiomePlains)
	stone := block.DefaultStateID("minecraft:stone")
	caveAir := block.DefaultStateID("minecraft:cave_air")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			chunk.SetBlock(x, 3+(x+z)%19, z, stone)
		}
	}
	chunk.SetBlock(1, 52, 1, caveAir)
	chunk.SetBlock(2, 63, 2, stone)
	got := surfaceHeights(chunk)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			want := chunk.MinY() - 1
			for y := chunk.MaxY(); y >= chunk.MinY(); y-- {
				if !isAirState(chunk.GetBlock(x, y, z)) {
					want = y
					break
				}
			}
			if got[x][z] != want {
				t.Fatalf("height at (%d,%d) = %d, want %d", x, z, got[x][z], want)
			}
			if column := surfaceColumnHeight(chunk, x, z); column != want {
				t.Fatalf("single column height (%d,%d) = %d, want %d", x, z, column, want)
			}
		}
	}
}

func TestFiddleHandlesNegativeSeeds(t *testing.T) {
	for _, seed := range []int64{-1 << 63, -123456789, -1, 0, 1, 123456789, 1<<63 - 1} {
		value := (seed >> 24) % 1024
		if value < 0 {
			value += 1024
		}
		want := (float64(value)/1024 - 0.5) * 0.9
		if got := fiddle(seed); got != want {
			t.Fatalf("fiddle(%d) = %v, want %v", seed, got, want)
		}
	}
}
