package world

import (
	"math"
	"math/rand"
	"sort"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/spawning"
)

const maxMobPickupChecks = 4096

// broadcastItemStack publishes the remaining full stack to current item observers.
func (w *World) broadcastItemStack(dropped *entity.ItemEntity) {
	data := metadata.New()
	data.SetSlot(metadata.IndexItemEntityItem, dropped.Stack())
	w.broadcastTrackedPacket(dropped.ID(), &play.SetEntityMetadata{EntityID: dropped.ID(), Metadata: data.Encode()})
}

// pickupChunkKey identifies an item bucket without losing signed chunk coordinates.
func pickupChunkKey(x, z int32) int64 { return int64(uint64(uint32(x))<<32 | uint64(uint32(z))) }

// initializeMobPickup applies the natural humanoid pickup chance from local difficulty.
func (w *World) initializeMobPickup(mob *entity.MobEntity) {
	switch mob.TypeName() {
	case "zombie", "husk", "drowned", "skeleton", "stray", "wither_skeleton":
		pos := mob.Position()
		local := spawning.EffectiveDifficulty(w.SpawnSettings(), w.SpawnInhabitedTime(int(math.Floor(pos.X)), int(math.Floor(pos.Z))))
		special := max(float32(0), min(float32(1), (local-2)/2))
		mob.SetCanPickUpLoot(w.randomTicks.NextFloat() < .55*special)
	}
}

// pickupMobEquipment performs bounded nearby item transfers on the owning world tick.
func (w *World) pickupMobEquipment() {
	if w.GameRule("mobGriefing") != "true" || w.items.Count() == 0 {
		return
	}
	var mobs []*entity.MobEntity
	for _, mob := range w.mobs.Mobs() {
		if mob.CanPickUpLoot() && !mob.IsDead() && !mob.Removed() && entity.IsTickingAt(w, mob.Position()) {
			mobs = append(mobs, mob)
		}
	}
	if len(mobs) == 0 {
		return
	}
	buckets := map[int64][]*entity.ItemEntity{}
	for _, dropped := range w.items.All() {
		if dropped.CanPickup() && entity.IsTickingAt(w, dropped.Position()) {
			key := pickupChunkKey(dropped.ChunkX(), dropped.ChunkZ())
			buckets[key] = append(buckets[key], dropped)
		}
	}
	if w.mobPickupItems == nil {
		w.mobPickupItems = map[int64]int32{}
	}
	for key := range w.mobPickupItems {
		if len(buckets[key]) == 0 {
			delete(w.mobPickupItems, key)
		}
	}
	for _, items := range buckets {
		sort.Slice(items, func(i, j int) bool { return items[i].ID() < items[j].ID() })
	}
	sort.Slice(mobs, func(i, j int) bool { return mobs[i].ID() < mobs[j].ID() })
	start := sort.Search(len(mobs), func(i int) bool { return mobs[i].ID() > w.mobPickupCursor }) % len(mobs)
	rules := w.equipmentRuntime.PickupRules()
	random := rand.New(rand.NewSource(w.randomTicks.NextLong()))
	checks := 0
	for offset := range mobs {
		mob := mobs[(start+offset)%len(mobs)]
		w.mobPickupCursor = mob.ID()
		pos := mob.Position()
		dimensions := physics.EntityDimensions{Width: mob.TypeDef().Width, Height: mob.TypeDef().Height}
		body := dimensions.MakeBoundingBox(pos.X, pos.Y, pos.Z)
		reach := physics.NewAABB(body.MinX-1, body.MinY, body.MinZ-1, body.MaxX+1, body.MaxY, body.MaxZ+1)
		for x := int32(math.Floor(reach.MinX / 16)); x <= int32(math.Floor(reach.MaxX/16)); x++ {
			for z := int32(math.Floor(reach.MinZ / 16)); z <= int32(math.Floor(reach.MaxZ/16)); z++ {
				key := pickupChunkKey(x, z)
				items := buckets[key]
				if len(items) == 0 {
					continue
				}
				first := sort.Search(len(items), func(i int) bool { return items[i].ID() > w.mobPickupItems[key] }) % len(items)
				for index := range items {
					if checks >= maxMobPickupChecks {
						return
					}
					dropped := items[(first+index)%len(items)]
					w.mobPickupItems[key] = dropped.ID()
					checks++
					position := dropped.Position()
					if !dropped.CanPickup() || w.items.Get(dropped.ID()) != dropped || !reach.Intersects(physics.ItemDimensions().MakeBoundingBox(position.X, position.Y, position.Z)) {
						continue
					}
					stack := dropped.Stack()
					picked, replaced, err := mob.EquipPickedItem(stack, rules, random)
					if err != nil {
						logger.Error("equip mob pickup failed", "world", w.name, "entity_id", mob.ID(), "item_entity_id", dropped.ID(), "error", err)
						continue
					}
					if picked.Empty() {
						continue
					}
					w.broadcastTrackedPacket(dropped.ID(), &play.CollectItem{CollectedEntityID: dropped.ID(), CollectorEntityID: mob.ID(), PickupItemCount: picked.ItemCount})
					stack.ItemCount -= picked.ItemCount
					if stack.Empty() {
						dropped.Remove()
						w.items.Remove(dropped.ID())
						w.entities.Remove(dropped.ID())
						w.broadcastRemoveEntity(dropped.ID())
					} else {
						dropped.SetStack(stack)
						w.broadcastItemStack(dropped)
					}
					if !replaced.Empty() {
						w.SpawnItemEntity(entity.NewItemEntity(w.entities.AllocateID(), protocol.GenerateUUID(), mob.Position(), replaced))
					}
				}
			}
		}
	}
}
