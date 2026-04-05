package entity

import (
	"sync"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// Modifier represents an attribute modifier with an operation type.
// Operations: 0 = add, 1 = multiply_base, 2 = multiply_total
type Modifier struct {
	ID        string
	Amount    float64
	Operation int8
}

// Attribute holds the base value and active modifiers for a single attribute.
type Attribute struct {
	Base      float64
	Modifiers []Modifier
}

// Value computes the effective attribute value after all modifiers.
func (a *Attribute) Value() float64 {
	base := a.Base
	var addSum float64
	var mulBaseSum float64
	mulTotal := 1.0
	for _, m := range a.Modifiers {
		switch m.Operation {
		case 0:
			addSum += m.Amount
		case 1:
			mulBaseSum += m.Amount
		case 2:
			mulTotal *= 1.0 + m.Amount
		}
	}
	return (base + addSum) * (1.0 + mulBaseSum) * mulTotal
}

// AttributeContainer holds a set of entity attributes keyed by protocol ID.
type AttributeContainer struct {
	mu    sync.RWMutex
	attrs map[int32]*Attribute
	dirty map[int32]bool
}

// NewAttributeContainer creates an empty attribute container.
func NewAttributeContainer() *AttributeContainer {
	return &AttributeContainer{
		attrs: make(map[int32]*Attribute),
		dirty: make(map[int32]bool),
	}
}

// DefaultPlayerAttributes returns an attribute container initialized with
// standard player attribute defaults for Minecraft 1.21.4.
func DefaultPlayerAttributes() *AttributeContainer {
	ac := NewAttributeContainer()
	ac.attrs[genattr.AttrMaxHealth] = &Attribute{Base: 20.0}
	ac.attrs[genattr.AttrMovementSpeed] = &Attribute{Base: 0.1}
	ac.attrs[genattr.AttrAttackDamage] = &Attribute{Base: 1.0}
	ac.attrs[genattr.AttrAttackSpeed] = &Attribute{Base: 4.0}
	ac.attrs[genattr.AttrArmor] = &Attribute{Base: 0.0}
	ac.attrs[genattr.AttrArmorToughness] = &Attribute{Base: 0.0}
	ac.attrs[genattr.AttrAttackKnockback] = &Attribute{Base: 0.0}
	ac.attrs[genattr.AttrKnockbackResistance] = &Attribute{Base: 0.0}
	ac.attrs[genattr.AttrLuck] = &Attribute{Base: 0.0}
	ac.attrs[genattr.AttrFlyingSpeed] = &Attribute{Base: 0.02}
	ac.attrs[genattr.AttrBlockBreakSpeed] = &Attribute{Base: 1.0}
	ac.attrs[genattr.AttrGravity] = &Attribute{Base: 0.08}
	ac.attrs[genattr.AttrJumpStrength] = &Attribute{Base: 0.42}
	ac.attrs[genattr.AttrSafeFallDistance] = &Attribute{Base: 3.0}
	ac.attrs[genattr.AttrScale] = &Attribute{Base: 1.0}
	ac.attrs[genattr.AttrStepHeight] = &Attribute{Base: 0.6}
	ac.attrs[genattr.AttrBlockInteractionRange] = &Attribute{Base: 4.5}
	ac.attrs[genattr.AttrEntityInteractionRange] = &Attribute{Base: 3.0}
	ac.attrs[genattr.AttrSweepingDamageRatio] = &Attribute{Base: 0.0}
	ac.attrs[genattr.AttrFallDamageMultiplier] = &Attribute{Base: 1.0}
	return ac
}

// SetBase sets the base value for an attribute and marks it dirty.
func (ac *AttributeContainer) SetBase(id int32, value float64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	a, ok := ac.attrs[id]
	if !ok {
		a = &Attribute{}
		ac.attrs[id] = a
	}
	a.Base = value
	ac.dirty[id] = true
}

// GetBase returns the base value for an attribute.
func (ac *AttributeContainer) GetBase(id int32) float64 {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	if a, ok := ac.attrs[id]; ok {
		return a.Base
	}
	return 0
}

// GetValue returns the computed value for an attribute after modifiers.
func (ac *AttributeContainer) GetValue(id int32) float64 {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	if a, ok := ac.attrs[id]; ok {
		return a.Value()
	}
	return 0
}

// AddModifier adds a modifier to an attribute and marks it dirty.
func (ac *AttributeContainer) AddModifier(attrID int32, mod Modifier) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	a, ok := ac.attrs[attrID]
	if !ok {
		a = &Attribute{}
		ac.attrs[attrID] = a
	}
	a.Modifiers = append(a.Modifiers, mod)
	ac.dirty[attrID] = true
}

// RemoveModifier removes all modifiers with the given ID from an attribute.
func (ac *AttributeContainer) RemoveModifier(attrID int32, modID string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	a, ok := ac.attrs[attrID]
	if !ok {
		return
	}
	filtered := a.Modifiers[:0]
	for _, m := range a.Modifiers {
		if m.ID != modID {
			filtered = append(filtered, m)
		}
	}
	a.Modifiers = filtered
	ac.dirty[attrID] = true
}

// ClearModifiers removes all modifiers from an attribute.
func (ac *AttributeContainer) ClearModifiers(attrID int32) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if a, ok := ac.attrs[attrID]; ok {
		a.Modifiers = nil
		ac.dirty[attrID] = true
	}
}

// ToProperties converts all attributes to protocol AttributeProperty slice.
func (ac *AttributeContainer) ToProperties() []playpacket.AttributeProperty {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	props := make([]playpacket.AttributeProperty, 0, len(ac.attrs))
	for id, a := range ac.attrs {
		prop := playpacket.AttributeProperty{
			ID:    id,
			Value: a.Base,
		}
		for _, m := range a.Modifiers {
			prop.Modifiers = append(prop.Modifiers, playpacket.AttributeModifier{
				ID:        m.ID,
				Amount:    m.Amount,
				Operation: m.Operation,
			})
		}
		props = append(props, prop)
	}
	return props
}

// DirtyProperties returns properties for attributes that changed since last call.
// Clears the dirty set.
func (ac *AttributeContainer) DirtyProperties() []playpacket.AttributeProperty {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if len(ac.dirty) == 0 {
		return nil
	}
	props := make([]playpacket.AttributeProperty, 0, len(ac.dirty))
	for id := range ac.dirty {
		a := ac.attrs[id]
		if a == nil {
			continue
		}
		prop := playpacket.AttributeProperty{
			ID:    id,
			Value: a.Base,
		}
		for _, m := range a.Modifiers {
			prop.Modifiers = append(prop.Modifiers, playpacket.AttributeModifier{
				ID:        m.ID,
				Amount:    m.Amount,
				Operation: m.Operation,
			})
		}
		props = append(props, prop)
	}
	ac.dirty = make(map[int32]bool)
	return props
}
