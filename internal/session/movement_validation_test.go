package session

import (
	"fmt"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/states"
)

// movementTestWorld supplies a floor, wall and unavailable chunk boundary.
type movementTestWorld struct {
	*DefaultWorldAccessor
	tick uint64
}

// HasMovementGeometry enables authoritative movement validation in tests.
func (w *movementTestWorld) HasMovementGeometry() bool { return true }

// MovementTick returns the controlled server tick.
func (w *movementTestWorld) MovementTick() uint64 { return w.tick }

// GetBlockStateAt exposes collision shapes without any chunk generation.
func (w *movementTestWorld) GetBlockStateAt(x, y, z int) int32 {
	if x >= 16 {
		return -1
	}
	if y < 64 || (x == 3 && y < 67) {
		return block.DefaultStateID("minecraft:stone")
	}
	return 0
}

// movementTestPlayer creates a tick-owned player and a geometry-backed command.
func movementTestPlayer(t *testing.T) (*entity.Player, *OnlinePlayer, movementCommand) {
	t.Helper()
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Connection: newMockConn(), Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s.Lifecycle() == LifecycleActive {
			_ = s.ForceClose(nil)
		}
	})
	p := entity.NewPlayer(1, protocol.UUID{1}, "moving", entity.Vec3{X: .5, Y: 64, Z: .5}, entity.Vec2{}, s, 0)
	p.SetClientLoaded(true)
	s.BindPlayer(p)
	pm := NewPlayerManager()
	online := &OnlinePlayer{Session: s, EntityID: p.ID(), UUID: p.UUID(), Name: p.Username(), X: .5, Y: 64, Z: .5}
	if err := pm.AddPlayer(online); err != nil {
		t.Fatal(err)
	}
	w := &movementTestWorld{DefaultWorldAccessor: &DefaultWorldAccessor{}, tick: 1}
	return p, online, movementCommand{session: s, world: w, players: pm, hasPosition: true, x: 1, y: 64, z: .5}
}

// TestMovementGeometryRejectsWallsAndUnavailableChunks preserves authoritative position.
func TestMovementGeometryRejectsWallsAndUnavailableChunks(t *testing.T) {
	for _, x := range []float64{4, 16} {
		t.Run(fmt.Sprint(x), func(t *testing.T) {
			p, _, command := movementTestPlayer(t)
			command.x = x
			command.Execute()
			if p.Position().X != .5 {
				t.Fatal("obstructed movement changed the player position")
			}
			if !command.session.(*DefaultSession).hasPendingTeleport() {
				t.Fatal("rejected movement did not request teleport acknowledgement")
			}
		})
	}
}

// TestMovementGeometryDerivesGround ignores contradictory client flags.
func TestMovementGeometryDerivesGround(t *testing.T) {
	p, _, command := movementTestPlayer(t)
	command.onGround = false
	command.Execute()
	if p.Position().X != 1 || !p.OnGround() {
		t.Fatal("valid movement did not use the server's floor")
	}
	command.x, command.y, command.onGround = 1, 65, true
	command.Execute()
	if p.Position().Y != 65 || p.OnGround() {
		t.Fatal("client forged ground support while airborne")
	}
}

// TestMovementBudgetBoundsCumulativePackets prevents repeated small movement bypasses.
func TestMovementBudgetBoundsCumulativePackets(t *testing.T) {
	p, online, command := movementTestPlayer(t)
	p.Living().SetGameMode(3)
	for i := 1; i <= 6; i++ {
		command.x = .5 + float64(i)*2
		command.Execute()
	}
	if p.Position().X != 10.5 || online.movement.initialized {
		t.Fatal("same-tick movement escaped the cumulative displacement limit")
	}
}

// TestLandingIncludesFinalDescent applies fall damage from the accepted final segment.
func TestLandingIncludesFinalDescent(t *testing.T) {
	p, _, command := movementTestPlayer(t)
	p.SetPosition(entity.Vec3{X: .5, Y: 68, Z: .5})
	checkFallDamage(command.session, 68, 64, true)
	if p.Living().Health() != 19 || p.Living().FallDistance() != 0 {
		t.Fatal("landing omitted the final downward displacement")
	}
}

// TestCorrectionBlocksMovementUntilAcknowledged resets the next tick's displacement origin.
func TestCorrectionBlocksMovementUntilAcknowledged(t *testing.T) {
	p, online, command := movementTestPlayer(t)
	command.x = 4
	command.Execute()
	session := command.session.(*DefaultSession)
	command.x = 1
	command.Execute()
	if p.Position().X != .5 {
		t.Fatal("movement bypassed the pending correction")
	}
	(&teleportAckCommand{session: session, players: command.players, id: session.pendingTeleport.Load()}).Execute()
	if session.hasPendingTeleport() || online.movement.initialized {
		t.Fatal("acknowledgement retained the correction or displacement budget")
	}
	command.Execute()
	if p.Position().X != 1 || !online.movement.initialized || online.movement.start.X != .5 {
		t.Fatal("acknowledged movement used a stale origin")
	}
}
