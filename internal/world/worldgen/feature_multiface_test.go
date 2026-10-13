package worldgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// multifaceFixture records actual versioned Minecraft writes and the remaining random stream.
type multifaceFixture struct {
	Name          string
	Seed          int64
	Origin        [3]int
	Feature       json.RawMessage
	From, Toward  string
	Input, Writes []multifaceFixtureState
	Placed        bool
	Tail          int64
}

// multifaceFixtureState identifies a block by canonical registry properties rather than numeric IDs.
type multifaceFixtureState struct {
	Position   [3]int
	Name       string
	Properties map[string]string
}

// multifaceFixtures loads an independently exported reference corpus and its required block tags.
func multifaceFixtures(t *testing.T) ([]multifaceFixture, *datapack.Snapshot) {
	t.Helper()
	data, err := os.ReadFile("testdata/multiface_1_21_4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []multifaceFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 362 {
		t.Fatalf("reference cases = %d, want 362", len(fixtures))
	}
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	return fixtures, manager.Current()
}

// multifaceStates resolves fixture states without depending on Minecraft's state numbering.
func multifaceStates(t *testing.T, states []multifaceFixtureState) map[blockPosition]int32 {
	t.Helper()
	result := make(map[blockPosition]int32, len(states))
	for _, entry := range states {
		state, err := block.BlockStateFromProperties(entry.Name, entry.Properties)
		if err != nil {
			t.Fatal(err)
		}
		result[blockPosition{entry.Position[0], entry.Position[1], entry.Position[2]}] = state.ID()
	}
	return result
}

// multifaceWorkspace creates an owned chunk with an immutable sparse neighboring terrain view.
func multifaceWorkspace(t *testing.T, x, z int32, input map[blockPosition]int32) *section.Chunk {
	t.Helper()
	c := section.NewChunkWithBounds(x, z, -64, 128, 0)
	for p, state := range input {
		if p.x>>4 == int(x) && p.z>>4 == int(z) {
			c.SetBlock(p.x, p.y, p.z, state)
		}
	}
	read := func(x, y, z int) int32 { return input[blockPosition{x, y, z}] }
	height := func(int, int, section.HeightmapKind) (int, bool) { return -64, false }
	if err := c.UseFeatureRegion(read, height, make([]uint32, len(c.Sections)*section.BlocksPerSection)); err != nil {
		t.Fatal(err)
	}
	c.SetFeatureStep(8, 0)
	return c
}

