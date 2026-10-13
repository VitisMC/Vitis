package session

import (
	"context"
	"github.com/vitismc/vitis/internal/block"
	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	enchdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/world/chunk"
	"math"
	"net"
	"sync"
	"testing"
)

type captureSession struct {
	mu     sync.Mutex
	sent   []protocol.Packet
	player Player
}

func (s *captureSession) ID() uint64                                       { return 1 }
func (s *captureSession) NetworkSessionID() uint64                         { return 1 }
func (s *captureSession) Context() context.Context                         { return context.Background() }
func (s *captureSession) RemoteAddr() net.Addr                             { return nil }
func (s *captureSession) Lifecycle() LifecycleState                        { return LifecycleActive }
func (s *captureSession) ProtocolState() protocol.State                    { return protocol.StatePlay }
func (s *captureSession) ProtocolVersion() int32                           { return 769 }
func (s *captureSession) Player() Player                                   { return s.player }
func (s *captureSession) BindPlayer(player Player)                         { s.player = player }
func (s *captureSession) CompressionEnabled() bool                         { return false }
func (s *captureSession) EncryptionEnabled() bool                          { return false }
func (s *captureSession) EnableCompression(bool)                           {}
func (s *captureSession) EnableEncryption(bool)                            {}
func (s *captureSession) HandleNetworkPacket(*network.Packet) error        { return nil }
func (s *captureSession) EnableNetworkCompression(int)                     {}
func (s *captureSession) EnableNetworkEncryption(interface{}, interface{}) {}
func (s *captureSession) Close() error                                     { return nil }
func (s *captureSession) ForceClose(error) error                           { return nil }

func (s *captureSession) Send(packet protocol.Packet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, packet)
	return nil
}

func (s *captureSession) packets() []protocol.Packet {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]protocol.Packet, len(s.sent))
	copy(out, s.sent)
	return out
}

type emptySpawnChunks struct{}

func (emptySpawnChunks) GenerateSpawnChunk(_, _ int32) []byte { return nil }

func TestServerControlSetDifficultyBroadcasts(t *testing.T) {
	pm := NewPlayerManager()
	first := &captureSession{}
	second := &captureSession{}
	firstUUID := protocol.OfflinePlayerUUID("first")
	secondUUID := protocol.OfflinePlayerUUID("second")
	pm.players[firstUUID] = &OnlinePlayer{Session: first, UUID: firstUUID, Name: "first"}
	pm.players[secondUUID] = &OnlinePlayer{Session: second, UUID: secondUUID, Name: "second"}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.SetDifficulty(3); err != nil {
		t.Fatalf("set difficulty failed: %v", err)
	}
	if got := control.GetDifficulty(); got != 3 {
		t.Fatalf("difficulty = %d, want 3", got)
	}

	for _, session := range []*captureSession{first, second} {
		packets := session.packets()
		if len(packets) != 1 {
			t.Fatalf("packet count = %d, want 1", len(packets))
		}
		diff, ok := packets[0].(*playpacket.Difficulty)
		if !ok {
			t.Fatalf("packet type = %T, want *play.Difficulty", packets[0])
		}
		if diff.DifficultyLevel != 3 || diff.Locked {
			t.Fatalf("difficulty packet = level %d locked %v, want level 3 unlocked", diff.DifficultyLevel, diff.Locked)
		}
	}
}

func TestServerControlSetGameModeSyncsPackets(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		GameMode: 0,
	}

	world := &commandTestWorld{}
	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	control.World = world
	if err := control.SetGameMode(7, 1); err != nil {
		t.Fatalf("set game mode failed: %v", err)
	}
	if len(world.queued) != 1 {
		t.Fatal("game mode was not queued")
	}
	world.queued[0]()
	if got := pm.players[uuid].GameMode; got != 1 {
		t.Fatalf("game mode = %d, want 1", got)
	}

	packets := session.packets()
	if len(packets) != 2 {
		t.Fatalf("packet count = %d, want 2", len(packets))
	}
	event, ok := packets[0].(*playpacket.GameEvent)
	if !ok {
		t.Fatalf("packet 0 type = %T, want *play.GameEvent", packets[0])
	}
	if event.Event != 3 || event.Value != 1 {
		t.Fatalf("game event = event %d value %.1f, want event 3 value 1.0", event.Event, event.Value)
	}
	abilities, ok := packets[1].(*playpacket.PlayerAbilitiesClientbound)
	if !ok {
		t.Fatalf("packet 1 type = %T, want *play.PlayerAbilitiesClientbound", packets[1])
	}
	if abilities.Flags&playpacket.AbilityAllowFlying == 0 {
		t.Fatalf("creative abilities flags = %08b, want allow flying", abilities.Flags)
	}
}

