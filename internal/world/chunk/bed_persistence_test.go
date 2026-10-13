package chunk

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestLegacyBedsRepairAndPersist verifies old chunks recover both renderer entities.
func TestLegacyBedsRepairAndPersist(t *testing.T) {
	c := New(-1, 2)
	for index, part := range []string{"head", "foot"} {
		c.SetBlock(-2, 64, 34+index, block.StateID("minecraft:red_bed", map[string]string{"part": part, "facing": "north", "occupied": "false"}))
	}
	c.Data().BlockEntities = nil
	encoded, err := EncodeChunkNBT(c)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeChunkNBT(encoded, -1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Dirty() || len(loaded.Data().BlockEntities) != 2 {
		t.Fatal("missing bed entities were not repaired for autosave")
	}
	for _, z := range []int32{34, 35} {
		be := loaded.Data().GetBlockEntity(-2, 64, z)
		if be == nil || be.TypeName() != "minecraft:bed" {
			t.Fatal("legacy bed half was not repaired")
		}
	}
	custom := blockentity.NewRawBlockEntity(nbt.NewCompound().PutString("id", "minecraft:bed").PutInt("x", -2).PutInt("y", 64).PutInt("z", 34).PutString("FutureField", "keep"))
	loaded.Data().AddBlockEntity(custom)
	snapshot, err := SnapshotForSave(loaded)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = EncodeChunkNBT(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := DecodeChunkNBT(encoded, -1, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Dirty() || len(restarted.Data().BlockEntities) != 2 {
		t.Fatal("saved bed entities were duplicated or repaired again")
	}
	root := restarted.Data().GetBlockEntity(-2, 64, 34).(persistentBlockEntity).PersistentNBT()
	if value, _ := root.GetString("FutureField"); value != "keep" {
		t.Fatal("repair or snapshot lost existing bed data")
	}
}
