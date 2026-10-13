package entity

import (
	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
)

// SyncEffectMovement reconciles speed effects with one modifier per active effect.
func (l *LivingEntity) SyncEffectMovement() {
	if l.effects == nil || l.attributes == nil {
		return
	}
	for index, definition := range [...]struct {
		id     int32
		name   string
		amount float32
	}{
		{effectdata.EffectSpeed, "minecraft:effect.speed", .2},
		{effectdata.EffectSlowness, "minecraft:effect.slowness", -.15},
	} {
		level := int32(0)
		if instance := l.effects.Get(definition.id); instance != nil {
			level = instance.Amplifier + 1
		}
		if l.effectMovementInitialized && l.effectMovementLevels[index] == level {
			continue
		}
		l.attributes.RemoveModifier(genattr.AttrMovementSpeed, definition.name)
		if level > 0 {
			l.attributes.AddModifier(genattr.AttrMovementSpeed, Modifier{ID: definition.name, Amount: float64(definition.amount) * float64(level), Operation: 2})
		}
		l.effectMovementLevels[index] = level
	}
	l.effectMovementInitialized = true
}
