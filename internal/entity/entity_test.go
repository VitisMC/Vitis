package entity

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

func TestNewEntityIDAndUUID(t *testing.T) {
	uuid := protocol.UUID{0x1234, 0x5678}
	e := NewEntity(1, uuid, EntityTypePlayer, Vec3{10, 64, 20}, Vec2{90, 0})

	if e.ID() != 1 {
		t.Fatalf("expected id 1, got %d", e.ID())
	}
	if e.UUID() != uuid {
		t.Fatalf("expected uuid %v, got %v", uuid, e.UUID())
	}
	if e.Type() != EntityTypePlayer {
		t.Fatalf("expected type player, got %d", e.Type())
	}
}

func TestEntityDirtyFlags(t *testing.T) {
	e := NewEntity(1, protocol.UUID{}, EntityTypeMob, Vec3{}, Vec2{})

	if e.Dirty() != 0 {
		t.Fatalf("expected clean, got %d", e.Dirty())
	}

	e.SetPosition(Vec3{1, 2, 3})
	if e.Dirty()&DirtyPosition == 0 {
		t.Fatal("expected DirtyPosition flag set")
	}

	e.SetRotation(Vec2{45, 30})
	if e.Dirty()&DirtyRotation == 0 {
		t.Fatal("expected DirtyRotation flag set")
	}

	e.SetVelocity(Vec3{0.1, 0, 0})
	if e.Dirty()&DirtyVelocity == 0 {
		t.Fatal("expected DirtyVelocity flag set")
	}

	e.ClearDirty()
	if e.Dirty() != 0 {
		t.Fatalf("expected clean after ClearDirty, got %d", e.Dirty())
	}
}

func TestEntitySnapshotPrev(t *testing.T) {
	e := NewEntity(1, protocol.UUID{}, EntityTypeMob, Vec3{10, 20, 30}, Vec2{1, 2})
	e.SetPosition(Vec3{11, 21, 31})
	e.SetRotation(Vec2{3, 4})

	e.SnapshotPrev()

	prev := e.PrevPosition()
	if prev.X != 11 || prev.Y != 21 || prev.Z != 31 {
		t.Fatalf("snapshot prev position mismatch: %+v", prev)
	}
	prevRot := e.PrevRotation()
	if prevRot.X != 3 || prevRot.Y != 4 {
		t.Fatalf("snapshot prev rotation mismatch: %+v", prevRot)
	}
}

func TestEntityRemove(t *testing.T) {
	e := NewEntity(1, protocol.UUID{}, EntityTypeMob, Vec3{}, Vec2{})
	if e.Removed() {
		t.Fatal("new entity should not be removed")
	}
	e.Remove()
	if !e.Removed() {
		t.Fatal("entity should be removed after Remove()")
	}
}

