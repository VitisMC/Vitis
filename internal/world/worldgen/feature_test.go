package worldgen

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestCompiledFeatureBiomeMembership verifies numeric membership against the resource lists.
func TestCompiledFeatureBiomeMembership(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	resources := snapshot.features.resources
	for id, placed := range resources.placed {
		if placed.allowedBiomes == nil {
			t.Fatal("compiled placement has no biome table")
		}
		for _, biome := range generatedbiome.Biomes {
			want := biomeHasFeature(resources.biomeFeatures[biome.Name], id)
			if got := placed.acceptsBiome(resources, id, biome.ID); got != want {
				t.Fatalf("feature %s biome %s: %v != %v", id, biome.Name, got, want)
			}
		}
		if placed.acceptsBiome(resources, id, -1) || placed.acceptsBiome(resources, id, generatedbiome.BiomeCount) {
			t.Fatal("invalid biome accepted")
		}
		unbound := &placedFeature{}
		if unbound.acceptsBiome(resources, id, generatedbiome.BiomePlains) != placed.acceptsBiome(resources, id, generatedbiome.BiomePlains) {
			t.Fatal("unbound placement fallback changed membership")
		}
	}
}

type recordingFeature struct {
	positions []blockPosition
}

// placeFeature records each candidate and consumes feature RNG.
func (f *recordingFeature) placeFeature(_ *featureSystem, _ *section.Chunk, random worldrandom.Source, position blockPosition) bool {
	f.positions = append(f.positions, position)
	random.NextIntBound(7)
	return true
}

// TestPlacedFeatureInterleavesModifierAndFeatureRandomness checks stream order.
func TestPlacedFeatureInterleavesModifierAndFeatureRandomness(t *testing.T) {
	feature := &recordingFeature{}
	placed := &placedFeature{feature: feature, placement: []featurePlacement{
		{kind: "minecraft:count", count: constantInt(2)},
		{kind: "minecraft:in_square"},
		{kind: "minecraft:height_range", height: featureHeightProvider{min: verticalAnchor{offset: 0}, max: verticalAnchor{offset: 9}}},
	}}
	random := worldrandom.NewLegacy(42)
	if !placed.place(&featureSystem{minY: 0, height: 20}, nil, "", random, blockPosition{}, nil, nil) {
		t.Fatal("feature was not placed")
	}
	expectedRandom := worldrandom.NewLegacy(42)
	for i, got := range feature.positions {
		want := blockPosition{x: int(expectedRandom.NextIntBound(16)), z: int(expectedRandom.NextIntBound(16))}
		want.y = int(expectedRandom.NextIntBound(10))
		expectedRandom.NextIntBound(7)
		if got != want {
			t.Fatalf("candidate %d = %+v, want %+v", i, got, want)
		}
	}
	if len(feature.positions) != 2 {
		t.Fatalf("candidate count = %d", len(feature.positions))
	}
}

// TestOreEarlySurfaceCheckPreservesRandomSequence checks a rejected ore attempt.
func TestOreEarlySurfaceCheckPreservesRandomSequence(t *testing.T) {
	chunk := section.NewChunkWithBounds(0, 0, 0, 128, 0)
	if !oreMayIntersectSurface(chunk, blockPosition{x: 8, y: 0, z: 8}, 4) {
		t.Fatal("empty column below world minimum was incorrectly rejected")
	}
	feature := &configuredOre{size: 4}
	random := worldrandom.NewLegacy(73)
	if feature.place(chunk, random, blockPosition{x: 8, y: 100, z: 8}) {
		t.Fatal("ore placed above an empty column")
	}
	expected := worldrandom.NewLegacy(73)
	expected.NextFloat()
	expected.NextIntBound(3)
	expected.NextIntBound(3)
	if got, want := random.NextLong(), expected.NextLong(); got != want {
		t.Fatalf("next random value = %d, want %d", got, want)
	}
}

// TestClampedIntProviderAllowsSourceOutsideOutputBounds checks nested sampling.
func TestClampedIntProviderAllowsSourceOutsideOutputBounds(t *testing.T) {
	raw := json.RawMessage(`{"type":"minecraft:clamped","min_inclusive":0,"max_inclusive":3,"source":{"type":"minecraft:uniform","min_inclusive":-1,"max_inclusive":3}}`)
	provider, err := decodeIntProviderRange(raw, 0, 256)
	if err != nil {
		t.Fatal(err)
	}
	random := worldrandom.NewLegacy(42)
	for i := 0; i < 100; i++ {
		if value := provider.sample(random); value < 0 || value > 3 {
			t.Fatalf("clamped value = %d", value)
		}
	}
}

// TestFlowerFeaturesCompile verifies every built-in flower variant is executable.
func TestFlowerFeaturesCompile(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"flower_cherry", "flower_default", "flower_flower_forest", "flower_meadow",
		"flower_pale_garden", "flower_plain", "flower_swamp",
	} {
		raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
		if !ok {
			t.Fatalf("missing configured feature %s", name)
		}
		feature, supported, err := decodeConfiguredFeature(manager.Current(), raw)
		if err != nil || !supported {
			t.Fatalf("compile %s: supported=%t, error=%v", name, supported, err)
		}
		if _, ok := feature.(*randomPatchFeature); !ok {
			t.Fatalf("%s compiled as %T", name, feature)
		}
	}
}

