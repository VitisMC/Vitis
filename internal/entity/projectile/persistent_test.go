package projectile

import (
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestPersistentProjectilesRoundTrip verifies flight state, ownership, and pickup policy.
func TestPersistentProjectilesRoundTrip(t *testing.T) {
	uuid, owner := protocol.UUID{3, 4}, protocol.UUID{8, 9}
	pos, vel := entity.Vec3{X: -17, Y: 70, Z: 33}, entity.Vec3{X: 0.1, Y: -0.2, Z: 0.3}
	arrow := NewArrow(1, uuid, pos, vel, 99, owner)
	arrow.life = 71
	arrow.inGround = true
	arrow.SetDamage(4)
	arrow.SetPickup(ArrowPickupCreative)
	arrow.SetPiercing(3)
	arrow.SetCritical(true)
	trident := NewTrident(2, uuid, pos, vel, 99, owner)
	trident.SetLoyalty(3)
	trident.SetChanneling(true)
	trident.returnTimer = 7
	pearl := NewThrownEntity(3, uuid, ThrownEnderPearl, pos, vel, 99, owner)
	pearl.life = 17
	manager := NewManager()
	for _, source := range []entity.PersistentEntity{arrow, trident, pearl} {
		raw, err := source.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		raw.PutString("example:unknown", "keep")
		decoded, err := DecodePersistent(0, raw)
		if err != nil {
			t.Fatal(err)
		}
		again, err := decoded.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(raw, again) {
			t.Fatalf("projectile state changed: %#v -> %#v", raw.Entries(), again.Entries())
		}
		switch e := decoded.(type) {
		case *Arrow:
			manager.AddArrow(e)
		case *Trident:
			manager.AddTrident(e)
		case *ThrownEntity:
			manager.AddThrown(e)
		}
	}
	manager.ResolvePersistentOwners(map[protocol.UUID]int32{owner: 17})
	if manager.GetArrow(0).OwnerID() != 17 || manager.GetThrown(0).OwnerID() != 17 || manager.GetTrident(0).OwnerID() != 17 {
		t.Fatal("owners did not rebind by UUID")
	}
	manager.ResolvePersistentOwners(nil)
	if manager.GetArrow(0).OwnerID() != -1 {
		t.Fatal("stale owner ID retained")
	}
}

// TestGroundedProjectileLifetimeRemainsPersistable exercises cleanup after restoring an old arrow.
func TestGroundedProjectileLifetimeRemainsPersistable(t *testing.T) {
	arrow := NewArrow(1, protocol.UUID{1, 2}, entity.Vec3{}, entity.Vec3{}, -1, protocol.UUID{})
	arrow.life, arrow.inGround = 1200, true
	arrow.Tick(nil, nil)
	if !arrow.Removed() {
		t.Fatal("grounded arrow bypassed lifetime cleanup")
	}
}
