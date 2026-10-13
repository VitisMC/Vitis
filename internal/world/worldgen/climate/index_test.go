package climate

import "testing"

func TestPresetSizesAndKnownTargets(t *testing.T) {
	overworld, err := NewPreset("minecraft:overworld")
	if err != nil {
		t.Fatal(err)
	}
	if overworld.Len() != 7593 {
		t.Fatalf("overworld points = %d", overworld.Len())
	}
	if got := overworld.Find(NewTarget(0, 0, -1.1, 0, 0, 0)); got != "minecraft:mushroom_fields" {
		t.Fatalf("offshore biome = %s", got)
	}
	nether, err := NewPreset("minecraft:nether")
	if err != nil {
		t.Fatal(err)
	}
	if got := nether.Find(NewTarget(0.4, 0, 0, 0, 0, 0)); got != "minecraft:crimson_forest" {
		t.Fatalf("nether biome = %s", got)
	}
	queries := []struct {
		values [6]float64
		want   string
	}{
		{[6]float64{}, "minecraft:river"},
		{[6]float64{-0.8, 0.2, 0.4, -0.6, 0, 0.7}, "minecraft:frozen_peaks"},
		{[6]float64{0.9, -0.9, 0.1, 0.8, 1, 0.05}, "minecraft:mangrove_swamp"},
		{[6]float64{-0.2, 0.75, 0.9, -0.3, 0.5, -0.8}, "minecraft:lush_caves"},
	}
	for _, query := range queries {
		v := query.values
		if got := overworld.Find(NewTarget(v[0], v[1], v[2], v[3], v[4], v[5])); got != query.want {
			t.Errorf("overworld target %v = %s, want %s", v, got, query.want)
		}
	}
}

func TestIndexRejectsInvalidInput(t *testing.T) {
	if _, err := NewIndex(nil); err == nil {
		t.Fatal("empty index accepted")
	}
	if _, err := NewIndex([]Point{{Biome: "minecraft:test", Parameters: [7]Parameter{{Min: 1, Max: 0}}}}); err == nil {
		t.Fatal("reversed interval accepted")
	}
}

// TestIndexBiomesPreservesEncounterOrder checks distinct feature-order inputs.
func TestIndexBiomesPreservesEncounterOrder(t *testing.T) {
	index, err := NewIndex([]Point{{Biome: "minecraft:b"}, {Biome: "minecraft:a"}, {Biome: "minecraft:b"}})
	if err != nil {
		t.Fatal(err)
	}
	biomes := index.Biomes()
	if len(biomes) != 2 || biomes[0] != "minecraft:b" || biomes[1] != "minecraft:a" {
		t.Fatalf("biomes = %v", biomes)
	}
	biomes[0] = "changed"
	if index.Biomes()[0] != "minecraft:b" {
		t.Fatal("caller changed index biome order")
	}
}

func TestCursorMatchesUncachedClimateSearch(t *testing.T) {
	index, err := NewPreset("minecraft:overworld")
	if err != nil {
		t.Fatal(err)
	}
	var cursor Cursor
	for x := -20; x <= 20; x++ {
		for z := -20; z <= 20; z++ {
			target := NewTarget(float64(x)/20, float64(z)/20, float64(x+z)/40, float64(x-z)/40, 0, 0)
			if got, want := index.FindWithCursor(target, &cursor), index.Find(target); got != want {
				t.Fatalf("target %v: cursor biome %q, want %q", target, got, want)
			}
		}
	}
	nether, err := NewPreset("minecraft:nether")
	if err != nil {
		t.Fatal(err)
	}
	target := NewTarget(0.4, 0, 0, 0, 0, 0)
	if got, want := nether.FindWithCursor(target, &cursor), nether.Find(target); got != want {
		t.Fatalf("cursor reused with another index: biome %q, want %q", got, want)
	}
}
