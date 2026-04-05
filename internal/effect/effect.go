package effect

import (
	"sync"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
)

const (
	FlagAmbient   byte = 0x01
	FlagParticles byte = 0x02
	FlagIcon      byte = 0x04
)

// Instance represents an active status effect on an entity.
type Instance struct {
	ID        int32
	Amplifier int32
	Duration  int32
	Flags     byte
}

// IsAmbient returns whether the effect is from a beacon or similar source.
func (i *Instance) IsAmbient() bool { return i.Flags&FlagAmbient != 0 }

// ShowParticles returns whether particles should be shown.
func (i *Instance) ShowParticles() bool { return i.Flags&FlagParticles != 0 }

// ShowIcon returns whether the effect icon should be shown.
func (i *Instance) ShowIcon() bool { return i.Flags&FlagIcon != 0 }

// IsExpired returns true when the duration has run out.
func (i *Instance) IsExpired() bool { return i.Duration <= 0 }

// TickAction describes a gameplay action resulting from an effect tick.
type TickAction struct {
	Heal         float32
	Damage       float32
	DamageSource string
	Exhaustion   float32
	FoodRestore  int32
	SatRestore   float32
	Absorption   float32
}

// Diff represents changes to broadcast after a tick.
type Diff struct {
	Added   []Instance
	Removed []int32
	Actions []TickAction
}

// Manager tracks all active effects on an entity.
type Manager struct {
	mu      sync.RWMutex
	effects map[int32]*Instance
}

// NewManager creates an empty effect manager.
func NewManager() *Manager {
	return &Manager{
		effects: make(map[int32]*Instance),
	}
}

// Add applies a new effect or replaces an existing one if stronger/longer.
// Returns true if the effect was added or replaced.
func (m *Manager) Add(inst Instance) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.effects[inst.ID]
	if ok {
		if inst.Amplifier < existing.Amplifier {
			return false
		}
		if inst.Amplifier == existing.Amplifier && inst.Duration <= existing.Duration {
			return false
		}
	}

	copied := inst
	m.effects[inst.ID] = &copied
	return true
}

// Remove removes an effect by ID. Returns true if it existed.
func (m *Manager) Remove(id int32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.effects[id]; ok {
		delete(m.effects, id)
		return true
	}
	return false
}

// Clear removes all active effects and returns their IDs.
func (m *Manager) Clear() []int32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int32, 0, len(m.effects))
	for id := range m.effects {
		ids = append(ids, id)
	}
	m.effects = make(map[int32]*Instance)
	return ids
}

// Has returns true if the effect is currently active.
func (m *Manager) Has(id int32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.effects[id]
	return ok
}

// Get returns the active instance for an effect ID, or nil.
func (m *Manager) Get(id int32) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.effects[id]; ok {
		copied := *inst
		return &copied
	}
	return nil
}

// Active returns a snapshot of all active effects.
func (m *Manager) Active() []Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Instance, 0, len(m.effects))
	for _, inst := range m.effects {
		result = append(result, *inst)
	}
	return result
}

// Count returns the number of active effects.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.effects)
}

// Tick advances all effect durations by one tick and computes gameplay actions.
func (m *Manager) Tick() Diff {
	m.mu.Lock()
	defer m.mu.Unlock()

	var diff Diff

	for id, inst := range m.effects {
		action := tickEffect(inst)
		if action != nil {
			diff.Actions = append(diff.Actions, *action)
		}

		inst.Duration--
		if inst.Duration <= 0 {
			diff.Removed = append(diff.Removed, id)
			delete(m.effects, id)
		}
	}

	return diff
}

func tickEffect(inst *Instance) *TickAction {
	switch inst.ID {
	case effectdata.EffectRegeneration:
		interval := regenInterval(inst.Amplifier)
		if inst.Duration%interval == 0 {
			return &TickAction{Heal: 1.0}
		}
	case effectdata.EffectPoison:
		interval := poisonInterval(inst.Amplifier)
		if inst.Duration%interval == 0 {
			return &TickAction{Damage: 1.0, DamageSource: "poison"}
		}
	case effectdata.EffectWither:
		interval := witherInterval(inst.Amplifier)
		if inst.Duration%interval == 0 {
			return &TickAction{Damage: 1.0, DamageSource: "wither"}
		}
	case effectdata.EffectHunger:
		return &TickAction{Exhaustion: 0.005 * float32(inst.Amplifier+1)}
	case effectdata.EffectSaturation:
		return &TickAction{
			FoodRestore: inst.Amplifier + 1,
			SatRestore:  float32(inst.Amplifier+1) * 2.0,
		}
	}
	return nil
}

// ApplyInstant handles instant effects (health/damage) and returns the action.
func ApplyInstant(id int32, amplifier int32) *TickAction {
	switch id {
	case effectdata.EffectInstantHealth:
		amount := float32(int32(4) << uint(amplifier))
		return &TickAction{Heal: amount}
	case effectdata.EffectInstantDamage:
		amount := float32(int32(6) << uint(amplifier))
		return &TickAction{Damage: amount, DamageSource: "magic"}
	case effectdata.EffectSaturation:
		return &TickAction{
			FoodRestore: amplifier + 1,
			SatRestore:  float32(amplifier+1) * 2.0,
		}
	}
	return nil
}

// IsInstant returns true if the effect ID is an instant effect.
func IsInstant(id int32) bool {
	switch id {
	case effectdata.EffectInstantHealth, effectdata.EffectInstantDamage:
		return true
	}
	return false
}

// DefaultFlags returns the standard flags for a non-ambient effect.
func DefaultFlags() byte {
	return FlagParticles | FlagIcon
}

func regenInterval(amplifier int32) int32 {
	t := int32(50) >> uint(amplifier)
	if t < 1 {
		t = 1
	}
	return t
}

func poisonInterval(amplifier int32) int32 {
	t := int32(25) >> uint(amplifier)
	if t < 1 {
		t = 1
	}
	return t
}

func witherInterval(amplifier int32) int32 {
	t := int32(40) >> uint(amplifier)
	if t < 1 {
		t = 1
	}
	return t
}
