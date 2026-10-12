package session

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestMovementPacketsExecuteInSessionOrderOnTick verifies deferred state changes.
func TestMovementPacketsExecuteInSessionOrderOnTick(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{21}
	player := entity.NewPlayer(21, uuid, "mover", entity.Vec3{}, entity.Vec2{}, s, 2)
	player.SetClientLoaded(true)
	s.BindPlayer(player)
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{EntityID: 21, UUID: uuid, Name: "mover", Session: s}); err != nil {
		t.Fatal(err)
	}
	var queued []func()
	accessor := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = append(queued, task)
		return true
	}}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, accessor); err != nil {
		t.Fatal(err)
	}
	packets := []protocol.Packet{
		&playpacket.SetPlayerPosition{X: 1, Y: 64, Z: 2},
		&playpacket.SetPlayerPositionAndRotation{X: 2, Y: 64, Z: 3, Yaw: 90, Pitch: 10},
		&playpacket.SetPlayerRotation{Yaw: 180, Pitch: 20},
		&playpacket.SetPlayerOnGround{OnGround: true},
	}
	for _, packet := range packets {
		if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
			t.Fatalf("handle movement %T: %v", packet, err)
		}
	}
	if len(queued) != len(packets) {
		t.Fatalf("queued %d movements, want %d", len(queued), len(packets))
	}
	if player.Position() != (entity.Vec3{}) || player.OnGround() {
		t.Fatal("player moved before the world tick")
	}
	for _, task := range queued {
		task()
	}
	if got := player.Position(); got != (entity.Vec3{X: 2, Y: 64, Z: 3}) {
		t.Fatalf("player position = %+v", got)
	}
	if got := player.Rotation(); got != (entity.Vec2{X: 180, Y: 20}) {
		t.Fatalf("player rotation = %+v", got)
	}
	if !player.OnGround() {
		t.Fatal("on-ground state was not applied")
	}
	if online := players.GetByUUID(uuid); online.X != 2 || online.Yaw != 180 {
		t.Fatalf("online player state = %+v", online)
	}
}

func TestMovementWaitsForPlayerLoaded(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	player := entity.NewPlayer(22, protocol.UUID{22}, "loading", entity.Vec3{Y: 64}, entity.Vec2{}, s, 2)
	s.BindPlayer(player)
	var queued []func()
	accessor := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = append(queued, task)
		return true
	}}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, accessor); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetPlayerPosition{X: 0, Y: -80, Z: 0}); err != nil {
		t.Fatal(err)
	}
	queued[0]()
	if got := player.Position().Y; got != 64 {
		t.Fatalf("loading player movement changed Y to %v", got)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.PlayerLoaded{}); err != nil {
		t.Fatal(err)
	}
	queued[1]()
	if !player.HasClientLoaded() {
		t.Fatal("PlayerLoaded did not release movement gate")
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetPlayerPosition{X: 1, Y: 64, Z: 1}); err != nil {
		t.Fatal(err)
	}
	queued[2]()
	if got := player.Position(); got != (entity.Vec3{X: 1, Y: 64, Z: 1}) {
		t.Fatalf("loaded player movement = %+v", got)
	}
}

// TestMovementRejectsInvalidCoordinatesBeforeQueue limits malformed traffic.
func TestMovementRejectsInvalidCoordinatesBeforeQueue(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	s.BindPlayer(entity.NewPlayer(1, protocol.UUID{1}, "mover", entity.Vec3{}, entity.Vec2{}, s, 2))
	queued := 0
	accessor := &DefaultWorldAccessor{SubmitTask: func(func()) bool { queued++; return true }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, accessor); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetPlayerPosition{X: math.NaN()}); err == nil {
		t.Fatal("NaN position was accepted")
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetPlayerRotation{Yaw: float32(math.Inf(1))}); err == nil {
		t.Fatal("infinite rotation was accepted")
	}
	if queued != 0 {
		t.Fatalf("queued %d malformed movement packets", queued)
	}
}

// TestMovementBackpressureReturnsError prevents unbounded network-side buildup.
func TestMovementBackpressureReturnsError(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	s.BindPlayer(entity.NewPlayer(1, protocol.UUID{1}, "mover", entity.Vec3{}, entity.Vec2{}, s, 2))
	accessor := &DefaultWorldAccessor{SubmitTask: func(func()) bool { return false }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, accessor); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetPlayerOnGround{OnGround: true}); err == nil {
		t.Fatal("full world queue accepted movement")
	}
}

