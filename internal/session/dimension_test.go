package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	worldruntime "github.com/vitismc/vitis/internal/world"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// dimensionFixture supplies isolated connected runtimes without starting a second tick loop.
type dimensionFixture struct {
	manager     *worldruntime.Manager
	players     *PlayerManager
	worlds      []PlayWorld
	sessions    []*DefaultSession
	conns       []*mockConn
	connections []*dimensionConn
	runtime     []*worldruntime.World
	storage     []*chunk.Storage
}

// newDimensionFixture builds two independent block stores and a shared ID allocator.
func newDimensionFixture(t *testing.T) *dimensionFixture {
	return newDimensionFixtureWith(t, []string{"minecraft:overworld", "minecraft:the_nether"})
}

// newDimensionFixtureWith creates the requested isolated runtime dimensions.
func newDimensionFixtureWith(t *testing.T, dimensions []string) *dimensionFixture {
	t.Helper()
	f := &dimensionFixture{manager: worldruntime.NewManager(), players: NewPlayerManager()}
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	for index, dimension := range dimensions {
		storage := chunk.NewStorage(25)
		for x := int32(-2); x <= 2; x++ {
			for z := int32(-2); z <= 2; z++ {
				c := chunk.New(x, z)
				if dimension != "minecraft:overworld" {
					c.SetData(section.NewChunkWithBounds(x, z, 0, 256, 0))
				}
				for bx := 0; bx < 16; bx++ {
					for bz := 0; bz < 16; bz++ {
						c.SetBlock(bx, 63, bz, 1)
					}
				}
				storage.Set(c)
			}
		}
		w, err := f.manager.CreateWithConfig(worldruntime.Config{Name: dimension, Dimension: dimension, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage}), SimulationDistance: 1})
		if err != nil {
			t.Fatal(err)
		}
		accessor := &DefaultWorldAccessor{DimensionName: dimension, Chunks: w.ChunkManager(), SubmitTask: w.Submit, NextEntityID: w.EntityManager().AllocateID, Items: w.ItemEntities(), RegisterItem: w.SpawnItemEntity, BlockEntityStore: w.BlockEntities(), ResetPlayerChunks: w.ResendPlayerChunks, UpdateNeighbors: w.NotifyNeighbors, TickScheduler: w.TickScheduler(), CurrentTick: w.CurrentTick}
		config := PlayBootstrapConfig{DimensionName: dimension, SpawnX: 8.5, SpawnY: 64, SpawnZ: 8.5, ReadWorldSpawn: func() (int32, int32, int32, float32) { return 8, 64, 8, 0 }}
		f.worlds = append(f.worlds, PlayWorld{Dimension: dimension, Accessor: accessor, Config: config})
		conn := newMockConn()
		conn.sent = make(chan network.Packet, 1024)
		connection := &dimensionConn{mockConn: conn}
		s, err := New(Config{ID: uint64(index + 1), Connection: connection, Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
		if err != nil {
			t.Fatal(err)
		}
		s.BindWorldDimension(dimension)
		p := entity.NewPlayer(w.EntityManager().AllocateID(), protocol.UUID{1, uint64(index + 1)}, dimension, entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}, s, 2)
		p.SetClientLoaded(true)
		s.BindPlayer(p)
		op := &OnlinePlayer{Session: s, EntityID: p.ID(), UUID: p.UUID(), Name: dimension, X: 8.5, Y: 64, Z: 8.5, Windows: inventory.NewWindowManager(), ManagedTracking: true}
		if err := f.players.AddPlayer(op); err != nil {
			t.Fatal(err)
		}
		w.AddPlayer(p)
		f.sessions = append(f.sessions, s)
		f.conns = append(f.conns, conn)
		f.connections = append(f.connections, connection)
		f.runtime = append(f.runtime, w)
		f.storage = append(f.storage, storage)
	}
	t.Cleanup(func() {
		for _, s := range f.sessions {
			_ = s.Close()
		}
		if err := f.manager.Close(nil); err != nil {
			t.Error(err)
		}
	})
	return f
}

