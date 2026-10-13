package session

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
)

// TestInventoryActionsKeepSessionOrderOnWorldTick verifies queued hotbar and creative changes.
func TestInventoryActionsKeepSessionOrderOnWorldTick(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID: 1, Connection: conn, Registry: registry,
		InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	player := entity.NewPlayer(7, protocol.UUID{1}, "player", entity.Vec3{}, entity.Vec2{}, s, 10)
	player.Living().SetGameMode(1)
	s.BindPlayer(player)
	windows := inventory.NewWindowManager()
	windows.Inventory().Set(37, inventory.NewSlot(1, 2))
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, UUID: protocol.UUID{1}, Name: "player", EntityID: 7, Windows: windows,
	}); err != nil {
		t.Fatalf("add player: %v", err)
	}
	var queued []func()
	world := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = append(queued, task)
		return true
	}}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, world); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.ServerboundHeldItemSlot{Slot: 1}); err != nil {
		t.Fatalf("held slot: %v", err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetCreativeSlot{
		SlotIndex: 37, Item: inventory.NewSlot(2, 3),
	}); err != nil {
		t.Fatalf("creative slot: %v", err)
	}
	if len(queued) != 2 || windows.HeldSlot() != 0 || windows.GetSlot(37).ItemID != 1 {
		t.Fatal("network handlers mutated inventory or did not preserve queue order")
	}
	for _, task := range queued {
		task()
	}
	if windows.HeldSlot() != 1 || windows.GetSlot(37).ItemID != 2 || windows.GetSlot(37).ItemCount != 3 {
		t.Fatalf("queued actions not applied: held=%d slot=%+v", windows.HeldSlot(), windows.GetSlot(37))
	}
	select {
	case packet := <-conn.sent:
		t.Fatalf("accepted creative edit unexpectedly reset client menu with packet ID %d", packet.ID)
	default:
	}
}

// TestQueuedCreativeActionRechecksCapability prevents a stale creative packet from granting items.
func TestQueuedCreativeActionRechecksCapability(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID: 1, Connection: conn, Registry: registry,
		InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	player := entity.NewPlayer(7, protocol.UUID{1}, "player", entity.Vec3{}, entity.Vec2{}, s, 10)
	player.Living().SetGameMode(1)
	s.BindPlayer(player)
	windows := inventory.NewWindowManager()
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, UUID: protocol.UUID{1}, Name: "player", EntityID: 7, Windows: windows,
	}); err != nil {
		t.Fatalf("add player: %v", err)
	}
	var queued func()
	world := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = task
		return true
	}}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, world); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.SetCreativeSlot{
		SlotIndex: 9, Item: inventory.NewSlot(1, 1),
	}); err != nil {
		t.Fatalf("creative slot: %v", err)
	}
	player.Living().SetGameMode(0)
	queued()
	if !windows.GetSlot(9).Empty() {
		t.Fatal("creative action granted an item after capability was revoked")
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.SetContainerContent{}).ID() {
		t.Fatalf("rejected action response ID = %d, want container resync", packet.ID)
	}
}

// TestCloseWindowIgnoresStaleIDOnWorldTick prevents delayed close packets from closing a newer window.
func TestCloseWindowIgnoresStaleIDOnWorldTick(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID: 1, Connection: conn, Registry: registry,
		InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	windows := inventory.NewWindowManager()
	oldWindow := windows.OpenWindow(inventory.WindowTypeGeneric9x1, "old", inventory.NewContainer(9))
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, UUID: protocol.UUID{1}, Name: "player", Windows: windows,
	}); err != nil {
		t.Fatalf("add player: %v", err)
	}
	var queued []func()
	world := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = append(queued, task)
		return true
	}}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, world); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.CloseWindow{WindowID: oldWindow.ID}); err != nil {
		t.Fatalf("close old window: %v", err)
	}
	newWindow := windows.OpenWindow(inventory.WindowTypeGeneric9x1, "new", inventory.NewContainer(9))
	queued[0]()
	if active := windows.ActiveWindow(); active == nil || active.ID != newWindow.ID {
		t.Fatal("stale close packet closed a newer window")
	}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.CloseWindow{WindowID: newWindow.ID}); err != nil {
		t.Fatalf("close new window: %v", err)
	}
	if windows.ActiveWindow() == nil {
		t.Fatal("window closed before the world tick")
	}
	queued[1]()
	if windows.ActiveWindow() != nil {
		t.Fatal("matching window remained open after world tick")
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.SetContainerContent{}).ID() {
		t.Fatalf("close resync packet ID = %d, want player inventory", packet.ID)
	}
}
