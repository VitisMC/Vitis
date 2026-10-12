package world

import (
	"testing"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	trackeddata "github.com/vitismc/vitis/internal/data/generated/tracked_data"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestStrayArrowKeepsEffectAfterShooterRemoval connects the actual launch, flight and hit paths.
func TestStrayArrowKeepsEffectAfterShooterRemoval(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	mob := w.SpawnMob("stray", entity.Vec3{X: 10.5, Y: 64, Z: 4.5})
	if !w.AttackMobTarget(mob, player.ID(), true) {
		t.Fatal("stray did not shoot")
	}
	mob.Remove()
	for range 12 {
		w.tickProjectiles()
	}
	instance := player.Effects().Get(effectdata.EffectSlowness)
	if player.Health() >= 20 || instance == nil || instance.Duration != 600 {
		t.Fatalf("stray effect missing: health=%v effect=%+v", player.Health(), instance)
	}
	if player.Attributes().GetValue(genattr.AttrMovementSpeed) >= player.Attributes().GetBase(genattr.AttrMovementSpeed) {
		t.Fatal("stray effect did not change movement speed")
	}
	seen := false
	for _, packet := range sender.packets {
		if applied, ok := packet.(*play.EntityEffect); ok && applied.EntityID == player.ID() && applied.EffectID == effectdata.EffectSlowness {
			seen = true
		}
	}
	if !seen {
		t.Fatal("stray effect not synchronized")
	}
}

// TestArrowEffectsRespectAcceptedDamageAndSpecies validates shields, cooldowns, observers and immunity.
func TestArrowEffectsRespectAcceptedDamageAndSpecies(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	arrow := projectile.NewArrow(w.entities.AllocateID(), protocol.GenerateUUID(), entity.Vec3{X: 4.5, Y: 65, Z: 6.5}, entity.Vec3{Z: -1}, -1, protocol.UUID{})
	if err := arrow.AddEffect(effect.Instance{ID: effectdata.EffectPoison, Duration: 200, Flags: effect.DefaultFlags()}); err != nil {
		t.Fatal(err)
	}
	w.SpawnArrow(arrow)
	player.UpdateTracking(w.entities)
	source := entity.NewAttackDamageSource(entity.DamageTypeProjectile, 4, -1)
	source.DirectID, source.DirectType = arrow.ID(), "minecraft:arrow"
	player.SetBlocking(true)
	w.applyProjectileDamage(player.ID(), source, -1)
	if player.Effects().Count() != 0 || player.Health() != 20 {
		t.Fatal("blocked arrow applied effect")
	}
	player.SetBlocking(false)
	w.applyProjectileDamage(player.ID(), source, -1)
	if player.Effects().Get(effectdata.EffectPoison) == nil {
		t.Fatal("accepted arrow omitted effect")
	}
	player.Effects().Remove(effectdata.EffectPoison)
	w.applyProjectileDamage(player.ID(), source, -1)
	if player.Effects().Count() != 0 {
		t.Fatal("rejected repeat reapplied effect")
	}
	for _, name := range []string{"zombie", "cow"} {
		mob := w.SpawnMob(name, entity.Vec3{X: 6.5, Y: 64, Z: 4.5})
		player.UpdateTracking(w.entities)
		w.applyProjectileDamage(mob.ID(), source, -1)
		defended := float32(4)
		if name == "zombie" {
			defended = 3.936
		}
		if mob.Health() != mob.MaxHealth()-defended || mob.Effects().Has(effectdata.EffectPoison) != (name == "cow") {
			t.Fatalf("%s effect policy bypassed", name)
		}
	}
	w.syncArrowStates()
	seen := false
	for _, packet := range sender.packets {
		if update, ok := packet.(*play.SetEntityMetadata); ok && update.EntityID == arrow.ID() && len(update.Metadata) > 2 && update.Metadata[0] == trackeddata.ArrowIdEffectColor {
			seen = true
		}
	}
	if !seen {
		t.Fatal("arrow color not synchronized")
	}
}

// TestArrowEffectRestoredBeforeImpact survives a detached entity snapshot without the shooter.
func TestArrowEffectRestoredBeforeImpact(t *testing.T) {
	w, player, _, _ := survivalTestWorld(t, nil)
	arrow := projectile.NewArrow(7, protocol.UUID{7}, entity.Vec3{X: 8.5, Y: 65.5, Z: 4.5}, entity.Vec3{X: -1.6}, 50, protocol.UUID{50})
	if err := arrow.AddEffect(effect.Instance{ID: effectdata.EffectSlowness, Duration: 600, Flags: effect.DefaultFlags()}); err != nil {
		t.Fatal(err)
	}
	raw, err := arrow.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := projectile.DecodePersistent(w.entities.AllocateID(), raw)
	if err != nil {
		t.Fatal(err)
	}
	w.SpawnArrow(restored.(*projectile.Arrow))
	for range 8 {
		w.tickProjectiles()
	}
	if effect := player.Effects().Get(effectdata.EffectSlowness); effect == nil || effect.Duration != 600 {
		t.Fatal("restored arrow lost its effect")
	}
}

// TestMobEffectExpirySynchronizesAttributeRecovery checks observer-visible cleanup after a real tick.
func TestMobEffectExpirySynchronizesAttributeRecovery(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	mob := w.SpawnMob("cow", entity.Vec3{X: 6.5, Y: 64, Z: 4.5})
	mob.SetNoAI(true)
	mob.Effects().Add(effect.Instance{ID: effectdata.EffectSlowness, Duration: 1, Flags: effect.DefaultFlags()})
	mob.SyncEffectMovement()
	player.UpdateTracking(w.entities)
	w.tickMobs()
	if mob.Effects().Has(effectdata.EffectSlowness) || mob.Attributes().GetValue(genattr.AttrMovementSpeed) != mob.Attributes().GetBase(genattr.AttrMovementSpeed) {
		t.Fatal("expired effect did not restore speed")
	}
	removed, updated := false, false
	for _, packet := range sender.packets {
		if p, ok := packet.(*play.RemoveEntityEffect); ok && p.EntityID == mob.ID() && p.EffectID == effectdata.EffectSlowness {
			removed = true
		}
		if p, ok := packet.(*play.EntityUpdateAttributes); ok && p.EntityID == mob.ID() {
			updated = true
		}
	}
	if !removed || !updated {
		t.Fatal("effect expiry or recovered attributes were not synchronized")
	}
}

// TestGroundedArrowDecayPublishesOneEventAndColor checks the connected projectile and observer paths.
func TestGroundedArrowDecayPublishesOneEventAndColor(t *testing.T) {
	w, player, _, sender := survivalTestWorld(t, nil)
	arrow := projectile.NewArrow(w.entities.AllocateID(), protocol.GenerateUUID(), entity.Vec3{X: 5.5, Y: 63.5, Z: 4.5}, entity.Vec3{}, -1, protocol.UUID{})
	if err := arrow.AddEffect(effect.Instance{ID: effectdata.EffectSlowness, Duration: 600, Flags: effect.DefaultFlags()}); err != nil {
		t.Fatal(err)
	}
	w.SpawnArrow(arrow)
	player.UpdateTracking(w.entities)
	for range 599 {
		w.tickProjectiles()
	}
	if len(arrow.HitEffects()) != 1 {
		t.Fatal("grounded potion expired early")
	}
	w.tickProjectiles()
	w.tickProjectiles()
	events := 0
	for _, packet := range sender.packets {
		if p, ok := packet.(*play.EntityStatus); ok && p.EntityID == arrow.ID() && p.Status == 0 {
			events++
		}
	}
	if events != 1 || arrow.PotionColor() != -1 || len(arrow.HitEffects()) != 0 {
		t.Fatal("grounded decay event or color incorrect")
	}
}
