package session

import (
	"context"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestSafeBedRespawnRequiresCompleteBed checks the bed and open spawn space.
func TestSafeBedRespawnRequiresCompleteBed(t *testing.T) {
	storage := chunk.NewStorage(1)
	owner := chunk.New(0, 0)
	storage.Set(owner)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	head := block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "head", "occupied": "false"})
	foot := block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "foot", "occupied": "false"})
	if head < 0 || foot < 0 {
		t.Fatal("bed block states are unavailable")
	}
	owner.SetBlock(1, 64, 1, head)
	owner.SetBlock(1, 64, 2, foot)
	world := &DefaultWorldAccessor{Chunks: manager}
	if x, y, z, ok := safeBedRespawn(world, 1, 64, 1); !ok || x != 1.5 || y != 64.5625 || z != 1.5 {
		t.Fatalf("bed respawn = %v,%v,%v (%v)", x, y, z, ok)
	}
	owner.SetBlock(1, 64, 2, 0)
	if _, _, _, ok := safeBedRespawn(world, 1, 64, 1); ok {
		t.Fatal("incomplete bed remained a respawn point")
	}
}

// TestBedSetsSpawnByDay verifies that daytime use saves a spawn without sleeping.
func TestBedSetsSpawnByDay(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{ID: 1, Connection: newMockConn(), Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	player := entity.NewPlayer(7, protocol.UUID{1}, "sleep", entity.Vec3{X: 1, Y: 64, Z: 2}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	pm := NewPlayerManager()
	op := &OnlinePlayer{Session: s, UUID: protocol.UUID{1}, EntityID: 7, Name: "sleep"}
	if err := pm.AddPlayer(op); err != nil {
		t.Fatal(err)
	}
	storage := chunk.NewStorage(1)
	owner := chunk.New(0, 0)
	storage.Set(owner)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	head := block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "head", "occupied": "false"})
	foot := block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "foot", "occupied": "false"})
	owner.SetBlock(1, 64, 1, head)
	owner.SetBlock(1, 64, 2, foot)
	world := &DefaultWorldAccessor{Chunks: manager, ReadTimeOfDay: func() int64 { return 6000 }}
	if err := handleBedUse(s, pm, world, foot, 1, 64, 2, 1); err != nil {
		t.Fatal(err)
	}
	if !op.HasSpawnPoint || op.SpawnX != 1 || op.SpawnY != 64 || op.SpawnZ != 1 || op.Sleeping {
		t.Fatalf("daytime bed state = %+v", op)
	}
}

// TestSkipToDawnAdvancesWorldClock verifies that sleeping changes daytime.
func TestSkipToDawnAdvancesWorldClock(t *testing.T) {
	daytime := int64(18000)
	world := &DefaultWorldAccessor{ReadTimeOfDay: func() int64 { return daytime }, WriteTimeOfDay: func(value int64) { daytime = value }}
	skipToDawn(NewPlayerManager(), world)
	if daytime != 24000 {
		t.Fatalf("daytime = %d, want dawn", daytime)
	}
}

// TestAllSleepingIgnoresSpectators prevents spectators from blocking dawn.
func TestAllSleepingIgnoresSpectators(t *testing.T) {
	pm := NewPlayerManager()
	pm.players[protocol.UUID{1}] = &OnlinePlayer{Sleeping: true, GameMode: 0}
	pm.players[protocol.UUID{2}] = &OnlinePlayer{Sleeping: false, GameMode: 3}
	if !pm.AllSleeping() {
		t.Fatal("spectator blocked sleep")
	}
	pm.players[protocol.UUID{1}].Sleeping = false
	if pm.AllSleeping() {
		t.Fatal("awake survival player did not block sleep")
	}
}

func TestRespawnResendsChunksAtSpawn(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	conn := newMockConn()
	s, err := New(Config{ID: 1, Connection: conn, Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	player := entity.NewPlayer(7, protocol.UUID{1}, "respawn", entity.Vec3{}, entity.Vec2{}, s, 10)
	player.Living().SetHealth(0)
	s.BindPlayer(player)
	pm := NewPlayerManager()
	if err := pm.AddPlayer(&OnlinePlayer{Session: s, UUID: protocol.UUID{1}, EntityID: 7, Name: "respawn", Windows: inventory.NewWindowManager()}); err != nil {
		t.Fatal(err)
	}
	called := false
	wa := &DefaultWorldAccessor{ResetPlayerChunks: func(playerID, x, z int32) {
		called = true
		if playerID != 7 || x != 2 || z != -1 {
			t.Errorf("chunk reset = %d,%d,%d", playerID, x, z)
		}
	}}
	if err := handleRespawn(s, PlayBootstrapConfig{SpawnX: 32.5, SpawnY: 64, SpawnZ: -0.5}, pm, wa); err != nil {
		t.Fatal(err)
	}
	if !called || player.Living().IsDead() {
		t.Fatal("respawn did not restore player and restart chunk streaming")
	}
	ids := []int32{(&playpacket.Respawn{}).ID(), (&playpacket.SetCenterChunk{}).ID(), (&playpacket.SyncPlayerPosition{}).ID()}
	for _, want := range ids {
		packet := <-conn.sent
		if packet.ID != want {
			t.Fatalf("respawn packet ID = %d, want %d", packet.ID, want)
		}
	}
}
