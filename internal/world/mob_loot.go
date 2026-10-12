package world

import (
	"strings"

	ench "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/loot"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// mobDeathLoot evaluates death rewards once against the active resource snapshot.
func (w *World) mobDeathLoot(mob *entity.MobEntity) []entity.MobDeathDrop {
	if mob == nil || !mob.IsDead() {
		return nil
	}
	w.broadcastTrackedPacket(mob.ID(), &play.EntityStatus{EntityID: mob.ID(), Status: 3})
	if w.GameRule("doMobLoot") != "true" {
		return nil
	}
	pos := mob.Position()
	ctx := loot.NewContext(nil)
	ctx.KilledByPlayer = mob.HasPlayerKillCredit()
	ctx.DamageType = mob.LastDamageRegistryName()
	direct := mob.LastDamageIsDirect()
	ctx.DamageIsDirect = &direct
	ctx.Entity = &loot.EntityInfo{Type: "minecraft:" + strings.TrimPrefix(mob.TypeName(), "minecraft:"), OnFire: mob.FireTicks() > 0, Color: "white", Size: 4}
	ctx.Entity.Baby, ctx.Entity.Color, ctx.Entity.Sheared, ctx.Entity.Size = mob.LootTraits()
	weapon := mob.DeathWeapon()
	if !weapon.Empty() {
		enchantments := make(map[string]int, len(weapon.Enchantments))
		for id, level := range weapon.Enchantments {
			if info := ench.EnchantmentByID(id); info != nil {
				enchantments[info.Name] = int(level)
			}
		}
		ctx.WithTool(item.NameByID(weapon.ItemID), enchantments)
	}
	if name := mob.LastAttackerType(); name != "" {
		ctx.Attacker = &loot.EntityInfo{Type: name, Mainhand: ctx.Tool}
	}
	if name := mob.LastDirectAttackerType(); name != "" {
		ctx.DirectAttacker = &loot.EntityInfo{Type: name}
		if name == "minecraft:player" {
			ctx.DirectAttacker.Mainhand = ctx.Tool
		}
	}
	var drops []entity.MobDeathDrop
	if w.lootRuntime != nil {
		for _, stack := range w.lootRuntime.RollStacks("minecraft:entities/"+strings.TrimPrefix(mob.TypeName(), "minecraft:"), ctx) {
			drops = append(drops, entity.MobDeathDrop{X: pos.X, Y: pos.Y, Z: pos.Z, ItemID: stack.ItemID, Count: stack.ItemCount, Stack: stack})
		}
	}
	for _, stack := range mob.RollEquipmentDrops(ctx.Rng, ctx.KilledByPlayer, weapon.EnchantLevel(ench.EnchantmentIDByName("minecraft:looting"))) {
		drops = append(drops, entity.MobDeathDrop{X: pos.X, Y: pos.Y, Z: pos.Z, ItemID: stack.ItemID, Count: stack.ItemCount, Stack: stack})
	}
	if ctx.KilledByPlayer && mob.XPReward() > 0 {
		xp := mob.XPReward()
		if mob.Category() == entity.MobCategoryPassive {
			xp = 1 + w.randomTicks.NextIntBound(3)
		}
		xp += mob.EquipmentXPBonus(ctx.Rng)
		drops = append(drops, entity.MobDeathDrop{X: pos.X, Y: pos.Y, Z: pos.Z, XP: xp})
	}
	return drops
}
