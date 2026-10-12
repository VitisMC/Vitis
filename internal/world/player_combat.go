package world

import (
	"math"

	"github.com/vitismc/vitis/internal/entity"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// AttackMob validates and applies a player's structured attack to an active mob.
func (w *World) AttackMob(player *entity.Player, target int32, source *entity.DamageSource, reach float64) bool {
	if player == nil || player.Removed() || player.IsDead() || !player.HasClientLoaded() || source == nil || source.Type != entity.DamageTypePlayerAttack || source.AttackerID != player.ID() || player.Living().GameMode() == 3 || reach <= 0 || reach > 5 || math.IsNaN(reach) {
		return false
	}
	found := false
	for _, tracked := range w.tracker.Players() {
		if tracked == player {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	mob := w.mobs.Get(target)
	if mob == nil || mob.Removed() || mob.IsDead() || !entity.IsTickingAt(w, mob.Position()) {
		return false
	}
	origin, destination := player.Position(), mob.Position()
	if mob.DistanceToSq(origin) > reach*reach {
		return false
	}
	origin.Y += 1.62
	destination.Y += mob.TypeDef().Height * .5
	if !w.MobCanSee(origin, destination) {
		return false
	}
	actual := mob.ApplyDamage(source, mob.ArmorDefense(), mob.ArmorToughness(), mob.ArmorEnchProtection())
	if actual <= 0 {
		return false
	}
	mob.SetTarget(player.ID())
	velocity := mob.Velocity()
	dx, dz := mob.Position().X-player.Position().X, mob.Position().Z-player.Position().Z
	if length := math.Hypot(dx, dz); length > 1e-6 {
		strength := .4 * (1 - mob.KnockbackResistance())
		velocity.X, velocity.Z = velocity.X/2+dx/length*strength, velocity.Z/2+dz/length*strength
		if mob.OnGround() {
			velocity.Y = math.Min(.4, velocity.Y/2+strength)
		}
		mob.SetVelocity(velocity)
	}
	w.broadcastTrackedPacket(mob.ID(), &play.HurtAnimation{EntityID: mob.ID(), Yaw: mob.Rotation().X})
	return true
}
