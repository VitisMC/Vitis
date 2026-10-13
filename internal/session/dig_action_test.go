package session

import (
	"context"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestBlockDigExecutesOnWorldTick verifies ordered digging does not mutate chunks in the network handler.
func TestBlockDigExecutesOnWorldTick(t *testing.T) {
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
	player := entity.NewPlayer(7, protocol.UUID{1}, "player", entity.Vec3{X: 1.5, Y: 65, Z: 1.5}, entity.Vec2{}, s, 10)
	player.Living().SetGameMode(1)
	s.BindPlayer(player)
	storage := chunk.NewStorage(1)
	loaded := chunk.New(0, 0)
	loaded.SetBlock(1, 65, 1, 1)
	storage.Set(loaded)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Close(ctx)
	}()
	var queued func()
	world := &DefaultWorldAccessor{Chunks: manager, SubmitTask: func(task func()) bool {
		queued = task
		return true
	}}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, world); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	position := playpacket.BlockPos{X: 1, Y: 65, Z: 1}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.BlockDig{
		Status: playpacket.DigStarted, Position: position, Sequence: 12,
	}); err != nil {
		t.Fatalf("handle dig: %v", err)
	}
	if queued == nil || world.GetBlock(1, 65, 1) != 1 || len(conn.sent) != 0 {
		t.Fatal("digging executed before the world tick")
	}
	queued()
	if got := world.GetBlock(1, 65, 1); got != 0 {
		t.Fatalf("block state after world tick = %d, want air", got)
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.BlockUpdate{}).ID() {
		t.Fatalf("first packet ID = %d, want block update", packet.ID)
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.AcknowledgeBlockChange{}).ID() {
		t.Fatalf("second packet ID = %d, want dig acknowledgment", packet.ID)
	}
}

// TestFinalToolUsePreservesSilkTouchDrop evaluates loot before removing a broken tool.
func TestFinalToolUsePreservesSilkTouchDrop(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{
		Connection: newMockConn(), Registry: registry,
		InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay,
	})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{44}
	player := entity.NewPlayer(44, uuid, "miner", entity.Vec3{X: 1.5, Y: 65, Z: 1.5}, entity.Vec2{}, s, 2)
	s.BindPlayer(player)
	players := NewPlayerManager()
	windows := inventory.NewWindowManager()
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, EntityID: 44, UUID: uuid, Name: "miner", Windows: windows,
	}); err != nil {
		t.Fatal(err)
	}
	pickaxeID := item.IDByName("minecraft:diamond_pickaxe")
	tool := inventory.NewSlot(pickaxeID, 1)
	tool.SetDamage(item.MaxDurability(pickaxeID) - 1)
	tool.SetEnchant(33, 1)
	windows.Inventory().Set(inventory.HotbarStart, tool)

	storage := chunk.NewStorage(1)
	loaded := chunk.New(0, 0)
	oreState := block.DefaultStateID("minecraft:diamond_ore")
	loaded.SetBlock(1, 65, 1, oreState)
	storage.Set(loaded)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	defer manager.Close(context.Background())
	items := entity.NewItemEntityManager()
	world := &DefaultWorldAccessor{
		Chunks: manager, Items: items,
		NextEntityID: func() int32 { return 100 },
	}
	breakBlock(s, players, world, playpacket.BlockPos{X: 1, Y: 65, Z: 1}, 0, true)
	if !windows.HeldItem().Empty() {
		t.Fatal("tool survived its final durability point")
	}
	if got := world.GetBlock(1, 65, 1); got != 0 {
		t.Fatalf("ore state = %d, want air", got)
	}
	if items.Count() != 1 {
		t.Fatalf("spawned %d drops, want 1", items.Count())
	}
	if got := items.Get(100).Stack().ItemID; got != item.IDByName("minecraft:diamond_ore") {
		t.Fatalf("drop item = %d, want diamond ore", got)
	}
}

// TestBlockDropSurvivesSendFailure keeps world effects after a disconnect.
func TestBlockDropSurvivesSendFailure(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{45}
	player := entity.NewPlayer(45, uuid, "miner", entity.Vec3{}, entity.Vec2{}, s, 2)
	s.BindPlayer(player)
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{
		Session: s, EntityID: 45, UUID: uuid, Name: "miner", Windows: inventory.NewWindowManager(),
	}); err != nil {
		t.Fatal(err)
	}
	storage := chunk.NewStorage(1)
	loaded := chunk.New(0, 0)
	loaded.SetBlock(1, 65, 1, block.DefaultStateID("minecraft:diamond_ore"))
	storage.Set(loaded)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	defer manager.Close(context.Background())
	items := entity.NewItemEntityManager()
	world := &DefaultWorldAccessor{Chunks: manager, Items: items, NextEntityID: func() int32 { return 101 }}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	breakBlock(s, players, world, playpacket.BlockPos{X: 1, Y: 65, Z: 1}, 0, true)
	if items.Count() != 1 || world.GetBlock(1, 65, 1) != 0 {
		t.Fatal("failed network send discarded block drop or world mutation")
	}
}

// TestBlockDigRejectsInvalidActionBeforeQueue verifies malformed actions cannot enter the world tick.
func TestBlockDigRejectsInvalidActionBeforeQueue(t *testing.T) {
	router := NewPacketRouter()
	world := &DefaultWorldAccessor{SubmitTask: func(func()) bool {
		t.Fatal("invalid digging action was queued")
		return true
	}}
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, world); err != nil {
		t.Fatalf("register handlers: %v", err)
	}
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, packet := range []*playpacket.BlockDig{
		{Status: -1}, {Status: 7}, {Face: -1}, {Face: 6}, {Sequence: -1},
	} {
		if err := router.Handle(s, protocol.StatePlay, packet); err == nil {
			t.Fatalf("accepted malformed action: %+v", packet)
		}
	}
}
