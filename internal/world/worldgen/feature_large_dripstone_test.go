package worldgen

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// largeDripstoneInput mirrors the controlled stone and lava layers in the exporter.
func largeDripstoneInput(origin blockPosition, shape int) map[blockPosition]int32 {
	input := make(map[blockPosition]int32)
	stone := block.DefaultStateID("minecraft:stone")
	lava := block.DefaultStateID("minecraft:lava")
	threshold := 12
	if shape == 1 {
		threshold = 5
	}
	for x := origin.x - 20; x <= origin.x+20; x++ {
		for z := origin.z - 20; z <= origin.z+20; z++ {
			for offset := 0; offset <= 4; offset++ {
				input[blockPosition{x, origin.y + threshold + offset, z}] = stone
				input[blockPosition{x, origin.y - threshold - offset, z}] = stone
			}
			if shape == 3 {
				input[blockPosition{x, origin.y + threshold, z}] = lava
			}
		}
	}
	if shape == 2 {
		input[origin] = stone
	}
	return input
}

// largeDripstoneWorkspace exposes the same cave surface height to neighboring writes.
func largeDripstoneWorkspace(t *testing.T, origin blockPosition, shape int, input map[blockPosition]int32) *section.Chunk {
	t.Helper()
	chunk := section.NewChunkWithBounds(int32(origin.x>>4), int32(origin.z>>4), -64, 128, 0)
	for position, state := range input {
		if position.x>>4 == int(chunk.X) && position.z>>4 == int(chunk.Z) {
			chunk.SetBlock(position.x, position.y, position.z, state)
		}
	}
	read := func(x, y, z int) int32 { return input[blockPosition{x, y, z}] }
	height := func(x, z int, _ section.HeightmapKind) (int, bool) {
		if x < origin.x-20 || x > origin.x+20 || z < origin.z-20 || z > origin.z+20 {
			return -64, false
		}
		if shape == 1 {
			return origin.y + 10, true
		}
		return origin.y + 17, true
	}
	if err := chunk.UseFeatureRegion(read, height, make([]uint32, len(chunk.Sections)*section.BlocksPerSection)); err != nil {
		t.Fatal(err)
	}
	chunk.SetFeatureStep(8, 0)
	return chunk
}