// packetCount drains an observer's packets and counts a specific packet type.
func packetCount(conn *mockConn, id int32) int {
	count := 0
	for {
		select {
		case p := <-conn.sent:
			if p.ID == id {
				count++
			}
		default:
			return count
		}
	}
}

// TestDimensionPacketsPreserveOrderAndIsolation covers real registered drops, held slots, and movement.
func TestDimensionPacketsPreserveOrderAndIsolation(t *testing.T) {
	f := newDimensionFixture(t)
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.conns {
		packetCount(c, -1)
	}
	op := f.players.GetBySession(f.sessions[1])
	windows := op.Windows
	windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:stone"), 2))
	windows.Inventory().Set(inventory.HotbarStart+1, inventory.NewSlot(item.IDByName("minecraft:dirt"), 3))
	for _, pkt := range []protocol.Packet{&play.BlockDig{Status: play.DigDropStack, Sequence: 1}, &play.ServerboundHeldItemSlot{Slot: 1}, &play.BlockDig{Status: play.DigDropStack, Sequence: 2}, &play.SetPlayerPosition{X: 9.5, Y: 64, Z: 8.5, OnGround: true}} {
		if err := router.Handle(f.sessions[1], protocol.StatePlay, pkt); err != nil {
			t.Fatal(err)
		}
	}
	if f.runtime[1].ItemEntities().Count() != 0 || windows.HeldItem().ItemCount != 2 {
		t.Fatal("network handler mutated runtime")
	}
	f.runtime[0].Scheduler().Drain(128)
	if f.runtime[0].ItemEntities().Count() != 0 || f.runtime[1].ItemEntities().Count() != 0 {
		t.Fatal("packet entered the wrong dimension queue")
	}
	f.runtime[1].Scheduler().Drain(128)
	if f.runtime[1].ItemEntities().Count() != 2 || !windows.Inventory().Get(inventory.HotbarStart).Empty() || !windows.Inventory().Get(inventory.HotbarStart+1).Empty() || op.X != 9.5 {
		t.Fatal("nested actions lost TCP order")
	}
	if packetCount(f.conns[0], (&play.SyncEntityPosition{}).ID()) != 0 {
		t.Fatal("movement leaked into another dimension")
	}
}

// TestDimensionStalePacketsAndBackpressure rejects old memberships and full ingress queues.
func TestDimensionStalePacketsAndBackpressure(t *testing.T) {
	f := newDimensionFixture(t)
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	p := f.sessions[0].Player().(*entity.Player)
	if err := router.Handle(f.sessions[0], protocol.StatePlay, &play.SetPlayerPosition{X: 9.5, Y: 64, Z: 8.5}); err != nil {
		t.Fatal(err)
	}
	f.sessions[0].BindWorldDimension("minecraft:overworld")
	f.runtime[0].Scheduler().Drain(128)
	if p.Position().X != 8.5 {
		t.Fatal("pre-respawn packet mutated a new membership")
	}
	full := *f.worlds[0].Accessor.(*DefaultWorldAccessor)
	full.SubmitTask = func(func()) bool { return false }
	blocked := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(blocked, f.players, []PlayWorld{{Dimension: "minecraft:overworld", Accessor: &full}}); err != nil {
		t.Fatal(err)
	}
	if err := blocked.Handle(f.sessions[0], protocol.StatePlay, &play.SetPlayerPosition{X: 10.5, Y: 64, Z: 8.5}); err == nil {
		t.Fatal("full queue accepted mutation")
	}
	f.sessions[0].BindWorldDimension("minecraft:missing")
	if err := router.Handle(f.sessions[0], protocol.StatePlay, &play.TickEnd{}); err == nil {
		t.Fatal("unknown dimension used a fallback world")
	}
}

