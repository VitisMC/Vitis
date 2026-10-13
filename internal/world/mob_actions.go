package world

import (
	"math"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/spawning"
)

// SetPlayerInventoryHandler exposes tick-owned inventory and equipment for gameplay.
func (w *World) SetPlayerInventoryHandler(read func(*entity.Player) *inventory.WindowManager) {
	w.playerInventory = read
}

// MobCanSee traces collision shapes through resident chunks with bounded work.
func (w *World) MobCanSee(from, to entity.Vec3) bool {
	dx, dy, dz := to.X-from.X, to.Y-from.Y, to.Z-from.Z
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance > 128 {
		return false
	}
	steps := max(1, int(math.Ceil(distance*8)))
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x, y, z := from.X+dx*t, from.Y+dy*t, from.Z+dz*t
		if w.GetBlockStateAt(int(math.Floor(x)), int(math.Floor(y)), int(math.Floor(z))) < 0 {
			return false
		}
		box := physics.NewAABB(x-.001, y-.001, z-.001, x+.001, y+.001, z+.001)
		if len(physics.CollectBlockCollisions(w, box)) > 0 {
			return false
		}
	}
	return true
}

// AttackMobTarget revalidates and applies one active mob's attack or arrow shot.
func (w *World) AttackMobTarget(mob *entity.MobEntity, target int32, ranged bool) bool {
	if mob == nil || mob.Removed() || mob.IsDead() || mob.NoAI() || w.mobs.Get(mob.ID()) != mob || !entity.IsTickingAt(w, mob.Position()) {
		return false
	}
	if mob.TypeName() == "slime" && (ranged || mob.SlimeSize() <= 1) {
		return false
	}
	if ranged && !mob.UsesBow() {
		return false
	}
	for _, player := range w.tracker.Players() {
		if player.ID() != target || player.Removed() || player.IsDead() || !player.HasClientLoaded() || player.Living().GameMode() == 1 || player.Living().GameMode() == 3 {
			continue
		}
		origin, destination := mob.Position(), player.Position()
		origin.Y += mob.TypeDef().Height * .85
		destination.Y += 1.62
		limit := 2.0
		if ranged {
			limit = 15
		}
		inRange := mob.DistanceTo(player.Position()) <= limit
		if mob.TypeName() == "slime" {
			position := mob.Position()
			box := (physics.EntityDimensions{Width: mob.TypeDef().Width, Height: mob.TypeDef().Height}).MakeBoundingBox(position.X, position.Y, position.Z)
			position = player.Position()
			inRange = box.Grow(.2).Intersects(physics.PlayerDimensions().MakeBoundingBox(position.X, position.Y, position.Z))
		}
		if !inRange || !w.MobCanSee(origin, destination) {
			return false
		}
		if ranged {
			dx, dy, dz := destination.X-origin.X, destination.Y-origin.Y, destination.Z-origin.Z
			length := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if length < 1e-6 {
				return false
			}
			arrow := projectile.NewArrow(w.entities.AllocateID(), protocol.GenerateUUID(), origin, entity.Vec3{X: dx / length * 1.6, Y: dy/length*1.6 + math.Hypot(dx, dz)*.0075, Z: dz / length * 1.6}, mob.ID(), mob.UUID())
			arrow.SetPickup(projectile.ArrowPickupDisallowed)
			if mob.TypeName() == "stray" {
				if err := arrow.AddEffect(effect.Instance{ID: effectdata.EffectSlowness, Duration: 600, Flags: effect.DefaultFlags()}); err != nil {
					return false
				}
			}
			w.SpawnArrow(arrow)
			return true
		}
		living := player.Living()
		if living.IsBlocking() && living.ShieldCooldown() == 0 && shieldFaces(player, mob.Position()) {
			return false
		}
		source := entity.NewAttackDamageSource(entity.DamageTypeMobAttack, float32(mob.Attributes().GetValue(genattr.AttrAttackDamage)), mob.ID())
		source.AttackerType = "minecraft:" + mob.TypeName()
		source.Weapon = mob.Equipment(entity.EquipmentMainHand)
		source = w.playerDifficultyDamage(source)
		actual := living.ApplyDamage(source, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
		if actual <= 0 {
			return false
		}
		w.applyMobMeleeEffects(mob, player)
		velocity := player.Velocity()
		dx, dz := player.Position().X-mob.Position().X, player.Position().Z-mob.Position().Z
		length := math.Hypot(dx, dz)
		if length > 1e-6 {
			velocity.X = velocity.X/2 + dx/length*.4
			velocity.Z = velocity.Z/2 + dz/length*.4
			velocity.Y = math.Min(.4, velocity.Y/2+.4)
			player.SetVelocity(velocity)
		}
		w.broadcastTrackedPacket(player.ID(), &play.HurtAnimation{EntityID: player.ID(), Yaw: player.Rotation().X})
		if player.Session() != nil {
			_ = player.Session().Send(&play.UpdateHealth{Health: living.Health(), Food: living.FoodLevel(), FoodSaturation: living.FoodSaturation()})
			if living.IsDead() {
				_ = player.Session().Send(&play.DeathCombatEvent{PlayerID: player.ID(), Message: formatDeathMessage(source.Type, player.Username(), mob.TypeName())})
			}
		}
		return true
	}
	return false
}

// applyMobMeleeEffects applies and synchronizes species effects only after an accepted hit.
func (w *World) applyMobMeleeEffects(mob *entity.MobEntity, player *entity.Player) {
	if player.IsDead() {
		return
	}
	instance := effect.Instance{Flags: effect.DefaultFlags()}
	switch mob.TypeName() {
	case "husk":
		if !mob.Equipment(entity.EquipmentMainHand).Empty() {
			return
		}
		pos := mob.Position()
		local := spawning.EffectiveDifficulty(w.SpawnSettings(), w.SpawnInhabitedTime(int(math.Floor(pos.X)), int(math.Floor(pos.Z))))
		instance.ID, instance.Duration = effectdata.EffectHunger, 140*int32(local)
	case "wither_skeleton":
		instance.ID, instance.Duration = effectdata.EffectWither, 200
	case "cave_spider":
		instance.ID = effectdata.EffectPoison
		if w.Difficulty() == 2 {
			instance.Duration = 140
		} else if w.Difficulty() == 3 {
			instance.Duration = 300
		}
	default:
		return
	}
	if instance.Duration <= 0 {
		return
	}
	if player.Effects().Add(instance) {
		w.broadcastTrackedPacket(player.ID(), &play.EntityEffect{EntityID: player.ID(), EffectID: instance.ID, Amplifier: instance.Amplifier, Duration: instance.Duration, Flags: instance.Flags})
	}
}

// shieldFaces checks whether an attacker is in front of the blocking player.
func shieldFaces(player *entity.Player, source entity.Vec3) bool {
	yaw := float64(player.Rotation().X) * math.Pi / 180
	pos := player.Position()
	return (source.X-pos.X)*-math.Sin(yaw)+(source.Z-pos.Z)*math.Cos(yaw) > 0
}

// projectileBlocked applies directional shielding and the piercing-arrow exception.
func (w *World) projectileBlocked(player *entity.Player, source *entity.DamageSource) bool {
	if source.Type != entity.DamageTypeProjectile || !player.IsBlocking() || player.ShieldCooldown() > 0 {
		return false
	}
	if arrow := w.projectiles.GetArrow(source.DirectID); arrow != nil && arrow.Piercing() > 0 {
		return false
	}
	direct := w.entities.Get(source.DirectID)
	return direct != nil && shieldFaces(player, direct.Position())
}

// ExplodeMob removes a creeper before executing its explosion exactly once.
func (w *World) ExplodeMob(mob *entity.MobEntity, power float64) bool {
	if mob == nil || mob.Removed() || mob.IsDead() || w.mobs.Get(mob.ID()) != mob || !entity.IsTickingAt(w, mob.Position()) {
		return false
	}
	if power <= 0 || power > 12 || math.IsNaN(power) {
		return false
	}
	pos := mob.Position()
	mob.Remove()
	cause := entity.NewAttackDamageSource(entity.DamageTypeExplosion, 0, mob.ID())
	cause.AttackerType = "minecraft:" + mob.TypeName()
	w.doExplosionWithSource(pos.X, pos.Y, pos.Z, power, w.GameRule("mobGriefing") == "true", cause)
	return true
}

// MobSunlit checks daylight, weather and full sky exposure in a skylit dimension.
func (w *World) MobSunlit(pos entity.Vec3) bool {
	if w.dimension != "minecraft:overworld" || w.weather.IsRaining() {
		return false
	}
	time := (w.TimeOfDay()%24000 + 24000) % 24000
	_, sky, ok := w.LightLevels(int(math.Floor(pos.X)), int(math.Floor(pos.Y+1)), int(math.Floor(pos.Z)))
	return ok && time < 12000 && sky == 15
}

// MobBrightness returns weather- and time-adjusted local brightness.
func (w *World) MobBrightness(pos entity.Vec3) uint8 {
	_, _, brightness, ok := w.SpawnLight(int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z)))
	if !ok {
		return 15
	}
	return brightness
}