// TestFeatureStepRejectsConflictingBiomeOrder checks cyclic feature dependencies.
func TestFeatureStepRejectsConflictingBiomeOrder(t *testing.T) {
	biomes := []string{"minecraft:first", "minecraft:second"}
	features := map[string][][]string{
		"minecraft:first":  {{"minecraft:a", "minecraft:b"}},
		"minecraft:second": {{"minecraft:b", "minecraft:a"}},
	}
	if _, err := sortFeatureStep(biomes, features, 0); err == nil || !strings.Contains(err.Error(), "conflicting biome feature order") {
		t.Fatalf("conflicting order error = %v", err)
	}
	features["minecraft:second"] = [][]string{{"minecraft:a", "minecraft:b"}}
	step, err := sortFeatureStep(biomes, features, 0)
	if err != nil || len(step.ids) != 2 || step.ids[0] != "minecraft:a" || step.ids[1] != "minecraft:b" {
		t.Fatalf("valid order = %v, error = %v", step.ids, err)
	}
}

// TestFeatureSorterUsesReversePostorder checks seed indices for independent features.
func TestFeatureSorterUsesReversePostorder(t *testing.T) {
	features := map[string][][]string{
		"minecraft:first":  {{"minecraft:a"}, {"minecraft:c"}},
		"minecraft:second": {{"minecraft:b"}, {"minecraft:d"}},
	}
	steps, err := sortFeatureSteps([]string{"minecraft:first", "minecraft:second"}, features, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps[0].ids) != 2 || steps[0].ids[0] != "minecraft:b" || steps[0].ids[1] != "minecraft:a" {
		t.Fatalf("first step order = %v", steps[0].ids)
	}
	if len(steps[1].ids) != 2 || steps[1].ids[0] != "minecraft:d" || steps[1].ids[1] != "minecraft:c" {
		t.Fatalf("second step order = %v", steps[1].ids)
	}
}

// TestFeatureOrderUsesOnlySourceBiomes checks that unrelated biomes do not shift seed indices.
func TestFeatureOrderUsesOnlySourceBiomes(t *testing.T) {
	resources := &featureSystemResources{biomeFeatures: map[string][][]string{
		"minecraft:target": {{"minecraft:a", "minecraft:b"}},
		"minecraft:other":  {{"minecraft:c"}},
	}, placed: map[string]*placedFeature{}}
	ordered, err := resources.forBiomeOrder([]string{"minecraft:target"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ordered.steps[0].ids; len(got) != 2 || got[0] != "minecraft:a" || got[1] != "minecraft:b" {
		t.Fatalf("feature order = %v", got)
	}
	if _, err := resources.forBiomeOrder([]string{"minecraft:missing"}); err == nil {
		t.Fatal("missing biome accepted")
	}
}

// TestOrePlacementIsDeterministic verifies seeded vein placement and target replacement.
func TestOrePlacementIsDeterministic(t *testing.T) {
	stone, err := block.DefaultBlockState("minecraft:stone")
	if err != nil {
		t.Fatal(err)
	}
	diamond, err := block.DefaultBlockState("minecraft:diamond_ore")
	if err != nil {
		t.Fatal(err)
	}
	ore := configuredOre{size: 8, targets: []oreTarget{{states: map[int32]struct{}{stone.ID(): {}}, result: diamond.ID()}}}
	generate := func() *section.Chunk {
		chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				for y := -64; y < 64; y++ {
					chunk.SetBlock(x, y, z, stone.ID())
				}
			}
		}
		if !ore.place(chunk, worldrandom.NewXoroshiro(918273645), blockPosition{x: 8, y: -20, z: 8}) {
			t.Fatal("ore feature placed no blocks")
		}
		return chunk
	}
	first, second := generate(), generate()
	count := 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := -64; y < 64; y++ {
				if first.GetBlock(x, y, z) != second.GetBlock(x, y, z) {
					t.Fatalf("non-deterministic ore at %d,%d,%d", x, y, z)
				}
				if first.GetBlock(x, y, z) == diamond.ID() {
					count++
				}
			}
		}
	}
	if count == 0 || count > ore.size*8 {
		t.Fatalf("diamond count = %d", count)
	}
}

// TestFeatureHeightProvidersResolveDimensionAnchors verifies uniform and triangular ranges.
func TestFeatureHeightProvidersResolveDimensionAnchors(t *testing.T) {
	random := worldrandom.NewLegacy(42)
	uniform := featureHeightProvider{kind: "minecraft:uniform", min: verticalAnchor{kind: 1, offset: 8}, max: verticalAnchor{kind: 2, offset: 8}}
	for i := 0; i < 100; i++ {
		if y := uniform.sample(random, -64, 384); y < -56 || y > 311 {
			t.Fatalf("uniform height = %d", y)
		}
	}
	triangle := featureHeightProvider{kind: "minecraft:trapezoid", min: verticalAnchor{offset: -64}, max: verticalAnchor{offset: 32}}
	for i := 0; i < 100; i++ {
		if y := triangle.sample(random, -64, 384); y < -64 || y > 32 {
			t.Fatalf("trapezoid height = %d", y)
		}
	}
}
