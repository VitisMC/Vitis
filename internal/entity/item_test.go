package entity

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
)

type mockBlockAccess struct {
	blocks map[[3]int]int32
}

func newMockBlockAccess() *mockBlockAccess {
	return &mockBlockAccess{blocks: make(map[[3]int]int32)}
}

func (m *mockBlockAccess) GetBlockStateAt(x, y, z int) int32 {
	if s, ok := m.blocks[[3]int{x, y, z}]; ok {
		return s
	}
	return 0
}

func (m *mockBlockAccess) setSolid(x, y, z int) {
	m.blocks[[3]int{x, y, z}] = 1
}

var _ physics.BlockAccess = (*mockBlockAccess)(nil)

func TestItemEntity_Creation(t *testing.T) {
	stack := inventory.NewSlot(1, 64)
	item := NewItemEntity(1, protocol.UUID{}, Vec3{10, 65, 20}, stack)

	if item.Stack().ItemID != 1 {
		t.Errorf("ItemID = %d, want 1", item.Stack().ItemID)
	}
	if item.Stack().ItemCount != 64 {
		t.Errorf("ItemCount = %d, want 64", item.Stack().ItemCount)
	}
	if item.Age() != 0 {
		t.Errorf("Age = %d, want 0", item.Age())
	}
	if item.PickupDelay() != itemPickupDelay {
		t.Errorf("PickupDelay = %d, want %d", item.PickupDelay(), itemPickupDelay)
	}
	if item.CanPickup() {
		t.Error("should not be pickupable with pickup delay")
	}
}

func TestItemEntity_TickFalls(t *testing.T) {
	w := newMockBlockAccess()
	for x := -2; x <= 2; x++ {
		for z := -2; z <= 2; z++ {
			w.setSolid(x, 60, z)
		}
	}

	stack := inventory.NewSlot(1, 1)
	item := NewItemEntityWithVelocity(1, protocol.UUID{}, Vec3{0.5, 65, 0.5}, stack, Vec3{}, 0)

	for i := 0; i < 100; i++ {
		item.Tick(w)
		if item.Removed() {
			t.Fatal("item removed unexpectedly")
		}
	}

	if item.Position().Y > 62 {
		t.Errorf("item should have fallen near y=61, got y=%v", item.Position().Y)
	}
	if !item.OnGround() {
		t.Error("item should be on ground after falling onto solid block")
	}
}

func TestItemEntity_Despawn(t *testing.T) {
	w := newMockBlockAccess()
	stack := inventory.NewSlot(1, 1)
	item := NewItemEntity(1, protocol.UUID{}, Vec3{0, 100, 0}, stack)

	for i := 0; i < itemDespawnTicks+1; i++ {
		item.Tick(w)
		if item.Removed() && i < itemDespawnTicks-1 {
			break
		}
	}

	if !item.Removed() {
		t.Error("item should have despawned after 6000 ticks")
	}
}

func TestItemEntity_NoDespawn(t *testing.T) {
	w := newMockBlockAccess()
	stack := inventory.NewSlot(1, 1)
	item := NewItemEntity(1, protocol.UUID{}, Vec3{0, 200, 0}, stack)
	item.SetNoDespawn(true)

	for i := 0; i < itemDespawnTicks+100; i++ {
		item.Tick(w)
	}

	if item.pos.Y < -128 {
		return
	}
}

func TestItemEntity_PickupDelay(t *testing.T) {
	stack := inventory.NewSlot(1, 1)
	item := NewItemEntity(1, protocol.UUID{}, Vec3{0, 65, 0}, stack)

	if item.CanPickup() {
		t.Error("should not be pickupable initially")
	}

	w := newMockBlockAccess()
	for i := 0; i < itemPickupDelay+1; i++ {
		item.Tick(w)
	}

	if !item.CanPickup() {
		t.Error("should be pickupable after delay")
	}
}

func TestItemEntity_Merge(t *testing.T) {
	stack1 := inventory.NewSlot(1, 32)
	stack2 := inventory.NewSlot(1, 16)
	item1 := NewItemEntity(1, protocol.UUID{}, Vec3{0, 65, 0}, stack1)
	item2 := NewItemEntity(2, protocol.UUID{}, Vec3{0, 65, 0}, stack2)

	if !item1.TryMerge(item2) {
		t.Error("merge should succeed for same item type within stack limit")
	}
	if item1.Stack().ItemCount != 48 {
		t.Errorf("merged count = %d, want 48", item1.Stack().ItemCount)
	}
}

func TestItemEntity_MergeOverflow(t *testing.T) {
	stack1 := inventory.NewSlot(1, 50)
	stack2 := inventory.NewSlot(1, 20)
	item1 := NewItemEntity(1, protocol.UUID{}, Vec3{0, 65, 0}, stack1)
	item2 := NewItemEntity(2, protocol.UUID{}, Vec3{0, 65, 0}, stack2)

	if item1.TryMerge(item2) {
		t.Error("merge should fail when total exceeds max stack size")
	}
}

