package main

import (
	"context"
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/world/level"
)

// TestSnapshotPlayerData verifies that disconnect persistence captures current state.
func TestSnapshotPlayerData(t *testing.T) {
	uuid := protocol.UUID{1}
	p := entity.NewPlayer(1, uuid, "tester", entity.Vec3{}, entity.Vec2{}, nil, 2)
	p.SetPositionXYZ(12.5, 70, -4)
	p.SetRotationYP(90, 15)
	p.Living().SetHealth(13)
	p.Living().SetFoodLevel(17)
	p.Living().SetXP(3, 25, 0)
	op := &session.OnlinePlayer{
		UUID: uuid, X: 1, Y: 2, Z: 3, Yaw: 0, Pitch: 0,
		GameMode: 1, Windows: inventory.NewWindowManager(),
	}
	stoneID := item.IDByName("minecraft:stone")
	if stoneID < 0 {
		t.Fatal("stone is not registered")
	}
	op.Windows.Inventory().Set(36, inventory.NewSlot(stoneID, 32))
	op.Windows.SetHeldSlot(7)
	loaded := level.DefaultPlayerData("uuid", 0, 0, 0, 0)

	got := snapshotPlayerData(loaded, "uuid", op, p)
	if got == loaded {
		t.Fatal("snapshot aliases mutable loaded playerdata")
	}
	if got.PosX != 12.5 || got.PosY != 70 || got.PosZ != -4 || got.Yaw != 90 || got.Pitch != 15 {
		t.Fatalf("position/rotation was not captured: %+v", got)
	}
	if got.Health != 13 || got.FoodLevel != 17 || got.XPLevel != 3 || got.XPTotal != 25 {
		t.Fatalf("living state was not captured: %+v", got)
	}
	if len(got.Inventory) != 1 || got.Inventory[0].Slot != 0 || got.Inventory[0].Count != 32 {
		t.Fatalf("hotbar was not mapped to playerdata: %+v", got.Inventory)
	}
	if got.SelectedItemSlot != 7 {
		t.Fatal("selected hotbar slot was not captured")
	}
}

// TestSnapshotPlayerDataRejectsMissingPlayer ensures incomplete snapshots are not saved.
func TestSnapshotPlayerDataRejectsMissingPlayer(t *testing.T) {
	if got := snapshotPlayerData(nil, "uuid", &session.OnlinePlayer{}, nil); got != nil {
		t.Fatal("snapshot without entity must be rejected")
	}
}

// TestReconnectLoadsUnwrittenFinalSnapshot avoids inventory rollback before disk flush.
func TestReconnectLoadsUnwrittenFinalSnapshot(t *testing.T) {
	dir := t.TempDir()
	uuid := "00000000-0000-0000-0000-000000000001"
	old := level.DefaultPlayerData(uuid, 1, 64, 1, 0)
	if err := old.Save(dir); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	writer := level.NewPlayerWriter(1, func(data *level.PlayerData) error {
		close(started)
		<-release
		return data.Save(dir)
	})
	current := old.Clone()
	current.PosX = 9
	if !writer.Offer(level.PlayerWrite{Data: current, Sequence: 1}) {
		t.Fatal("final snapshot rejected")
	}
	<-started
	loaded, err := loadPlayerSnapshot(writer, dir, uuid)
	if err != nil || loaded.PosX != 9 {
		t.Fatalf("reconnect loaded old position: %+v error=%v", loaded, err)
	}
	loaded.PosX = 100
	close(release)
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadPlayerSnapshot(writer, dir, uuid)
	if err != nil || loaded.PosX != 9 {
		t.Fatalf("flushed snapshot changed: %+v error=%v", loaded, err)
	}
}

// snapshotDimensionSession supplies the immutable world binding used by player snapshots.
type snapshotDimensionSession struct {
	session.Session
	dimension string
}

// WorldDimension returns the prepared runtime dimension for snapshot tests.
func (s *snapshotDimensionSession) WorldDimension() string { return s.dimension }

// TestPlayerSnapshotPersistsCurrentDimensionAndPortalCooldown avoids restoring a previous world's binding.
func TestPlayerSnapshotPersistsCurrentDimensionAndPortalCooldown(t *testing.T) {
	uuid := "00000000-0000-0000-0000-000000000001"
	loaded := level.DefaultPlayerData(uuid, 8, 64, 8, 0)
	online := &session.OnlinePlayer{Session: &snapshotDimensionSession{dimension: "minecraft:the_nether"}, PortalCooldown: 271}
	player := entity.NewPlayer(1, protocol.UUID{1}, "test", entity.Vec3{X: 9, Y: 64, Z: 9}, entity.Vec2{}, nil, 2)
	snapshot := snapshotPlayerData(loaded, uuid, online, player)
	dir := t.TempDir()
	if err := snapshot.Save(dir); err != nil {
		t.Fatal(err)
	}
	restored, err := level.LoadPlayerData(dir, uuid)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Dimension != "minecraft:the_nether" || restored.PortalCooldown != 271 || restored.PosX != 9 || loaded.Dimension != "minecraft:overworld" {
		t.Fatal("snapshot lost the current dimension or cooldown")
	}
	online.Session = &snapshotDimensionSession{dimension: "minecraft:overworld"}
	online.PortalCooldown = 0
	returned := snapshotPlayerData(restored, uuid, online, player)
	if returned.Dimension != "minecraft:overworld" || returned.PortalCooldown != 0 {
		t.Fatal("return reused the previous saved dimension")
	}
}
