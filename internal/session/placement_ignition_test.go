package session

import (
	"context"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/world/chunk"
)

func placementFixture(t *testing.T) (Session, *PlayerManager, *DefaultWorldAccessor, *chunk.Chunk) {
	t.Helper()
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{ID: 1, Connection: newMockConn(), Registry: registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	player := entity.NewPlayer(7, protocol.UUID{1}, "builder", entity.Vec3{X: 0.5, Y: 64, Z: 0.5}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	pm := NewPlayerManager()
	op := &OnlinePlayer{Session: s, UUID: protocol.UUID{1}, EntityID: 7, Name: "builder", X: 0.5, Y: 64, Z: 0.5, Windows: inventory.NewWindowManager()}
	if err := pm.AddPlayer(op); err != nil {
		t.Fatal(err)
	}
	storage := chunk.NewStorage(1)
	section := chunk.New(0, 0)
	storage.Set(section)
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	return s, pm, &DefaultWorldAccessor{Chunks: manager}, section
}

func TestBedPlacementWritesBothHalvesAndConsumesOneItem(t *testing.T) {
	s, pm, world, section := placementFixture(t)
	stone := block.DefaultStateID("minecraft:stone")
	section.SetBlock(2, 63, 2, stone)
	section.SetBlock(2, 63, 3, stone)
	op := pm.GetBySession(s)
	op.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:red_bed"), 1))
	state := resolveDirectionalBlockState(item.IDByName("minecraft:red_bed"), 1, 0)
	(&placeBlockCommand{session: s, players: pm, world: world, player: op, x: 2, y: 64, z: 2, stateID: state, sequence: 1}).Execute()
	foot := section.GetBlock(2, 64, 2)
	head := section.GetBlock(2, 64, 3)
	if !behavior.IsBed(foot) || !behavior.IsBed(head) || block.PropertiesFromState(foot)["part"] != "foot" || block.PropertiesFromState(head)["part"] != "head" {
		t.Fatalf("bed halves: foot=%d head=%d", foot, head)
	}
	if !op.Windows.HeldItem().Empty() {
		t.Fatal("bed placement did not consume exactly one item")
	}
	for _, z := range []int32{2, 3} {
		if be := section.Data().GetBlockEntity(2, 64, z); be == nil || be.TypeName() != "minecraft:bed" {
			t.Fatal("placed bed half lacks its renderer entity")
		}
	}
	breakBlock(s, pm, world, playpacket.BlockPos{X: 2, Y: 64, Z: 2}, 1, true)
	if !block.IsAir(section.GetBlock(2, 64, 2)) || !block.IsAir(section.GetBlock(2, 64, 3)) {
		t.Fatal("breaking bed left an orphan half")
	}
	if len(section.Data().BlockEntities) != 0 {
		t.Fatal("breaking bed left orphan renderer entities")
	}
}

func TestBedPlacementRejectsObstructedHeadWithoutConsumption(t *testing.T) {
	s, pm, world, section := placementFixture(t)
	stone := block.DefaultStateID("minecraft:stone")
	section.SetBlock(2, 63, 2, stone)
	section.SetBlock(2, 63, 3, stone)
	section.SetBlock(2, 64, 3, stone)
	op := pm.GetBySession(s)
	op.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:red_bed"), 1))
	state := resolveDirectionalBlockState(item.IDByName("minecraft:red_bed"), 1, 0)
	(&placeBlockCommand{session: s, players: pm, world: world, player: op, x: 2, y: 64, z: 2, stateID: state, sequence: 1}).Execute()
	if !block.IsAir(section.GetBlock(2, 64, 2)) || op.Windows.HeldItem().ItemCount != 1 {
		t.Fatal("invalid bed placement changed world or inventory")
	}
}

func TestFlintAndSteelPlacesFireAndDamagesTool(t *testing.T) {
	s, pm, world, section := placementFixture(t)
	section.SetBlock(2, 64, 2, block.DefaultStateID("minecraft:stone"))
	op := pm.GetBySession(s)
	op.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:flint_and_steel"), 1))
	(&ignitionCommand{session: s, players: pm, world: world, packet: playpacket.UseItemOn{Position: playpacket.BlockPos{X: 2, Y: 64, Z: 2}, Face: 1, Sequence: 1}}).Execute()
	if got := block.NameFromState(section.GetBlock(2, 65, 2)); got != "minecraft:fire" {
		t.Fatalf("ignition placed %q, want fire", got)
	}
	if damage := op.Windows.HeldItem().Damage; damage != 1 {
		t.Fatalf("flint and steel damage = %d, want 1", damage)
	}
}

func TestFlintAndSteelPrimesTNT(t *testing.T) {
	s, pm, world, section := placementFixture(t)
	section.SetBlock(2, 64, 2, block.DefaultStateID("minecraft:tnt"))
	op := pm.GetBySession(s)
	op.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:flint_and_steel"), 1))
	primed := 0
	world.PrimeTNT = func(x, y, z int, ownerID int32) bool {
		primed++
		if x != 2 || y != 64 || z != 2 || ownerID != 7 {
			t.Errorf("prime TNT args = %d,%d,%d owner=%d", x, y, z, ownerID)
		}
		section.SetBlock(x, y, z, 0)
		return true
	}
	(&ignitionCommand{session: s, players: pm, world: world, packet: playpacket.UseItemOn{Position: playpacket.BlockPos{X: 2, Y: 64, Z: 2}, Face: 1, Sequence: 1}}).Execute()
	if primed != 1 || !block.IsAir(section.GetBlock(2, 64, 2)) || op.Windows.HeldItem().Damage != 1 {
		t.Fatal("TNT was not primed or flint and steel was not damaged")
	}
}

// TestFlintAndSteelActivatesNetherPortal verifies complete frame activation.
func TestFlintAndSteelActivatesNetherPortal(t *testing.T) {
	s, pm, world, section := placementFixture(t)
	obsidian := block.DefaultStateID("minecraft:obsidian")
	for x := 1; x <= 4; x++ {
		section.SetBlock(x, 63, 2, obsidian)
		section.SetBlock(x, 67, 2, obsidian)
	}
	for y := 63; y <= 67; y++ {
		section.SetBlock(1, y, 2, obsidian)
		section.SetBlock(4, y, 2, obsidian)
	}
	op := pm.GetBySession(s)
	op.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:flint_and_steel"), 1))
	(&ignitionCommand{session: s, players: pm, world: world, packet: playpacket.UseItemOn{Position: playpacket.BlockPos{X: 2, Y: 63, Z: 2}, Face: 1, Sequence: 1}}).Execute()
	for x := 2; x <= 3; x++ {
		for y := 64; y <= 66; y++ {
			if name := block.NameFromState(section.GetBlock(x, y, 2)); name != "minecraft:nether_portal" {
				t.Fatalf("portal block %d,%d = %q", x, y, name)
			}
		}
	}
}

func TestUseItemOnIgnitionRunsOnlyOnWorldTick(t *testing.T) {
	s, pm, world, section := placementFixture(t)
	section.SetBlock(2, 64, 2, block.DefaultStateID("minecraft:stone"))
	pm.GetBySession(s).Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:flint_and_steel"), 1))
	var queued func()
	world.SubmitTask = func(task func()) bool { queued = task; return true }
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, pm, world); err != nil {
		t.Fatal(err)
	}
	pkt := &playpacket.UseItemOn{Position: playpacket.BlockPos{X: 2, Y: 64, Z: 2}, Face: 1, Sequence: 1}
	if err := router.Handle(s, protocol.StatePlay, pkt); err != nil {
		t.Fatal(err)
	}
	if queued == nil || !block.IsAir(section.GetBlock(2, 65, 2)) {
		t.Fatal("ignition mutated world before world tick")
	}
	queued()
	if block.NameFromState(section.GetBlock(2, 65, 2)) != "minecraft:fire" {
		t.Fatal("queued ignition did not create fire")
	}
}
