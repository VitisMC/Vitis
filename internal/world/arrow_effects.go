package world

import (
	trackeddata "github.com/vitismc/vitis/internal/data/generated/tracked_data"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// applyArrowHitEffects applies the projectile's saved effects after accepted damage.
func (w *World) applyArrowHitEffects(source *entity.DamageSource, target *entity.LivingEntity) {
	if source.DirectType != "minecraft:arrow" || target.IsDead() {
		return
	}
	arrow := w.projectiles.GetArrow(source.DirectID)
	if arrow == nil {
		return
	}
	for _, instance := range arrow.HitEffects() {
		if instance.Duration > 0 && target.Effects().Add(instance) {
			w.broadcastTrackedPacket(target.ID(), &play.EntityEffect{EntityID: target.ID(), EffectID: instance.ID, Amplifier: instance.Amplifier, Duration: instance.Duration, Flags: instance.Flags})
		}
	}
	target.SyncEffectMovement()
	if properties := target.Attributes().DirtyProperties(); len(properties) != 0 {
		w.broadcastTrackedPacket(target.ID(), &play.EntityUpdateAttributes{EntityID: target.ID(), Properties: properties})
	}
}

// syncArrowStates publishes potion color changes to current observers on the world tick.
func (w *World) syncArrowStates() {
	for _, arrow := range w.projectiles.Arrows() {
		if arrow.Removed() || !arrow.ConsumeMetadataDirty() {
			continue
		}
		data := metadata.New()
		if arrow.ConsumePotionExpiration() {
			w.broadcastTrackedPacket(arrow.ID(), &play.EntityStatus{EntityID: arrow.ID(), Status: 0})
		}
		data.SetVarInt(trackeddata.ArrowIdEffectColor, arrow.PotionColor())
		w.broadcastTrackedPacket(arrow.ID(), &play.SetEntityMetadata{EntityID: arrow.ID(), Metadata: data.Encode()})
	}
}
