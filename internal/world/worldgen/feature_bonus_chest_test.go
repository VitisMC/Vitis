package worldgen

import (
	"encoding/json"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestBonusChestPlacesLootAndTorches checks placement, saved loot, and RNG order.
func TestBonusChestPlacesLootAndTorches(t *testing.T) {
	f, err := decodeBonusChestFeature(json.RawMessage(`{"config":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	chunk := section.NewChunkWithBounds(-2, 3, -64, 128, 0)
	stone := block.DefaultStateID("minecraft:stone")
	for x := -32; x < -16; x++ {
		for z := 48; z < 64; z++ {
			chunk.SetBlock(x, 30, z, stone)
		}
	}
	const seed = int64(417)
	wantRandom := worldrandom.NewLegacy(seed)
	var xOrder, zOrder [16]int
	for index := range xOrder {
		xOrder[index], zOrder[index] = -32+index, 48+index
	}
	shuffleBonusChestAxis(wantRandom, &xOrder)
	shuffleBonusChestAxis(wantRandom, &zOrder)
	wantPosition := blockPosition{xOrder[0], 31, zOrder[0]}
	wantLootSeed := wantRandom.NextLong()
	wantTail := wantRandom.NextLong()
	random := worldrandom.NewLegacy(seed)
	if !f.placeFeature(nil, chunk, random, blockPosition{-24, 0, 56}) {
		t.Fatal("bonus chest was not placed")
	}
	if state := getChunkBlock(chunk, wantPosition); state != f.chest {
		t.Fatalf("chest at %+v = %s", wantPosition, block.NameFromState(state))
	}
	entity := chunk.GetBlockEntity(int32(wantPosition.x), int32(wantPosition.y), int32(wantPosition.z))
	persistent, ok := entity.(interface{ PersistentNBT() *nbt.Compound })
	if !ok {
		t.Fatalf("chest block entity missing: %T", entity)
	}
	root := persistent.PersistentNBT()
	if table, _ := root.GetString("LootTable"); table != "minecraft:chests/spawn_bonus_chest" {
		t.Fatalf("loot table = %q", table)
	}
	if lootSeed, _ := root.GetLong("LootTableSeed"); lootSeed != wantLootSeed {
		t.Fatalf("loot seed = %d, want %d", lootSeed, wantLootSeed)
	}
	if tail := random.NextLong(); tail != wantTail {
		t.Fatalf("RNG tail = %d, want %d", tail, wantTail)
	}
	for _, offset := range featureFaceOffsets[featureNorth:] {
		neighbor := blockPosition{wantPosition.x + offset.x, wantPosition.y, wantPosition.z + offset.z}
		want := int32(0)
		if neighbor.x >= -32 && neighbor.x < -16 && neighbor.z >= 48 && neighbor.z < 64 {
			want = f.torch
		}
		if state := getChunkBlock(chunk, neighbor); state != want {
			t.Fatalf("torch at %+v = %s, want %s", neighbor, block.NameFromState(state), block.NameFromState(want))
		}
	}
}

// TestBonusChestRejectsUnexpectedConfiguration prevents silent datapack changes.
func TestBonusChestRejectsUnexpectedConfiguration(t *testing.T) {
	if _, err := decodeBonusChestFeature(json.RawMessage(`{"config":{"unknown":true}}`)); err == nil {
		t.Fatal("unexpected bonus chest configuration was accepted")
	}
}

// TestBonusChestWorldGenerationAtConfiguredSpawn checks the optional world lifecycle hook.
func TestBonusChestWorldGenerationAtConfiguredSpawn(t *testing.T) {
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
	generator.EnableBonusChest(-8, 8)
	spawn, err := generator.Generate(-1, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found int
	var bonusX, bonusY, bonusZ int32
	for _, entity := range spawn.BlockEntities {
		persistent, ok := entity.(interface{ PersistentNBT() *nbt.Compound })
		if !ok {
			continue
		}
		root := persistent.PersistentNBT()
		if table, _ := root.GetString("LootTable"); table == "minecraft:chests/spawn_bonus_chest" {
			found++
			x, _, z := entity.Position()
			bonusX, bonusY, bonusZ = entity.Position()
			if x < -16 || x >= 0 || z < 0 || z >= 16 {
				t.Fatalf("bonus chest outside spawn chunk: (%d,%d)", x, z)
			}
		}
	}
	if found != 1 {
		t.Fatalf("spawn chunk bonus chest count = %d, want 1", found)
	}
	stored := chunk.New(-1, 0)
	stored.SetData(spawn)
	encoded, err := chunk.EncodeChunkNBT(stored)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := chunk.DecodeChunkNBT(encoded, -1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	restored := loaded.Data().GetBlockEntity(bonusX, bonusY, bonusZ)
	persistent, ok := restored.(interface{ PersistentNBT() *nbt.Compound })
	if !ok {
		t.Fatalf("saved bonus chest block entity missing: %T", restored)
	}
	if table, _ := persistent.PersistentNBT().GetString("LootTable"); table != "minecraft:chests/spawn_bonus_chest" {
		t.Fatalf("saved bonus chest loot table = %q", table)
	}
	neighbor, err := generator.Generate(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range neighbor.BlockEntities {
		persistent, ok := entity.(interface{ PersistentNBT() *nbt.Compound })
		if !ok {
			continue
		}
		if table, _ := persistent.PersistentNBT().GetString("LootTable"); table == "minecraft:chests/spawn_bonus_chest" {
			t.Fatal("bonus chest duplicated in neighboring chunk")
		}
	}
}