func TestBlockToChunk(t *testing.T) {
	tests := []struct {
		v    float64
		want int32
	}{
		{0, 0},
		{15, 0},
		{16, 1},
		{31, 1},
		{-1, -1},
		{-16, -1},
		{-17, -2},
		{32, 2},
	}
	for _, tt := range tests {
		got := BlockToChunk(tt.v)
		if got != tt.want {
			t.Errorf("BlockToChunk(%v) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

func TestChunkKey(t *testing.T) {
	k1 := ChunkKey(0, 0)
	k2 := ChunkKey(1, 0)
	k3 := ChunkKey(0, 1)
	if k1 == k2 || k1 == k3 || k2 == k3 {
		t.Fatal("chunk keys should be unique for different coords")
	}
	if ChunkKey(5, 10) != ChunkKey(5, 10) {
		t.Fatal("same coords should produce same key")
	}
}

func TestEntityUpdateChunkCoords(t *testing.T) {
	e := NewEntity(1, protocol.UUID{}, EntityTypeMob, Vec3{8, 64, 8}, Vec2{})
	if e.ChunkX() != 0 || e.ChunkZ() != 0 {
		t.Fatalf("initial chunk should be 0,0, got %d,%d", e.ChunkX(), e.ChunkZ())
	}

	e.pos = Vec3{24, 64, 8}
	changed := e.UpdateChunkCoords()
	if !changed {
		t.Fatal("expected chunk change")
	}
	if e.ChunkX() != 1 {
		t.Fatalf("expected chunkX=1, got %d", e.ChunkX())
	}

	changed = e.UpdateChunkCoords()
	if changed {
		t.Fatal("expected no chunk change on same position")
	}
}

func TestManagerAllocateID(t *testing.T) {
	mgr := NewManager()
	id1 := mgr.AllocateID()
	id2 := mgr.AllocateID()
	if id1 == id2 {
		t.Fatal("allocated IDs should be unique")
	}
	if id2 != id1+1 {
		t.Fatalf("expected sequential IDs: %d, %d", id1, id2)
	}
}

func TestManagerAddGetRemove(t *testing.T) {
	mgr := NewManager()
	e := NewEntity(mgr.AllocateID(), protocol.UUID{}, EntityTypeMob, Vec3{8, 64, 8}, Vec2{})
	mgr.Add(e)

	if mgr.Count() != 1 {
		t.Fatalf("expected count 1, got %d", mgr.Count())
	}
	if mgr.Get(e.ID()) != e {
		t.Fatal("Get should return the added entity")
	}

	mgr.Remove(e.ID())
	if !e.Removed() {
		t.Fatal("entity should be marked removed")
	}

	mgr.Tick(1)
	if mgr.Count() != 0 {
		t.Fatalf("expected count 0 after tick, got %d", mgr.Count())
	}
	if mgr.Get(e.ID()) != nil {
		t.Fatal("entity should be gone after tick processes removal")
	}
}

func TestManagerSpatialIndex(t *testing.T) {
	mgr := NewManager()
	e1 := NewEntity(mgr.AllocateID(), protocol.UUID{}, EntityTypeMob, Vec3{8, 64, 8}, Vec2{})
	e2 := NewEntity(mgr.AllocateID(), protocol.UUID{}, EntityTypeMob, Vec3{24, 64, 8}, Vec2{})
	mgr.Add(e1)
	mgr.Add(e2)

	key0 := ChunkKey(0, 0)
	key1 := ChunkKey(1, 0)

	ids0 := mgr.EntitiesInChunk(key0)
	if len(ids0) != 1 || ids0[0] != e1.ID() {
		t.Fatalf("chunk(0,0) should contain e1, got %v", ids0)
	}

	ids1 := mgr.EntitiesInChunk(key1)
	if len(ids1) != 1 || ids1[0] != e2.ID() {
		t.Fatalf("chunk(1,0) should contain e2, got %v", ids1)
	}
}

func TestManagerChunkMembershipUpdate(t *testing.T) {
	mgr := NewManager()
	e := NewEntity(mgr.AllocateID(), protocol.UUID{}, EntityTypeMob, Vec3{8, 64, 8}, Vec2{})
	mgr.Add(e)

	oldKey := ChunkKey(0, 0)
	if len(mgr.EntitiesInChunk(oldKey)) != 1 {
		t.Fatal("entity should be in chunk(0,0)")
	}

	e.SetPosition(Vec3{24, 64, 8})
	mgr.Tick(1)

	newKey := ChunkKey(1, 0)
	if len(mgr.EntitiesInChunk(oldKey)) != 0 {
		t.Fatal("entity should no longer be in chunk(0,0)")
	}
	if len(mgr.EntitiesInChunk(newKey)) != 1 {
		t.Fatal("entity should be in chunk(1,0)")
	}
}

func TestPositionDeltaSmallMove(t *testing.T) {
	prev := Vec3{100, 64, 200}
	cur := Vec3{100.5, 64.1, 200.2}
	dx, dy, dz, fits := PositionDelta(prev, cur)
	if !fits {
		t.Fatal("small move should fit in relative delta")
	}
	if dx == 0 && dy == 0 && dz == 0 {
		t.Fatal("deltas should be non-zero for a move")
	}
}

func TestPositionDeltaLargeMove(t *testing.T) {
	prev := Vec3{0, 64, 0}
	cur := Vec3{100, 64, 0}
	_, _, _, fits := PositionDelta(prev, cur)
	if fits {
		t.Fatal("large move (100 blocks) should not fit in relative delta")
	}
}

func TestAngleToByte(t *testing.T) {
	b0 := AngleToByte(0)
	if b0 != 0 {
		t.Fatalf("expected 0, got %d", b0)
	}

	b90 := AngleToByte(90)
	if b90 != 64 {
		t.Fatalf("expected 64 for 90 degrees, got %d", b90)
	}

	b180 := AngleToByte(180)
	if b180 != 128 {
		t.Fatalf("expected 128 for 180 degrees, got %d", b180)
	}
}

func TestPositionChanged(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{1, 2, 3}
	if PositionChanged(a, b) {
		t.Fatal("same positions should not be changed")
	}
	c := Vec3{1.1, 2, 3}
	if !PositionChanged(a, c) {
		t.Fatal("different positions should be changed")
	}
}

func TestRotationChanged(t *testing.T) {
	a := Vec2{90, 0}
	b := Vec2{90, 0}
	if RotationChanged(a, b) {
		t.Fatal("same rotations should not be changed")
	}
	c := Vec2{91, 0}
	if !RotationChanged(a, c) {
		t.Fatal("different rotations should be changed")
	}
}

type mockSender struct {
	packets []protocol.Packet
}

func (m *mockSender) Send(pkt protocol.Packet) error {
	m.packets = append(m.packets, pkt)
	return nil
}

func TestPlayerTrackingEnterLeave(t *testing.T) {
	mgr := NewManager()

	sender := &mockSender{}
	player := NewPlayer(mgr.AllocateID(), protocol.UUID{1, 1}, "TestPlayer", Vec3{8, 64, 8}, Vec2{}, sender, 2)
	mgr.Add(player.Entity)

	mob := NewMob(mgr.AllocateID(), protocol.UUID{2, 2}, 10, Vec3{10, 64, 10}, Vec2{})
	mgr.Add(mob.Entity)

	spawns, despawns := player.UpdateTracking(mgr)
	if len(spawns) != 1 || spawns[0] != mob.ID() {
		t.Fatalf("expected mob to spawn, got spawns=%v", spawns)
	}
	if len(despawns) != 0 {
		t.Fatalf("expected no despawns, got %v", despawns)
	}

	spawns2, despawns2 := player.UpdateTracking(mgr)
	if len(spawns2) != 0 {
		t.Fatalf("expected no new spawns on second tick, got %v", spawns2)
	}
	if len(despawns2) != 0 {
		t.Fatalf("expected no despawns on second tick, got %v", despawns2)
	}

	mob.SetPosition(Vec3{10000, 64, 10000})
	mgr.Tick(1)

	spawns3, despawns3 := player.UpdateTracking(mgr)
	if len(spawns3) != 0 {
		t.Fatalf("expected no new spawns after mob moves far away, got %v", spawns3)
	}
	if len(despawns3) != 1 || despawns3[0] != mob.ID() {
		t.Fatalf("expected mob to despawn, got despawns=%v", despawns3)
	}
}

func TestPlayerSendSpawnEntity(t *testing.T) {
	sender := &mockSender{}
	player := NewPlayer(1, protocol.UUID{}, "Test", Vec3{}, Vec2{}, sender, 2)

	mob := NewEntity(2, protocol.UUID{3, 4}, EntityTypeMob, Vec3{10, 64, 20}, Vec2{45, 30})
	player.SendSpawnEntity(mob)

	if len(sender.packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(sender.packets))
	}
	if sender.packets[0].ID() != 0x01 {
		t.Fatalf("expected SpawnEntity packet id 0x01, got 0x%02x", sender.packets[0].ID())
	}
}

func TestPlayerSendDespawnEntities(t *testing.T) {
	sender := &mockSender{}
	player := NewPlayer(1, protocol.UUID{}, "Test", Vec3{}, Vec2{}, sender, 2)

	player.SendDespawnEntities([]int32{5, 6, 7})

	if len(sender.packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(sender.packets))
	}
	expectedRemoveID := playpacket.NewRemoveEntities().ID()
	if sender.packets[0].ID() != expectedRemoveID {
		t.Fatalf("expected RemoveEntities packet id 0x%02x, got 0x%02x", expectedRemoveID, sender.packets[0].ID())
	}
}

func TestPlayerSendMovementUpdatePosition(t *testing.T) {
	sender := &mockSender{}
	player := NewPlayer(1, protocol.UUID{}, "Test", Vec3{}, Vec2{}, sender, 2)

	mob := NewEntity(2, protocol.UUID{}, EntityTypeMob, Vec3{10, 64, 20}, Vec2{})
	mob.SnapshotPrev()
	mob.SetPosition(Vec3{10.5, 64, 20})

	player.SendMovementUpdate(mob)

	if len(sender.packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(sender.packets))
	}
	expectedPosID := playpacket.NewUpdateEntityPosition().ID()
	if sender.packets[0].ID() != expectedPosID {
		t.Fatalf("expected UpdateEntityPosition packet id 0x%02x, got 0x%02x", expectedPosID, sender.packets[0].ID())
	}
}

func TestPlayerSendMovementUpdateRotation(t *testing.T) {
	sender := &mockSender{}
	player := NewPlayer(1, protocol.UUID{}, "Test", Vec3{}, Vec2{}, sender, 2)

	mob := NewEntity(2, protocol.UUID{}, EntityTypeMob, Vec3{10, 64, 20}, Vec2{0, 0})
	mob.SnapshotPrev()
	mob.SetRotation(Vec2{45, 30})

	player.SendMovementUpdate(mob)

	if len(sender.packets) != 2 {
		t.Fatalf("expected 2 packets (rotation + head), got %d", len(sender.packets))
	}
	expectedRotID := playpacket.NewUpdateEntityRotation().ID()
	if sender.packets[0].ID() != expectedRotID {
		t.Fatalf("expected UpdateEntityRotation packet id 0x%02x, got 0x%02x", expectedRotID, sender.packets[0].ID())
	}
	expectedHeadID := playpacket.NewSetHeadRotation().ID()
	if sender.packets[1].ID() != expectedHeadID {
		t.Fatalf("expected SetHeadRotation packet id 0x%02x, got 0x%02x", expectedHeadID, sender.packets[1].ID())
	}
}

func TestPlayerSendMovementUpdateTeleport(t *testing.T) {
	sender := &mockSender{}
	player := NewPlayer(1, protocol.UUID{}, "Test", Vec3{}, Vec2{}, sender, 2)

	mob := NewEntity(2, protocol.UUID{}, EntityTypeMob, Vec3{0, 64, 0}, Vec2{})
	mob.SnapshotPrev()
	mob.SetPosition(Vec3{100, 64, 0})

	player.SendMovementUpdate(mob)

	if len(sender.packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(sender.packets))
	}
	expectedTeleportID := playpacket.NewTeleportEntity().ID()
	if sender.packets[0].ID() != expectedTeleportID {
		t.Fatalf("expected TeleportEntity packet id 0x%02x, got 0x%02x", expectedTeleportID, sender.packets[0].ID())
	}
}

