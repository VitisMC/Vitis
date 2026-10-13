package session

import (
	"testing"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
)

// TestWindowClickExecutesOnWorldTick verifies inventory mutations do not run in the network handler.
func TestWindowClickExecutesOnWorldTick(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID:             1,
		Connection:     conn,
		Registry:       registry,
		InitialVersion: protocol.CurrentVersion,
		InitialState:   protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	players := NewPlayerManager()
	windows := inventory.NewWindowManager()
	windows.Inventory().Set(9, inventory.NewSlot(1, 2))
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, UUID: protocol.UUID{1}, Name: "player", Windows: windows,
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
	click := &playpacket.WindowClick{
		WindowID: 0, StateID: windows.CurrentStateID(), Slot: 9,
		Button: inventory.ButtonLeftClick, Mode: inventory.ClickTypeNormalClick,
	}
	if err := router.Handle(s, protocol.StatePlay, click); err != nil {
		t.Fatalf("handle click: %v", err)
	}
	if queued == nil {
		t.Fatal("click was not queued")
	}
	if got := windows.Inventory().Get(9); got.ItemCount != 2 || !windows.CursorItem().Empty() {
		t.Fatalf("inventory changed before world tick: slot=%+v", got)
	}
	queued()
	if got := windows.Inventory().Get(9); !got.Empty() || windows.CursorItem().ItemCount != 2 {
		t.Fatalf("inventory transaction was not applied: slot=%+v cursor=%+v", got, windows.CursorItem())
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.SetContainerContent{}).ID() {
		t.Fatalf("resync packet ID = %d, want container content", packet.ID)
	}
}

// TestWindowClickQueueBackpressureResyncsWithoutMutation verifies safe rejection of overloaded clicks.
func TestWindowClickQueueBackpressureResyncsWithoutMutation(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID:             1,
		Connection:     conn,
		Registry:       registry,
		InitialVersion: protocol.CurrentVersion,
		InitialState:   protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	players := NewPlayerManager()
	windows := inventory.NewWindowManager()
	windows.Inventory().Set(9, inventory.NewSlot(1, 2))
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, UUID: protocol.UUID{1}, Name: "player", Windows: windows,
	}); err != nil {
		t.Fatalf("add player: %v", err)
	}
	world := &DefaultWorldAccessor{SubmitTask: func(func()) bool { return false }}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, world); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	click := &playpacket.WindowClick{
		WindowID: 0, StateID: windows.CurrentStateID(), Slot: 9,
		Button: inventory.ButtonLeftClick, Mode: inventory.ClickTypeNormalClick,
	}
	if err := router.Handle(s, protocol.StatePlay, click); err != nil {
		t.Fatalf("handle click: %v", err)
	}
	if got := windows.Inventory().Get(9); got.ItemCount != 2 || !windows.CursorItem().Empty() {
		t.Fatalf("overloaded click mutated inventory: slot=%+v", got)
	}
	if got := windows.CurrentStateID(); got != click.StateID {
		t.Fatalf("overloaded click advanced state ID from %d to %d", click.StateID, got)
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.SetContainerContent{}).ID() {
		t.Fatalf("resync packet ID = %d, want container content", packet.ID)
	}
}