// TestQueuedMovementSurvivesDisconnect preserves packet order before save snapshot.
func TestQueuedMovementSurvivesDisconnect(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{31}
	player := entity.NewPlayer(31, uuid, "mover", entity.Vec3{}, entity.Vec2{}, s, 2)
	player.SetClientLoaded(true)
	s.BindPlayer(player)
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{EntityID: 31, UUID: uuid, Name: "mover", Session: s}); err != nil {
		t.Fatal(err)
	}
	var queued func()
	accessor := &DefaultWorldAccessor{SubmitTask: func(task func()) bool { queued = task; return true }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, accessor); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetPlayerPosition{X: 5, Y: 64, Z: 6}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	<-s.Context().Done()
	queued()
	if got := player.Position(); got != (entity.Vec3{X: 5, Y: 64, Z: 6}) {
		t.Fatalf("queued movement was lost before disconnect snapshot: %+v", got)
	}
}

// TestDisconnectPreservesAcceptedLoadAckAndMovement verifies the snapshot after connection cancellation.
func TestDisconnectPreservesAcceptedLoadAckAndMovement(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	player := entity.NewPlayer(41, protocol.UUID{41}, "mover", entity.Vec3{}, entity.Vec2{}, s, 2)
	player.SetClientLoaded(false)
	s.BindPlayer(player)
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{EntityID: 41, UUID: player.UUID(), Name: "mover", Session: s}); err != nil {
		t.Fatal(err)
	}
	var queued []func()
	accessor := &DefaultWorldAccessor{SubmitTask: func(task func()) bool { queued = append(queued, task); return true }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, accessor); err != nil {
		t.Fatal(err)
	}
	s.setPendingTeleport(42)
	for _, packet := range []protocol.Packet{&playpacket.PlayerLoaded{}, &playpacket.ConfirmTeleportation{TeleportID: 42}, &playpacket.SetPlayerPosition{X: 2, Y: 2}} {
		if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	<-s.Context().Done()
	for _, task := range queued {
		task()
	}
	if !player.HasClientLoaded() || s.hasPendingTeleport() || player.Position() != (entity.Vec3{X: 2, Y: 2}) {
		t.Fatalf("accepted requests lost before snapshot: %+v", player.Position())
	}
	if accessor.Submit(&movementCommand{session: s, players: players, world: accessor, hasPosition: true, x: 3, y: 3}) {
		t.Fatal("closed connection admitted new movement")
	}
}

// TestTeleportAckPreservesMovementOrder rejects pre-ack movement on the tick.
func TestTeleportAckPreservesMovementOrder(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{32}
	player := entity.NewPlayer(32, uuid, "mover", entity.Vec3{}, entity.Vec2{}, s, 2)
	player.SetClientLoaded(true)
	s.BindPlayer(player)
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{EntityID: 32, UUID: uuid, Name: "mover", Session: s}); err != nil {
		t.Fatal(err)
	}
	var queued []func()
	accessor := &DefaultWorldAccessor{SubmitTask: func(task func()) bool { queued = append(queued, task); return true }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, accessor); err != nil {
		t.Fatal(err)
	}
	s.setPendingTeleport(42)
	for _, packet := range []protocol.Packet{
		&playpacket.SetPlayerPosition{X: 1, Y: 1},
		&playpacket.ConfirmTeleportation{TeleportID: 42},
		&playpacket.SetPlayerPosition{X: 2, Y: 2},
	} {
		if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
			t.Fatal(err)
		}
	}
	if len(queued) != 3 {
		t.Fatalf("queued %d tasks, want 3", len(queued))
	}
	queued[0]()
	if player.Position() != (entity.Vec3{}) {
		t.Fatal("pre-ack movement was applied")
	}
	queued[1]()
	if s.hasPendingTeleport() {
		t.Fatal("matching teleport ID remained pending")
	}
	queued[2]()
	if got := player.Position(); got.X != 2 || got.Y != 2 {
		t.Fatalf("post-ack movement = %+v", got)
	}
}