func TestPlayerSendMovementUpdatePositionAndRotation(t *testing.T) {
	sender := &mockSender{}
	player := NewPlayer(1, protocol.UUID{}, "Test", Vec3{}, Vec2{}, sender, 2)

	mob := NewEntity(2, protocol.UUID{}, EntityTypeMob, Vec3{10, 64, 20}, Vec2{0, 0})
	mob.SnapshotPrev()
	mob.SetPosition(Vec3{10.5, 64, 20})
	mob.SetRotation(Vec2{45, 30})

	player.SendMovementUpdate(mob)

	if len(sender.packets) != 2 {
		t.Fatalf("expected 2 packets (pos+rot + head), got %d", len(sender.packets))
	}
	expectedPosRotID := playpacket.NewUpdateEntityPositionAndRotation().ID()
	if sender.packets[0].ID() != expectedPosRotID {
		t.Fatalf("expected UpdateEntityPositionAndRotation packet id 0x%02x, got 0x%02x", expectedPosRotID, sender.packets[0].ID())
	}
	expectedHeadID2 := playpacket.NewSetHeadRotation().ID()
	if sender.packets[1].ID() != expectedHeadID2 {
		t.Fatalf("expected SetHeadRotation packet id 0x%02x, got 0x%02x", expectedHeadID2, sender.packets[1].ID())
	}
}

