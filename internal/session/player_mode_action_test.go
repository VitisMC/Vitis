package session

import (
	"testing"

	pose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestPlayerModeActionsRespectTickOrderingAndOwnership rejects foreign entity actions.
func TestPlayerModeActionsRespectTickOrderingAndOwnership(t *testing.T) {
	p, online, movement := movementTestPlayer(t)
	var queued []func()
	world := &DefaultWorldAccessor{SubmitTask: func(task func()) bool { queued = append(queued, task); return true }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, movement.players, world); err != nil {
		t.Fatal(err)
	}
	for _, action := range []play.EntityAction{
		{EntityID: p.ID() + 1, ActionID: 3},
		{EntityID: p.ID(), ActionID: 0},
		{EntityID: p.ID(), ActionID: 3},
	} {
		if err := router.Handle(movement.session, protocol.StatePlay, &action); err != nil {
			t.Fatal(err)
		}
	}
	if online.Flags != 0 || len(queued) != 3 {
		t.Fatal("mode action bypassed the world queue")
	}
	queued[0]()
	if online.Flags != 0 {
		t.Fatal("foreign entity action changed player flags")
	}
	queued[1]()
	queued[2]()
	if online.Flags&0x0A != 0x0A || online.Pose != pose.PoseCrouching {
		t.Fatal("tick did not apply the owned pose and sprint actions")
	}
	p.Damage(100, "generic")
	(&playerModeCommand{session: movement.session, players: movement.players, world: world, request: play.EntityAction{EntityID: p.ID(), ActionID: 1}}).Execute()
	if online.Pose != pose.PoseCrouching {
		t.Fatal("dead player changed pose")
	}
}
