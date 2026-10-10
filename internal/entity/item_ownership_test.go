package entity

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestItemEntityOwnsComponentStacks prevents mutable equipment and item-transfer aliases.
func TestItemEntityOwnsComponentStacks(t *testing.T) {
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	stack.SetDamage(7)
	stack.SetComponent(dc.ComponentCustomData, inventory.NBTComponent{Value: nbt.NewCompound().PutString("identity", "owned")})
	drops := []*ItemEntity{NewItemEntity(1, protocol.UUID{1}, Vec3{}, stack), NewItemEntityWithVelocity(2, protocol.UUID{2}, Vec3{}, stack, Vec3{}, 0)}
	stack.SetDamage(90)
	custom, _ := stack.Component(dc.ComponentCustomData)
	custom.(inventory.NBTComponent).Value.PutString("identity", "changed")
	for _, dropped := range drops {
		custom, _ := dropped.Stack().Component(dc.ComponentCustomData)
		identity, _ := custom.(inventory.NBTComponent).Value.GetString("identity")
		if dropped.Stack().Damage != 7 || identity != "owned" {
			t.Fatal("item constructor retained mutable components")
		}
		copy := dropped.Stack()
		copy.SetDamage(50)
		if dropped.Stack().Damage != 7 {
			t.Fatal("item getter exposed mutable components")
		}
		dropped.SetStack(copy)
		copy.SetDamage(80)
		if dropped.Stack().Damage != 50 {
			t.Fatal("item setter retained mutable components")
		}
	}
}
