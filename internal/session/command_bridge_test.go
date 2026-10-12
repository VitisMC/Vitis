package session

import (
	"math"
	"testing"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/states"
)

type commandTestWorld struct {
	queued   []func()
	reject   bool
	daylight bool
	weather  bool
	fire     bool
}

func (w *commandTestWorld) SetDaylightCycle(enabled bool) { w.daylight = enabled }
func (w *commandTestWorld) SetWeatherCycle(enabled bool)  { w.weather = enabled }
func (w *commandTestWorld) SetFireTick(enabled bool)      { w.fire = enabled }

// TestKeepInventoryDefaultsOff verifies ordinary deaths drop inventory.
func TestKeepInventoryDefaultsOff(t *testing.T) {
	control := &ServerControlAdapter{}
	value, err := control.GetGameRule("keepInventory")
	if err != nil || value != "false" {
		t.Fatalf("default keepInventory = %q, err=%v", value, err)
	}
	if err := control.SetGameRule("keepInventory", "true"); err != nil {
		t.Fatal(err)
	}
	value, err = control.GetGameRule("keepInventory")
	if err != nil || value != "true" {
		t.Fatalf("changed keepInventory = %q, err=%v", value, err)
	}
}

func TestGameRuleValidationAndWorldTickApply(t *testing.T) {
	world := &commandTestWorld{daylight: true}
	control := &ServerControlAdapter{World: world}
	if err := control.SetGameRule("doDaylightCycle", "invalid"); err == nil {
		t.Fatal("invalid boolean accepted")
	}
	if err := control.SetGameRule("randomTickSpeed", "not-a-number"); err == nil {
		t.Fatal("invalid integer accepted")
	}
	if err := control.SetGameRule("doDaylightCycle", "false"); err != nil {
		t.Fatal(err)
	}
	if !world.daylight || len(world.queued) != 1 {
		t.Fatal("game rule bypassed world queue")
	}
	world.queued[0]()
	if world.daylight {
		t.Fatal("queued rule did not freeze daylight")
	}
}

// TimeOfDay satisfies the command world's read-only time interface.
func (*commandTestWorld) TimeOfDay() int64 { return 0 }

// SetTimeOfDay satisfies the command world's time mutation interface.
func (*commandTestWorld) SetTimeOfDay(int64) {}

// WorldAge satisfies the command world's read-only age interface.
func (*commandTestWorld) WorldAge() int64 { return 0 }

// Submit records world commands until the test advances the tick.
func (w *commandTestWorld) Submit(task func()) bool {
	if w.reject {
		return false
	}
	w.queued = append(w.queued, task)
	return true
}

