package session

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	worldruntime "github.com/vitismc/vitis/internal/world"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
)

// TestDropPacketRegistersPersistentRuntimeItem verifies packet-to-storage ownership and repeated requests.
func TestDropPacketRegistersPersistentRuntimeItem(t *testing.T) {
	store := persistence.NewChunkStore(t.TempDir())
	storage := chunk.NewStorage(1)
	storage.Set(chunk.New(0, 0))
	w, err := worldruntime.New(worldruntime.Config{Name: "drop-test", Dimension: "minecraft:overworld", ChunkStore: store, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage}), SimulationDistance: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	if err := w.SetGameRule("spawnChunkRadius", "0"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetGameRule("doMobSpawning", "false"); err != nil {
		t.Fatal(err)
	}
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{ID: 1, Connection: newMockConn(), Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := entity.NewPlayer(1, protocol.UUID{1, 2}, "player", entity.Vec3{X: 8, Y: 64, Z: 8}, entity.Vec2{}, s, 2)
	s.BindPlayer(p)
	w.AddPlayer(p)
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_pickaxe"), 1)
	stack.SetDamage(7)
	windows := inventory.NewWindowManager()
	windows.Inventory().Set(inventory.HotbarStart, stack)
	players := NewPlayerManager()
	if err := players.AddPlayer(&OnlinePlayer{Session: s, EntityID: p.ID(), UUID: p.UUID(), Name: "player", Windows: windows}); err != nil {
		t.Fatal(err)
	}
	accessor := &DefaultWorldAccessor{Chunks: w.ChunkManager(), SubmitTask: w.Submit, NextEntityID: w.EntityManager().AllocateID, Items: w.ItemEntities(), RegisterItem: w.SpawnItemEntity}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, players, accessor); err != nil {
		t.Fatal(err)
	}
	packet := &playpacket.BlockDig{Status: playpacket.DigDropStack, Sequence: 7}
	if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
		t.Fatal(err)
	}
	if w.ItemEntities().Count() != 0 || windows.HeldItem().Empty() {
		t.Fatal("drop executed outside world tick")
	}
	w.Tick()
	if w.ItemEntities().Count() != 1 || w.EntityManager().Count() != 2 || !windows.HeldItem().Empty() {
		t.Fatal("drop was not registered in both runtime managers")
	}
	if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
		t.Fatal(err)
	}
	w.Tick()
	if w.ItemEntities().Count() != 1 {
		t.Fatal("repeated empty-hand drop duplicated item")
	}
	if _, err := w.SaveAllDirty(); err != nil {
		t.Fatal(err)
	}
	data, err := store.ReadEntityNBT(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(data).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := root.GetList("Entities")
	if list.Len() != 1 {
		t.Fatal("packet drop was omitted from entity region")
	}
	decoded, err := entity.DecodePersistentEntity(0, list.Elements()[0].(*nbt.Compound))
	if err != nil {
		t.Fatal(err)
	}
	saved := decoded.(*entity.ItemEntity).Stack()
	if saved.ItemID != stack.ItemID || saved.ItemCount != 1 || saved.Damage != 7 {
		t.Fatal("packet drop lost item components")
	}
}
