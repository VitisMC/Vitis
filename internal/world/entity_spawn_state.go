package world

import (
	trackeddata "github.com/vitismc/vitis/internal/data/generated/tracked_data"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// SetEntitySpawnStateHandler installs additional observer-only player state synchronization.
func (w *World) SetEntitySpawnStateHandler(handler func(*entity.Player, *entity.Entity)) {
	w.entitySpawnState = handler
}

// sendEntitySpawnState sends authoritative state after tracking publishes the entity spawn.
func (w *World) sendEntitySpawnState(player *entity.Player, base *entity.Entity) {
	if player == nil || base == nil || player.Session() == nil {
		return
	}
	if w.entitySpawnState != nil {
		w.entitySpawnState(player, base)
	}
	data := metadata.New()
	if item := w.items.Get(base.ID()); item != nil {
		data.SetSlot(metadata.IndexItemEntityItem, item.Stack().Clone())
	}
	if tnt := w.tnts.Get(base.ID()); tnt != nil {
		data.SetVarInt(trackeddata.PrimedTntFuseId, tnt.Fuse())
	}
	if arrow := w.projectiles.GetArrow(base.ID()); arrow != nil {
		data.SetVarInt(trackeddata.ArrowIdEffectColor, arrow.PotionColor())
	}
	if mob := w.mobs.Get(base.ID()); mob != nil {
		mob.SyncEffectMovement()
		fillMobMetadata(data, mob)
		_ = player.Session().Send(mobEquipmentPacket(mob, 0x7f))
		_ = player.Session().Send(&playpacket.EntityUpdateAttributes{EntityID: base.ID(), Properties: mob.Attributes().ToProperties()})
		for _, inst := range mob.Effects().Active() {
			_ = player.Session().Send(&playpacket.EntityEffect{EntityID: base.ID(), EffectID: inst.ID, Amplifier: inst.Amplifier, Duration: inst.Duration, Flags: inst.Flags})
		}
	}
	encoded := data.Encode()
	if len(encoded) > 1 {
		_ = player.Session().Send(&playpacket.SetEntityMetadata{EntityID: base.ID(), Metadata: encoded})
	}
}

// fillMobMetadata encodes current living state and supported species metadata.
func fillMobMetadata(data *metadata.Map, mob *entity.MobEntity) {
	data.SetFloat(metadata.IndexLivingHealth, mob.Health())
	flags := int8(0)
	if mob.FireTicks() > 0 {
		flags = 1
	}
	data.SetByte(metadata.IndexBase, flags)
	flags = 0
	if mob.NoAI() {
		flags = 1
	}
	data.SetByte(trackeddata.MobMobFlagsId, flags)
	if mob.TypeName() == "creeper" {
		data.SetVarInt(trackeddata.CreeperSwellDir, mob.SwellDirection())
	}
	if mob.TypeName() == "slime" {
		data.SetVarInt(trackeddata.SlimeIdSize, int32(mob.SlimeSize()))
	}
}

// syncMobStates publishes living state, attribute changes and effect expiry after simulation.
func (w *World) syncMobStates() {
	for _, mob := range w.mobs.Mobs() {
		if mob.Removed() {
			continue
		}
		diff := mob.ConsumeEffectDiff()
		if slots := mob.ConsumeEquipmentBroken(); slots != 0 {
			for slot, status := range [...]byte{47, 48, 52, 51, 50, 49, 65} {
				if slots&(1<<slot) != 0 {
					w.broadcastTrackedPacket(mob.ID(), &playpacket.EntityStatus{EntityID: mob.ID(), Status: status})
				}
			}
		}
		if slots := mob.ConsumeEquipmentDirty(); slots != 0 {
			w.broadcastTrackedPacket(mob.ID(), mobEquipmentPacket(mob, slots))
		}
		for _, id := range diff.Removed {
			w.broadcastTrackedPacket(mob.ID(), &playpacket.RemoveEntityEffect{EntityID: mob.ID(), EffectID: id})
		}
		if properties := mob.Attributes().DirtyProperties(); len(properties) != 0 {
			w.broadcastTrackedPacket(mob.ID(), &playpacket.EntityUpdateAttributes{EntityID: mob.ID(), Properties: properties})
		}
		if mob.Dirty()&entity.DirtyLivingState == 0 {
			continue
		}
		data := metadata.New()
		fillMobMetadata(data, mob)
		packet := &playpacket.SetEntityMetadata{EntityID: mob.ID(), Metadata: data.Encode()}
		w.broadcastTrackedPacket(mob.ID(), packet)
	}
}

// mobEquipmentPacket snapshots changed slots, including explicit empty stacks.
func mobEquipmentPacket(mob *entity.MobEntity, mask uint8) *playpacket.EntityEquipment {
	packet := &playpacket.EntityEquipment{EntityID: mob.ID()}
	for slot := entity.EquipmentMainHand; slot <= entity.EquipmentBody; slot++ {
		if mask&(1<<slot) != 0 {
			stack := mob.Equipment(slot)
			packet.Slots = append(packet.Slots, playpacket.EquipmentSlot{SlotID: byte(slot), Stack: stack, Empty: stack.Empty()})
		}
	}
	return packet
}

// broadcastTrackedPacket sends entity-specific state only to observers and the entity itself.
func (w *World) broadcastTrackedPacket(id int32, packet protocol.Packet) {
	for _, observer := range w.tracker.Players() {
		_, tracked := observer.TrackedEntities()[id]
		if (tracked || observer.ID() == id) && !observer.Removed() && observer.Session() != nil {
			_ = observer.Session().Send(packet)
		}
	}
}
