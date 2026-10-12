package main

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/world/level"
)

// TestStonecuttingCleanupPersistsCostsAndComponents verifies restart state after menu cleanup.
func TestStonecuttingCleanupPersistsCostsAndComponents(t *testing.T) {
	wm := inventory.NewWindowManager()
	stone, slabs := item.IDByName("minecraft:stone"), item.IDByName("minecraft:stone_slab")
	wm.SetStonecuttingMatcher(func(id int32) []inventory.StonecuttingChoice {
		if id != stone {
			return nil
		}
		output := inventory.NewSlot(slabs, 2)
		output.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(16))
		return []inventory.StonecuttingChoice{{RecipeID: "test:slabs", Output: output}}
	})
	backing := inventory.NewContainer(2)
	input := inventory.NewSlot(stone, 3)
	input.SetComponent(dc.ComponentMaxStackSize, inventory.VarIntComponent(32))
	backing.Set(0, input)
	window := wm.OpenWindow(inventory.WindowTypeStonecutter, "Stonecutter", backing)
	if !wm.SelectStonecuttingRecipe(window.ID, 0) || !wm.HandleClick(window.ID, wm.CurrentStateID(), 1, 0, inventory.ClickTypeNormalClick, nil, inventory.EmptySlot()) {
		t.Fatal("transaction failed")
	}
	if drops := wm.CloseTemporaryCrafting(); len(drops) != 0 {
		t.Fatalf("unexpected overflow: %+v", drops)
	}
	uuid := "00000000-0000-0000-0000-000000000001"
	player := entity.NewPlayer(1, protocol.UUID{1}, "menu", entity.Vec3{Y: 64}, entity.Vec2{}, nil, 0)
	online := &session.OnlinePlayer{UUID: player.UUID(), Windows: wm}
	snapshot := snapshotPlayerData(nil, uuid, online, player)
	root := t.TempDir()
	if err := snapshot.Save(root); err != nil {
		t.Fatal(err)
	}
	restored, err := level.LoadPlayerData(root, uuid)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Inventory) != 2 {
		t.Fatalf("saved inventory = %+v", restored.Inventory)
	}
	for _, entry := range restored.Inventory {
		wantMax := int32(32)
		if entry.ItemName == "minecraft:stone_slab" {
			wantMax = 16
		} else if entry.ItemName != "minecraft:stone" {
			t.Fatalf("unexpected saved item %s", entry.ItemName)
		}
		if entry.Count != 2 || entry.Stack.MaxStackSize() != wantMax {
			t.Fatalf("saved stack lost cost or components: %+v", entry)
		}
	}
}
