package physics

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	gencollision "github.com/vitismc/vitis/internal/data/generated/collision"
)

// TestStepUpOntoSlabPreservesVerticalDisplacement rejects double-counted descent.
func TestStepUpOntoSlabPreservesVerticalDisplacement(t *testing.T) {
	world := newMockWorld()
	for x := -1; x < 4; x++ {
		world.blocks[[3]int{x, 63, 0}] = block.DefaultStateID("minecraft:stone")
	}
	world.blocks[[3]int{1, 64, 0}] = block.StateID("minecraft:stone_slab", map[string]string{"type": "bottom", "waterlogged": "false"})
	box := PlayerDimensions().MakeBoundingBox(.5, 64, .5)
	result := MoveWithStepUp(world, box, 1, .1, 0, .6, true)
	if math.Abs(result.Dx-1) > 1e-8 || math.Abs(result.Dy-.5) > 1e-8 || !result.OnGround {
		t.Fatalf("slab step displacement: %+v", result)
	}
}

// TestCollisionQueryBoundsCoverRegistry guards the query's shape overhang limits.
func TestCollisionQueryBoundsCoverRegistry(t *testing.T) {
	for index, shapes := range gencollision.Shapes {
		for _, shape := range shapes {
			if shape.MinX < -0.25 || shape.MaxX > 1.25 || shape.MinZ < -0.25 || shape.MaxZ > 1.25 ||
				shape.MinY < -0.25 || shape.MaxY > 1.5 {
				t.Fatalf("shape %d exceeds collision query bounds: %+v", index, shape)
			}
		}
	}
}

// TestCollisionQueriesRespectPartialShapes checks body overlap and actual support.
func TestCollisionQueriesRespectPartialShapes(t *testing.T) {
	w := newMockWorld()
	w.blocks[[3]int{0, 64, 0}] = block.StateID("minecraft:stone_slab", map[string]string{"type": "bottom", "waterlogged": "false"})
	box := PlayerDimensions().MakeBoundingBox(0.5, 64.5, 0.5)
	if IsCollidingAt(w, box) || len(CollectBlockCollisions(w, box)) != 0 {
		t.Fatal("standing above a slab was treated as penetration")
	}
	if !CheckOnGround(w, box) {
		t.Fatal("slab did not support the player")
	}
	box = box.Offset(0, 0.25, 0)
	if CheckOnGround(w, box) {
		t.Fatal("a non-intersecting partial block supplied false ground contact")
	}
}

// TestCollisionQueriesIncludeNeighborOverhangs checks tall and horizontal shapes.
func TestCollisionQueriesIncludeNeighborOverhangs(t *testing.T) {
	w := newMockWorld()
	w.blocks[[3]int{0, 64, 0}] = block.StateID("minecraft:oak_fence", map[string]string{
		"east": "false", "west": "false", "north": "false", "south": "false", "waterlogged": "false",
	})
	if !IsCollidingAt(w, PlayerDimensions().MakeBoundingBox(0.5, 65.1, 0.5)) {
		t.Fatal("fence protruding from the block below was missed")
	}
	if !CheckOnGround(w, PlayerDimensions().MakeBoundingBox(0.5, 65.5, 0.5)) {
		t.Fatal("tall fence did not supply ground contact")
	}
	w.blocks[[3]int{0, 64, 0}] = block.StateID("minecraft:piston_head", map[string]string{
		"facing": "north", "short": "false", "type": "normal",
	})
	if !IsCollidingAt(w, NewAABB(0.4, 64.4, 1.2, 0.6, 64.6, 1.8)) {
		t.Fatal("piston arm extending into the neighboring cell was missed")
	}
}
