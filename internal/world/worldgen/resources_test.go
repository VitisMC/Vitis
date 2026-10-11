package worldgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
)

// TestRuntimeLoadsVanillaWorldgen verifies the complete generated registry set.
func TestRuntimeLoadsVanillaWorldgen(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	if runtime.Count() != 916 {
		t.Fatalf("worldgen resource count = %d", runtime.Count())
	}
	overworld, ok := runtime.NoiseSettings("minecraft:overworld")
	if !ok || overworld.Noise.MinY != -64 || overworld.Noise.Height != 384 || overworld.DefaultBlock.Name != "minecraft:stone" {
		t.Fatalf("overworld settings = %+v", overworld)
	}
	nether, ok := runtime.NoiseSettings("minecraft:nether")
	if !ok || nether.Noise.MinY != 0 || nether.Noise.Height != 128 || nether.DefaultBlock.Name != "minecraft:netherrack" {
		t.Fatalf("nether settings = %+v", nether)
	}
	netherType, ok := runtime.DimensionType("minecraft:the_nether")
	if !ok || netherType.MinY != 0 || netherType.Height != 256 || netherType.CoordinateScale != 8 {
		t.Fatalf("Nether dimension type = %+v", netherType)
	}
	dimensions := runtime.Dimensions()
	if len(dimensions) != 3 || dimensions[0] != "minecraft:overworld" || dimensions[1] != "minecraft:the_end" || dimensions[2] != "minecraft:the_nether" {
		t.Fatalf("dimensions = %v", dimensions)
	}
	set := runtime.current()
	if len(set.carvers) != 4 {
		t.Fatalf("configured carvers = %d", len(set.carvers))
	}
	if len(set.structureSets) != 20 {
		t.Fatalf("structure sets = %d", len(set.structureSets))
	}
	if got := set.biomeCarvers["minecraft:plains"]; len(got) != 3 || got[0] != "minecraft:cave" || got[2] != "minecraft:canyon" {
		t.Fatalf("plains carvers = %v", got)
	}
	if set.features == nil || len(set.features.placed) != 235 {
		t.Fatalf("compiled feature placements = %d", len(set.features.placed))
	}
	for _, id := range []string{"minecraft:flower_forest_flowers", "minecraft:forest_flowers", "minecraft:flower_plain", "minecraft:ice_spike", "minecraft:trees_plains", "minecraft:trees_birch_and_oak", "minecraft:trees_taiga", "minecraft:trees_savanna", "minecraft:trees_jungle", "minecraft:trees_swamp", "minecraft:trees_cherry", "minecraft:rooted_azalea_tree", "minecraft:mega_jungle_tree_checked", "minecraft:mega_pine_checked", "minecraft:mega_spruce_checked", "minecraft:mangrove_checked", "minecraft:tall_mangrove_checked", "minecraft:dark_forest_vegetation", "minecraft:mushroom_island_vegetation", "minecraft:lush_caves_vegetation", "minecraft:lush_caves_ceiling_vegetation", "minecraft:warm_ocean_vegetation"} {
		if set.features.placed[id] == nil {
			t.Fatalf("missing compiled feature placement %s", id)
		}
	}
	orePlacements := 0
	for _, placed := range set.features.placed {
		if _, ok := placed.feature.(*configuredOre); ok {
			orePlacements++
		}
	}
	if orePlacements != 40 {
		t.Fatalf("compiled ore placements = %d", orePlacements)
	}
	undergroundOres := set.features.steps[6]
	if _, ok := undergroundOres.index["minecraft:ore_diamond"]; len(undergroundOres.ids) == 0 || !ok {
		t.Fatalf("underground ore feature order is missing: %v", undergroundOres.ids)
	}
	caveEntrance, ok := runtime.NoiseParameters("minecraft:cave_entrance")
	if !ok || caveEntrance.FirstOctave != -7 || len(caveEntrance.Amplitudes) != 3 || caveEntrance.Amplitudes[2] != 1 {
		t.Fatalf("cave entrance noise = %+v", caveEntrance)
	}
	caveEntrance.Amplitudes[2] = 99
	unchanged, _ := runtime.NoiseParameters("minecraft:cave_entrance")
	if unchanged.Amplitudes[2] != 1 {
		t.Fatal("worldgen noise parameters are mutable")
	}
}

func TestCustomNoiseDimensionLoads(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"custom dimension test"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(pack, "data", "test", "dimension")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	definition := `{
		"type":"minecraft:the_end",
		"generator":{"type":"minecraft:noise","settings":"minecraft:end","biome_source":{"type":"minecraft:the_end"}}
	}`
	if err := os.WriteFile(filepath.Join(directory, "moon.json"), []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla", pack}); err != nil {
		t.Fatal(err)
	}
	dimension, ok := runtime.Dimension("test:moon")
	if !ok || dimension.Type != "minecraft:the_end" || dimension.Generator.Settings != "minecraft:end" {
		t.Fatalf("custom dimension = %+v", dimension)
	}
	generator, err := runtime.NewDimensionTerrainGenerator("test:moon", 42, 0)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := generator.Generate(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if generated.MinY() != 0 || generated.MaxY() != 255 {
		t.Fatalf("custom dimension bounds = %d..%d", generated.MinY(), generated.MaxY())
	}
}

func TestDensityValidationKeepsPreviousSnapshot(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	vanilla := "../../../.mcdata/1.21.4/vanilla"
	if err := resources.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	previousCount := runtime.Count()

	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"invalid density test"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	definitionDir := filepath.Join(pack, "data", "test", "worldgen", "density_function")
	if err := os.MkdirAll(definitionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(definitionDir, "broken.json"), []byte(`"test:missing"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{vanilla, pack}); err == nil {
		t.Fatal("expected invalid density reference error")
	}
	if runtime.Count() != previousCount {
		t.Fatalf("failed reload changed resource count from %d to %d", previousCount, runtime.Count())
	}
}

func TestVanillaDensityGraphsCompile(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	compiler, err := runtime.NewDensityCompiler("minecraft:overworld", 123456789)
	if err != nil {
		t.Fatal(err)
	}
	set := runtime.current()
	ids := make([]string, 0, len(set.resources["density_function"]))
	for id := range set.resources["density_function"] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := compiler.Compile(id); err != nil {
			t.Fatalf("compile %s: %v", id, err)
		}
	}
	if len(ids) != 35 {
		t.Fatalf("density function count = %d", len(ids))
	}

	for _, settingsID := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		settings := set.noiseSettings[settingsID]
		compiler, err := runtime.NewDensityCompiler(settingsID, 123456789)
		if err != nil {
			t.Fatal(err)
		}
		var router map[string]json.RawMessage
		if err := json.Unmarshal(settings.NoiseRouter, &router); err != nil {
			t.Fatal(err)
		}
		for name, data := range router {
			if _, err := compiler.CompileInline(data); err != nil {
				t.Fatalf("compile %s router field %s: %v", settingsID, name, err)
			}
		}
	}
}
