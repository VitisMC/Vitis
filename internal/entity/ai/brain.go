package ai

import "github.com/vitismc/vitis/internal/entity"

// Brain holds the AI state for a single mob — its goal and target selectors.
type Brain struct {
	Goals   *GoalSelector
	Targets *GoalSelector
}

// NewBrain creates a Brain with goals appropriate for the mob's type definition.
func NewBrain(def *entity.MobTypeDef) *Brain {
	goals, targets := GoalsForMob(def)
	return &Brain{
		Goals:   goals,
		Targets: targets,
	}
}

// BrainRegistry maps entity IDs to their AI brains.
// All methods must be called from the world tick goroutine.
type BrainRegistry struct {
	brains map[int32]*Brain
}

// NewBrainRegistry creates an empty brain registry.
func NewBrainRegistry() *BrainRegistry {
	return &BrainRegistry{
		brains: make(map[int32]*Brain, 64),
	}
}

// Register assigns a brain to a mob entity ID.
func (r *BrainRegistry) Register(entityID int32, brain *Brain) {
	r.brains[entityID] = brain
}

// RegisterMob creates and assigns a brain for the given mob.
func (r *BrainRegistry) RegisterMob(mob *entity.MobEntity) {
	if mob.NoAI() {
		return
	}
	r.brains[mob.ID()] = NewBrain(mob.TypeDef())
}

// Remove removes a brain for the given entity ID.
func (r *BrainRegistry) Remove(entityID int32) {
	delete(r.brains, entityID)
}

// Get returns the brain for the given entity ID, or nil.
func (r *BrainRegistry) Get(entityID int32) *Brain {
	return r.brains[entityID]
}

// TickAll ticks all brains for all living mobs.
func (r *BrainRegistry) TickAll(mobs map[int32]*entity.MobEntity, players []PlayerInfo, worldTick uint64, blocks BlockAccess) {
	var toRemove []int32
	for id, brain := range r.brains {
		mob, ok := mobs[id]
		if !ok || mob.Entity.Removed() || mob.IsDead() || mob.NoAI() {
			toRemove = append(toRemove, id)
			continue
		}

		ctx := &Context{
			Mob:     mob,
			Players: players,
			Tick:    worldTick,
			Blocks:  blocks,
		}

		if brain.Targets != nil {
			brain.Targets.Tick(ctx)
		}
		brain.Goals.Tick(ctx)
	}
	for _, id := range toRemove {
		delete(r.brains, id)
	}
}
