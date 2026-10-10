package entity

import (
	"math/rand"

	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// MobManager tracks all living mob entities in a world.
// All methods must be called exclusively from the world tick goroutine.
type MobManager struct {
	mobs          map[int32]*MobEntity
	entityManager *Manager
	nextEntityID  func() int32
	removeScratch []int32
	deathLoot     func(*MobEntity) []MobDeathDrop
	onSpawn       func(*MobEntity)
}

// SetSpawnHandler installs tick-owned initialization for spawned and restored mobs.
func (mm *MobManager) SetSpawnHandler(handler func(*MobEntity)) { mm.onSpawn = handler }

// SetDeathLootHandler installs the world's resource-backed death evaluator.
func (mm *MobManager) SetDeathLootHandler(handler func(*MobEntity) []MobDeathDrop) {
	mm.deathLoot = handler
}

// NewMobManager creates a new mob manager.
func NewMobManager(entityManager *Manager, nextEntityID func() int32) *MobManager {
	return &MobManager{
		mobs:          make(map[int32]*MobEntity, 64),
		entityManager: entityManager,
		nextEntityID:  nextEntityID,
	}
}

// SpawnMob creates and registers a mob entity at the given position.
// Returns the created mob or nil if the type name is unknown.
func (mm *MobManager) SpawnMob(typeName string, pos Vec3) *MobEntity {
	def := GetMobTypeDef(typeName)
	if def == nil {
		return nil
	}
	eid := mm.nextEntityID()
	uuid := protocol.UUID{uint64(rand.Int63()), uint64(rand.Int63())}
	mob := NewMobEntity(eid, uuid, def, pos, Vec2{})
	mm.mobs[eid] = mob
	mm.entityManager.Add(mob.Entity)
	if mm.onSpawn != nil {
		mm.onSpawn(mob)
	}
	return mob
}

// Add registers an existing mob entity with the manager.
func (mm *MobManager) Add(mob *MobEntity) {
	if mob == nil {
		return
	}
	mm.mobs[mob.ID()] = mob
	mm.entityManager.Add(mob.Entity)
	if mm.onSpawn != nil {
		mm.onSpawn(mob)
	}
}

// Remove marks a mob for removal.
func (mm *MobManager) Remove(id int32) {
	if mob, ok := mm.mobs[id]; ok {
		mob.Entity.Remove()
	}
}

// Evict releases a saved mob without generating death drops.
func (mm *MobManager) Evict(id int32) { delete(mm.mobs, id) }

// Get returns a mob by entity ID, or nil.
func (mm *MobManager) Get(id int32) *MobEntity {
	return mm.mobs[id]
}

// Count returns the number of living mobs.
func (mm *MobManager) Count() int {
	return len(mm.mobs)
}

// CountByCategory returns the number of mobs with the given category.
func (mm *MobManager) CountByCategory(cat MobCategory) int {
	count := 0
	for _, mob := range mm.mobs {
		if mob.Category() == cat {
			count++
		}
	}
	return count
}

// Mobs returns the full mob map for iteration.
func (mm *MobManager) Mobs() map[int32]*MobEntity {
	return mm.mobs
}

// Tick advances all mob entities for one tick and processes removals.
// Returns loot drops from mobs that died this tick.
func (mm *MobManager) Tick(world physics.BlockAccess) []MobDeathDrop {
	mm.removeScratch = mm.removeScratch[:0]
	var drops []MobDeathDrop
	var split []*MobEntity

	for id, mob := range mm.mobs {
		if !mob.Removed() && IsTickingAt(world, mob.Position()) {
			mob.TickMob(world)
			if mob.IsDead() && !mob.DeathHandled() {
				mob.MarkDeathHandled()
				if mm.deathLoot != nil {
					drops = append(drops, mm.deathLoot(mob)...)
				} else {
					drops = append(drops, mm.generateDeathDrops(mob)...)
				}
			}
			if mob.Removed() && mob.IsDead() && mob.DeathHandled() && mob.DeathTime() >= 20 && mob.TypeName() == "slime" && mob.SlimeSize() > 1 {
				split = append(split, mob)
			}
		}

		if mob.Entity.Removed() {
			mm.removeScratch = append(mm.removeScratch, id)
		}
	}

	for _, id := range mm.removeScratch {
		delete(mm.mobs, id)
	}
	for _, parent := range split {
		mm.splitSlime(parent)
	}

	return drops
}

// MobDeathDrop represents an item drop from a mob death.
type MobDeathDrop struct {
	X, Y, Z float64
	ItemID  int32
	Count   int32
	XP      int32
	Stack   inventory.ItemStack
}

func (mm *MobManager) generateDeathDrops(mob *MobEntity) []MobDeathDrop {
	pos := mob.Position()
	var drops []MobDeathDrop

	for _, loot := range mob.typeDef.Drops {
		if mob.TypeName() == "slime" && mob.SlimeSize() > 1 {
			break
		}
		if loot.Chance < 1.0 && rand.Float64() > loot.Chance {
			continue
		}
		count := loot.MinCount
		if loot.MaxCount > loot.MinCount {
			count += rand.Int31n(loot.MaxCount - loot.MinCount + 1)
		}
		if count <= 0 {
			continue
		}
		itemID := item.IDByName(loot.ItemName)
		if itemID <= 0 {
			continue
		}
		drops = append(drops, MobDeathDrop{
			X: pos.X, Y: pos.Y, Z: pos.Z,
			ItemID: itemID, Count: count,
		})
	}

	if mob.XPReward() > 0 {
		drops = append(drops, MobDeathDrop{
			X: pos.X, Y: pos.Y, Z: pos.Z,
			XP: mob.XPReward(),
		})
	}

	return drops
}
