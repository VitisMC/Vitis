package worldgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// TestBiomeCarversAcceptInlineAndReferencedEntries verifies mixed placement order.
func TestBiomeCarversAcceptInlineAndReferencedEntries(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	inline := runtime.current().resources["configured_carver"]["minecraft:cave"]
	biome := append([]byte(`{"carvers":[`), inline...)
	biome = append(biome, []byte(`,"minecraft:canyon"]}`)...)
	set := &ResourceSet{resources: map[string]map[string]json.RawMessage{
		"configured_carver": runtime.current().resources["configured_carver"],
		"biome":             {"minecraft:test": biome},
	}}
	if err := compileCarverResources(manager.Current(), set); err != nil {
		t.Fatal(err)
	}
	got := set.biomeCarvers["minecraft:test"]
	if len(got) != 2 || got[1] != "minecraft:canyon" || set.carvers[got[0]] == nil {
		t.Fatalf("mixed carver entries = %v", got)
	}
	if set.carvers[got[0]].kind != "minecraft:cave" {
		t.Fatalf("inline carver kind = %q", set.carvers[got[0]].kind)
	}
}

// TestUniformCarversMatchBiomeSelection preserves cave output and seed ordering.
func TestUniformCarversMatchBiomeSelection(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
		t.Run(dimension, func(t *testing.T) {
			generator, err := runtime.NewTerrainGenerator(dimension, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := generator.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !snapshot.carvers.uniform {
				t.Fatal("standard source has nonuniform carvers")
			}
			generic := newCarverSystem(snapshot.resources, snapshot.settings, 0, snapshot.biomes, snapshot.aquifer, snapshot.surface)
			generic.uniform = false
			for _, coordinate := range [][2]int32{{-1, -2}, {0, 0}} {
				base := section.NewChunkWithBounds(coordinate[0], coordinate[1], snapshot.minY, snapshot.height, 0)
				generator.generateTerrainRange(snapshot, base, int(coordinate[0])*16, int(coordinate[1])*16, snapshot.settings.Noise.MinY, snapshot.settings.Noise.MinY+snapshot.settings.Noise.Height)
				fast := base.CloneRaw()
				snapshot.carvers.apply(fast, coordinate[0], coordinate[1])
				generic.apply(base, coordinate[0], coordinate[1])
				for y := base.MinY(); y <= base.MaxY(); y++ {
					for z := 0; z < 16; z++ {
						for x := 0; x < 16; x++ {
							if fast.GetBlock(x, y, z) != base.GetBlock(x, y, z) {
								t.Fatalf("carver differs at %v: %d,%d,%d", coordinate, x, y, z)
							}
						}
					}
				}
			}
		})
	}
}

// TestMixedBiomeCarversKeepGenericSelection rejects uniform specialization for custom lists.
func TestMixedBiomeCarversKeepGenericSelection(t *testing.T) {
	resources := &ResourceSet{biomeCarvers: map[string][]string{
		"minecraft:plains": {"minecraft:cave"}, "minecraft:desert": {"minecraft:canyon"},
	}}
	source := checkerboardBiomeSource{biomes: []int32{generatedbiome.BiomePlains, generatedbiome.BiomeDesert}}
	if newCarverSystem(resources, NoiseSettings{}, 0, source, aquiferConfig{}, nil).uniform {
		t.Fatal("different biome lists selected uniform carvers")
	}
}

// TestInvalidInlineCarverKeepsPreviousSnapshot verifies atomic resource reload.
func TestInvalidInlineCarverKeepsPreviousSnapshot(t *testing.T) {
	manager := &datapack.Manager{}
	if _, err := NewRuntime(manager); err != nil {
		t.Fatal(err)
	}
	vanilla := "../../../.mcdata/1.21.4/vanilla"
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	previous := manager.Current()
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"invalid carver"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	biomeDir := filepath.Join(pack, "data", "minecraft", "worldgen", "biome")
	if err := os.MkdirAll(biomeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(biomeDir, "plains.json"), []byte(`{"carvers":[{"type":"minecraft:unknown"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err == nil || !strings.Contains(err.Error(), "biome minecraft:plains carver 0") {
		t.Fatalf("invalid carver reload error = %v", err)
	}
	if manager.Current() != previous {
		t.Fatal("failed reload replaced the previous resource snapshot")
	}
}

// TestBiomeCarverRejectsInvalidInlineEntry verifies contextual reload errors.
func TestBiomeCarverRejectsInvalidInlineEntry(t *testing.T) {
	set := &ResourceSet{resources: map[string]map[string]json.RawMessage{
		"biome": {"minecraft:test": json.RawMessage(`{"carvers":[{"type":"minecraft:unknown"}]}`)},
	}}
	err := compileCarverResources(nil, set)
	if err == nil || !strings.Contains(err.Error(), "biome minecraft:test carver 0") {
		t.Fatalf("invalid inline carver error = %v", err)
	}
}
