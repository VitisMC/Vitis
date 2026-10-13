package entity

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

// simulationTestWorld exposes a mutable activity gate and empty loaded block data.
type simulationTestWorld struct{ active bool }

// IsEntityTicking returns the test's authoritative simulation policy.
func (w *simulationTestWorld) IsEntityTicking(int32, int32) bool { return w.active }

// GetBlockStateAt returns air for physics checks in the isolated test world.
func (w *simulationTestWorld) GetBlockStateAt(int, int, int) int32 { return 0 }

// TestManagersPauseEntityStateAndContinueRemovals checks fuse, age, movement, and cleanup.
func TestManagersPauseEntityStateAndContinueRemovals(t *testing.T) {
	world := &simulationTestWorld{}
	pos := Vec3{X: 8, Y: 64, Z: 8}
	tnt := NewTNTEntityWithFuse(1, protocol.UUID{1}, pos, 0, 2)
	tnts := NewTNTManager()
	tnts.Add(tnt)
	orb := NewXPOrb(2, protocol.UUID{2}, pos, 1)
	orbs := NewXPOrbManager()
	orbs.Add(orb)
	entities := NewManager()
	mobs := NewMobManager(entities, entities.AllocateID)
	mob := mobs.SpawnMob("zombie", pos)
	if mob == nil {
		t.Fatal("missing zombie type")
	}
	tnts.TickAll(world)
	orbs.TickAll(world, nil)
	mobs.Tick(world)
	if tnt.Fuse() != 2 || orb.age != 0 || mob.AgeTicks() != 0 || orb.Position() != pos {
		t.Fatal("inactive entity state advanced")
	}
	world.active = true
	tnts.TickAll(world)
	orbs.TickAll(world, nil)
	mobs.Tick(world)
	if tnt.Fuse() != 1 || orb.age != 1 || mob.AgeTicks() != 1 || orb.Position() == pos {
		t.Fatal("reactivated entity state did not advance")
	}
	world.active = false
	orb.Remove()
	removed := orbs.TickAll(world, nil)
	if len(removed) != 1 || orbs.Get(orb.ID()) != nil {
		t.Fatal("inactive entity removal was deferred indefinitely")
	}
}
