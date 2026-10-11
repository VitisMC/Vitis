package chunk

import (
	"testing"

	blockentity "github.com/vitismc/vitis/internal/block/entity"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestSaveSnapshotOwnsPalettesAndBlockEntities protects asynchronous encoders from mutation.
func TestSaveSnapshotOwnsPalettesAndBlockEntities(t *testing.T) {
	original := New(-1, 2)
	original.SetBlock(3, 64, 4, genblock.StoneDefaultState)
	original.Data().Sections[8].SetBiome(1, 2, 3, 7)
	root := nbt.NewCompound().PutString("custom", "preserved")
	original.Data().SetPersistentNBT(root)
	chest := blockentity.NewRawBlockEntity(nbt.NewCompound().
		PutString("id", "minecraft:chest").PutInt("x", -13).PutInt("y", 64).PutInt("z", 36).
		PutString("LootTable", "minecraft:chests/buried_treasure"))
	original.Data().AddBlockEntity(chest)
	snapshot, err := SnapshotForSave(original)
	if err != nil {
		t.Fatal(err)
	}
	original.SetBlock(3, 64, 4, genblock.DirtDefaultState)
	original.Data().Sections[8].SetBiome(1, 2, 3, 9)
	chest.SetPosition(-12, 65, 36)
	original.Data().SetPersistentNBT(nbt.NewCompound().PutString("custom", "changed"))
	encoded, err := EncodeChunkNBT(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeChunkNBT(encoded, -1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Data().GetBlock(3, 64, 4) != genblock.StoneDefaultState || decoded.Data().Sections[8].GetBiome(1, 2, 3) != 7 {
		t.Fatal("snapshot palettes changed with the source")
	}
	if value, _ := decoded.Data().PersistentNBT().GetString("custom"); value != "preserved" {
		t.Fatal("snapshot lost custom NBT")
	}
	entity := decoded.Data().GetBlockEntity(-13, 64, 36)
	if entity == nil {
		t.Fatal("snapshot block entity moved with the source")
	}
	if value, _ := entity.(persistentBlockEntity).PersistentNBT().GetString("LootTable"); value != "minecraft:chests/buried_treasure" {
		t.Fatal("snapshot lost loot table")
	}
}
