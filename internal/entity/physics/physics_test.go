package physics

import (
	"math"
	"testing"
)

type mockWorld struct {
	blocks map[[3]int]int32
}

func newMockWorld() *mockWorld {
	return &mockWorld{blocks: make(map[[3]int]int32)}
}

func (m *mockWorld) GetBlockStateAt(x, y, z int) int32 {
	if s, ok := m.blocks[[3]int{x, y, z}]; ok {
		return s
	}
	return 0
}

func (m *mockWorld) setSolid(x, y, z int) {
	m.blocks[[3]int{x, y, z}] = 1
}

func TestCollectBlockCollisions_Empty(t *testing.T) {
	w := newMockWorld()
	box := AABB{0, 0, 0, 1, 1, 1}
	collisions := CollectBlockCollisions(w, box)
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions in empty world, got %d", len(collisions))
	}
}

func TestCollectBlockCollisions_SolidBlock(t *testing.T) {
	w := newMockWorld()
	w.setSolid(0, 0, 0)
	box := AABB{-0.5, -0.5, -0.5, 0.5, 0.5, 0.5}
	collisions := CollectBlockCollisions(w, box)
	if len(collisions) == 0 {
		t.Error("expected at least one collision with solid block at origin")
	}
	found := false
	for _, c := range collisions {
		if c.MinX == 0 && c.MinY == 0 && c.MinZ == 0 &&
			c.MaxX == 1 && c.MaxY == 1 && c.MaxZ == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected full-block AABB at (0,0,0)")
	}
}

func TestMoveWithCollision_FreeFall(t *testing.T) {
	w := newMockWorld()
	box := AABB{0, 10, 0, 1, 11, 1}
	result := MoveWithCollision(w, box, 0, -5, 0)
	if result.Dy != -5 {
		t.Errorf("free fall Dy = %v, want -5", result.Dy)
	}
	if result.OnGround {
		t.Error("should not be on ground in empty world")
	}
}

func TestMoveWithCollision_LandOnBlock(t *testing.T) {
	w := newMockWorld()
	w.setSolid(0, 0, 0)
	box := AABB{0, 2, 0, 0.6, 3.8, 0.6}
	result := MoveWithCollision(w, box, 0, -5, 0)
	if math.Abs(result.Dy-(-1)) > 1e-9 {
		t.Errorf("land Dy = %v, want -1 (from y=2 to y=1)", result.Dy)
	}
	if !result.OnGround {
		t.Error("should be on ground after landing")
	}
}

func TestMoveWithCollision_WalkIntoWall(t *testing.T) {
	w := newMockWorld()
	w.setSolid(2, 0, 0)
	w.setSolid(2, 1, 0)
	box := AABB{0, 0, 0, 0.6, 1.8, 0.6}
	result := MoveWithCollision(w, box, 5, 0, 0)
	expected := 2.0 - 0.6
	if math.Abs(result.Dx-expected) > 1e-9 {
		t.Errorf("wall Dx = %v, want %v", result.Dx, expected)
	}
	if !result.CollidedX {
		t.Error("should have collided X")
	}
}

func TestMoveWithStepUp_StepsOverSlab(t *testing.T) {
	w := newMockWorld()
	w.setSolid(1, 0, 0)
	for x := -1; x <= 2; x++ {
		w.setSolid(x, -1, 0)
	}
	box := AABB{0, 0, 0, 0.6, 1.8, 0.6}
	result := MoveWithStepUp(w, box, 2, 0, 0, StepHeightDefault, true)
	if result.Dx <= 0 {
		t.Errorf("step-up should have positive Dx, got %v", result.Dx)
	}
}

func TestCheckOnGround_OnSolid(t *testing.T) {
	w := newMockWorld()
	w.setSolid(0, 0, 0)
	box := AABB{0, 1, 0, 0.6, 2.8, 0.6}
	if !CheckOnGround(w, box) {
		t.Error("entity at y=1 should be on ground above solid block at y=0")
	}
}

func TestCheckOnGround_InAir(t *testing.T) {
	w := newMockWorld()
	box := AABB{0, 5, 0, 0.6, 6.8, 0.6}
	if CheckOnGround(w, box) {
		t.Error("entity at y=5 in empty world should not be on ground")
	}
}

func TestApplyGravityAndDrag(t *testing.T) {
	vx, vy, vz := ApplyGravityAndDrag(1.0, 0.0, 1.0, GravityDefault, DragDefault, TerminalVelocity)
	if vy >= 0 {
		t.Errorf("vy after gravity = %v, should be negative", vy)
	}
	if vx >= 1.0 {
		t.Errorf("vx after drag = %v, should be < 1.0", vx)
	}
	if vz >= 1.0 {
		t.Errorf("vz after drag = %v, should be < 1.0", vz)
	}
}

func TestApplyGravityAndDrag_TerminalVelocity(t *testing.T) {
	_, vy, _ := ApplyGravityAndDrag(0, -100, 0, GravityDefault, DragDefault, TerminalVelocity)
	if vy < TerminalVelocity {
		t.Errorf("vy = %v, should be clamped to terminal %v", vy, TerminalVelocity)
	}
}

func TestIsCollidingAt(t *testing.T) {
	w := newMockWorld()
	w.setSolid(0, 0, 0)

	inside := AABB{0.1, 0.1, 0.1, 0.9, 0.9, 0.9}
	if !IsCollidingAt(w, inside) {
		t.Error("box inside solid block should be colliding")
	}

	outside := AABB{5, 5, 5, 6, 6, 6}
	if IsCollidingAt(w, outside) {
		t.Error("box far from block should not be colliding")
	}
}

func TestDimensionsMakeBoundingBox(t *testing.T) {
	d := PlayerDimensions()
	box := d.MakeBoundingBox(10.0, 65.0, 20.0)
	if math.Abs(box.MinX-9.7) > 1e-9 {
		t.Errorf("MinX = %v, want 9.7", box.MinX)
	}
	if math.Abs(box.MaxX-10.3) > 1e-9 {
		t.Errorf("MaxX = %v, want 10.3", box.MaxX)
	}
	if box.MinY != 65.0 {
		t.Errorf("MinY = %v, want 65", box.MinY)
	}
	if math.Abs(box.MaxY-66.8) > 1e-9 {
		t.Errorf("MaxY = %v, want 66.8", box.MaxY)
	}
}