func TestServerControlSetGameModeRejectsInvalidMode(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		GameMode: 0,
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.SetGameMode(7, 4); err == nil {
		t.Fatal("expected invalid game mode error")
	}
	if got := pm.players[uuid].GameMode; got != 0 {
		t.Fatalf("game mode = %d, want unchanged 0", got)
	}
	if got := len(session.packets()); got != 0 {
		t.Fatalf("packet count = %d, want 0", got)
	}
}

func TestServerControlGiveItemStacksAndSyncsSlots(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	windows := inventory.NewWindowManager()
	itemID := item.IDByName("minecraft:stone")
	windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(itemID, 63))
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		Windows:  windows,
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	var queued func()
	control.WorldAccess = &DefaultWorldAccessor{SubmitTask: func(task func()) bool { queued = task; return true }}
	if err := control.GiveItem(7, "minecraft:stone", 3); err != nil {
		t.Fatalf("give item failed: %v", err)
	}

	if queued == nil {
		t.Fatal("give action was not queued")
	}
	queued()
	first := windows.Inventory().Get(inventory.HotbarStart)
	second := windows.Inventory().Get(inventory.HotbarStart + 1)
	if first.ItemID != itemID || first.ItemCount != 64 {
		t.Fatalf("first stack = id %d count %d, want stone x64", first.ItemID, first.ItemCount)
	}
	if second.ItemID != itemID || second.ItemCount != 2 {
		t.Fatalf("second stack = id %d count %d, want stone x2", second.ItemID, second.ItemCount)
	}

	packets := session.packets()
	if len(packets) != 1 {
		t.Fatalf("inventory packet count=%d", len(packets))
	}
	packet, ok := packets[0].(*playpacket.SetContainerContent)
	if !ok || packet.Slots[inventory.HotbarStart].ItemCount != 64 || packet.Slots[inventory.HotbarStart+1].ItemCount != 2 {
		t.Fatal("give action did not synchronize complete inventory")
	}
}

func TestServerControlGiveItemDropsOverflowWhenInventoryIsFull(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	windows := inventory.NewWindowManager()
	stoneID := item.IDByName("minecraft:stone")
	for _, idx := range giveInventorySlots() {
		windows.Inventory().Set(idx, inventory.NewSlot(stoneID, item.StackSize(stoneID)))
	}
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		Windows:  windows,
	}

	var queued func()
	items := entity.NewItemEntityManager()
	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	control.WorldAccess = &DefaultWorldAccessor{Items: items, NextEntityID: func() int32 { return 99 }, SubmitTask: func(task func()) bool { queued = task; return true }}
	if err := control.GiveItem(7, "minecraft:dirt", 1); err != nil {
		t.Fatal(err)
	}
	if queued == nil || items.Count() != 0 {
		t.Fatal("overflow bypassed the world tick")
	}
	queued()
	if items.Count() != 1 || items.Get(99).Stack().ItemID != item.IDByName("minecraft:dirt") || items.Get(99).Stack().ItemCount != 1 {
		t.Fatal("overflow was lost or duplicated")
	}
	for _, idx := range giveInventorySlots() {
		if windows.Inventory().Get(idx).ItemID != stoneID || windows.Inventory().Get(idx).ItemCount != item.StackSize(stoneID) {
			t.Fatal("overflow changed a full slot")
		}
	}
}

func TestServerControlEnchantItemSyncsHeldSlot(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	windows := inventory.NewWindowManager()
	windows.SetHeldSlot(2)
	swordID := item.IDByName("minecraft:diamond_sword")
	heldIdx := inventory.HotbarStart + 2
	windows.Inventory().Set(heldIdx, inventory.NewSlot(swordID, 1))
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		Windows:  windows,
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.EnchantItem(7, "minecraft:sharpness", 3); err != nil {
		t.Fatalf("enchant item failed: %v", err)
	}

	sharpnessID := enchdata.EnchantmentIDByName("minecraft:sharpness")
	held := windows.Inventory().Get(heldIdx)
	if got := held.EnchantLevel(sharpnessID); got != 3 {
		t.Fatalf("sharpness level = %d, want 3", got)
	}

	packets := session.packets()
	if len(packets) != 1 {
		t.Fatalf("packet count = %d, want 1", len(packets))
	}
	slotPacket, ok := packets[0].(*playpacket.SetPlayerInventorySlot)
	if !ok {
		t.Fatalf("packet type = %T, want *play.SetPlayerInventorySlot", packets[0])
	}
	if slotPacket.SlotIdx != int32(heldIdx) {
		t.Fatalf("slot index = %d, want %d", slotPacket.SlotIdx, heldIdx)
	}
	if got := slotPacket.SlotData.EnchantLevel(sharpnessID); got != 3 {
		t.Fatalf("synced sharpness level = %d, want 3", got)
	}
}

