package ai

import (
	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	"github.com/vitismc/vitis/internal/entity"
)

// movementSpeed scales a goal's configured speed using current movement attributes.
func movementSpeed(mob *entity.MobEntity, configured float64) float64 {
	mob.SyncEffectMovement()
	base := mob.TypeDef().MoveSpeed
	if base <= 0 {
		return configured
	}
	return configured * max(0, mob.Attributes().GetValue(genattr.AttrMovementSpeed)) / base
}
