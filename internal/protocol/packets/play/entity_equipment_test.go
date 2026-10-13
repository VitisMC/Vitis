package play

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestEquipmentPreservesComponents verifies equipment packets carry complete stacks.
func TestEquipmentPreservesComponents(t *testing.T) {
	raw := nbt.NewCompound().PutString("id", "minecraft:diamond_helmet").PutInt("count", 1).PutCompound("components", nbt.NewCompound().PutString("minecraft:custom_name", `{"text":"Helmet","bold":true}`).PutList("minecraft:lore", nbt.NewList(nbt.TagString).Add(`"Lore"`)))
	stack, err := inventory.ItemStackFromNBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	packet := EntityEquipment{EntityID: 300, Slots: []EquipmentSlot{{SlotID: 5, Stack: stack}, {SlotID: 1, Stack: inventory.NewSlot(item.IDByName("minecraft:shield"), 1)}}}
	buffer := protocol.NewBuffer(128)
	if err := packet.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	var decoded EntityEquipment
	if err := decoded.Decode(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer.Remaining() != 0 || decoded.EntityID != 300 || len(decoded.Slots) != 2 || !decoded.Slots[0].Stack.StackableWith(stack) {
		t.Fatal("equipment data changed")
	}
	if _, ok := decoded.Slots[0].Stack.Component(dc.ComponentCustomName); !ok {
		t.Fatal("custom name lost")
	}
	packet.Slots[1].SlotID = 5
	if err := packet.Encode(protocol.NewBuffer(128)); err == nil {
		t.Fatal("duplicate slot accepted")
	}
	for _, data := range [][]byte{{1, 7, 0}, {1, 0x80, 0, 0, 0}, {1, 0x80, 0}} {
		if err := decoded.Decode(protocol.WrapBuffer(data)); err == nil {
			t.Fatalf("malformed equipment accepted: %x", data)
		}
	}
}

// TestRecipeRemovalRejectsInvalidIDs checks bounded recipe display identifiers.
func TestRecipeRemovalRejectsInvalidIDs(t *testing.T) {
	packet := RecipeBookRemove{RecipeIDs: []int32{-1}}
	buffer := protocol.NewBuffer(16)
	if err := packet.Encode(buffer); err == nil {
		t.Fatal("negative ID accepted")
	}
	buffer = protocol.NewBuffer(16)
	buffer.WriteVarInt(1)
	buffer.WriteVarInt(-1)
	if err := packet.Decode(buffer); err == nil {
		t.Fatal("negative wire ID accepted")
	}
	packet.RecipeIDs = make([]int32, 16385)
	if err := packet.Encode(protocol.NewBuffer(16)); err == nil {
		t.Fatal("unbounded list accepted")
	}
}
