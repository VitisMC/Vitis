package physics

import "testing"

// TestUnknownBlocksStopMovement keeps unavailable chunk cells from acting as air.
func TestUnknownBlocksStopMovement(t *testing.T) {
	world := newMockWorld()
	world.blocks[[3]int{1, 64, 0}] = -1
	box := AABB{MinX: 0.5, MinY: 64, MinZ: 0.2, MaxX: 0.9, MaxY: 64.8, MaxZ: 0.8}
	result := MoveWithCollision(world, box, 0.5, 0, 0)
	if !result.CollidedX || result.Dx > 0.100001 {
		t.Fatalf("movement crossed unknown block: %+v", result)
	}
}
