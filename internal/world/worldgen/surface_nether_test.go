package worldgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
)

// TestGeneratedNetherSurfaceMatchesInterpreter checks every Nether biome and surface branch inputs.
func TestGeneratedNetherSurfaceMatchesInterpreter(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []int64{0, -123456789, 987654321} {
		generator, err := runtime.NewTerrainGenerator("minecraft:nether", seed, 0)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := generator.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		surface := snapshot.surface
		if !surface.standardNether {
			t.Fatal("standard Nether surface not selected")
		}
		for _, biome := range []int32{generatedbiome.BiomeNetherWastes, generatedbiome.BiomeSoulSandValley, generatedbiome.BiomeCrimsonForest, generatedbiome.BiomeWarpedForest, generatedbiome.BiomeBasaltDeltas} {
			for _, coordinate := range [][2]int{{0, 0}, {-32, 48}, {-30000000, 29999999}} {
				for y := 0; y < 256; y++ {
					for _, depth := range []int{0, 1, 3, 7} {
						for _, water := range []int{noWaterHeight, 32} {
							context := surfaceContext{system: surface, x: coordinate[0], z: coordinate[1], y: y, biome: biome, biomeSet: true, surfaceDepth: depth, surfaceSecondary: .3, waterHeight: water, stoneDepthAbove: 1 + y%9, stoneDepthBelow: 1 + (255-y)%7, columnConditions: make([]uint8, surface.columnConditionCount)}
							expected, expectedOK := surface.rule.apply(&context)
							clear(context.columnConditions)
							got, gotOK := applyStandardNetherSurface(&context)
							if got != expected || gotOK != expectedOK {
								t.Fatalf("seed=%d biome=%d xyz=%d,%d,%d depth=%d water=%d generated=(%d,%t) interpreted=(%d,%t)", seed, biome, context.x, y, context.z, depth, water, got, gotOK, expected, expectedOK)
							}
						}
					}
				}
			}
		}
	}
}

// TestNetherSurfaceReloadFallsBackAndRestoresGeneratedRules protects modified datapack settings.
func TestNetherSurfaceReloadFallsBackAndRestoresGeneratedRules(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	vanilla := "../../../.mcdata/1.21.4/vanilla"
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:nether", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := generator.snapshot()
	if err != nil || !initial.surface.standardNether {
		t.Fatalf("initial Nether path: %v", err)
	}
	pack := t.TempDir()
	directory := filepath.Join(pack, "data", "minecraft", "worldgen", "noise_settings")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"Nether surface fallback test"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, ok := runtime.Resource("noise_settings", "minecraft:nether")
	if !ok {
		t.Fatal("Nether settings missing")
	}
	var definition map[string]json.RawMessage
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	definition["surface_rule"] = json.RawMessage(`{"type":"minecraft:block","result_state":{"Name":"minecraft:diamond_block"}}`)
	raw, err = json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "nether.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err != nil {
		t.Fatal(err)
	}
	modified, err := generator.snapshot()
	if err != nil || modified.surface.standardNether || modified == initial {
		t.Fatalf("modified Nether path: %v", err)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	restored, err := generator.snapshot()
	if err != nil || !restored.surface.standardNether || restored == modified {
		t.Fatalf("restored Nether path: %v", err)
	}
}
