package session

import (
	"context"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/world/blockentity"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestCraftingTableOpensWithPlayerSlots verifies the world command opens a usable 3x3 menu.
func TestCraftingTableOpensWithPlayerSlots(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	conn := newMockConn()
	s, err := New(Config{ID: 1, Connection: conn, Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	player := entity.NewPlayer(7, protocol.UUID{1}, "craft", entity.Vec3{X: 1, Y: 64, Z: 2}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	windows := inventory.NewWindowManager()
	windows.Inventory().Set(inventory.MainInventoryStart, inventory.NewSlot(1, 2))
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{Session: s, UUID: protocol.UUID{1}, EntityID: 7, Name: "craft", Windows: windows}); err != nil {
		t.Fatal(err)
	}
	owner := chunk.New(0, 0)
	owner.SetBlock(1, 64, 2, block.DefaultStateID("minecraft:crafting_table"))
	lookup := func(x, z int32) (*chunk.Chunk, bool) { return owner, x == 0 && z == 0 }
	storage := chunk.NewStorage(1)
	storage.Set(owner)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	world := &DefaultWorldAccessor{Chunks: manager, BlockEntityStore: blockentity.NewStore(lookup)}
	(&openBlockWindowCommand{session: s, players: players, world: world, x: 1, y: 64, z: 2, sequence: 1}).Execute()
	active := windows.ActiveWindow()
	if active == nil || active.Type != inventory.WindowTypeCrafting || active.Container.Size() != 46 || active.Container.Get(10).ItemCount != 2 {
		t.Fatalf("crafting table window = %+v", active)
	}
}
