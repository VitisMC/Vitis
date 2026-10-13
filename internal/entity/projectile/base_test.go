package projectile

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

type fixedFinder []EntityHit

func (f fixedFinder) FindEntitiesInAABB(physics.AABB, int32) []EntityHit { return f }

func TestEntityHitRequiresSegmentIntersection(t *testing.T) {
	base := NewBase(1, protocol.UUID{}, entity.EntityType(0), entity.Vec3{}, entity.Vec3{}, 0, protocol.UUID{}, 0, 0)
	dims := physics.EntityDimensions{Width: 0.25, Height: 0.25}
	hit := base.checkEntityHit(fixedFinder{{ID: 2, Position: entity.Vec3{X: 5, Y: 4, Z: 0}, Width: 1, Height: 1}}, entity.Vec3{}, entity.Vec3{X: 10, Y: 0, Z: 0}, dims)
	if hit != nil {
		t.Fatalf("missed entity reported as hit: %#v", hit)
	}
}

func TestEntityHitUsesNearestSegmentIntersection(t *testing.T) {
	base := NewBase(1, protocol.UUID{}, entity.EntityType(0), entity.Vec3{}, entity.Vec3{}, 0, protocol.UUID{}, 0, 0)
	dims := physics.EntityDimensions{Width: 0.25, Height: 0.25}
	hit := base.checkEntityHit(fixedFinder{
		{ID: 2, Position: entity.Vec3{X: 8, Y: 0, Z: 0}, Width: 1, Height: 1},
		{ID: 3, Position: entity.Vec3{X: 3, Y: 0, Z: 0}, Width: 1, Height: 1},
	}, entity.Vec3{}, entity.Vec3{X: 10, Y: 0, Z: 0}, dims)
	if hit == nil || hit.EntityID != 3 {
		t.Fatalf("nearest hit = %#v, want entity 3", hit)
	}
}
