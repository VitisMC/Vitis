package entity

import (
	"bytes"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestMalformedContainerPreservesStoredItems prevents corrupt slots being overwritten.
func TestMalformedContainerPreservesStoredItems(t *testing.T) {
	cases := map[string]*nbt.List{
		"component": nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutByte("Slot", 0).PutString("id", "minecraft:stone").PutCompound("components", nbt.NewCompound().PutString("minecraft:rarity", "invalid"))),
		"slot":      nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutByte("Slot", 100).PutString("id", "minecraft:stone")),
		"duplicate": nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutByte("Slot", 0).PutString("id", "minecraft:stone")).Add(nbt.NewCompound().PutByte("Slot", 0).PutString("id", "minecraft:dirt")),
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			root := nbt.NewCompound().PutList("Items", entries)
			container := NewContainerBlockEntity("minecraft:chest", 0, 64, 0, 27, root)
			furnace := NewFurnaceBlockEntity(0, 64, 0)
			furnace.LoadPersistentNBT(root)
			if container.InventoryValid() || furnace.InventoryValid() || furnace.Tick() {
				t.Fatal("malformed inventory remained mutable")
			}
			container.Container().Set(0, inventory.NewSlot(item.IDByName("minecraft:diamond"), 1))
			for _, persisted := range []*nbt.Compound{container.PersistentNBT(), furnace.PersistentNBT()} {
				stored, _ := persisted.GetList("Items")
				expected, err := nbt.ToJSON(entries)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := nbt.ToJSON(stored)
				if err != nil || !bytes.Equal(actual, expected) {
					t.Fatalf("raw contents changed: %s", actual)
				}
			}
		})
	}
}

// TestEmptyUntypedContainerList accepts the NBT representation of an empty inventory.
func TestEmptyUntypedContainerList(t *testing.T) {
	root := nbt.NewCompound().PutList("Items", nbt.NewList(nbt.TagEnd))
	container := NewContainerBlockEntity("minecraft:chest", 0, 64, 0, 27, root)
	if !container.InventoryValid() {
		t.Fatal("empty list rejected")
	}
}

// TestFurnaceOutputUsesCompleteStackSemantics prevents named or resized outputs merging.
func TestFurnaceOutputUsesCompleteStackSemantics(t *testing.T) {
	furnace := NewFurnaceBlockEntity(0, 64, 0)
	input := inventory.NewSlot(item.IDByName("minecraft:sand"), 1)
	recipe := furnace.getRecipe(input)
	if recipe == nil {
		t.Fatal("missing sand recipe")
	}
	output := inventory.NewSlot(item.IDByName("minecraft:glass"), 1)
	if !furnace.canAcceptOutput(recipe, output) {
		t.Fatal("matching output rejected")
	}
	output.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(16))
	if furnace.canAcceptOutput(recipe, output) {
		t.Fatal("effective stack components ignored")
	}
	output = inventory.NewSlot(item.IDByName("minecraft:glass"), 1)
	output.PersistentData = nbt.NewCompound().PutString("Unknown", "keep")
	if furnace.canAcceptOutput(recipe, output) {
		t.Fatal("output metadata ignored")
	}
}
