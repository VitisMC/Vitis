package session

import (
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// startTrackedPlayerActions starts mining and food use through the registered packet handlers.
func startTrackedPlayerActions(t *testing.T, f *dimensionFixture, router PacketRouter) *playerActions {
	t.Helper()
	s := f.sessions[0]
	player := s.Player().(*entity.Player)
	online := f.players.GetBySession(s)
	online.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:bread"), 2))
	player.Living().SetFoodLevel(10)
	f.worlds[0].Accessor.SetBlock(9, 64, 8, block.DefaultStateID("minecraft:stone"))
	for _, packet := range []protocol.Packet{
		&play.BlockDig{Status: play.DigStarted, Position: play.BlockPos{X: 9, Y: 64, Z: 8}, Face: 1, Sequence: 1},
		&play.UseItem{Hand: 0, Sequence: 2},
	} {
		if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
			t.Fatal(err)
		}
	}
	f.runtime[0].Scheduler().Drain(128)
	stored, ok := s.SessionData().Load(playerActionsKey)
	if !ok {
		t.Fatal("actions were not bound to the session")
	}
	actions := stored.(*playerActions)
	if actions.mining.Get(player.ID()) == nil || actions.eating.Get(player.ID()) == nil {
		t.Fatal("connected handlers did not retain active actions")
	}
	actions.mining.Get(player.ID()).StartedAt = time.Now().Add(-time.Hour)
	player.Living().SetBlocking(true)
	return actions
}

// TestTravelCancelsTransientActionsBeforeReturningToOrigin rejects a finish from the old membership.
func TestTravelCancelsTransientActionsBeforeReturningToOrigin(t *testing.T) {
	f, transfers := transferFixture(t)
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	actions := startTrackedPlayerActions(t, f, router)
	s := f.sessions[0]
	player := s.Player().(*entity.Player)
	for _, dimension := range []string{"minecraft:the_nether", "minecraft:overworld"} {
		if err := transfers.Request(s, dimension, entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}); err != nil {
			t.Fatal(err)
		}
		transfers.Tick()
		if SessionDimension(s) != dimension || actions.mining.Get(player.ID()) != nil || actions.eating.Get(player.ID()) != nil || player.Living().IsBlocking() {
			t.Fatal("travel retained transient actions from the origin")
		}
		if err := s.acknowledgeTeleport(s.pendingTeleport.Load()); err != nil {
			t.Fatal(err)
		}
		player.SetClientLoaded(true)
	}
	if err := router.Handle(s, protocol.StatePlay, &play.BlockDig{Status: play.DigFinished, Position: play.BlockPos{X: 9, Y: 64, Z: 8}, Face: 1, Sequence: 3}); err != nil {
		t.Fatal(err)
	}
	f.runtime[0].Scheduler().Drain(128)
	if f.worlds[0].Accessor.GetBlock(9, 64, 8) != block.DefaultStateID("minecraft:stone") || f.runtime[0].ItemEntities().Count() != 0 {
		t.Fatal("old mining progress survived a return to the same world")
	}
}

// TestDeathAndDisconnectReleasePlayerActions avoids leaked tracker membership after lifecycle cleanup.
func TestDeathAndDisconnectReleasePlayerActions(t *testing.T) {
	for _, death := range []bool{false, true} {
		t.Run(map[bool]string{false: "disconnect", true: "death"}[death], func(t *testing.T) {
			f := newDimensionFixture(t)
			router := NewPacketRouter()
			if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
				t.Fatal(err)
			}
			actions := startTrackedPlayerActions(t, f, router)
			s := f.sessions[0]
			player := s.Player().(*entity.Player)
			online := f.players.GetBySession(s)
			if death {
				player.Living().SetHealth(0)
				ProcessPlayerDeath(player, f.players.ForDimension("minecraft:overworld"), f.worlds[0].Accessor, true)
			} else {
				PreparePlayerDisconnect(f.players.ForDimension("minecraft:overworld"), online, f.worlds[0].Accessor)
			}
			if _, found := s.SessionData().Load(playerActionsKey); found || actions.mining.Get(player.ID()) != nil || actions.eating.Get(player.ID()) != nil || player.Living().IsBlocking() {
				t.Fatal("lifecycle cleanup retained active actions")
			}
		})
	}
}
