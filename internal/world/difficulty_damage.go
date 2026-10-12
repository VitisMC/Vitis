package world

import (
	"strings"

	"github.com/vitismc/vitis/internal/entity"
)

// playerDifficultyDamage copies scaled combat damage without changing shared hit context.
func (w *World) playerDifficultyDamage(source *entity.DamageSource) *entity.DamageSource {
	scales := source.Type == entity.DamageTypeExplosion
	if source.Type == entity.DamageTypeMobAttack || source.Type == entity.DamageTypeProjectile {
		scales = entity.GetMobTypeDef(strings.TrimPrefix(source.AttackerType, "minecraft:")) != nil
	}
	if !scales {
		return source
	}
	copied := *source
	switch w.Difficulty() {
	case 0:
		copied.Amount = 0
	case 1:
		copied.Amount = min(source.Amount/2+1, source.Amount)
	case 3:
		copied.Amount = source.Amount * 3 / 2
	}
	return &copied
}