func TestServerControlClearInventorySyncsChangedSlots(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	windows := inventory.NewWindowManager()
	stoneID := item.IDByName("minecraft:stone")
	dirtID := item.IDByName("minecraft:dirt")
	windows.Inventory().Set(inventory.MainInventoryStart, inventory.NewSlot(stoneID, 4))
	windows.Inventory().Set(inventory.MainInventoryStart+1, inventory.NewSlot(dirtID, 2))
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		Windows:  windows,
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	removed, err := control.ClearInventory(7, "", -1)
	if err != nil {
		t.Fatalf("clear inventory failed: %v", err)
	}
	if removed != 6 {
		t.Fatalf("removed = %d, want 6", removed)
	}
	if !windows.Inventory().Get(inventory.MainInventoryStart).Empty() || !windows.Inventory().Get(inventory.MainInventoryStart+1).Empty() {
		t.Fatal("expected changed inventory slots to be empty")
	}

	packets := session.packets()
	if len(packets) != 2 {
		t.Fatalf("packet count = %d, want 2", len(packets))
	}
	for index, packet := range packets {
		slotPacket, ok := packet.(*playpacket.SetPlayerInventorySlot)
		if !ok {
			t.Fatalf("packet %d type = %T, want *play.SetPlayerInventorySlot", index, packet)
		}
		wantSlot := int32(inventory.MainInventoryStart + index)
		if slotPacket.SlotIdx != wantSlot || !slotPacket.SlotData.Empty() {
			t.Fatalf("packet %d = slot %d empty=%v, want slot %d empty", index, slotPacket.SlotIdx, slotPacket.SlotData.Empty(), wantSlot)
		}
	}
}