// TestDimensionViewsKeepGlobalAdmissionAndLocalGameplay verifies two-player visibility and sleeping.
func TestDimensionViewsKeepGlobalAdmissionAndLocalGameplay(t *testing.T) {
	f := newDimensionFixture(t)
	ow := f.players.ForDimension("minecraft:overworld")
	nether := f.players.ForDimension("minecraft:the_nether")
	if ow.Count() != 1 || nether.Count() != 1 || f.players.Count() != 2 || ow.GetBySession(f.sessions[1]) != nil || nether.GetByEntityID(f.sessions[0].Player().ID()) != nil {
		t.Fatal("dimension view leaked players")
	}
	for _, c := range f.conns {
		packetCount(c, -1)
	}
	ow.Broadcast(&play.BlockUpdate{})
	if packetCount(f.conns[0], (&play.BlockUpdate{}).ID()) != 1 || packetCount(f.conns[1], (&play.BlockUpdate{}).ID()) != 0 {
		t.Fatal("block broadcast leaked")
	}
	f.players.GetBySession(f.sessions[0]).Sleeping = true
	if !ow.AllSleeping() || nether.AllSleeping() || f.players.AllSleeping() {
		t.Fatal("sleeping depended on another dimension")
	}
	ow.Global().Broadcast(play.NewSystemChatText("global"))
	for _, c := range f.conns {
		if packetCount(c, (&play.SystemChatMessage{}).ID()) != 1 {
			t.Fatal("global chat was scoped")
		}
	}
}

// TestSessionDimensionBindingConcurrent reads immutable memberships across network and tick callers.
func TestSessionDimensionBindingConcurrent(t *testing.T) {
	f := newDimensionFixture(t)
	s := f.sessions[0]
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for i := 0; i < 1000; i++ {
			_ = SessionDimension(s)
			_ = f.players.ForDimension("minecraft:overworld").Count()
		}
	}()
	for i := 0; i < 1000; i++ {
		s.BindWorldDimension("minecraft:overworld")
		s.BindWorldDimension("minecraft:the_nether")
	}
	readers.Wait()
}

// dimensionConn provides race-safe fault injection at the protocol boundary.
type dimensionConn struct {
	*mockConn
	failPacket atomic.Int32
}

// Send rejects only the configured packet while preserving normal traffic.
func (c *dimensionConn) Send(packet *network.Packet) error {
	if id := c.failPacket.Load(); id != 0 && packet.ID == id {
		return errors.New("injected transfer write failure")
	}
	return c.mockConn.Send(packet)
}

// TestPlayBootstrapUsesPreparedDimension tests the actual LoginPlay payload for a restored Nether login.
func TestPlayBootstrapUsesPreparedDimension(t *testing.T) {
	f := newDimensionFixture(t)
	s := f.sessions[1]
	if err := SendPlayBootstrap(s, s.Player().ID(), PlayBootstrapConfig{DimensionName: "minecraft:the_nether", PortalCooldown: 271, InitialSelectedSlot: 7, ViewDistance: 2, SimulationDistance: 1, SpawnY: 64}); err != nil {
		t.Fatal(err)
	}
	var login *play.LoginPlay
	heldSlot := int32(-1)
	for len(f.conns[1].sent) > 0 {
		packet := <-f.conns[1].sent
		if packet.ID == (&play.LoginPlay{}).ID() {
			login = &play.LoginPlay{}
			if err := login.Decode(protocol.WrapBuffer(packet.Payload)); err != nil {
				t.Fatal(err)
			}
		}
		if packet.ID == (&play.ClientboundHeldItemSlot{}).ID() {
			slot, err := protocol.WrapBuffer(packet.Payload).ReadVarInt()
			if err != nil {
				t.Fatal(err)
			}
			heldSlot = slot
		}
	}
	if login == nil || login.DimensionName != "minecraft:the_nether" || login.PortalCooldown != 271 || heldSlot != 7 {
		t.Fatal("login packet ignored the restored dimension or cooldown")
	}
	if err := SendPlayBootstrap(s, s.Player().ID(), PlayBootstrapConfig{InitialSelectedSlot: 9}); err == nil || len(f.conns[1].sent) != 0 {
		t.Fatal("malformed selected slot sent partial bootstrap packets")
	}
}
