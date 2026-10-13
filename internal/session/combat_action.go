package session

import (
	"math"

	"github.com/vitismc/vitis/internal/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
)

// attackEntityCommand orders entity attacks with inventory and movement mutations.
type attackEntityCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
	target  int32
}

// OwnerSession prevents queued combat from crossing disconnects or dimension transfers.
func (c *attackEntityCommand) OwnerSession() Session { return c.session }

// Execute resolves current life, equipment, cooldown and target state on tick.
func (c *attackEntityCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil || c.players == nil {
		return
	}
	if c.players.GetByEntityID(c.target) != nil {
		if err := handleAttack(c.session, c.target, c.players); err != nil {
			logger.Debug("reject player attack", "error", err)
		}
		return
	}
	player, ok := c.session.Player().(*entity.Player)
	online := c.players.GetBySession(c.session)
	if !ok || player == nil || online == nil || player.IsDead() || !player.HasClientLoaded() || player.Living().GameMode() == 3 {
		return
	}
	actions, ok := c.world.(interface {
		AttackMob(*entity.Player, int32, *entity.DamageSource, float64) bool
	})
	if !ok {
		return
	}
	weapon := inventory.ItemStack{}
	if online.Windows != nil {
		weapon = online.Windows.HeldItem()
	}
	damage, cooldown, _ := playerAttackDamage(player.Living(), weapon)
	source := entity.NewAttackDamageSource(entity.DamageTypePlayerAttack, damage, player.ID())
	source.AttackerType, source.Weapon = "minecraft:player", weapon.Clone()
	reach := 3.0
	if player.Living().GameMode() == 1 {
		reach = 5
	}
	if !actions.AttackMob(player, c.target, source, reach) {
		return
	}
	player.Living().ResetAttackCooldown(cooldown)
	player.Living().AddExhaustion(.1)
	if online.Windows != nil && player.Living().GameMode() != 1 && !weapon.Empty() && item.IsDurable(weapon.ItemID) && item.ShouldConsumeDurability(weapon.EnchantLevel(item.EnchantUnbreakingID), false) {
		damage, broken := item.ApplyDamage(weapon.ItemID, weapon.Damage, 1)
		if broken {
			weapon = inventory.ItemStack{}
		} else {
			weapon.SetDamage(damage)
		}
		online.Windows.Inventory().Set(inventory.HotbarStart+int(online.Windows.HeldSlot()), weapon)
		if err := sendWindowSnapshot(c.session, online.Windows, 0); err != nil {
			logger.Warn("sync attack weapon failed", "error", err)
		}
	}
}

// playerAttackDamage resolves weapon damage and the accepted attack's cooldown scale.
func playerAttackDamage(living *entity.LivingEntity, weapon inventory.ItemStack) (float32, int32, float32) {
	base, cooldown := float32(1), int32(10)
	if properties := equipment.GetWeapon(item.NameByID(weapon.ItemID)); properties != nil {
		base = float32(properties.AttackDamage)
		if properties.AttackSpeed > 0 {
			cooldown = max(1, int32(20/properties.AttackSpeed))
		}
	}
	progress := float32(math.Max(0, math.Min(1, float64(cooldown-living.AttackCooldown())/float64(cooldown))))
	damage := base * (.2 + progress*progress*.8)
	if progress > .9 && !living.OnGround() && living.FallDistance() > 0 {
		damage *= 1.5
	}
	damage += float32(enchantment.SharpnessDamage(slotEnchantList(weapon))) * progress
	return damage, cooldown, progress
}
