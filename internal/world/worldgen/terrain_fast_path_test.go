package worldgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// TestStandardTerrainMatchesGeneralGraph protects all generated stages and lighting.
func TestStandardTerrainMatchesGeneralGraph(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []int64{0, 123456789, -987654321} {
		for _, dimension := range []string{"minecraft:overworld", "minecraft:nether", "minecraft:end"} {
			t.Run(fmt.Sprintf("%s/seed=%d", dimension, seed), func(t *testing.T) {
				fast, err := runtime.NewTerrainGenerator(dimension, seed, 0)
				if err != nil {
					t.Fatal(err)
				}
				general, err := runtime.NewTerrainGenerator(dimension, seed, 0)
				if err != nil {
					t.Fatal(err)
				}
				fastSnapshot, err := fast.snapshot()
				if err != nil {
					t.Fatal(err)
				}
				if fastSnapshot.interpolation == nil {
					t.Fatal("standard definitions did not select cell filling")
				}
				snapshot, err := general.snapshot()
				if err != nil {
					t.Fatal(err)
				}
				// The specialized wrapper retains its general graph for scalar sampling.
				snapshot.interpolation = nil
				snapshot.surface.standard = false
				snapshot.surface.standardNether = false
				general.compiled = snapshot
				fastGenerator, err := chunk.NewWorldgenGenerator(fast)
				if err != nil {
					t.Fatal(err)
				}
				generalGenerator, err := chunk.NewWorldgenGenerator(general)
				if err != nil {
					t.Fatal(err)
				}
				coordinates := [][2]int32{{-1, -2}, {1, 0}}
				if dimension == "minecraft:end" {
					coordinates = append(coordinates, [2]int32{80, -80})
				}
				for _, coordinate := range coordinates {
					got, err := fastGenerator.Generate(coordinate[0], coordinate[1])
					if err != nil {
						t.Fatal(err)
					}
					want, err := generalGenerator.Generate(coordinate[0], coordinate[1])
					if err != nil {
						t.Fatal(err)
					}
					assertGeneratedContent(t, got.Data(), want.Data())
					encodedFast, err := chunk.EncodeChunkNBT(got)
					if err != nil {
						t.Fatal(err)
					}
					encodedGeneral, err := chunk.EncodeChunkNBT(want)
					if err != nil {
						t.Fatal(err)
					}
					decodedFast, err := chunk.DecodeChunkNBT(encodedFast, coordinate[0], coordinate[1], 0)
					if err != nil {
						t.Fatal(err)
					}
					decodedGeneral, err := chunk.DecodeChunkNBT(encodedGeneral, coordinate[0], coordinate[1], 0)
					if err != nil {
						t.Fatal(err)
					}
					assertGeneratedContent(t, decodedFast.Data(), decodedGeneral.Data())
					assertGeneratedContent(t, decodedFast.Data(), got.Data())
				}
			})
		}
	}
}

// TestReloadDuringParallelGeneration verifies immutable snapshots while workers overlap reloads.
func TestReloadDuringParallelGeneration(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	vanilla := "../../../.mcdata/1.21.4/vanilla"
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	reference, err := runtime.NewTerrainGenerator("minecraft:overworld", -1234, 0)
	if err != nil {
		t.Fatal(err)
	}
	coordinates := [][2]int32{{-2, -1}, {0, 0}, {1, 2}}
	expected := make([]*section.Chunk, len(coordinates))
	for index, position := range coordinates {
		expected[index], err = reference.Generate(position[0], position[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", -1234, 0)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for offset := range 3 {
				index := (worker + offset) % len(coordinates)
				position := coordinates[index]
				actual, err := generator.Generate(position[0], position[1])
				if err != nil {
					t.Error(err)
					return
				}
				assertGeneratedContent(t, actual, expected[index])
			}
		}()
	}
	close(start)
	for range 3 {
		if err := manager.Reload([]string{vanilla}); err != nil {
			t.Error(err)
			break
		}
	}
	workers.Wait()
}

// assertGeneratedContent compares blocks, biomes, light and persisted block entities.
func assertGeneratedContent(t *testing.T, got, want *section.Chunk) {
	t.Helper()
	if got.MinY() != want.MinY() || got.MaxY() != want.MaxY() {
		t.Fatal("dimension bounds changed")
	}
	for y := want.MinY(); y <= want.MaxY(); y++ {
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				if got.GetBlock(x, y, z) != want.GetBlock(x, y, z) {
					t.Fatalf("chunk %d,%d differs at %d,%d,%d: %d vs %d", got.X, got.Z, x, y, z, got.GetBlock(x, y, z), want.GetBlock(x, y, z))
				}
			}
		}
	}
	for index, current := range want.Sections {
		for y := 0; y < 4; y++ {
			for z := 0; z < 4; z++ {
				for x := 0; x < 4; x++ {
					if got.Sections[index].GetBiome(x, y, z) != current.GetBiome(x, y, z) {
						t.Fatal("biome changed")
					}
				}
			}
		}
		for i, value := range current.SkyLight {
			if got.Sections[index].SkyLight[i] != value {
				t.Fatal("sky light changed")
			}
		}
		for i, value := range current.BlockLight {
			if got.Sections[index].BlockLight[i] != value {
				t.Fatal("block light changed")
			}
		}
	}
	if len(got.BlockEntities) != len(want.BlockEntities) {
		t.Fatal("block entities changed")
	}
	for key, entity := range want.BlockEntities {
		if other := got.BlockEntities[key]; other == nil || other.TypeName() != entity.TypeName() || !reflect.DeepEqual(other.WriteNBT(), entity.WriteNBT()) {
			t.Fatal("block entity missing or changed")
		}
	}
}

// TestTerrainReloadSelectsGeneralAndRestoresFastPath validates snapshot-local selection.
func TestTerrainReloadSelectsGeneralAndRestoresFastPath(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	vanilla := "../../../.mcdata/1.21.4/vanilla"
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 42, 0)
	if err != nil {
		t.Fatal(err)
	}
	before, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if before.interpolation == nil {
		t.Fatal("standard terrain uses general path")
	}
	if _, err := generator.Generate(-1, -1); err != nil {
		t.Fatal(err)
	}
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"custom terrain"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(pack, "data", "minecraft", "worldgen", "noise_settings")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(before.resources.resources["noise_settings"]["minecraft:overworld"], &settings); err != nil {
		t.Fatal(err)
	}
	settings["sea_level"] = json.RawMessage(`70`)
	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "overworld.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err != nil {
		t.Fatal(err)
	}
	modified, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if modified == before || modified.interpolation != nil || modified.settings.SeaLevel != 70 {
		t.Fatal("reload did not select general path")
	}
	for key := range generator.rawCache {
		if key.snapshot == before {
			t.Fatal("reload retained completed terrain from an old resource set")
		}
	}
	if before.interpolation == nil || before.settings.SeaLevel != 63 {
		t.Fatal("reload mutated an existing snapshot")
	}
	if _, err := generator.Generate(-1, -1); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	restored, err := generator.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if restored == modified || restored.interpolation == nil {
		t.Fatal("standard reload did not restore fast path")
	}
}
