package session

import (
	"context"
	"strconv"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// lifecycleFixture binds one player to a loaded, complete bed and absolute clock.
func lifecycleFixture(t *testing.T) (*PlayerManager, *OnlinePlayer, *DefaultWorldAccessor, *int64) {
	t.Helper()
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
	op := &OnlinePlayer{Session: s, UUID: protocol.UUID{1}, EntityID: 7, Name: "sleep", Windows: inventory.NewWindowManager()}
	pm := NewPlayerManager()
	if err := pm.AddPlayer(op); err != nil {
		t.Fatal(err)
	}
	storage := chunk.NewStorage(1)
	owner := chunk.New(0, 0)
	storage.Set(owner)
	for x := int32(-1); x <= 1; x++ {
		for z := int32(-1); z <= 1; z++ {
			if x != 0 || z != 0 {
				storage.Set(chunk.New(x, z))
			}
		}
	}
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	head := block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "head", "occupied": "false"})
	foot := block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "foot", "occupied": "false"})
	owner.SetBlock(1, 64, 1, head)
	owner.SetBlock(1, 64, 2, foot)
	daytime := int64(42000)
	world := &DefaultWorldAccessor{Chunks: manager, ReadTimeOfDay: func() int64 { return daytime }, WriteTimeOfDay: func(n int64) { daytime = n }}
	return pm, op, world, &daytime
}

// TestSleepDelayAndOccupiedPosition checks the 100-tick delay and separate spawn state.
func TestSleepDelayAndOccupiedPosition(t *testing.T) {
	pm, op, world, daytime := lifecycleFixture(t)
	state := world.GetBlock(1, 64, 1)
	if err := handleBedUse(op.Session, pm, world, state, 1, 64, 1, 1); err != nil {
		t.Fatal(err)
	}
	if !op.Sleeping {
		t.Fatal("valid bed refused")
	}
	op.SpawnX = 9
	config := PlayBootstrapConfig{GameRule: func(name string) string {
		if name == "playersSleepingPercentage" {
			return "50"
		}
		return "true"
	}}
	for i := 0; i < 99; i++ {
		TickPlayerLifecycle(pm, world, config)
	}
	if *daytime != 42000 || !op.Sleeping {
		t.Fatal("night skipped before sleep delay")
	}
	TickPlayerLifecycle(pm, world, config)
	if *daytime != 48000 || op.Sleeping || op.SpawnX != 9 {
		t.Fatal("sleep changed absolute day or spawn")
	}
	for _, z := range []int{1, 2} {
		if block.PropertiesFromState(world.GetBlock(1, 64, z))["occupied"] != "false" {
			t.Fatal("sleep bed remained occupied")
		}
	}
}

// TestDestroyedSleepingHeadReleasesFoot verifies the retained bed orientation.
func TestDestroyedSleepingHeadReleasesFoot(t *testing.T) {
	pm, op, world, _ := lifecycleFixture(t)
	if err := handleBedUse(op.Session, pm, world, world.GetBlock(1, 64, 1), 1, 64, 1, 1); err != nil {
		t.Fatal(err)
	}
	world.SetBlock(1, 64, 1, 0)
	TickPlayerLifecycle(pm, world, PlayBootstrapConfig{})
	foot := world.GetBlock(1, 64, 2)
	if op.Sleeping || !behavior.IsBed(foot) || block.PropertiesFromState(foot)["occupied"] != "false" {
		t.Fatal("destroyed head retained occupied foot")
	}
}

// TestSleepWeatherWithFrozenDaylight keeps absolute time while waking the player.
func TestSleepWeatherWithFrozenDaylight(t *testing.T) {
	pm, op, world, daytime := lifecycleFixture(t)
	cleared := false
	world.ClearWeatherAfterSleep = func() { cleared = true }
	if err := handleBedUse(op.Session, pm, world, world.GetBlock(1, 64, 1), 1, 64, 1, 1); err != nil {
		t.Fatal(err)
	}
	op.SleepTicks = 99
	TickPlayerLifecycle(pm, world, PlayBootstrapConfig{GameRule: func(name string) string {
		if name == "doDaylightCycle" {
			return "false"
		}
		if name == "playersSleepingPercentage" {
			return "100"
		}
		return "true"
	}})
	if *daytime != 42000 || op.Sleeping || !cleared {
		t.Fatal("frozen daylight sleep rules not applied")
	}
}