func TestServerControlClearInventorySupportsMaxCount(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	windows := inventory.NewWindowManager()
	stoneID := item.IDByName("minecraft:stone")
	windows.Inventory().Set(inventory.MainInventoryStart, inventory.NewSlot(stoneID, 5))
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		Windows:  windows,
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	removed, err := control.ClearInventory(7, "minecraft:stone", 2)
	if err != nil {
		t.Fatalf("clear inventory failed: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	slot := windows.Inventory().Get(inventory.MainInventoryStart)
	if slot.ItemID != stoneID || slot.ItemCount != 3 {
		t.Fatalf("slot = id %d count %d, want stone x3", slot.ItemID, slot.ItemCount)
	}

	packets := session.packets()
	if len(packets) != 1 {
		t.Fatalf("packet count = %d, want 1", len(packets))
	}
	slotPacket, ok := packets[0].(*playpacket.SetPlayerInventorySlot)
	if !ok {
		t.Fatalf("packet type = %T, want *play.SetPlayerInventorySlot", packets[0])
	}
	if slotPacket.SlotIdx != int32(inventory.MainInventoryStart) || slotPacket.SlotData.ItemCount != 3 {
		t.Fatalf("slot packet = idx %d count %d, want idx %d count 3", slotPacket.SlotIdx, slotPacket.SlotData.ItemCount, inventory.MainInventoryStart)
	}
}

func TestServerControlGameRulesValidateAndNormalizeValues(t *testing.T) {
	control := NewServerControl(nil, nil, nil).(*ServerControlAdapter)

	if value, err := control.GetGameRule("keepInventory"); err != nil || value != "false" {
		t.Fatalf("keepInventory default = %q err=%v, want false", value, err)
	}
	if value, err := control.GetGameRule("randomTickSpeed"); err != nil || value != "3" {
		t.Fatalf("randomTickSpeed default = %q err=%v, want 3", value, err)
	}

	if err := control.SetGameRule("keepInventory", "TRUE"); err != nil {
		t.Fatalf("set boolean rule failed: %v", err)
	}
	if value, _ := control.GetGameRule("keepInventory"); value != "true" {
		t.Fatalf("keepInventory = %q, want true", value)
	}

	if err := control.SetGameRule("randomTickSpeed", "003"); err != nil {
		t.Fatalf("set integer rule failed: %v", err)
	}
	if value, _ := control.GetGameRule("randomTickSpeed"); value != "3" {
		t.Fatalf("randomTickSpeed = %q, want 3", value)
	}
}

func TestServerControlGameRulesRejectWrongValueTypes(t *testing.T) {
	control := NewServerControl(nil, nil, nil).(*ServerControlAdapter)

	if err := control.SetGameRule("keepInventory", "yes"); err == nil {
		t.Fatal("expected boolean validation error")
	}
	if value, _ := control.GetGameRule("keepInventory"); value != "false" {
		t.Fatalf("keepInventory = %q, want unchanged false", value)
	}

	if err := control.SetGameRule("randomTickSpeed", "fast"); err == nil {
		t.Fatal("expected integer validation error")
	}
	if value, _ := control.GetGameRule("randomTickSpeed"); value != "3" {
		t.Fatalf("randomTickSpeed = %q, want unchanged 3", value)
	}
}

func TestServerControlDefaultGameModeStoresRuntimeValue(t *testing.T) {
	control := NewServerControl(nil, nil, nil).(*ServerControlAdapter)

	if got := control.GetDefaultGameMode(); got != 0 {
		t.Fatalf("default game mode = %d, want 0", got)
	}
	if err := control.SetDefaultGameMode(3); err != nil {
		t.Fatalf("set default game mode failed: %v", err)
	}
	if got := control.GetDefaultGameMode(); got != 3 {
		t.Fatalf("default game mode = %d, want 3", got)
	}
	if err := control.SetDefaultGameMode(4); err == nil {
		t.Fatal("expected invalid game mode error")
	}
	if got := control.GetDefaultGameMode(); got != 3 {
		t.Fatalf("default game mode after invalid set = %d, want 3", got)
	}
}

func TestServerControlWorldSpawnStoresAndBroadcastsRuntimeValue(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.SetWorldSpawn(10, 70, -5); err != nil {
		t.Fatalf("set world spawn failed: %v", err)
	}
	x, y, z := control.GetWorldSpawn()
	if x != 10 || y != 70 || z != -5 {
		t.Fatalf("world spawn = %d %d %d, want 10 70 -5", x, y, z)
	}

	packets := session.packets()
	if len(packets) != 1 {
		t.Fatalf("packet count = %d, want 1", len(packets))
	}
	spawnPacket, ok := packets[0].(*playpacket.SetDefaultSpawnPosition)
	if !ok {
		t.Fatalf("packet type = %T, want *play.SetDefaultSpawnPosition", packets[0])
	}
	if spawnPacket.X != 10 || spawnPacket.Y != 70 || spawnPacket.Z != -5 {
		t.Fatalf("spawn packet = %d %d %d, want 10 70 -5", spawnPacket.X, spawnPacket.Y, spawnPacket.Z)
	}
}

func TestServerControlSetSpawnPointStoresPlayerState(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	uuid := protocol.OfflinePlayerUUID("player")
	op := &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
	}
	pm.players[uuid] = op

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.SetSpawnPoint(7, -12, 70, 30); err != nil {
		t.Fatalf("set spawn point failed: %v", err)
	}
	if !op.HasSpawnPoint || op.SpawnX != -12 || op.SpawnY != 70 || op.SpawnZ != 30 {
		t.Fatalf("spawn point = has=%v %d %d %d, want true -12 70 30", op.HasSpawnPoint, op.SpawnX, op.SpawnY, op.SpawnZ)
	}

	if !op.SpawnForced || op.SpawnDimension != SessionDimension(session) {
		t.Fatal("personal spawn lost its forced position or dimension")
	}
	if len(session.packets()) != 0 {
		t.Fatal("personal spawn changed the world spawn packet")
	}
}

func TestHandleRespawnUsesPlayerSpawnPoint(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	player := entity.NewPlayer(7, protocol.UUID{1}, "player", entity.Vec3{}, entity.Vec2{}, session, 10)
	session.BindPlayer(player)
	player.Living().SetHealth(0)
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:       session,
		EntityID:      7,
		UUID:          uuid,
		Name:          "player",
		SpawnX:        10,
		SpawnY:        72,
		SpawnZ:        -4,
		HasSpawnPoint: true,
		SpawnForced:   true,
	}
	cfg := DefaultPlayBootstrapConfig()

	storage := chunk.NewStorage(12)
	for x := int32(-1); x <= 1; x++ {
		for z := int32(-2); z <= 1; z++ {
			storage.Set(chunk.New(x, z))
		}
	}
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() {
		if err := manager.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	world := &DefaultWorldAccessor{Chunks: manager}
	world.SetBlock(10, 71, -4, block.DefaultStateID("minecraft:stone"))
	if err := handleRespawn(session, cfg, pm, world); err != nil {
		t.Fatalf("handle respawn failed: %v", err)
	}

	for _, packet := range session.packets() {
		sync, ok := packet.(*playpacket.SyncPlayerPosition)
		if !ok {
			continue
		}
		if sync.X != 10.5 || sync.Y != 72 || sync.Z != -3.5 {
			t.Fatalf("respawn position = %.1f %.1f %.1f, want 10.5 72.0 -3.5", sync.X, sync.Y, sync.Z)
		}
		return
	}
	t.Fatal("sync player position packet was not sent")
}

func TestHandleRespawnKeepsPlayerGameMode(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	player := entity.NewPlayer(7, protocol.UUID{1}, "player", entity.Vec3{}, entity.Vec2{}, session, 10)
	session.BindPlayer(player)
	player.Living().SetHealth(0)
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		GameMode: 2,
	}
	cfg := DefaultPlayBootstrapConfig()
	cfg.GameMode = 0
	player.Living().SetGameMode(2)

	if err := handleRespawn(session, cfg, pm, nil); err != nil {
		t.Fatalf("handle respawn failed: %v", err)
	}

	for _, packet := range session.packets() {
		respawn, ok := packet.(*playpacket.Respawn)
		if !ok {
			continue
		}
		if respawn.GameMode != 2 {
			t.Fatalf("respawn game mode = %d, want 2", respawn.GameMode)
		}
		return
	}
	t.Fatal("respawn packet was not sent")
}

