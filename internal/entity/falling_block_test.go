package entity

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestNewFallingBlock(t *testing.T) {
	uuid := protocol.UUID{}
	pos := Vec3{X: 10.5, Y: 64.0, Z: 20.5}
	fb := NewFallingBlock(1, uuid, pos, 118)

	if fb.ID() != 1 {
		t.Errorf("expected ID 1, got %d", fb.ID())
	}
	if fb.BlockStateID() != 118 {
		t.Errorf("expected block state 118, got %d", fb.BlockStateID())
	}
	if fb.ProtocolType() != 49 {
		t.Errorf("expected protocol type 49, got %d", fb.ProtocolType())
	}
	if fb.SpawnData() != 118 {
		t.Errorf("expected spawn data 118, got %d", fb.SpawnData())
	}
}

func TestFallingBlockGravity(t *testing.T) {
	uuid := protocol.UUID{}
	pos := Vec3{X: 10.5, Y: 64.0, Z: 20.5}
	fb := NewFallingBlock(1, uuid, pos, 118)

	noCollision := func(x, y, z float64) bool { return false }

	fb.Tick(noCollision)
	fb.Tick(noCollision)

	newPos := fb.Position()
	if newPos.Y >= 64.0 {
		t.Errorf("expected Y to decrease due to gravity, got %f", newPos.Y)
	}
	if fb.OnGround() {
		t.Error("should not be on ground without collision")
	}
}

func TestFallingBlockLanding(t *testing.T) {
	uuid := protocol.UUID{}
	pos := Vec3{X: 10.5, Y: 64.0, Z: 20.5}
	fb := NewFallingBlock(1, uuid, pos, 118)

	alwaysCollide := func(x, y, z float64) bool { return true }

	fb.Tick(alwaysCollide)
	fb.Tick(alwaysCollide)
	fb.Tick(alwaysCollide)

	if !fb.OnGround() {
		t.Error("should be on ground after collision")
	}
	if !fb.ShouldLand() {
		t.Error("should land after being on ground for more than 1 tick")
	}
}

func TestFallingBlockRemovalBelowWorld(t *testing.T) {
	uuid := protocol.UUID{}
	pos := Vec3{X: 10.5, Y: -120.0, Z: 20.5}
	fb := NewFallingBlock(1, uuid, pos, 118)

	noCollision := func(x, y, z float64) bool { return false }

	for i := 0; i < 50; i++ {
		fb.Tick(noCollision)
	}

	if !fb.Removed() {
		t.Error("falling block should be removed when below Y=-128")
	}
}

func TestFallingBlockManager(t *testing.T) {
	mgr := NewFallingBlockManager()

	uuid := protocol.UUID{}
	fb1 := NewFallingBlock(1, uuid, Vec3{X: 0, Y: 64, Z: 0}, 118)
	fb2 := NewFallingBlock(2, uuid, Vec3{X: 0, Y: 64, Z: 0}, 124)

	mgr.Add(fb1)
	mgr.Add(fb2)

	if mgr.Count() != 2 {
		t.Errorf("expected 2 falling blocks, got %d", mgr.Count())
	}

	if mgr.Get(1) != fb1 {
		t.Error("failed to get falling block 1")
	}

	mgr.Remove(1)
	if mgr.Count() != 1 {
		t.Errorf("expected 1 falling block after removal, got %d", mgr.Count())
	}
}
