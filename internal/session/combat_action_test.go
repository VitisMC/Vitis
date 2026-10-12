package session

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestEntityAttackRunsOnTickWithCurrentWeapon verifies queue ordering and weapon snapshots.
func TestEntityAttackRunsOnTickWithCurrentWeapon(t *testing.T) {
	p, online, movement := movementTestPlayer(t)
	online.Windows = inventory.NewWindowManager()
	var queued []func()
	attacks := 0
	var source *entity.DamageSource
	world := &DefaultWorldAccessor{
		SubmitTask: func(task func()) bool { queued = append(queued, task); return true },
		AttackMobEntity: func(player *entity.Player, target int32, damage *entity.DamageSource, reach float64) bool {
			if player != p || target != 99 || reach != 3 {
				t.Fatal("wrong authoritative attack context")
			}
			attacks++
			source = damage
			return true
		},
	}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, movement.players, world); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(movement.session, protocol.StatePlay, &play.UseEntity{EntityID: 99, Type: play.UseEntityAttack}); err != nil {
		t.Fatal(err)
	}
	if attacks != 0 || len(queued) != 1 {
		t.Fatal("network handler executed an attack before the tick")
	}
	weapon := inventory.NewSlot(item.IDByName("minecraft:diamond_sword"), 1)
	online.Windows.Inventory().Set(inventory.HotbarStart, weapon)
	queued[0]()
	if attacks != 1 || source.Amount != 7 || source.Weapon.ItemID != weapon.ItemID || source.AttackerID != p.ID() || p.AttackCooldown() == 0 {
		t.Fatal("queued attack ignored current weapon or cooldown")
	}
	if online.Windows.HeldItem().Damage != 1 || source.Weapon.Damage != 0 {
		t.Fatal("attack weapon snapshot changed with durability")
	}
}

// TestEntityAttackRejectsDeadPlayersAndSaturatedQueues keeps invalid requests from changing the world.
func TestEntityAttackRejectsDeadPlayersAndSaturatedQueues(t *testing.T) {
	p, _, movement := movementTestPlayer(t)
	calls := 0
	world := &DefaultWorldAccessor{AttackMobEntity: func(*entity.Player, int32, *entity.DamageSource, float64) bool { calls++; return true }}
	p.Damage(100, "generic")
	command := &attackEntityCommand{session: movement.session, players: movement.players, world: world, target: 99}
	command.Execute()
	if calls != 0 {
		t.Fatal("dead player attacked a mob")
	}
	router := NewPacketRouter()
	world.SubmitTask = func(func()) bool { return false }
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, movement.players, world); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(movement.session, protocol.StatePlay, &play.UseEntity{EntityID: 99, Type: play.UseEntityAttack}); err == nil {
		t.Fatal("saturated combat queue accepted an action")
	}
}
