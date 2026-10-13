package mapdata

import (
	"path/filepath"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/level"
)

// TestMapCraftCommitAndRestart verifies pure validation, allocation, and SavedData.
func TestMapCraftCommitAndRestart(t *testing.T) {
	dir := t.TempDir()
	root := nbt.NewCompound().PutInt("DataVersion", protocol.CurrentDataVersion).PutCompound("data", nbt.NewCompound().PutString("dimension", "minecraft:overworld").PutByte("scale", 0).PutInt("xCenter", 0).PutInt("zCenter", 0).PutByteArray("colors", make([]byte, Size*Size)).PutString("custom", "preserved"))
	if err := level.WriteSavedData(filepath.Join(dir, "data", "map_0.dat"), root); err != nil {
		t.Fatal(err)
	}
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !store.CanExtend(0) || store.lastID != 0 {
		t.Fatal("map validation changed state")
	}
	stack := inventory.NewSlot(item.IDByName("minecraft:filled_map"), 1)
	stack.SetComponent(dc.ComponentMapId, inventory.VarIntComponent(0))
	stack.SetComponent(dc.ComponentMapPostProcessing, inventory.VarIntComponent(0))
	result, err := store.CommitCraft(stack)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.Component(dc.ComponentMapId)
	if id.(inventory.VarIntComponent) != 1 {
		t.Fatal("new map ID not allocated")
	}
	if _, present := result.Component(dc.ComponentMapPostProcessing); present {
		t.Fatal("processing marker remained")
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.lastID != 1 || !restarted.CanExtend(1) {
		t.Fatal("map state did not survive restart")
	}
	data, _ := restarted.maps[1].GetCompound("data")
	scale, _ := data.GetByte("scale")
	custom, _ := data.GetString("custom")
	if scale != 1 || custom != "preserved" {
		t.Fatal("map metadata lost")
	}
}

// TestEmptyMapCreationPreservesCounterMetadata checks allocation and counter restart.
func TestEmptyMapCreationPreservesCounterMetadata(t *testing.T) {
	dir := t.TempDir()
	counter := nbt.NewCompound().PutInt("DataVersion", protocol.CurrentDataVersion).PutString("FutureRoot", "keep").PutCompound("data", nbt.NewCompound().PutInt("map", 5).PutLong("FutureCounter", 100))
	if err := level.WriteSavedData(filepath.Join(dir, "data", "idcounts.dat"), counter); err != nil {
		t.Fatal(err)
	}
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	stack, err := store.Create("minecraft:overworld", -1, 70)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := stack.Component(dc.ComponentMapId)
	if id.(inventory.VarIntComponent) != 6 || !store.CanExtend(6) {
		t.Fatal("incorrect empty-map allocation")
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.lastID != 6 {
		t.Fatal("counter was not restored")
	}
	root, err := level.ReadSavedData(filepath.Join(dir, "data", "idcounts.dat"))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := root.GetCompound("data")
	future, _ := root.GetString("FutureRoot")
	futureCounter, _ := data.GetLong("FutureCounter")
	if future != "keep" || futureCounter != 100 {
		t.Fatal("counter metadata lost")
	}
}
