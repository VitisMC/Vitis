package level

import (
	"path/filepath"
	"testing"

	enchantments "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestPlayerInventorySlotMapping covers hotbar, armor, offhand, and legacy slots.
func TestPlayerInventorySlotMapping(t *testing.T) {
	cases := []struct {
		window int
		nbt    int8
	}{
		{36, 0}, {44, 8}, {9, 9}, {35, 35},
		{5, 103}, {6, 102}, {7, 101}, {8, 100}, {45, -106},
	}
	for _, tc := range cases {
		slot, ok := WindowSlotToNBT(tc.window)
		if !ok || slot != tc.nbt {
			t.Errorf("window %d -> %d, %v; want %d", tc.window, slot, ok, tc.nbt)
		}
		window, ok := NBTSlotToWindow(tc.nbt)
		if !ok || window != tc.window {
			t.Errorf("NBT %d -> %d, %v; want %d", tc.nbt, window, ok, tc.window)
		}
	}
	for _, invalid := range []int{0, 1, 4, 46, 100} {
		if _, ok := WindowSlotToNBT(invalid); ok {
			t.Errorf("window slot %d was accepted", invalid)
		}
	}
	if window, ok := NBTSlotToWindow(36); !ok || window != 36 {
		t.Fatal("legacy hotbar slot was not recognized")
	}
}

// TestModernPlayerInventoryPreservesRawNBT verifies read-modify-write for items.
func TestModernPlayerInventoryPreservesRawNBT(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	pd := DefaultPlayerData("modern-inventory", 0, 64, 0, 0)
	pd.raw = nbt.NewCompound().PutString("UnknownPlayerTag", "preserve")
	pd.Inventory = []InventorySlot{{
		Slot: 0, ItemName: "minecraft:diamond_pickaxe", Count: 1, Damage: 7,
		Enchantments: map[int32]int32{enchantments.EnchantmentIDByName("minecraft:efficiency"): 4},
		raw: nbt.NewCompound().PutString("UnknownItemTag", "preserve").
			PutCompound("components", nbt.NewCompound().
				PutString("minecraft:custom_name", `"preserve"`).
				PutCompound("minecraft:enchantments", nbt.NewCompound().
					PutCompound("levels", nbt.NewCompound().PutInt("mod:unknown", 3)))),
	}}
	if err := pd.Save(worldDir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPlayerData(worldDir, pd.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Inventory) != 1 || loaded.Inventory[0].Count != 1 || loaded.Inventory[0].Damage != 7 {
		t.Fatalf("loaded inventory = %#v", loaded.Inventory)
	}
	loaded.ReplaceInventoryPreservingRaw([]InventorySlot{{
		Slot: 0, ItemName: "minecraft:diamond_pickaxe", Count: 1, Damage: 8,
		Enchantments: loaded.Inventory[0].Enchantments,
	}})
	if err := loaded.Save(worldDir); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadPlayerData(worldDir, pd.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := reloaded.raw.GetString("UnknownPlayerTag"); value != "preserve" {
		t.Fatal("unknown player NBT was lost")
	}
	item := reloaded.Inventory[0]
	if item.Count != 1 || item.Damage != 8 {
		t.Fatalf("item count=%d damage=%d", item.Count, item.Damage)
	}
	if _, exists := item.raw.GetByte("Count"); exists {
		t.Fatal("legacy Count tag was written")
	}
	if value, _ := item.raw.GetString("UnknownItemTag"); value != "preserve" {
		t.Fatal("unknown item NBT was lost")
	}
	components, _ := item.raw.GetCompound("components")
	if name, _ := components.GetString("minecraft:custom_name"); name != `"preserve"` {
		t.Fatal("item component was lost")
	}
	enchantment, _ := components.GetCompound("minecraft:enchantments")
	levels, _ := enchantment.GetCompound("levels")
	if value, _ := levels.GetInt("mod:unknown"); value != 3 {
		t.Fatal("unknown enchantment was lost")
	}
}

// TestChangedInventoryItemDoesNotInheritRawTags prevents component leakage.
func TestChangedInventoryItemDoesNotInheritRawTags(t *testing.T) {
	pd := DefaultPlayerData("changed-item", 0, 64, 0, 0)
	pd.Inventory = []InventorySlot{{Slot: 0, ItemName: "minecraft:stone", Count: 1, raw: nbt.NewCompound().PutString("Secret", "old")}}
	pd.ReplaceInventoryPreservingRaw([]InventorySlot{{Slot: 0, ItemName: "minecraft:dirt", Count: 1}})
	if pd.Inventory[0].raw != nil {
		t.Fatal("new item inherited the previous item's raw NBT")
	}
}