func TestTrackerFullCycle(t *testing.T) {
	mgr := NewManager()
	tracker := NewTracker()

	sender := &mockSender{}
	player := NewPlayer(mgr.AllocateID(), protocol.UUID{1, 1}, "TestPlayer", Vec3{8, 64, 8}, Vec2{}, sender, 2)
	mgr.Add(player.Entity)
	tracker.AddPlayer(player)

	mob := NewMob(mgr.AllocateID(), protocol.UUID{2, 2}, 10, Vec3{10, 64, 10}, Vec2{})
	mgr.Add(mob.Entity)

	tracker.Tick(mgr)

	foundSpawn := false
	for _, pkt := range sender.packets {
		if pkt.ID() == playpacket.NewSpawnEntity().ID() {
			foundSpawn = true
			break
		}
	}
	if !foundSpawn {
		t.Fatal("expected SpawnEntity packet after first tracker tick")
	}

	sender.packets = sender.packets[:0]
	mob.SetPosition(Vec3{10.5, 64, 10})
	tracker.Tick(mgr)

	foundMove := false
	for _, pkt := range sender.packets {
		if pkt.ID() == playpacket.NewUpdateEntityPosition().ID() || pkt.ID() == playpacket.NewUpdateEntityPositionAndRotation().ID() || pkt.ID() == playpacket.NewTeleportEntity().ID() {
			foundMove = true
			break
		}
	}
	if !foundMove {
		t.Fatal("expected movement packet after mob moved")
	}

	sender.packets = sender.packets[:0]
	mob.SetPosition(Vec3{10000, 64, 10000})
	mgr.Tick(1)
	tracker.Tick(mgr)

	foundDespawn := false
	for _, pkt := range sender.packets {
		if pkt.ID() == playpacket.NewRemoveEntities().ID() {
			foundDespawn = true
			break
		}
	}
	if !foundDespawn {
		t.Fatal("expected RemoveEntities packet after mob moved far away")
	}
}

