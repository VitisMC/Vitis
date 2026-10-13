package worldgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// icebergFixture records one reference placement and its next random value.
type icebergFixture struct {
	Material string                   `json:"material"`
	Shape    int                      `json:"shape"`
	Seed     int64                    `json:"seed"`
	Origin   [3]int                   `json:"origin"`
	Placed   bool                     `json:"placed"`
	Tail     int64                    `json:"tail"`
	Changes  []pointedDripstoneChange `json:"changes"`
}

// icebergFixtureState reproduces the reference ocean and its optional ice shelf.
func icebergFixtureState(origin, position blockPosition, shape int) int32 {
	if position.y <= 59 {
		return block.DefaultStateID("minecraft:stone")
	}
	if position.y <= 63 {
		return block.DefaultStateID("minecraft:water")
	}
	if shape == 1 && position.y == 64 && absInt(position.x-origin.x) <= 3 && absInt(position.z-origin.z) <= 3 {
		return block.DefaultStateID("minecraft:ice")
	}
	return 0
}

// icebergFixtureChunk prepares current and neighboring chunk reads at sea level.
func icebergFixtureChunk(t *testing.T, origin blockPosition, shape int) *section.Chunk {
	t.Helper()
	chunk := section.NewChunkWithBounds(int32(origin.x>>4), int32(origin.z>>4), -64, 192, 0)
	for x := int(chunk.X) * 16; x < int(chunk.X)*16+16; x++ {
		for z := int(chunk.Z) * 16; z < int(chunk.Z)*16+16; z++ {
			for y := chunk.MinY(); y <= 64; y++ {
				state := icebergFixtureState(origin, blockPosition{x, y, z}, shape)
				if state != 0 {
					chunk.SetBlock(x, y, z, state)
				}
			}
		}
	}
	read := func(x, y, z int) int32 { return icebergFixtureState(origin, blockPosition{x, y, z}, shape) }
	height := func(int, int, section.HeightmapKind) (int, bool) { return 64, true }
	if err := chunk.UseFeatureRegionRadius(read, height, make([]uint32, len(chunk.Sections)*section.BlocksPerSection), 1); err != nil {
		t.Fatal(err)
	}
	chunk.SetFeatureStep(7, 0)
	chunk.SetFeatureWriteRadius(1)
	return chunk
}

// TestIcebergGeometryMatchesMinecraft1214 compares all changed blocks and RNG tails.
func TestIcebergGeometryMatchesMinecraft1214(t *testing.T) {
	data, err := os.ReadFile("testdata/iceberg_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []icebergFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 16 {
		t.Fatalf("iceberg cases = %d, want 16", len(fixtures))
	}
	for _, fixture := range fixtures {
		origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
		chunk := icebergFixtureChunk(t, origin, fixture.Shape)
		random := worldrandom.NewLegacy(fixture.Seed)
		f := &icebergFeature{state: block.DefaultStateID(fixture.Material)}
		if placed := f.placeFeature(&featureSystem{seaLevel: 63}, chunk, random, origin); placed != fixture.Placed {
			t.Fatalf("%s shape %d seed %d: placed %v, want %v", fixture.Material, fixture.Shape, fixture.Seed, placed, fixture.Placed)
		}
		if tail := random.NextLong(); tail != fixture.Tail {
			t.Fatalf("%s shape %d seed %d: tail %d, want %d", fixture.Material, fixture.Shape, fixture.Seed, tail, fixture.Tail)
		}
		var changes []pointedDripstoneChange
		for x := origin.x - 15; x <= origin.x+15; x++ {
			for z := origin.z - 15; z <= origin.z+15; z++ {
				for y := 45; y <= 110; y++ {
					position := blockPosition{x, y, z}
					state := getChunkBlock(chunk, position)
					if state == icebergFixtureState(origin, position, fixture.Shape) {
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
		if !reflect.DeepEqual(changes, fixture.Changes) {
			limit := min(len(changes), len(fixture.Changes))
			for index := 0; index < limit; index++ {
				if !reflect.DeepEqual(changes[index], fixture.Changes[index]) {
					t.Fatalf("%s shape %d seed %d: change %d = %+v, want %+v", fixture.Material, fixture.Shape, fixture.Seed, index, changes[index], fixture.Changes[index])
				}
			}
			t.Fatalf("%s shape %d seed %d: %d changes, want %d", fixture.Material, fixture.Shape, fixture.Seed, len(changes), len(fixture.Changes))
		}
	}
}

// TestIcebergDecodeRejectsInvalidState catches malformed datapack configuration.
func TestIcebergDecodeRejectsInvalidState(t *testing.T) {
	if _, err := decodeIcebergFeature(json.RawMessage(`{"config":{"state":{"Name":"minecraft:missing_ice"}}}`)); err == nil {
		t.Fatal("invalid iceberg material accepted")
	}
}

// TestIcebergReloadReplacesCompiledState verifies resource snapshot isolation.
func TestIcebergReloadReplacesCompiledState(t *testing.T) {
	const vanilla = "../../../.mcdata/1.21.4/vanilla"
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	compiled := func() int32 {
		t.Helper()
		feature, ok := runtime.current().features.configured["minecraft:iceberg_blue"].(*icebergFeature)
		if !ok {
			t.Fatal("iceberg not compiled")
		}
		return feature.state
	}
	blue := block.DefaultStateID("minecraft:blue_ice")
	packed := block.DefaultStateID("minecraft:packed_ice")
	if got := compiled(); got != blue {
		t.Fatalf("initial iceberg state = %d, want %d", got, blue)
	}
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"iceberg reload"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(pack, "data", "minecraft", "worldgen", "configured_feature")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "iceberg_blue.json"),
		[]byte(`{"type":"minecraft:iceberg","config":{"state":{"Name":"minecraft:packed_ice"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{vanilla, pack}); err != nil {
		t.Fatal(err)
	}
	if got := compiled(); got != packed {
		t.Fatalf("reloaded iceberg state = %d, want %d", got, packed)
	}
	if err := manager.Reload([]string{vanilla}); err != nil {
		t.Fatal(err)
	}
	if got := compiled(); got != blue {
		t.Fatalf("restored iceberg state = %d, want %d", got, blue)
	}
}
