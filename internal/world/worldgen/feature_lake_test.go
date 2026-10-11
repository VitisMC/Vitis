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

type lakeChange struct {
	Position [3]int `json:"position"`
	Block    string `json:"block"`
}

type lakeFixture struct {
	Seed    int64        `json:"seed"`
	Origin  [3]int       `json:"origin"`
	Placed  bool         `json:"placed"`
	Tail    int64        `json:"tail"`
	Changes []lakeChange `json:"changes"`
}

// loadVanillaLake compiles the shipped lake and confirms both placements are active.
func loadVanillaLake(t *testing.T) *lakeFeature {
	t.Helper()
	manager := &datapack.Manager{}
	runtime, err := NewRuntime(manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	resources := runtime.current().features
	feature, ok := resources.configured["minecraft:lake_lava"].(*lakeFeature)
	if !ok {
		t.Fatal("lava lake is not compiled")
	}
	for _, id := range []string{"minecraft:lake_lava_surface", "minecraft:lake_lava_underground"} {
		if resources.placed[id] == nil {
			t.Fatalf("lake placement %s is missing", id)
		}
	}
	return feature
}

// lakeStoneChunk gives the feature solid terrain on either side of a chunk border.
func lakeStoneChunk(t *testing.T, origin blockPosition) *section.Chunk {
	t.Helper()
	stone := block.DefaultStateID("minecraft:stone")
	c := section.NewChunkWithBounds(int32(origin.x>>4), int32(origin.z>>4), -64, 128, 0)
	for x := int(c.X) * 16; x < int(c.X)*16+16; x++ {
		for z := int(c.Z) * 16; z < int(c.Z)*16+16; z++ {
			for y := origin.y - 4; y < origin.y+4; y++ {
				c.SetBlock(x, y, z, stone)
			}
		}
	}
	if err := c.UseFeatureRegion(func(int, int, int) int32 { return stone }, func(int, int, section.HeightmapKind) (int, bool) { return -64, false }, make([]uint32, len(c.Sections)*section.BlocksPerSection)); err != nil {
		t.Fatal(err)
	}
	c.SetFeatureStep(8, 0)
	return c
}

// TestLakeLavaBorderAndValidation checks cross-chunk carving and atomic shell rejection.
func TestLakeLavaBorderAndValidation(t *testing.T) {
	f := loadVanillaLake(t)
	origin := blockPosition{15, -20, -1}
	first := lakeStoneChunk(t, origin)
	second := lakeStoneChunk(t, origin)
	for _, c := range []*section.Chunk{first, second} {
		if !f.placeFeature(nil, c, worldrandom.NewLegacy(0), origin) {
			t.Fatal("lava lake was rejected in solid terrain")
		}
	}
	lava := block.DefaultStateID("minecraft:lava")
	foundLava, foundBorder := false, false
	for y := origin.y - 4; y < origin.y+4; y++ {
		for x := origin.x; x < origin.x+16; x++ {
			for z := origin.z; z < origin.z+16; z++ {
				p := blockPosition{x, y, z}
				if getChunkBlock(first, p) != getChunkBlock(second, p) {
					t.Fatalf("non-deterministic block at %+v", p)
				}
				if getChunkBlock(first, p) == lava {
					foundLava = true
				}
				if p.x>>4 != int(first.X) || p.z>>4 != int(first.Z) {
					if _, ok := first.GetBorderBlock(x, y, z); ok {
						foundBorder = true
					}
				}
			}
		}
	}
	if !foundLava || !foundBorder {
		t.Fatalf("missing lava=%v or border writes=%v", foundLava, foundBorder)
	}
	tooLow := lakeStoneChunk(t, origin)
	if f.placeFeature(nil, tooLow, worldrandom.NewLegacy(0), blockPosition{origin.x, tooLow.MinY() + 4, origin.z}) {
		t.Fatal("lake accepted origin below the minimum height")
	}
	rejectOrigin := blockPosition{0, -20, 0}
	rejected := lakeStoneChunk(t, rejectOrigin)
	water := block.DefaultStateID("minecraft:water")
	for x := int(rejected.X) * 16; x < int(rejected.X)*16+16; x++ {
		for z := int(rejected.Z) * 16; z < int(rejected.Z)*16+16; z++ {
			for y := rejectOrigin.y; y < rejectOrigin.y+4; y++ {
				rejected.SetBlock(x, y, z, water)
			}
		}
	}
	if f.placeFeature(nil, rejected, worldrandom.NewLegacy(0), rejectOrigin) {
		t.Fatal("lake accepted a water-filled upper shell")
	}
	borderWrites := 0
	rejected.FeatureBorderWrites().Range(func(section.BorderPosition, section.FeatureBorderWrite) { borderWrites++ })
	if borderWrites != 0 {
		t.Fatal("rejected lake wrote into neighboring chunks")
	}
}

// TestLakeLavaProtectedBlocks checks immutable tag restrictions in a carved cell and its rim.
func TestLakeLavaProtectedBlocks(t *testing.T) {
	f := loadVanillaLake(t)
	origin := blockPosition{0, -20, 0}
	probe := lakeStoneChunk(t, origin)
	if !f.placeFeature(nil, probe, worldrandom.NewLegacy(0), origin) {
		t.Fatal("reference lake was rejected")
	}
	lava := block.DefaultStateID("minecraft:lava")
	var core, rim blockPosition
	foundCore, foundRim := false, false
	for x := 1; x < 14; x++ {
		for z := 1; z < 14; z++ {
			for y := origin.y - 3; y < origin.y; y++ {
				p := blockPosition{x, y, z}
				if getChunkBlock(probe, p) != lava {
					continue
				}
				core, foundCore = p, true
				for _, offset := range featureFaceOffsets {
					candidate := blockPosition{p.x + offset.x, p.y + offset.y, p.z + offset.z}
					if getChunkBlock(probe, candidate) == block.DefaultStateID("minecraft:stone") {
						rim, foundRim = candidate, true
						break
					}
				}
				if foundRim {
					break
				}
			}
			if foundRim {
				break
			}
		}
		if foundRim {
			break
		}
	}
	if !foundCore || !foundRim {
		t.Fatal("reference lake lacks a local core and rim")
	}
	chunk := lakeStoneChunk(t, origin)
	bedrock := block.DefaultStateID("minecraft:bedrock")
	log := block.DefaultStateID("minecraft:oak_log")
	chunk.SetBlock(core.x, core.y, core.z, bedrock)
	chunk.SetBlock(rim.x, rim.y, rim.z, log)
	if !f.placeFeature(nil, chunk, worldrandom.NewLegacy(0), origin) {
		t.Fatal("protected blocks rejected the whole lake")
	}
	if got := getChunkBlock(chunk, core); got != bedrock {
		t.Fatalf("protected core = %s", block.NameFromState(got))
	}
	if got := getChunkBlock(chunk, rim); got != log {
		t.Fatalf("excluded rim = %s", block.NameFromState(got))
	}
}

// TestLakeLavaUpperShellUsesBlockLiquidFlag distinguishes liquid blocks from waterlogged solids.
func TestLakeLavaUpperShellUsesBlockLiquidFlag(t *testing.T) {
	f := loadVanillaLake(t)
	origin := blockPosition{0, -20, 0}
	probe := lakeStoneChunk(t, origin)
	if !f.placeFeature(nil, probe, worldrandom.NewLegacy(0), origin) {
		t.Fatal("reference lake was rejected")
	}
	stone := block.DefaultStateID("minecraft:stone")
	caveAir := block.DefaultStateID("minecraft:cave_air")
	var shell blockPosition
	found := false
	for x := 1; x < 14 && !found; x++ {
		for z := 1; z < 14 && !found; z++ {
			for y := origin.y; y < origin.y+3 && !found; y++ {
				p := blockPosition{x, y, z}
				if getChunkBlock(probe, p) != caveAir {
					continue
				}
				for _, offset := range featureFaceOffsets {
					candidate := blockPosition{p.x + offset.x, p.y + offset.y, p.z + offset.z}
					if candidate.y >= origin.y && getChunkBlock(probe, candidate) == stone {
						shell, found = candidate, true
						break
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("reference lake lacks an upper shell cell")
	}
	bubble := lakeStoneChunk(t, origin)
	bubble.SetBlock(shell.x, shell.y, shell.z, block.DefaultStateID("minecraft:bubble_column"))
	if f.placeFeature(nil, bubble, worldrandom.NewLegacy(0), origin) {
		t.Fatal("bubble column did not reject the upper shell")
	}
	fenceState, err := block.BlockStateFromID(block.DefaultStateID("minecraft:oak_fence"))
	if err != nil {
		t.Fatal(err)
	}
	fenceState, err = fenceState.WithProperty("waterlogged", "true")
	if err != nil {
		t.Fatal(err)
	}
	fence := lakeStoneChunk(t, origin)
	fence.SetBlock(shell.x, shell.y, shell.z, fenceState.ID())
	if !f.placeFeature(nil, fence, worldrandom.NewLegacy(0), origin) {
		t.Fatal("waterlogged solid rejected the upper shell")
	}
}

// TestLakeLavaReloadKeepsSnapshotTags verifies immutable replacement rules across datapack reloads.
func TestLakeLavaReloadKeepsSnapshotTags(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	old := manager.Current()
	raw, ok := old.Get("data/minecraft/worldgen/configured_feature/lake_lava.json")
	if !ok {
		t.Fatal("missing lake definition")
	}
	before, err := decodeLakeFeature(old, raw)
	if err != nil {
		t.Fatal(err)
	}
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"lake tags"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	tags := filepath.Join(pack, "data", "minecraft", "tags", "block")
	if err := os.MkdirAll(tags, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tags, "features_cannot_replace.json"), []byte(`{"replace":true,"values":["minecraft:stone"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla", pack}); err != nil {
		t.Fatal(err)
	}
	after, err := decodeLakeFeature(manager.Current(), raw)
	if err != nil {
		t.Fatal(err)
	}
	stone := block.InfoByName("minecraft:stone").ID
	bedrock := block.InfoByName("minecraft:bedrock").ID
	if before.protected[stone] || !before.protected[bedrock] || !after.protected[stone] || after.protected[bedrock] {
		t.Fatal("lake protection leaked across resource snapshots")
	}
}

// TestLakeLavaRejectsUnsupportedFluid prevents incomplete water-lake behavior from entering generation.
func TestLakeLavaRejectsUnsupportedFluid(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"config":{}}`,
		`{"config":{"fluid":{"type":"minecraft:simple_state_provider","state":{"Name":"minecraft:water"}},"barrier":{"type":"minecraft:simple_state_provider","state":{"Name":"minecraft:stone"}}}}`,
	} {
		if _, err := decodeLakeFeature(manager.Current(), json.RawMessage(input)); err == nil {
			t.Fatalf("unsupported lake configuration accepted: %s", input)
		}
	}
}

// TestLakeLavaMatchesMinecraft1214 compares final geometry and random streams with reference execution.
func TestLakeLavaMatchesMinecraft1214(t *testing.T) {
	data, err := os.ReadFile("testdata/lake_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []lakeFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 32 {
		t.Fatalf("reference lake cases = %d, want 32", len(fixtures))
	}
	f := loadVanillaLake(t)
	stone := block.DefaultStateID("minecraft:stone")
	for _, fixture := range fixtures {
		origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
		c := lakeStoneChunk(t, origin)
		random := worldrandom.NewLegacy(fixture.Seed)
		if got := f.placeFeature(nil, c, random, origin); got != fixture.Placed {
			t.Fatalf("seed %d origin %+v: placed %v, want %v", fixture.Seed, origin, got, fixture.Placed)
		}
		if tail := random.NextLong(); tail != fixture.Tail {
			t.Fatalf("seed %d origin %+v: RNG tail %d, want %d", fixture.Seed, origin, tail, fixture.Tail)
		}
		changes := make([]lakeChange, 0)
		for x := origin.x; x < origin.x+16; x++ {
			for z := origin.z; z < origin.z+16; z++ {
				for y := origin.y - 4; y < origin.y+4; y++ {
					state := getChunkBlock(c, blockPosition{x, y, z})
					if state == stone {
						continue
					}
					changes = append(changes, lakeChange{[3]int{x, y, z}, block.NameFromState(state)})
				}
			}
		}
		if !reflect.DeepEqual(changes, fixture.Changes) {
			t.Fatalf("seed %d origin %+v: changed blocks = %d, want %d; first difference: %v", fixture.Seed, origin, len(changes), len(fixture.Changes), firstLakeDifference(changes, fixture.Changes))
		}
	}
}

// firstLakeDifference reports the earliest mismatched reference entry.
func firstLakeDifference(left, right []lakeChange) any {
	for i := 0; i < min(len(left), len(right)); i++ {
		if left[i] != right[i] {
			return []any{i, left[i], right[i]}
		}
	}
	return []int{len(left), len(right)}
}