// TestSpawnPreparationKeepsValidSavedPosition verifies collision and cancellation checks.
func TestSpawnPreparationKeepsValidSavedPosition(t *testing.T) {
	_, _, world, _ := lifecycleFixture(t)
	position := entity.Vec3{X: 3.1, Y: 70.4, Z: 3.2}
	result := make(chan spawnPreparation, 1)
	(&spawnPlayerCommand{ctx: context.Background(), world: world, position: position, saved: true, result: result}).Execute()
	if resolved := <-result; resolved.err != nil || resolved.position != position {
		t.Fatalf("saved position changed: %+v", resolved)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	(&spawnPlayerCommand{ctx: ctx, world: world, position: position, result: result}).Execute()
	if resolved := <-result; resolved.err != context.Canceled {
		t.Fatalf("cancelled transition=%v", resolved.err)
	}
}

// TestDeathConservesCompleteStacksAndRunsOnce checks cursor, temporary grid and XP.
func TestDeathConservesCompleteStacksAndRunsOnce(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(strconv.FormatBool(keep), func(t *testing.T) {
			pm, op, world, _ := lifecycleFixture(t)
			world.Items = entity.NewItemEntityManager()
			id := int32(100)
			world.NextEntityID = func() int32 { id++; return id }
			root := nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", 1).PutString("UnknownItemField", "keep").PutCompound("components", nbt.NewCompound().PutString("minecraft:custom_name", `"Named"`).PutList("minecraft:lore", nbt.NewList(nbt.TagString).Add(`"Lore"`)))
			stack, err := inventory.ItemStackFromNBT(root)
			if err != nil {
				t.Fatal(err)
			}
			op.Windows.Inventory().Set(inventory.HotbarStart, stack)
			op.Windows.Inventory().Set(1, stack)
			op.Windows.SetCursorItem(stack)
			player := op.Session.Player().(*entity.Player)
			player.Living().SetXP(7, 70, 0.5)
			player.Living().SetHealth(0)
			xp := ProcessPlayerDeath(player, pm, world, keep)
			if repeated := ProcessPlayerDeath(player, pm, world, keep); repeated != 0 {
				t.Fatal("repeated death issued XP")
			}
			if keep {
				if xp != 0 || world.Items.Count() != 0 || player.Living().XPLevel() != 7 {
					t.Fatal("keepInventory lost items or XP")
				}
				count := int32(0)
				for _, s := range op.Windows.Inventory().Slots() {
					if !s.Empty() {
						if !s.StackableWith(stack) {
							t.Fatal("death changed components")
						}
						count += s.ItemCount
					}
				}
				if count != 3 {
					t.Fatalf("kept count=%d", count)
				}
			} else {
				if xp != 49 || world.Items.Count() < 1 || player.Living().XPLevel() != 0 {
					t.Fatal("death drops/XP incorrect")
				}
				count := int32(0)
				for _, drop := range world.Items.All() {
					count += drop.Stack().ItemCount
					if !drop.Stack().StackableWith(stack) {
						t.Fatal("drop lost components")
					}
				}
				if count != 3 {
					t.Fatalf("dropped count=%d", count)
				}
			}
			if !op.Windows.CursorItem().Empty() || !op.Windows.Inventory().Get(1).Empty() {
				t.Fatal("temporary items remain after death")
			}
		})
	}
}

// TestRespawnRetriesAreBoundedAndIdempotent prevents repeated transition work each tick.
func TestRespawnRetriesAreBoundedAndIdempotent(t *testing.T) {
	pm, op, world, _ := lifecycleFixture(t)
	world.SetBlock(1, 64, 1, 0)
	world.SetBlock(1, 64, 2, 0)
	player := op.Session.Player().(*entity.Player)
	player.Living().SetHealth(0)
	player.Living().MarkDeathHandled()
	config := PlayBootstrapConfig{SpawnX: 1.5, SpawnY: 65, SpawnZ: 1.5}
	command := &respawnPlayerCommand{session: op.Session, players: pm, world: world, config: config}
	command.Execute()
	if !op.RespawnPending || op.RespawnRetryTicks != 20 {
		t.Fatal("obstructed respawn was not deferred")
	}
	command.Execute()
	if op.RespawnRetryTicks != 20 {
		t.Fatal("duplicate packet restarted respawn")
	}
	world.SetBlock(1, 64, 1, block.StateID("minecraft:stone", nil))
	for i := 0; i < 19; i++ {
		TickPlayerLifecycle(pm, world, config)
	}
	if !player.Living().IsDead() {
		t.Fatal("respawn retried before delay")
	}
	TickPlayerLifecycle(pm, world, config)
	if player.Living().IsDead() || op.RespawnPending {
		t.Fatal("prepared respawn was not completed")
	}
	position := player.Position()
	command.Execute()
	if player.Position() != position || op.RespawnPending {
		t.Fatal("duplicate respawn mutated living player")
	}
}
