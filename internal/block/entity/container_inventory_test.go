package entity

import (
	"testing"

	enchantmentdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestContainerBlockEntityRoundTrip verifies slots and unknown NBT survive an edit.
func TestContainerBlockEntityRoundTrip(t *testing.T) {
	raw := nbt.NewCompound().PutString("id", "minecraft:chest").PutInt("x", 1).PutInt("y", 64).PutInt("z", 2).PutString("CustomName", `{"text":"Supply"}`)
	items := nbt.NewList(nbt.TagCompound)
	components := nbt.NewCompound().PutCompound("minecraft:enchantments", nbt.NewCompound().PutCompound("levels", nbt.NewCompound().PutInt("minecraft:unbreaking", 3)))
	items.Add(nbt.NewCompound().PutByte("Slot", 0).PutString("id", "minecraft:stone").PutInt("count", 2).PutCompound("components", components))
	raw.PutList("Items", items)
	be := NewContainerBlockEntity("minecraft:chest", 1, 64, 2, 27, raw)
	if be == nil || be.Container().Get(0).ItemCount != 2 {
		t.Fatal("container item was not loaded")
	}
	if got := be.Container().Get(0).EnchantLevel(enchantmentdata.EnchantmentIDByName("minecraft:unbreaking")); got != 3 {
		t.Fatalf("enchantment level = %d, want 3", got)
	}
	stack := be.Container().Get(0)
	stack.ItemCount = 3
	be.Container().Set(0, stack)
	out := be.PersistentNBT()
	if name, _ := out.GetString("CustomName"); name != `{"text":"Supply"}` {
		t.Fatalf("custom name lost: %q", name)
	}
	loaded := NewContainerBlockEntity("minecraft:chest", 1, 64, 2, 27, out)
	if loaded.Container().Get(0).ItemCount != 3 {
		t.Fatalf("saved count = %d, want 3", loaded.Container().Get(0).ItemCount)
	}
	if loaded.Container().Get(0).EnchantLevel(enchantmentdata.EnchantmentIDByName("minecraft:unbreaking")) != 3 {
		t.Fatal("enchantment lost after container round trip")
	}
}

// TestFurnaceVariantsCook verifies each furnace consumes fuel and produces the right output.
func TestFurnaceVariantsCook(t *testing.T) {
	cases := []struct {
		name       string
		newFurnace func(int32, int32, int32) *FurnaceLikeBlockEntity
		input      string
		output     string
	}{
		{"furnace", NewFurnaceBlockEntity, "minecraft:sand", "minecraft:glass"},
		{"smoker", NewSmokerBlockEntity, "minecraft:beef", "minecraft:cooked_beef"},
		{"blast", NewBlastFurnaceBlockEntity, "minecraft:raw_iron", "minecraft:iron_ingot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			furnace := tc.newFurnace(1, 64, 2)
			furnace.Container().Set(0, inventory.NewSlot(item.IDByName(tc.input), 1))
			furnace.Container().Set(1, inventory.NewSlot(item.IDByName("minecraft:coal"), 1))
			for tick := 0; tick < 210; tick++ {
				furnace.Tick()
			}
			if got := furnace.Container().Get(2); got.ItemID != item.IDByName(tc.output) || got.ItemCount != 1 {
				t.Fatalf("output = %+v, want %s", got, tc.output)
			}
			if !furnace.Container().Get(0).Empty() || !furnace.Container().Get(1).Empty() {
				t.Fatalf("inputs not consumed: input=%+v fuel=%+v", furnace.Container().Get(0), furnace.Container().Get(1))
			}
			persisted := furnace.PersistentNBT()
			restored := tc.newFurnace(1, 64, 2)
			restored.LoadPersistentNBT(persisted)
			if restored.Container().Get(2).ItemID != item.IDByName(tc.output) {
				t.Fatal("cooked output lost after NBT round trip")
			}
			if restored.GetProperty(PropertyLitTotalTime) != furnace.GetProperty(PropertyLitTotalTime) {
				t.Fatal("fuel duration lost after NBT round trip")
			}
			if xp := restored.ExtractExperience(); xp <= 0 {
				t.Fatalf("cooked output awarded no experience: %v", xp)
			}
			if xp := restored.ExtractExperience(); xp != 0 {
				t.Fatalf("experience awarded twice: %v", xp)
			}
		})
	}
}
