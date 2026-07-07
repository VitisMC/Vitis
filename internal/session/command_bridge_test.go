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
