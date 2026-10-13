package worldgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// geodeFixture keeps the exported random and world seeds independent.
type geodeFixture struct {
	Shape     int                      `json:"shape"`
	WorldSeed int64                    `json:"worldSeed"`
	Seed      int64                    `json:"seed"`
	Origin    [3]int                   `json:"origin"`
	Placed    bool                     `json:"placed"`
	Tail      int64                    `json:"tail"`
	Changes   []pointedDripstoneChange `json:"changes"`
}

// geodeFixtureState reproduces the controlled solid, wet, invalid, and protected terrain.
func geodeFixtureState(origin, position blockPosition, shape int) int32 {
	x, y, z := position.x-origin.x, position.y-origin.y, position.z-origin.z
	if shape == 2 {
		return 0
	}
	if shape == 1 && x >= -3 && x <= 3 && y >= -3 && y <= 3 && z >= -3 && z <= 3 {
		if y == 0 {
			return block.DefaultStateID("minecraft:water")
		}
		return 0
	}
	if shape == 3 && x >= 4 && x <= 6 && y >= 4 && y <= 6 && z >= 4 && z <= 6 {
		return block.DefaultStateID("minecraft:bedrock")
	}
	return block.DefaultStateID("minecraft:stone")
}

// geodeFixtureChunk installs neighboring reads and owns the origin column's initial blocks.
func geodeFixtureChunk(t *testing.T, origin blockPosition, shape int) *section.Chunk {
	t.Helper()
	chunk := section.NewChunkWithBounds(int32(origin.x>>4), int32(origin.z>>4), -64, 128, 0)
	for x := int(chunk.X) * 16; x < int(chunk.X)*16+16; x++ {
		for z := int(chunk.Z) * 16; z < int(chunk.Z)*16+16; z++ {
			for y := chunk.MinY(); y < chunk.MinY()+128; y++ {
				state := geodeFixtureState(origin, blockPosition{x, y, z}, shape)
				if state != 0 {
					chunk.SetBlock(x, y, z, state)
				}
			}
		}
	}
	read := func(x, y, z int) int32 { return geodeFixtureState(origin, blockPosition{x, y, z}, shape) }
	height := func(int, int, section.HeightmapKind) (int, bool) { return 64, true }
	if err := chunk.UseFeatureRegionRadius(read, height, make([]uint32, len(chunk.Sections)*section.BlocksPerSection), 2); err != nil {
		t.Fatal(err)
	}
	chunk.SetFeatureStep(7, 0)
	chunk.SetFeatureWriteRadius(2)
	return chunk
}