// runMultifaceFixture compares exact writes, success and RNG, including unexpected local mutations.
func runMultifaceFixture(t *testing.T, fixture multifaceFixture, f *multifaceGrowthFeature) *section.Chunk {
	t.Helper()
	input := multifaceStates(t, fixture.Input)
	want := multifaceStates(t, fixture.Writes)
	origin := blockPosition{fixture.Origin[0], fixture.Origin[1], fixture.Origin[2]}
	c := multifaceWorkspace(t, int32(origin.x>>4), int32(origin.z>>4), input)
	random := worldrandom.NewLegacy(fixture.Seed)
	var placed bool
	if fixture.From == "" {
		placed = f.placeFeature(nil, c, random, origin)
	} else {
		from, ok := parseFeatureFace(fixture.From)
		if !ok {
			t.Fatal("invalid fixture source face")
		}
		toward, ok := parseFeatureFace(fixture.Toward)
		if !ok {
			t.Fatal("invalid fixture spread direction")
		}
		placed = f.spreadDirection(c, origin, getChunkBlock(c, origin), from, toward)
	}
	if placed != fixture.Placed {
		t.Fatalf("placed = %v, want %v", placed, fixture.Placed)
	}
	if tail := random.NextLong(); tail != fixture.Tail {
		t.Fatalf("RNG tail = %d, want %d", tail, fixture.Tail)
	}
	got := make(map[blockPosition]int32)
	for x := int(c.X) * 16; x < int(c.X)*16+16; x++ {
		for z := int(c.Z) * 16; z < int(c.Z)*16+16; z++ {
			for y := c.MinY(); y <= c.MaxY(); y++ {
				p := blockPosition{x, y, z}
				if state := c.GetBlock(x, y, z); state != input[p] {
					got[p] = state
				}
			}
		}
	}
	c.FeatureBorderWrites().Range(func(p section.BorderPosition, write section.FeatureBorderWrite) {
		got[blockPosition{p.X, p.Y, p.Z}] = write.State
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	return c
}

// TestMultifaceMatchesMinecraft1214 checks placement and all three spreading modes concurrently.
func TestMultifaceMatchesMinecraft1214(t *testing.T) {
	fixtures, snapshot := multifaceFixtures(t)
	compiled := make(map[string]*multifaceGrowthFeature)
	for _, fixture := range fixtures {
		key := string(fixture.Feature)
		f := compiled[key]
		if f == nil {
			var err error
			f, err = decodeMultifaceGrowthFeature(snapshot, fixture.Feature)
			if err != nil {
				t.Fatal(err)
			}
			compiled[key] = f
		}
		t.Run(fmt.Sprintf("%s/%s/seed=%d", block.NameFromState(f.base), fixture.Name, fixture.Seed), func(t *testing.T) {
			t.Parallel()
			runMultifaceFixture(t, fixture, f)
		})
	}
}

// TestMultifaceBorderWaterAndLightRoundTrip checks ownership, waterlogging and recomputed saved lighting.
func TestMultifaceBorderWaterAndLightRoundTrip(t *testing.T) {
	fixtures, snapshot := multifaceFixtures(t)
	count := 0
	for _, fixture := range fixtures {
		if fixture.Name != "spread-2-1" {
			continue
		}
		count++
		f, err := decodeMultifaceGrowthFeature(snapshot, fixture.Feature)
		if err != nil {
			t.Fatal(err)
		}
		source := runMultifaceFixture(t, fixture, f)
		input := multifaceStates(t, fixture.Input)
		target := multifaceWorkspace(t, 1, -1, input)
		mergeRegionBorders(target, source.X, source.Z, &rawChunkEntry{writes: source.FeatureBorderWrites()})
		p := blockPosition{16, -20, -2}
		mask, same := f.mask(target.GetBlock(p.x, p.y, p.z))
		if !same || mask != 64|1<<featureSouth {
			t.Fatal("wrapped neighboring growth lost source water or attachment")
		}
		target.DropBorderBlocks()
		target.ComputeLight()
		level := target.LightEngine.GetBlockLightSection((p.y-target.MinY())/16+1).GetXYZ(p.x&15, p.y&15, p.z&15)
		want := uint8(7)
		if f.vein {
			want = 0
		}
		if level != want {
			t.Fatalf("emission = %d, want %d", level, want)
		}
		owned := chunk.New(target.X, target.Z)
		owned.SetData(target)
		encoded, err := chunk.EncodeChunkNBT(owned)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := chunk.DecodeChunkNBTWithBounds(encoded, target.X, target.Z, 0, -64, 128)
		if err != nil {
			t.Fatal(err)
		}
		loaded.Data().ComputeLight()
		assertGeneratedContent(t, loaded.Data(), target)
	}
	if count != 2 {
		t.Fatal("both block types were not tested")
	}
}

// TestMultifaceConfigurationValidation rejects malformed definitions and supports empty targets/defaults.
func TestMultifaceConfigurationValidation(t *testing.T) {
	valid := `{"type":"minecraft:multiface_growth","config":{"can_be_placed_on":[]}}`
	if _, err := decodeMultifaceGrowthFeature(nil, json.RawMessage(valid)); err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{
		`{}`, `{"can_be_placed_on":null}`, `{"can_be_placed_on":4}`,
		`{"can_be_placed_on":["minecraft:missing"]}`, `{"can_be_placed_on":"#minecraft:missing"}`,
		`{"can_be_placed_on":[],"search_range":0}`, `{"can_be_placed_on":[],"search_range":65}`,
		`{"can_be_placed_on":[],"chance_of_spreading":-0.1}`, `{"can_be_placed_on":[],"chance_of_spreading":1.1}`,
		`{"can_be_placed_on":[],"block":"minecraft:vine"}`, `{"can_be_placed_on":[],"block":"minecraft:sculk_vein"}`,
	} {
		if _, err := decodeMultifaceGrowthFeature(nil, json.RawMessage(`{"config":`+config+`}`)); err == nil {
			t.Fatalf("invalid configuration accepted: %s", config)
		}
	}
}

// TestMultifaceReloadKeepsSnapshotTags isolates attachment and fire rules across resource publication.
func TestMultifaceReloadKeepsSnapshotTags(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	old := manager.Current()
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"pack_format":61,"description":"multiface tags"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	tags := filepath.Join(pack, "data", "minecraft", "tags", "block")
	if err := os.MkdirAll(tags, 0755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"fire":              `{"replace":true,"values":["minecraft:short_grass"]}`,
		"multiface_support": `{"values":["minecraft:mud"]}`,
	} {
		if err := os.WriteFile(filepath.Join(tags, name+".json"), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla", pack}); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"config":{"block":"minecraft:sculk_vein","can_be_placed_on":["minecraft:stone"]}}`)
	a, err := decodeMultifaceGrowthFeature(old, raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeMultifaceGrowthFeature(manager.Current(), raw)
	if err != nil {
		t.Fatal(err)
	}
	grass, fire := block.InfoByName("minecraft:short_grass").ID, block.InfoByName("minecraft:fire").ID
	if a.fire[grass] || !a.fire[fire] || !b.fire[grass] || b.fire[fire] {
		t.Fatal("fire exclusions leaked across snapshots")
	}
	raw = json.RawMessage(`{"config":{"can_be_placed_on":"#minecraft:multiface_support"}}`)
	if _, err := decodeMultifaceGrowthFeature(old, raw); err == nil {
		t.Fatal("new tag appeared in old snapshot")
	}
	f, err := decodeMultifaceGrowthFeature(manager.Current(), raw)
	if err != nil || !f.targets[block.InfoByName("minecraft:mud").ID] {
		t.Fatalf("new attachment tag not resolved: %v", err)
	}
}

// TestGenerationTraitsInventory validates table bounds and logically necessary fluid relationships.
func TestGenerationTraitsInventory(t *testing.T) {
	if len(genblock.GenerationTraits) != genblock.TotalStates {
		t.Fatal("incomplete state traits")
	}
	for state, traits := range genblock.GenerationTraits {
		if traits>>22 != 0 || traits&featureWaterTrait != 0 && traits&featureFluidTrait == 0 ||
			traits&featureWaterSourceTrait != 0 && traits&featureWaterTrait == 0 {
			t.Fatalf("invalid traits for state %d: %x", state, traits)
		}
	}
	if generationBlockTraits(-1) != 0 || generationBlockTraits(genblock.TotalStates) != 0 {
		t.Fatal("invalid state accepted")
	}
	waterlogged, err := block.BlockStateFromID(block.DefaultStateID("minecraft:oak_fence"))
	if err != nil {
		t.Fatal(err)
	}
	waterlogged, err = waterlogged.WithProperty("waterlogged", "true")
	if err != nil {
		t.Fatal(err)
	}
	if generationBlockTraits(waterlogged.ID())&featureFluidTrait == 0 || generationBlockTraits(waterlogged.ID())&featureLiquidTrait != 0 {
		t.Fatal("waterlogged solid is incorrectly treated as a liquid block")
	}
	if generationBlockTraits(block.DefaultStateID("minecraft:bubble_column"))&featureLiquidTrait == 0 {
		t.Fatal("bubble column lacks the liquid block trait")
	}
}
