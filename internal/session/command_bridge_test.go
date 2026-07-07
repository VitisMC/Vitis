package session

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

type captureSession struct {
	mu   sync.Mutex
	sent []protocol.Packet
}

func (s *captureSession) ID() uint64                                       { return 1 }
func (s *captureSession) NetworkSessionID() uint64                         { return 1 }
func (s *captureSession) Context() context.Context                         { return context.Background() }
func (s *captureSession) RemoteAddr() net.Addr                             { return nil }
func (s *captureSession) Lifecycle() LifecycleState                        { return LifecycleActive }
func (s *captureSession) ProtocolState() protocol.State                    { return protocol.StatePlay }
func (s *captureSession) ProtocolVersion() int32                           { return 769 }
func (s *captureSession) Player() Player                                   { return nil }
func (s *captureSession) BindPlayer(Player)                                {}
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

func TestServerControlGiveItemStacksAndSyncsSlots(t *testing.T) {
	pm := NewPlayerManager()
	session := &captureSession{}
	windows := inventory.NewWindowManager()
	itemID := item.IDByName("minecraft:stone")
	windows.Inventory().Set(inventory.MainInventoryStart, inventory.NewSlot(itemID, 63))
	uuid := protocol.OfflinePlayerUUID("player")
	pm.players[uuid] = &OnlinePlayer{
		Session:  session,
		EntityID: 7,
		UUID:     uuid,
		Name:     "player",
		Windows:  windows,
	}

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.GiveItem(7, "minecraft:stone", 3); err != nil {
		t.Fatalf("give item failed: %v", err)
	}

	first := windows.Inventory().Get(inventory.MainInventoryStart)
	second := windows.Inventory().Get(inventory.MainInventoryStart + 1)
	if first.ItemID != itemID || first.ItemCount != 64 {
		t.Fatalf("first stack = id %d count %d, want stone x64", first.ItemID, first.ItemCount)
	}
	if second.ItemID != itemID || second.ItemCount != 2 {
		t.Fatalf("second stack = id %d count %d, want stone x2", second.ItemID, second.ItemCount)
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
		if slotPacket.SlotIdx != wantSlot {
			t.Fatalf("packet %d slot = %d, want %d", index, slotPacket.SlotIdx, wantSlot)
		}
	}
}

func TestServerControlGiveItemRejectsWhenInventoryIsFull(t *testing.T) {
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

	control := NewServerControl(pm, nil, nil).(*ServerControlAdapter)
	if err := control.GiveItem(7, "minecraft:dirt", 1); err == nil {
		t.Fatal("expected full inventory error")
	}
	if got := len(session.packets()); got != 0 {
		t.Fatalf("packet count = %d, want 0", got)
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

func TestSendPlayBootstrapSendsDifficulty(t *testing.T) {
	session := &captureSession{}
	cfg := DefaultPlayBootstrapConfig()
	cfg.SpawnChunks = emptySpawnChunks{}
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