// TestGeodeGeometryMatchesMinecraft1214 checks every changed state and random tail.
func TestGeodeGeometryMatchesMinecraft1214(t *testing.T) {
	data, err := os.ReadFile("testdata/geode_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []geodeFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 16 {
		t.Fatalf("geode cases = %d, want 16", len(fixtures))
	}
	manager := &datapack.Manager{}
	if _, err := NewRuntime(manager); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/amethyst_geode.json")
	if !ok {
		t.Fatal("missing geode definition")
	}
	f, err := decodeGeodeFeature(manager.Current(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.chunkRadius != 2 {
		t.Fatalf("geode source radius = %d, want 2", f.chunkRadius)
	}
	for _, fixture := range fixtures {
		origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
		chunk := geodeFixtureChunk(t, origin, fixture.Shape)
		random := worldrandom.NewLegacy(fixture.Seed)
		if placed := f.placeFeature(&featureSystem{seed: fixture.WorldSeed}, chunk, random, origin); placed != fixture.Placed {
			t.Fatalf("shape %d world %d seed %d: placed %v, want %v", fixture.Shape, fixture.WorldSeed, fixture.Seed, placed, fixture.Placed)
		}
		if tail := random.NextLong(); tail != fixture.Tail {
			t.Fatalf("shape %d world %d seed %d: tail %d, want %d", fixture.Shape, fixture.WorldSeed, fixture.Seed, tail, fixture.Tail)
		}
		var changes []pointedDripstoneChange
		for x := origin.x - 17; x <= origin.x+17; x++ {
			for z := origin.z - 17; z <= origin.z+17; z++ {
				for y := origin.y - 17; y <= origin.y+17; y++ {
					position := blockPosition{x, y, z}
					state := getChunkBlock(chunk, position)
					if state == geodeFixtureState(origin, position, fixture.Shape) {
						continue
					}
					properties := block.PropertiesFromState(state)
					if properties == nil {
						properties = map[string]string{}
					}
					changes = append(changes, pointedDripstoneChange{[3]int{x, y, z}, block.NameFromState(state), properties})
				}
			}
		}
		if len(changes) != len(fixture.Changes) || len(changes) > 0 && !reflect.DeepEqual(changes, fixture.Changes) {
			limit := min(len(changes), len(fixture.Changes))
			for index := 0; index < limit; index++ {
				if !reflect.DeepEqual(changes[index], fixture.Changes[index]) {
					t.Fatalf("shape %d world %d seed %d: change %d = %+v, want %+v", fixture.Shape, fixture.WorldSeed, fixture.Seed, index, changes[index], fixture.Changes[index])
				}
			}
			t.Fatalf("shape %d world %d seed %d: %d changes, want %d", fixture.Shape, fixture.WorldSeed, fixture.Seed, len(changes), len(fixture.Changes))
		}
	}
}

// TestGeodeRejectsUnsafeRegionBounds rejects unbounded datapack offsets.
func TestGeodeRejectsUnsafeRegionBounds(t *testing.T) {
	manager := &datapack.Manager{}
	if _, err := NewRuntime(manager); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/amethyst_geode.json")
	var definition map[string]any
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	definition["config"].(map[string]any)["max_gen_offset"] = 128
	modified, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeGeodeFeature(manager.Current(), modified); err == nil {
		t.Fatal("unsafe geode offsets accepted")
	}
}

// TestGeodeSourceSelection keeps two-chunk geodes separate from five-chunk dripstone.
func TestGeodeSourceSelection(t *testing.T) {
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
	if snapshot.farFeatureBiomes["minecraft:plains"] != 2 || snapshot.farFeatureBiomes["minecraft:dripstone_caves"] != 5 {
		t.Fatalf("unexpected far source radii: plains=%d dripstone=%d", snapshot.farFeatureBiomes["minecraft:plains"], snapshot.farFeatureBiomes["minecraft:dripstone_caves"])
	}
	for _, entry := range []struct {
		biome    int32
		distance int
		want     bool
	}{
		{generatedbiome.BiomePlains, 2, true},
		{generatedbiome.BiomePlains, 3, false},
		{generatedbiome.BiomeDripstoneCaves, 5, true},
	} {
		selected := &terrainSnapshot{
			features:         newFeatureSystem(nil, 0, -64, 384, 63, fixedBiomeSource{biome: entry.biome}),
			farFeatureBiomes: snapshot.farFeatureBiomes,
		}
		if got := selected.sourceMayWriteFar(-4, 7, entry.distance); got != entry.want {
			t.Fatalf("biome %d distance %d selected=%v, want %v", entry.biome, entry.distance, got, entry.want)
		}
	}
}

// TestGeodeReloadRebuildsSourceSelection updates far-source filtering after a datapack override.
func TestGeodeReloadRebuildsSourceSelection(t *testing.T) {
	const vanilla = "../../../.mcdata/1.21.4/vanilla"
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	before, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if before.farUnfiltered != 0 {
		t.Fatalf("standard geode far fallback = %d", before.farUnfiltered)
	}
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"geode selection"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(pack, "data", "minecraft", "worldgen", "placed_feature")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/placed_feature/amethyst_geode.json")
	if !ok {
		t.Fatal("missing placed geode")
	}
	var placed struct {
		Feature   string            `json:"feature"`
		Placement []json.RawMessage `json:"placement"`
	}
	if err := json.Unmarshal(raw, &placed); err != nil {
		t.Fatal(err)
	}
	placed.Placement = placed.Placement[:len(placed.Placement)-1]
	modified, err := json.Marshal(placed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "amethyst_geode.json"), modified, 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err != nil {
		t.Fatal(err)
	}
	updated, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if updated == before || updated.farUnfiltered != 2 || before.farUnfiltered != 0 {
		t.Fatalf("geode reload source radius: old=%d new=%d", before.farUnfiltered, updated.farUnfiltered)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	restored, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if restored == updated || restored.farUnfiltered != 0 {
		t.Fatalf("restored geode far source fallback = %d", restored.farUnfiltered)
	}
}