func TestTrackerAddRemovePlayer(t *testing.T) {
	tracker := NewTracker()

	p1 := NewPlayer(1, protocol.UUID{}, "p1", Vec3{}, Vec2{}, nil, 2)
	p2 := NewPlayer(2, protocol.UUID{}, "p2", Vec3{}, Vec2{}, nil, 2)
	tracker.AddPlayer(p1)
	tracker.AddPlayer(p2)

	if len(tracker.Players()) != 2 {
		t.Fatalf("expected 2 players, got %d", len(tracker.Players()))
	}

	tracker.RemovePlayer(1)
	if len(tracker.Players()) != 1 {
		t.Fatalf("expected 1 player after removal, got %d", len(tracker.Players()))
	}
	if tracker.Players()[0].ID() != 2 {
		t.Fatalf("expected remaining player id=2, got %d", tracker.Players()[0].ID())
	}
}

func TestMobStub(t *testing.T) {
	mob := NewMob(1, protocol.UUID{}, 42, Vec3{10, 64, 20}, Vec2{90, 0})
	if mob.MobType() != 42 {
		t.Fatalf("expected mob type 42, got %d", mob.MobType())
	}
	if mob.Type() != EntityTypeMob {
		t.Fatalf("expected EntityTypeMob, got %d", mob.Type())
	}
	mob.Tick()
}

func TestVelocityToProtocol(t *testing.T) {
	if velocityToProtocol(0) != 0 {
		t.Fatal("zero velocity should be 0")
	}
	if velocityToProtocol(1.0) != 8000 {
		t.Fatalf("expected 8000, got %d", velocityToProtocol(1.0))
	}
	if velocityToProtocol(100) != 32767 {
		t.Fatal("large velocity should be clamped to 32767")
	}
	if velocityToProtocol(-100) != -32768 {
		t.Fatal("large negative velocity should be clamped to -32768")
	}
}

func TestManagerSnapshotAll(t *testing.T) {
	mgr := NewManager()
	e := NewEntity(mgr.AllocateID(), protocol.UUID{}, EntityTypeMob, Vec3{10, 64, 20}, Vec2{0, 0})
	mgr.Add(e)

	e.SetPosition(Vec3{11, 65, 21})
	e.SetRotation(Vec2{45, 30})

	if e.Dirty() == 0 {
		t.Fatal("entity should be dirty")
	}

	mgr.SnapshotAll()

	if e.Dirty() != 0 {
		t.Fatal("entity should be clean after SnapshotAll")
	}
	prev := e.PrevPosition()
	if prev.X != 11 || prev.Y != 65 || prev.Z != 21 {
		t.Fatalf("prev position should be updated: %+v", prev)
	}
}
