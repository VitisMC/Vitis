package physics

import (
	"math"
	"testing"
)

func TestNewAABB_Normalizes(t *testing.T) {
	a := NewAABB(5, 5, 5, 1, 1, 1)
	if a.MinX != 1 || a.MinY != 1 || a.MinZ != 1 {
		t.Errorf("expected min (1,1,1), got (%v,%v,%v)", a.MinX, a.MinY, a.MinZ)
	}
	if a.MaxX != 5 || a.MaxY != 5 || a.MaxZ != 5 {
		t.Errorf("expected max (5,5,5), got (%v,%v,%v)", a.MaxX, a.MaxY, a.MaxZ)
	}
}

func TestIntersects(t *testing.T) {
	a := AABB{0, 0, 0, 2, 2, 2}
	tests := []struct {
		name string
		b    AABB
		want bool
	}{
		{"overlap", AABB{1, 1, 1, 3, 3, 3}, true},
		{"inside", AABB{0.5, 0.5, 0.5, 1.5, 1.5, 1.5}, true},
		{"touching_face", AABB{2, 0, 0, 3, 2, 2}, false},
		{"separate", AABB{3, 3, 3, 4, 4, 4}, false},
		{"edge_touch", AABB{2, 2, 0, 3, 3, 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.Intersects(tt.b); got != tt.want {
				t.Errorf("Intersects() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	a := AABB{0, 0, 0, 1, 1, 1}
	if !a.Contains(0.5, 0.5, 0.5) {
		t.Error("center should be contained")
	}
	if !a.Contains(0, 0, 0) {
		t.Error("min corner should be contained (inclusive)")
	}
	if !a.Contains(1, 1, 1) {
		t.Error("max corner should be contained (inclusive)")
	}
	if a.Contains(1.1, 0.5, 0.5) {
		t.Error("outside should not be contained")
	}
}

func TestOffset(t *testing.T) {
	a := AABB{1, 2, 3, 4, 5, 6}
	b := a.Offset(10, 20, 30)
	if b.MinX != 11 || b.MinY != 22 || b.MinZ != 33 {
		t.Errorf("min = (%v,%v,%v), want (11,22,33)", b.MinX, b.MinY, b.MinZ)
	}
	if b.MaxX != 14 || b.MaxY != 25 || b.MaxZ != 36 {
		t.Errorf("max = (%v,%v,%v), want (14,25,36)", b.MaxX, b.MaxY, b.MaxZ)
	}
}

func TestExpand(t *testing.T) {
	a := AABB{0, 0, 0, 1, 1, 1}

	pos := a.Expand(0.5, 0, 0)
	if pos.MaxX != 1.5 {
		t.Errorf("positive expand MaxX = %v, want 1.5", pos.MaxX)
	}
	if pos.MinX != 0 {
		t.Errorf("positive expand MinX = %v, want 0", pos.MinX)
	}

	neg := a.Expand(-0.5, 0, 0)
	if neg.MinX != -0.5 {
		t.Errorf("negative expand MinX = %v, want -0.5", neg.MinX)
	}
	if neg.MaxX != 1 {
		t.Errorf("negative expand MaxX = %v, want 1", neg.MaxX)
	}
}

func TestGrowContract(t *testing.T) {
	a := AABB{1, 1, 1, 3, 3, 3}
	g := a.Grow(0.5)
	if g.MinX != 0.5 || g.MaxX != 3.5 {
		t.Errorf("grow X: %v-%v, want 0.5-3.5", g.MinX, g.MaxX)
	}
	c := a.Contract(0.25)
	if c.MinX != 1.25 || c.MaxX != 2.75 {
		t.Errorf("contract X: %v-%v, want 1.25-2.75", c.MinX, c.MaxX)
	}
}

func TestSizeAndCenter(t *testing.T) {
	a := AABB{1, 2, 3, 5, 8, 9}
	if a.SizeX() != 4 {
		t.Errorf("SizeX = %v, want 4", a.SizeX())
	}
	if a.SizeY() != 6 {
		t.Errorf("SizeY = %v, want 6", a.SizeY())
	}
	if a.SizeZ() != 6 {
		t.Errorf("SizeZ = %v, want 6", a.SizeZ())
	}
	if a.CenterX() != 3 {
		t.Errorf("CenterX = %v, want 3", a.CenterX())
	}
	if a.CenterY() != 5 {
		t.Errorf("CenterY = %v, want 5", a.CenterY())
	}
}

func TestClipYCollide_FallingOntoBlock(t *testing.T) {
	entity := AABB{0, 1, 0, 1, 2, 1}
	block := AABB{0, 0, 0, 1, 1, 1}
	dy := entity.ClipYCollide(block, -0.5)
	if dy != 0 {
		t.Errorf("ClipYCollide = %v, want 0 (entity rests on block)", dy)
	}
}

func TestClipYCollide_FallingFar(t *testing.T) {
	entity := AABB{0, 5, 0, 1, 6, 1}
	block := AABB{0, 0, 0, 1, 1, 1}
	dy := entity.ClipYCollide(block, -10)
	if dy != -4 {
		t.Errorf("ClipYCollide = %v, want -4 (entity lands on block at y=1)", dy)
	}
}

func TestClipYCollide_NoOverlapXZ(t *testing.T) {
	entity := AABB{5, 1, 5, 6, 2, 6}
	block := AABB{0, 0, 0, 1, 1, 1}
	dy := entity.ClipYCollide(block, -10)
	if dy != -10 {
		t.Errorf("ClipYCollide = %v, want -10 (no XZ overlap)", dy)
	}
}

func TestClipXCollide_WalkIntoWall(t *testing.T) {
	entity := AABB{0, 0, 0, 0.6, 1.8, 0.6}
	wall := AABB{1, 0, 0, 2, 1, 1}
	dx := entity.ClipXCollide(wall, 1.0)
	if math.Abs(dx-0.4) > 1e-9 {
		t.Errorf("ClipXCollide = %v, want 0.4", dx)
	}
}

func TestClipZCollide_WalkIntoWall(t *testing.T) {
	entity := AABB{0, 0, 0, 0.6, 1.8, 0.6}
	wall := AABB{0, 0, 1, 1, 1, 2}
	dz := entity.ClipZCollide(wall, 1.0)
	if math.Abs(dz-0.4) > 1e-9 {
		t.Errorf("ClipZCollide = %v, want 0.4", dz)
	}
}

func TestClipYCollide_JumpingUpward(t *testing.T) {
	entity := AABB{0, 0, 0, 1, 1, 1}
	ceiling := AABB{0, 3, 0, 1, 4, 1}
	dy := entity.ClipYCollide(ceiling, 5)
	if dy != 2 {
		t.Errorf("ClipYCollide upward = %v, want 2", dy)
	}
}

func TestIsZero(t *testing.T) {
	zero := AABB{}
	if !zero.IsZero() {
		t.Error("zero AABB should be zero")
	}
	nonzero := AABB{0, 0, 0, 1, 1, 1}
	if nonzero.IsZero() {
		t.Error("unit AABB should not be zero")
	}
}
