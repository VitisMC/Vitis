package main

import (
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/world/level"
)

// snapshotPlayerData captures world-owned player state on the authoritative tick.
func snapshotPlayerData(loaded *level.PlayerData, uuid string, online *session.OnlinePlayer, player *entity.Player) *level.PlayerData {
	if online == nil || player == nil {
		return nil
	}
	pd := loaded.Clone()
	position := player.Position()
	rotation := player.Rotation()
	if pd == nil {
		pd = level.DefaultPlayerData(uuid, position.X, position.Y, position.Z, online.GameMode)
	}
	pd.PosX, pd.PosY, pd.PosZ = position.X, position.Y, position.Z
	pd.Yaw, pd.Pitch = rotation.X, rotation.Y
	pd.GameMode = online.GameMode
	pd.Dimension = session.SessionDimension(online.Session)
	pd.PortalCooldown = online.PortalCooldown
	living := player.Living()
	pd.Health = living.Health()
	pd.FoodLevel = living.FoodLevel()
	pd.FoodSat = living.FoodSaturation()
	pd.FoodExhaustion = living.FoodExhaustion()
	pd.XPLevel = living.XPLevel()
	pd.XPTotal = living.XPTotal()
	pd.XPBar = living.XPBar()
	pd.Effects = living.Effects().Active()
	pd.RecipeBook = online.RecipeBook.Snapshot()
	pd.RecipeProgress = online.RecipeProgress.Clone()
	pd.HasSpawnPoint = online.HasSpawnPoint
	pd.SpawnDimension, pd.SpawnAngle, pd.SpawnForced = online.SpawnDimension, online.SpawnAngle, online.SpawnForced
	pd.SpawnX, pd.SpawnY, pd.SpawnZ = int32(online.SpawnX), int32(online.SpawnY), int32(online.SpawnZ)

	if online.Windows == nil {
		return pd
	}
	pd.SelectedItemSlot = online.Windows.HeldSlot()
	slots := online.Windows.Inventory().Slots()
	current := make([]level.InventorySlot, 0, len(slots))
	for i, slot := range slots {
		if slot.Empty() {
			continue
		}
		nbtSlot, valid := level.WindowSlotToNBT(i)
		if !valid {
			continue
		}
		name := item.NameByID(slot.ItemID)
		if name == "" {
			continue
		}
		entry := level.InventorySlot{
			Stack:        slot.Clone(),
			Slot:         nbtSlot,
			ItemName:     name,
			Count:        slot.ItemCount,
			Damage:       slot.Damage,
			Enchantments: slot.Enchantments,
		}
		current = append(current, entry)
	}
	pd.ReplaceInventoryPreservingRaw(current)
	return pd
}

// loadPlayerSnapshot prefers unwritten final state over an older disk snapshot.
func loadPlayerSnapshot(writer *level.PlayerWriter, worldDir, uuid string) (*level.PlayerData, error) {
	if writer != nil {
		if pending := writer.Pending(uuid); pending != nil {
			return pending, nil
		}
	}
	return level.LoadPlayerData(worldDir, uuid)
}