func TestSendPlayBootstrapSendsDifficulty(t *testing.T) {
	session := &captureSession{}
	cfg := DefaultPlayBootstrapConfig()

	cfg.Difficulty = 1
	cfg.DifficultyLocked = true

	if err := SendPlayBootstrap(session, 42, cfg); err != nil {
		t.Fatalf("send play bootstrap failed: %v", err)
	}

	for _, packet := range session.packets() {
		diff, ok := packet.(*playpacket.Difficulty)
		if !ok {
			continue
		}
		if diff.DifficultyLevel != 1 || !diff.Locked {
			t.Fatalf("difficulty packet = level %d locked %v, want level 1 locked", diff.DifficultyLevel, diff.Locked)
		}
		return
	}
	t.Fatal("difficulty packet was not sent")
}

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

// giveInventorySlots returns the inventory slots used by command fixtures.
func giveInventorySlots() []int {
	slots := []int{}
	for i := inventory.MainInventoryStart; i <= inventory.MainInventoryEnd; i++ {
		slots = append(slots, i)
	}
	for i := inventory.HotbarStart; i <= inventory.HotbarEnd; i++ {
		slots = append(slots, i)
	}
	return slots
}

// commandDifficultyWorld exposes live difficulty while retaining explicit queued mutation.
type commandDifficultyWorld struct {
	commandTestWorld
	difficulty int32
}

// Difficulty returns the authoritative test-world difficulty.
func (w *commandDifficultyWorld) Difficulty() int32 { return w.difficulty }

// SetDifficulty changes the test-world difficulty when its command executes.
func (w *commandDifficultyWorld) SetDifficulty(value int32) { w.difficulty = value }

// TestServerControlDifficultyReadsWorldAndQueuesChanges preserves live login difficulty.
func TestServerControlDifficultyReadsWorldAndQueuesChanges(t *testing.T) {
	world := &commandDifficultyWorld{difficulty: 3}
	control := NewServerControl(nil, nil, nil).(*ServerControlAdapter)
	control.World = world
	if control.GetDifficulty() != 3 {
		t.Fatal("difficulty getter retained bootstrap state")
	}
	if err := control.SetDifficulty(1); err != nil {
		t.Fatal(err)
	}
	if world.difficulty != 3 || len(world.queued) != 1 {
		t.Fatal("difficulty mutation bypassed the world tick")
	}
	world.queued[0]()
	if control.GetDifficulty() != 1 {
		t.Fatal("live difficulty was not updated")
	}
}