// TestLargeDripstoneGeometryMatchesMinecraft1214 checks shapes and RNG tails.
func TestLargeDripstoneGeometryMatchesMinecraft1214(t *testing.T) {
	data, err := os.ReadFile("testdata/large_dripstone_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []pointedDripstoneFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 16 {
		t.Fatalf("large dripstone reference cases = %d, want 16", len(fixtures))
	}
	manager := &datapack.Manager{}
	if _, err := NewRuntime(manager); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/large_dripstone.json")
	if !ok {
		t.Fatal("missing large dripstone definition")
	}
	f, err := decodeLargeDripstoneFeature(manager.Current(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.chunkRadius != 5 {
		t.Fatalf("large dripstone standard source radius = %d, want 5", f.chunkRadius)
	}
	for _, fixture := range fixtures {
		origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
		input := largeDripstoneInput(origin, fixture.Shape)
		chunk := largeDripstoneWorkspace(t, origin, fixture.Shape, input)
		random := worldrandom.NewLegacy(fixture.Seed)
		if placed := f.placeFeature(nil, chunk, random, origin); placed != fixture.Placed {
			t.Fatalf("shape %d seed %d: placed %v, want %v", fixture.Shape, fixture.Seed, placed, fixture.Placed)
		}
		if tail := random.NextLong(); tail != fixture.Tail {
			t.Fatalf("shape %d seed %d: RNG tail %d, want %d", fixture.Shape, fixture.Seed, tail, fixture.Tail)
		}
		changes := make([]pointedDripstoneChange, 0)
		for x := origin.x - 20; x <= origin.x+20; x++ {
			for z := origin.z - 20; z <= origin.z+20; z++ {
				for y := origin.y - 30; y <= origin.y+30; y++ {
					position := blockPosition{x, y, z}
					state := getChunkBlock(chunk, position)
					if state == input[position] {
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
			t.Fatalf("shape %d seed %d: changed states = %v, want %v", fixture.Shape, fixture.Seed, changes, fixture.Changes)
		}
	}
}

// TestLargeDripstoneFarBorderWrite checks tall wind-shifted columns across chunks.
func TestLargeDripstoneFarBorderWrite(t *testing.T) {
	manager := &datapack.Manager{}
	if _, err := NewRuntime(manager); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := manager.Current().Get("data/minecraft/worldgen/configured_feature/large_dripstone.json")
	f, err := decodeLargeDripstoneFeature(manager.Current(), raw)
	if err != nil {
		t.Fatal(err)
	}
	f.radiusMin, f.radiusMax, f.maxRadiusHeightRatio = 19, 19, 1
	f.heightScale, f.ceilingBluntness, f.floorBluntness, f.windSpeed = constantFloat(2), constantFloat(.9), constantFloat(.9), constantFloat(.3)
	stone := block.DefaultStateID("minecraft:stone")
	origin := blockPosition{15, 100, 15}
	initial := func(x, y, z int) int32 {
		if x < origin.x-100 || x > origin.x+100 || z < origin.z-100 || z > origin.z+100 {
			return 0
		}
		if y >= 120 && y <= 140 || y >= 60 && y <= 80 {
			return stone
		}
		return 0
	}
	chunk := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 60; y <= 80; y++ {
				chunk.SetBlock(x, y, z, stone)
			}
			for y := 120; y <= 140; y++ {
				chunk.SetBlock(x, y, z, stone)
			}
		}
	}
	readHeight := func(x, z int, _ section.HeightmapKind) (int, bool) {
		if x < origin.x-100 || x > origin.x+100 || z < origin.z-100 || z > origin.z+100 {
			return -64, false
		}
		return 141, true
	}
	if err := chunk.UseFeatureRegionRadius(initial, readHeight, make([]uint32, len(chunk.Sections)*section.BlocksPerSection), 5); err != nil {
		t.Fatal(err)
	}
	chunk.SetFeatureStep(8, 0)
	chunk.SetFeatureWriteRadius(5)
	if !f.placeFeature(nil, chunk, worldrandom.NewLegacy(0), origin) {
		t.Fatal("large dripstone rejected synthetic cave")
	}
	var far section.BorderPosition
	var farWrite section.FeatureBorderWrite
	chunk.FeatureBorderWrites().Range(func(position section.BorderPosition, write section.FeatureBorderWrite) {
		if farWrite.Priority == 0 && (absInt(position.X>>4) > 1 || absInt(position.Z>>4) > 1) {
			far, farWrite = position, write
		}
	})
	if farWrite.Priority == 0 {
		t.Fatal("tall large dripstone produced no far-chunk writes")
	}
	target := section.NewChunkWithBounds(int32(far.X>>4), int32(far.Z>>4), -64, 384, 0)
	if err := target.UseFeatureRegionRadius(initial, readHeight, make([]uint32, len(target.Sections)*section.BlocksPerSection), 5); err != nil {
		t.Fatal(err)
	}
	mergeRegionBorders(target, 0, 0, &rawChunkEntry{writes: chunk.FeatureBorderWrites()})
	if state := target.GetBlock(far.X, far.Y, far.Z); state != farWrite.State {
		t.Fatalf("far write at %+v = %s, want %s", far, block.NameFromState(state), block.NameFromState(farWrite.State))
	}
}

// TestLargeDripstoneRegionSelection keeps dimensions without the feature narrow.
func TestLargeDripstoneRegionSelection(t *testing.T) {
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		dimension string
		radius    int
	}{{"minecraft:overworld", 5}, {"minecraft:nether", 1}, {"minecraft:end", 1}} {
		generator, err := runtime.NewTerrainGenerator(entry.dimension, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := generator.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.featureRadius != entry.radius {
			t.Fatalf("%s feature radius = %d, want %d", entry.dimension, snapshot.featureRadius, entry.radius)
		}
	}
}

// TestLargeDripstoneRejectsNonfiniteBounds avoids invalid reach calculations.
func TestLargeDripstoneRejectsNonfiniteBounds(t *testing.T) {
	if _, _, ok := largeDripstoneFloatBounds(uniformFloat{min: 0, max: float32(math.NaN())}); ok {
		t.Fatal("non-finite wind bound was accepted")
	}
}