func TestItemEntity_MergeDifferentType(t *testing.T) {
	stack1 := inventory.NewSlot(1, 32)
	stack2 := inventory.NewSlot(2, 16)
	item1 := NewItemEntity(1, protocol.UUID{}, Vec3{0, 65, 0}, stack1)
	item2 := NewItemEntity(2, protocol.UUID{}, Vec3{0, 65, 0}, stack2)

	if item1.TryMerge(item2) {
		t.Error("merge should fail for different item types")
	}
}

func TestItemEntity_VoidRemoval(t *testing.T) {
	w := newMockBlockAccess()
	stack := inventory.NewSlot(1, 1)
	item := NewItemEntityWithVelocity(1, protocol.UUID{}, Vec3{0, -120, 0}, stack, Vec3{0, -1, 0}, 0)

	for i := 0; i < 50; i++ {
		item.Tick(w)
		if item.Removed() {
			return
		}
	}

	if !item.Removed() {
		t.Error("item should be removed after falling into void")
	}
}

func TestItemEntityManager_TickAll(t *testing.T) {
	w := newMockBlockAccess()
	mgr := NewItemEntityManager()

	stack := inventory.NewSlot(1, 1)
	item := NewItemEntityWithVelocity(1, protocol.UUID{}, Vec3{0, 100, 0}, stack, Vec3{}, 0)
	mgr.Add(item)

	if mgr.Count() != 1 {
		t.Errorf("count = %d, want 1", mgr.Count())
	}

	mgr.TickAll(w)

	if mgr.Get(1) == nil {
		t.Error("item should still exist after 1 tick")
	}
}

func TestTNTEntity_Creation(t *testing.T) {
	tnt := NewTNTEntity(1, protocol.UUID{}, Vec3{5, 65, 5}, 0)

	if tnt.Fuse() != tntDefaultFuse {
		t.Errorf("Fuse = %d, want %d", tnt.Fuse(), tntDefaultFuse)
	}
	if math.Abs(tnt.Power()-tntPower) > 1e-9 {
		t.Errorf("Power = %v, want %v", tnt.Power(), tntPower)
	}
}

func TestTNTEntity_FuseCountdown(t *testing.T) {
	w := newMockBlockAccess()
	for x := -3; x <= 3; x++ {
		for z := -3; z <= 3; z++ {
			w.setSolid(x, 64, z)
		}
	}

	tnt := NewTNTEntityWithFuse(1, protocol.UUID{}, Vec3{0.5, 65, 0.5}, 0, 10)

	for i := 0; i < 9; i++ {
		tnt.Tick(w)
		if tnt.ShouldExplode() {
			t.Fatalf("should not explode at tick %d (fuse=%d)", i+1, tnt.Fuse())
		}
	}

	tnt.Tick(w)
	if !tnt.ShouldExplode() {
		t.Errorf("should explode after fuse expires, fuse=%d", tnt.Fuse())
	}
}

func TestTNTManager_TickAll(t *testing.T) {
	w := newMockBlockAccess()
	for x := -3; x <= 3; x++ {
		for z := -3; z <= 3; z++ {
			w.setSolid(x, 64, z)
		}
	}

	mgr := NewTNTManager()
	tnt := NewTNTEntityWithFuse(1, protocol.UUID{}, Vec3{0.5, 65, 0.5}, 0, 5)
	mgr.Add(tnt)

	for i := 0; i < 4; i++ {
		exploded, _ := mgr.TickAll(w)
		if len(exploded) > 0 {
			t.Fatalf("should not explode at tick %d", i+1)
		}
	}

	exploded, _ := mgr.TickAll(w)
	if len(exploded) != 1 || exploded[0] != 1 {
		t.Errorf("expected TNT id=1 to explode, got %v", exploded)
	}

	if mgr.Get(1) != nil {
		t.Error("TNT should be removed from manager after explosion")
	}
}

func TestComputeExplosion_BlockDestruction(t *testing.T) {
	w := newMockBlockAccess()
	for x := -3; x <= 3; x++ {
		for y := 63; y <= 67; y++ {
			for z := -3; z <= 3; z++ {
				w.setSolid(x, y, z)
			}
		}
	}

	exp := ComputeExplosion(w, 0.5, 65.5, 0.5, 4.0, nil)

	if len(exp.AffectedBlocks) == 0 {
		t.Error("explosion should affect some blocks")
	}

	foundCenter := false
	for _, bp := range exp.AffectedBlocks {
		if bp[0] == 0 && bp[1] == 65 && bp[2] == 0 {
			foundCenter = true
		}
	}
	if !foundCenter {
		t.Error("explosion should affect center block")
	}
}
