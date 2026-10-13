package worldgen

import (
	"encoding/json"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

type heightBiomeSource struct{}

func (heightBiomeSource) sample(_, quartY, _ int) int32 {
	if quartY == 15 {
		return generatedbiome.BiomeBeach
	}
	return generatedbiome.BiomePlains
}

// TestBuriedTreasureBiomeSelections checks direct entries, nested tags and invalid references.
func TestBuriedTreasureBiomeSelections(t *testing.T) {
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, biomes         string
		wantBeach, wantSnowy bool
		wantError            bool
	}{
		{name: "direct", biomes: `"minecraft:beach"`, wantBeach: true},
		{name: "list and tag", biomes: `["#minecraft:is_beach","minecraft:snowy_beach"]`, wantBeach: true, wantSnowy: true},
		{name: "missing tag", biomes: `"#minecraft:missing_biomes"`, wantError: true},
		{name: "empty list", biomes: `[]`, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			resources := &ResourceSet{resources: map[string]map[string]json.RawMessage{
				"structure": {"minecraft:buried_treasure": json.RawMessage(`{"type":"minecraft:buried_treasure","biomes":` + test.biomes + `}`)},
			}}
			err := compileBuriedTreasureBiomes(manager.Current(), resources)
			if (err != nil) != test.wantError {
				t.Fatalf("compile biomes: %v", err)
			}
			if err == nil && (resources.buriedTreasureBiomes[generatedbiome.BiomeBeach] != test.wantBeach || resources.buriedTreasureBiomes[generatedbiome.BiomeSnowyBeach] != test.wantSnowy) {
				t.Fatalf("selected biomes = %v", resources.buriedTreasureBiomes)
			}
		})
	}
}

// TestBuriedTreasurePlacement checks the biome gate, block placement and loot metadata.
func TestBuriedTreasurePlacement(t *testing.T) {
	resources := &ResourceSet{
		structureSets: map[string]*structureSet{
			"minecraft:buried_treasures": {entries: []structureEntry{{id: "minecraft:buried_treasure", weight: 1}}, placement: structurePlacement{
				kind: "minecraft:random_spread", spacing: 1, frequency: 1,
			}},
		},
		buriedTreasureBiomes: map[int32]bool{generatedbiome.BiomeBeach: true},
	}
	newGround := func() *section.Chunk {
		chunk := section.NewChunkWithBounds(-2, 3, -64, 384, generatedbiome.BiomeBeach)
		chunk.SetBlock(8, 60, 8, block.DefaultStateID("minecraft:stone"))
		chunk.SetBlock(9, 60, 9, block.DefaultStateID("minecraft:stone"))
		chunk.SetBlock(9, 61, 9, block.DefaultStateID("minecraft:sand"))
		return chunk
	}
	result := newGround()
	if !placeBuriedTreasure(resources, result, fixedBiomeSource{generatedbiome.BiomeBeach}, 61, 42, -2, 3) {
		t.Fatal("treasure was not placed")
	}
	if got := block.NameFromState(result.GetBlock(9, 61, 9)); got != "minecraft:chest" {
		t.Fatalf("placed block = %s", got)
	}
	chest, ok := result.GetBlockEntity(-2*16+9, 61, 3*16+9).(*blockentity.RawBlockEntity)
	if !ok {
		t.Fatal("chest block entity is missing")
	}
	if loot, _ := chest.PersistentNBT().GetString("LootTable"); loot != "minecraft:chests/buried_treasure" {
		t.Fatalf("loot table = %s", loot)
	}
	structures, ok := result.PersistentNBT().GetCompound("structures")
	if !ok {
		t.Fatal("structure metadata is missing")
	}
	starts, ok := structures.GetCompound("starts")
	if !ok {
		t.Fatal("structure starts are missing")
	}
	start, ok := starts.GetCompound("minecraft:buried_treasure")
	if !ok {
		t.Fatal("buried treasure start is missing")
	}
	children, ok := start.GetList("Children")
	if !ok || children.Len() != 1 {
		t.Fatal("buried treasure piece is missing")
	}
	piece, ok := children.Elements()[0].(*nbt.Compound)
	if !ok {
		t.Fatal("invalid buried treasure piece")
	}
	if id, _ := piece.GetString("id"); id != "minecraft:btp" {
		t.Fatalf("structure piece ID = %s", id)
	}
	box, kind, ok := piece.Get("BB")
	if !ok || kind != nbt.TagIntArray {
		t.Fatal("structure bounding box is missing")
	}
	if got := box.([]int32); len(got) != 6 || got[0] != -23 || got[1] != 61 || got[2] != 57 {
		t.Fatalf("structure bounding box = %v", got)
	}
	references, ok := structures.GetCompound("References")
	if !ok {
		t.Fatal("structure references are missing")
	}
	packed, kind, ok := references.Get("minecraft:buried_treasure")
	if !ok || kind != nbt.TagLongArray || len(packed.([]int64)) != 1 {
		t.Fatal("buried treasure reference is missing")
	}
	if placeBuriedTreasure(resources, newGround(), fixedBiomeSource{generatedbiome.BiomePlains}, 61, 42, -2, 3) {
		t.Fatal("treasure placed in an invalid biome")
	}
	resources.structureSets["minecraft:buried_treasures"].placement.frequency = 0
	if placeBuriedTreasure(resources, newGround(), fixedBiomeSource{generatedbiome.BiomeBeach}, 61, 42, -2, 3) {
		t.Fatal("treasure placed when frequency is zero")
	}
}

// TestBuriedTreasureUsesTerrainHeight keeps biome selection independent of later surface changes.
func TestBuriedTreasureUsesTerrainHeight(t *testing.T) {
	resources := &ResourceSet{
		structureSets: map[string]*structureSet{
			"minecraft:buried_treasures": {entries: []structureEntry{{id: "minecraft:buried_treasure", weight: 1}}, placement: structurePlacement{
				kind: "minecraft:random_spread", spacing: 1, frequency: 1,
			}},
		},
		buriedTreasureBiomes: map[int32]bool{generatedbiome.BiomeBeach: true},
	}
	chunk := section.NewChunkWithBounds(0, 0, -64, 384, generatedbiome.BiomeBeach)
	chunk.SetBlock(8, 100, 8, block.DefaultStateID("minecraft:stone"))
	chunk.SetBlock(9, 60, 9, block.DefaultStateID("minecraft:stone"))
	chunk.SetBlock(9, 61, 9, block.DefaultStateID("minecraft:sand"))
	if !placeBuriedTreasure(resources, chunk, heightBiomeSource{}, 61, 42, 0, 0) {
		t.Fatal("terrain-height biome was ignored")
	}
}
