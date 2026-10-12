package world

import (
	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// AwardExperience creates ordinary awards individually and bounds oversized batches.
func (w *World) AwardExperience(pos entity.Vec3, amount int32) {
	individual := amount <= 10000
	for _, value := range [...]int32{2477, 1237, 617, 307, 149, 73, 37, 17, 7, 3, 1} {
		count := amount / value
		if count <= 0 {
			continue
		}
		amount %= value
		if individual {
			for range count {
				w.awardExperienceValue(pos, value, 1)
			}
		} else {
			w.awardExperienceValue(pos, value, count)
		}
	}
}

// awardExperienceValue attempts the probabilistic equal-value merge before spawning.
func (w *World) awardExperienceValue(pos entity.Vec3, value, count int32) {
	group := w.randomTicks.NextIntBound(40)
	query := physics.NewAABB(pos.X-.5, pos.Y-.5, pos.Z-.5, pos.X+.5, pos.Y+.5, pos.Z+.5)
	for _, orb := range w.xpOrbs.All() {
		other := orb.Position()
		box := physics.XPOrbDimensions().MakeBoundingBox(other.X, other.Y, other.Z)
		if orb.ID()%40 == group && orb.XPValue() == value && query.Intersects(box) && orb.AddAwards(count) {
			return
		}
	}
	orb := entity.NewXPOrb(w.entities.AllocateID(), protocol.GenerateUUID(), pos, value)
	if count > 1 {
		orb.AddAwards(count - 1)
	}
	w.SpawnXPOrb(orb)
}

// repairWithXP repairs damaged equipped Mending items before crediting experience.
func (w *World) repairWithXP(player *entity.Player, xp int32) int32 {
	if w.playerInventory == nil || xp <= 0 {
		return xp
	}
	windows := w.playerInventory(player)
	if windows == nil {
		return xp
	}
	container := windows.Inventory()
	mending := ench.EnchantmentIDByName("minecraft:mending")
	changed := false
	var equipmentChanged [6]bool
	for xp > 0 {
		var candidates []int
		for _, slot := range [...]int{5, 6, 7, 8, inventory.OffhandSlot, inventory.HotbarStart + int(windows.HeldSlot())} {
			stack := container.Get(slot)
			if !stack.Empty() && stack.Damage > 0 && stack.EnchantLevel(mending) > 0 {
				candidates = append(candidates, slot)
			}
		}
		if len(candidates) == 0 {
			break
		}
		index := int(w.randomTicks.NextIntBound(int32(len(candidates))))
		slot := candidates[index]
		stack := container.Get(slot)
		repaired := int32(min(int64(stack.Damage), int64(xp)*2))
		stack.SetDamage(stack.Damage - repaired)
		container.Set(slot, stack)
		protocolSlot := byte(0)
		if slot >= inventory.ArmorStart && slot <= inventory.ArmorEnd {
			protocolSlot = byte(inventory.ArmorEnd - slot + 2)
		}
		if slot == inventory.OffhandSlot {
			protocolSlot = 1
		}
		equipmentChanged[protocolSlot] = true
		xp -= repaired / 2
		changed = true
	}
	if changed {
		var equipment []play.EquipmentSlot
		for protocolSlot, changed := range equipmentChanged {
			if !changed {
				continue
			}
			slot := inventory.HotbarStart + int(windows.HeldSlot())
			if protocolSlot == 1 {
				slot = inventory.OffhandSlot
			}
			if protocolSlot >= 2 {
				slot = inventory.ArmorEnd - protocolSlot + 2
			}
			stack := container.Get(slot)
			equipment = append(equipment, play.EquipmentSlot{SlotID: byte(protocolSlot), Stack: stack.Clone(), Empty: stack.Empty(), ItemID: stack.ItemID, Count: stack.ItemCount})
		}
		w.broadcastTrackedPacket(player.ID(), &play.EntityEquipment{EntityID: player.ID(), Slots: equipment})
		state, slots, cursor := windows.SnapshotForResync(0)
		if player.Session() != nil {
			_ = player.Session().Send(&play.SetContainerContent{WindowID: 0, StateID: state, Slots: slots, Carried: cursor})
		}
	}
	return xp
}
