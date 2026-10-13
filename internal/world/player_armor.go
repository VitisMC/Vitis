package world

import (
	"math"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/logger"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// SetEquipmentRuntime installs immutable equipment rules on the owning world tick.
func (w *World) SetEquipmentRuntime(runtime *equipment.Runtime) {
	w.equipmentRuntime = runtime
}

// bindPlayerArmor routes accepted damage through this world's current inventory handler.
func (w *World) bindPlayerArmor(player *entity.Player) {
	player.Living().SetArmorDamageHandler(func(source *entity.DamageSource, amount float32) {
		if w.entities.Get(player.ID()) == player.Entity {
			w.damagePlayerArmor(player, source, amount)
		}
	})
}

// damagePlayerArmor wears armor before reduction and publishes full changed stacks.
func (w *World) damagePlayerArmor(player *entity.Player, source *entity.DamageSource, amount float32) {
	if w.playerInventory == nil || amount <= 0 || math.IsNaN(float64(amount)) || math.IsInf(float64(amount), 0) {
		return
	}
	windows := w.playerInventory(player)
	if windows == nil {
		return
	}
	points := int32(min(float64(math.MaxInt32), math.Max(1, math.Floor(float64(amount)/4))))
	container := windows.Inventory()
	var slots []play.EquipmentSlot
	for slot := inventory.ArmorStart; slot <= inventory.ArmorEnd; slot++ {
		stack := container.Get(slot)
		if !equipment.WearsOnHurt(stack) || w.equipmentRuntime.ResistsDamage(stack, source.RegistryName()) {
			continue
		}
		worn, broken := equipment.Wear(stack, points)
		if !broken && worn.Damage == stack.Damage {
			continue
		}
		container.Set(slot, worn)
		protocolSlot := byte(inventory.ArmorEnd - slot + 2)
		slots = append(slots, play.EquipmentSlot{SlotID: protocolSlot, Stack: worn.Clone(), Empty: worn.Empty()})
		if broken {
			status := [...]byte{52, 51, 50, 49}[protocolSlot-2]
			w.broadcastTrackedPacket(player.ID(), &play.EntityStatus{EntityID: player.ID(), Status: status})
		}
	}
	if len(slots) == 0 {
		return
	}
	stats := equipment.PlayerArmorStats(container)
	player.Living().SetArmorStats(stats.Defense, stats.Toughness, stats.Protection)
	player.Living().SetArmorDamageProtection(stats.Fire, stats.Blast, stats.Projectile, stats.Fall)
	w.broadcastTrackedPacket(player.ID(), &play.EntityEquipment{EntityID: player.ID(), Slots: slots})
	state, stacks, cursor := windows.SnapshotForResync(0)
	if player.Session() != nil {
		if err := player.Session().Send(&play.SetContainerContent{WindowID: 0, StateID: state, Slots: stacks, Carried: cursor}); err != nil {
			logger.Error("synchronize worn player armor failed", "world", w.name, "entity_id", player.ID(), "error", err)
		}
	}
}