func TestTeleportPlayerUpdatesOnlineAndEntityState(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterPlay(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register play packets: %v", err)
	}
	s, err := New(Config{
		Connection:     newMockConn(),
		Registry:       registry,
		InitialVersion: protocol.CurrentVersion,
		InitialState:   protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	uuid := protocol.UUID{1, 2}
	player := entity.NewPlayer(42, uuid, "player", entity.Vec3{}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	manager := NewPlayerManager()
	if err := manager.AddPlayer(&OnlinePlayer{EntityID: 42, UUID: uuid, Name: "player", Session: s}); err != nil {
		t.Fatalf("add player: %v", err)
	}

	world := &commandTestWorld{}
	control := &ServerControlAdapter{PM: manager, World: world}
	if err := control.TeleportPlayer(42, 10.5, 64, -3.25); err != nil {
		t.Fatalf("teleport: %v", err)
	}
	if pos := player.Position(); pos != (entity.Vec3{}) {
		t.Fatalf("position changed before world tick: %+v", pos)
	}
	if len(world.queued) != 1 {
		t.Fatalf("queued commands = %d, want 1", len(world.queued))
	}
	world.queued[0]()

	pos := player.Position()
	if pos.X != 10.5 || pos.Y != 64 || pos.Z != -3.25 {
		t.Fatalf("entity position = %+v", pos)
	}
	online := manager.GetByUUID(uuid)
	if online == nil || online.X != 10.5 || online.Y != 64 || online.Z != -3.25 {
		t.Fatalf("online position = %#v", online)
	}
	if !s.hasPendingTeleport() {
		t.Fatal("teleport acknowledgement was not registered")
	}
	if err := s.acknowledgeTeleport(s.pendingTeleport.Load()); err != nil {
		t.Fatalf("confirm teleport: %v", err)
	}
}

// TestSetGameModeRunsOnWorldTick verifies ordered command-side player mutation.
func TestSetGameModeRunsOnWorldTick(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{4, 5}
	player := entity.NewPlayer(7, uuid, "player", entity.Vec3{}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	manager := NewPlayerManager()
	if err := manager.AddPlayer(&OnlinePlayer{EntityID: 7, UUID: uuid, Name: "player", Session: s}); err != nil {
		t.Fatal(err)
	}
	world := &commandTestWorld{}
	control := &ServerControlAdapter{PM: manager, World: world}
	if err := control.SetGameMode(7, 1); err != nil {
		t.Fatal(err)
	}
	if player.Living().GameMode() != 0 || manager.GetByUUID(uuid).GameMode != 0 {
		t.Fatal("game mode changed before world tick")
	}
	world.queued[0]()
	if player.Living().GameMode() != 1 || manager.GetByUUID(uuid).GameMode != 1 {
		t.Fatal("game mode was not applied on world tick")
	}
}

// TestSetPlayerSpeedRunsOnWorldTickAndResets verifies ordered speed changes.
func TestSetPlayerSpeedRunsOnWorldTickAndResets(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{4, 6}
	player := entity.NewPlayer(7, uuid, "player", entity.Vec3{}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	manager := NewPlayerManager()
	if err := manager.AddPlayer(&OnlinePlayer{EntityID: 7, UUID: uuid, Name: "player", Session: s, GameMode: 1}); err != nil {
		t.Fatal(err)
	}
	world := &commandTestWorld{}
	control := &ServerControlAdapter{PM: manager, World: world}
	if err := control.SetPlayerSpeed(7, 2.1124); err != nil {
		t.Fatal(err)
	}
	if got := player.Attributes().GetBase(genattr.AttrMovementSpeed); got != 0.1 {
		t.Fatalf("speed changed before world tick: %g", got)
	}
	world.queued[0]()
	want := 0.1 * 2.1124 / defaultWalkingBlocksPerSecond
	if got := player.Attributes().GetBase(genattr.AttrMovementSpeed); math.Abs(got-want) > 1e-10 {
		t.Fatalf("walking attribute = %g, want %g", got, want)
	}
	if got := manager.GetByUUID(uuid).SpeedBlocksPerSecond; got != 2.1124 {
		t.Fatalf("stored speed = %g", got)
	}
	ab := playerAbilitiesWithSpeed(1, 2.1124, true)
	if ab.Flags&2 == 0 || math.Abs(float64(ab.FlySpeed)-0.05*2.1124/defaultFlyingBlocksPerSecond) > 1e-7 {
		t.Fatalf("creative flying abilities = %+v", ab)
	}
	if err := control.SetPlayerSpeed(7, 0); err != nil {
		t.Fatal(err)
	}
	world.queued[1]()
	if got := player.Attributes().GetBase(genattr.AttrMovementSpeed); got != 0.1 {
		t.Fatalf("reset walking attribute = %g", got)
	}
	if err := control.SetPlayerSpeed(7, math.NaN()); err == nil {
		t.Fatal("non-finite speed accepted")
	}
}

// TestPlayerCommandsRejectFullWorldQueue prevents silent loss of mutations.
func TestPlayerCommandsRejectFullWorldQueue(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{8, 9}
	manager := NewPlayerManager()
	if err := manager.AddPlayer(&OnlinePlayer{EntityID: 7, UUID: uuid, Name: "player", Session: s}); err != nil {
		t.Fatal(err)
	}
	control := &ServerControlAdapter{PM: manager, World: &commandTestWorld{reject: true}}
	if err := control.SetGameMode(7, 1); err == nil {
		t.Fatal("game mode accepted a full world queue")
	}
	if err := control.SetPlayerSpeed(7, 10); err == nil {
		t.Fatal("speed accepted a full world queue")
	}
	if err := control.TeleportPlayer(7, 1, 2, 3); err == nil {
		t.Fatal("teleport accepted a full world queue")
	}
}

// TestTeleportSendFailureDoesNotLeavePendingAck prevents a stuck movement state.
func TestTeleportSendFailureDoesNotLeavePendingAck(t *testing.T) {
	s, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry(), InitialState: protocol.StatePlay})
	if err != nil {
		t.Fatal(err)
	}
	uuid := protocol.UUID{10, 11}
	player := entity.NewPlayer(7, uuid, "player", entity.Vec3{}, entity.Vec2{}, s, 10)
	s.BindPlayer(player)
	manager := NewPlayerManager()
	if err := manager.AddPlayer(&OnlinePlayer{EntityID: 7, UUID: uuid, Name: "player", Session: s}); err != nil {
		t.Fatal(err)
	}
	world := &commandTestWorld{}
	control := &ServerControlAdapter{PM: manager, World: world}
	if err := control.TeleportPlayer(7, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	world.queued[0]()
	if s.hasPendingTeleport() {
		t.Fatal("failed send left an unacknowledgeable teleport ID")
	}
}

func TestAdministrativeCommandsRejectUnavailableTargets(t *testing.T) {
	control := &ServerControlAdapter{}
	if err := control.SetOp("missing", 4); err == nil {
		t.Fatal("SetOp succeeded without a player manager")
	}
	if err := control.RemoveOp("missing"); err == nil {
		t.Fatal("RemoveOp succeeded without a player manager")
	}
	if err := control.KickPlayer("missing", "reason"); err == nil {
		t.Fatal("KickPlayer succeeded without a player manager")
	}
}

func TestGiveItemExecutesThroughWorldQueue(t *testing.T) {
	sess, err := New(Config{
		Connection:   newMockConn(),
		Registry:     protocol.NewRegistry(),
		InitialState: protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	manager := NewPlayerManager()
	windows := inventory.NewWindowManager()
	uuid := protocol.UUID{3, 4}
	if err := manager.AddPlayer(&OnlinePlayer{
		Session: sess, EntityID: 7, UUID: uuid, Name: "player", Windows: windows,
	}); err != nil {
		t.Fatalf("add player: %v", err)
	}
	var queued func()
	world := &DefaultWorldAccessor{SubmitTask: func(task func()) bool {
		queued = task
		return true
	}}
	control := &ServerControlAdapter{PM: manager, WorldAccess: world}
	if err := control.GiveItem(7, "minecraft:stone", 65); err != nil {
		t.Fatalf("GiveItem: %v", err)
	}
	if queued == nil {
		t.Fatal("give command was not queued")
	}
	if !windows.Inventory().Get(inventory.HotbarStart).Empty() {
		t.Fatal("inventory changed before world command execution")
	}
	queued()
	stoneID := item.IDByName("minecraft:stone")
	if got := windows.Inventory().Get(inventory.HotbarStart); got.ItemID != stoneID || got.ItemCount != 64 {
		t.Fatalf("first stack = %#v", got)
	}
	if got := windows.Inventory().Get(inventory.HotbarStart + 1); got.ItemID != stoneID || got.ItemCount != 1 {
		t.Fatalf("second stack = %#v", got)
	}
}
