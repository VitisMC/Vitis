package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	attr "github.com/vitismc/vitis/internal/data/generated/attribute"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/weather"
)

// TestRuntimeSunlightBreaksHeadGearAndSynchronizesOnce checks the connected world simulation.
func TestRuntimeSunlightBreaksHeadGearAndSynchronizesOnce(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	player.Living().SetGameMode(1)
	mob := w.SpawnMob("zombie", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
	mob.SetNoAI(true)
	mob.Attributes().SetBase(attr.AttrMovementSpeed, 0)
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)
	stack.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(1))
	if err := mob.SetEquipment(entity.EquipmentHead, stack); err != nil {
		t.Fatal(err)
	}
	player.TrackedEntities()[mob.ID()] = struct{}{}
	if !w.MobSunlit(mob.Position()) {
		t.Fatal("fresh resident test terrain has no sun exposure")
	}
	for range 8192 {
		w.tickMobs()
		if mob.FireTicks() != 0 {
			t.Fatal("protected mob ignited before the next unprotected tick")
		}
		if mob.Equipment(entity.EquipmentHead).Empty() {
			break
		}
	}
	if !mob.Equipment(entity.EquipmentHead).Empty() {
		t.Fatal("runtime sunlight did not break the helmet")
	}
	for range 8192 {
		w.tickMobs()
		if mob.FireTicks() > 0 {
			break
		}
	}
	if mob.FireTicks() <= 0 || mob.ArmorDefense() != 2 {
		t.Fatal("unprotected mob did not ignite or retained helmet attributes")
	}
	breaks, emptyUpdates := 0, 0
	for _, packet := range sender.packets {
		switch value := packet.(type) {
		case *play.EntityStatus:
			if value.EntityID == mob.ID() && value.Status == 49 {
				breaks++
			}
		case *play.EntityEquipment:
			for _, slot := range value.Slots {
				if value.EntityID == mob.ID() && slot.SlotID == byte(entity.EquipmentHead) && slot.Stack.Empty() {
					emptyUpdates++
				}
			}
		}
	}
	if breaks != 1 || emptyUpdates != 1 || w.items.Count() != 0 || w.xpOrbs.Count() != 0 {
		t.Fatalf("breaks=%d empty updates=%d drops=%d", breaks, emptyUpdates, w.items.Count())
	}
	for _, setting := range []string{"night", "rain", "shade", "inactive", "nether"} {
		other, p, _, _ := survivalTestWorld(t, nil)
		p.Living().SetGameMode(1)
		otherMob := other.SpawnMob("skeleton", entity.Vec3{X: 5.5, Y: 64, Z: 4.5})
		if err := otherMob.SetEquipment(entity.EquipmentHead, stack); err != nil {
			t.Fatal(err)
		}
		switch setting {
		case "night":
			other.SetTimeOfDay(13000)
		case "rain":
			other.SetWeather(weather.Rain, 100)
		case "shade":
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					other.setBlockAt(x, 66, z, block.DefaultStateID("minecraft:stone"))
				}
			}
		case "inactive":
			otherMob.SetPosition(entity.Vec3{X: 80.5, Y: 64, Z: 80.5})
		case "nether":
			other.dimension = "minecraft:the_nether"
		}
		for range 20 {
			other.tickMobs()
		}
		if !otherMob.Equipment(entity.EquipmentHead).StackableWith(stack) || otherMob.FireTicks() != 0 {
			t.Fatalf("%s performed sunlight wear or ignition", setting)
		}
	}
}
