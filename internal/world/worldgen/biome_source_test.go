package worldgen

import (
	"encoding/json"
	"testing"

	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
)

type countingBiomeSource struct {
	calls  int
	offset int32
}

func (s *countingBiomeSource) sample(quartX, quartY, quartZ int) int32 {
	s.calls++
	return s.offset + int32(quartX+quartY+quartZ)
}

func TestCompileFixedAndCheckerboardBiomeSources(t *testing.T) {
	fixed, err := compileBiomeSource(nil, json.RawMessage(`{"type":"minecraft:fixed","biome":"minecraft:desert"}`), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := fixed.sample(-100, 20, 400); got != generatedbiome.BiomeDesert {
		t.Fatalf("fixed biome = %d", got)
	}
	if names := possibleBiomeNames(fixed); len(names) != 1 || names[0] != "minecraft:desert" {
		t.Fatalf("fixed possible biomes = %v", names)
	}
	checkerboard, err := compileBiomeSource(nil, json.RawMessage(`{"type":"minecraft:checkerboard","biomes":["minecraft:plains","minecraft:desert"],"scale":0}`), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := checkerboard.sample(0, 0, 0); got != generatedbiome.BiomePlains {
		t.Fatalf("checkerboard origin = %d", got)
	}
	if got := checkerboard.sample(-1, 0, 0); got != generatedbiome.BiomeDesert {
		t.Fatalf("checkerboard negative coordinate = %d", got)
	}
	if names := possibleBiomeNames(checkerboard); len(names) != 2 || names[0] != "minecraft:plains" || names[1] != "minecraft:desert" {
		t.Fatalf("checkerboard possible biomes = %v", names)
	}
	if names := possibleBiomeNames(endBiomeSource{}); len(names) != 5 || names[0] != "minecraft:the_end" || names[4] != "minecraft:end_barrens" {
		t.Fatalf("end possible biomes = %v", names)
	}
}

func TestDecodeClimateParameter(t *testing.T) {
	point, err := decodeClimateParameter(json.RawMessage(`{"min":-0.45,"max":0.2}`))
	if err != nil {
		t.Fatal(err)
	}
	if point.Min != -4500 || point.Max != 2000 {
		t.Fatalf("interval = %+v", point)
	}
	if _, err := decodeClimateParameter(json.RawMessage(`{"min":1,"max":-1}`)); err == nil {
		t.Fatal("reversed interval accepted")
	}
}

func TestCachedBiomeSourceSamplesEachQuartOnce(t *testing.T) {
	source := &countingBiomeSource{}
	cached := newCachedBiomeSource(source)
	for range 3 {
		if got := cached.sample(-4, 7, 12); got != 15 {
			t.Fatalf("cached biome = %d, want 15", got)
		}
	}
	if got := cached.sample(-3, 7, 12); got != 16 {
		t.Fatalf("second biome = %d, want 16", got)
	}
	if source.calls != 2 {
		t.Fatalf("source calls = %d, want 2", source.calls)
	}
}

func TestCachedBiomeSourceResetDiscardsPreviousSamples(t *testing.T) {
	first := &countingBiomeSource{}
	cached := newCachedBiomeSource(first)
	if got := cached.sample(2, 3, 4); got != 9 {
		t.Fatalf("initial biome = %d, want 9", got)
	}
	second := fixedBiomeSource{biome: 42}
	cached.reset(second)
	if got := cached.sample(2, 3, 4); got != 42 {
		t.Fatalf("biome after reset = %d, want 42", got)
	}
	if first.calls != 1 {
		t.Fatalf("previous source called %d times, want 1", first.calls)
	}
}

// TestDenseBiomeCacheBoundsAndReuse covers negative columns and adjacent quart samples.
func TestDenseBiomeCacheBoundsAndReuse(t *testing.T) {
	source := &countingBiomeSource{offset: 128}
	cached := newCachedBiomeSource(source)
	cached.beginChunk(-32, 64, -64, 384)
	positions := [][3]int{{-8, -16, 16}, {-5, 79, 19}, {-9, -16, 16}, {-8, 80, 16}, {-8, -17, 16}, {-8, -16, 20}}
	for range 3 {
		for _, p := range positions {
			if got, want := cached.sample(p[0], p[1], p[2]), 128+int32(p[0]+p[1]+p[2]); got != want {
				t.Fatalf("sample %v = %d, want %d", p, got, want)
			}
		}
	}
	if source.calls != len(positions) {
		t.Fatalf("source calls = %d, want %d", source.calls, len(positions))
	}
	cached.reset(fixedBiomeSource{biome: 42})
	cached.beginChunk(16, -16, 0, 256)
	if got := cached.sample(4, 0, -4); got != 42 {
		t.Fatalf("reused cache retained previous biome: %d", got)
	}
}
