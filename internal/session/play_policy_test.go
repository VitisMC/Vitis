package session

import (
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
)

func TestPlayValidatedNoOpPolicyHasUniqueIDsAndReasons(t *testing.T) {
	seen := make(map[int32]struct{})
	for _, policy := range playValidatedNoOps() {
		if policy.reason == "" {
			t.Fatal("validated no-op has no reason")
		}
		packet := policy.factory()
		if packet == nil {
			t.Fatal("validated no-op factory returned nil")
		}
		if _, duplicate := seen[packet.ID()]; duplicate {
			t.Fatalf("duplicate validated no-op packet ID %d", packet.ID())
		}
		seen[packet.ID()] = struct{}{}
	}
}

// TestFixedWindowLimiterEnforcesAndResetsLimit verifies bounded chat and command windows.
func TestFixedWindowLimiterEnforcesAndResetsLimit(t *testing.T) {
	limiter := &fixedWindowLimiter{}
	now := time.Unix(100, 0)
	if !limiter.allow(now, 2) || !limiter.allow(now, 2) {
		t.Fatal("limiter rejected requests within the limit")
	}
	if limiter.allow(now, 2) {
		t.Fatal("limiter accepted a request above the limit")
	}
	if !limiter.allow(now.Add(10*time.Second), 2) {
		t.Fatal("limiter did not reset after the window")
	}
}

// TestBlockDigDoesNotConsumeChatRateLimit prevents gameplay actions from being classified as chat.
func TestBlockDigDoesNotConsumeChatRateLimit(t *testing.T) {
	router := NewPacketRouter()
	cfg := PlayBootstrapConfig{ChatRateLimit: 1}
	if err := RegisterPlayHandlers(router, cfg, nil, nil); err != nil {
		t.Fatalf("RegisterPlayHandlers: %v", err)
	}

	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register protocol packets: %v", err)
	}
	s, err := New(Config{
		ID:             1,
		Connection:     newMockConn(),
		Registry:       registry,
		InitialVersion: protocol.CurrentVersion,
		InitialState:   protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	for i := int32(0); i < 3; i++ {
		packet := &playpacket.BlockDig{Status: playpacket.DigCancelled, Sequence: i}
		if err := router.Handle(s, protocol.StatePlay, packet); err != nil {
			t.Fatalf("handle block dig %d: %v", i, err)
		}
	}
	if _, exists := s.SessionData().Load("chat_rate"); exists {
		t.Fatal("block digging consumed the chat rate limit")
	}
}

// TestChatRateLimitOnlyCountsMessages verifies that digging cannot exhaust the chat quota.
func TestChatRateLimitOnlyCountsMessages(t *testing.T) {
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{ChatRateLimit: 1}, nil, nil); err != nil {
		t.Fatalf("RegisterPlayHandlers: %v", err)
	}
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register protocol packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID:             1,
		Connection:     conn,
		Registry:       registry,
		InitialVersion: protocol.CurrentVersion,
		InitialState:   protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	for i := int32(0); i < 4; i++ {
		if err := router.Handle(s, protocol.StatePlay, &playpacket.BlockDig{Status: playpacket.DigCancelled, Sequence: i}); err != nil {
			t.Fatalf("handle dig %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := router.Handle(s, protocol.StatePlay, &playpacket.ServerboundChatMessage{Message: "hello"}); err != nil {
			t.Fatalf("handle chat %d: %v", i, err)
		}
	}
	if got := len(conn.sent); got != 9 {
		t.Fatalf("sent %d packets, want four dig resyncs, four acknowledgments and one rate-limit warning", got)
	}
	for i := 0; i < 4; i++ {
		if packet := <-conn.sent; packet.ID != (&playpacket.BlockUpdate{}).ID() {
			t.Fatalf("packet %d = %d, want digging resync", i*2, packet.ID)
		}
		if packet := <-conn.sent; packet.ID != (&playpacket.AcknowledgeBlockChange{}).ID() {
			t.Fatalf("packet %d = %d, want digging acknowledgment", i*2+1, packet.ID)
		}
	}
	if packet := <-conn.sent; packet.ID != (&playpacket.SystemChatMessage{}).ID() {
		t.Fatalf("rate-limit packet = %d, want system chat", packet.ID)
	}
}

// TestRejectedBlockDigResyncsAndAcknowledges verifies the complete client action sequence.
func TestRejectedBlockDigResyncsAndAcknowledges(t *testing.T) {
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, nil); err != nil {
		t.Fatalf("RegisterPlayHandlers: %v", err)
	}
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.CurrentVersion); err != nil {
		t.Fatalf("register protocol packets: %v", err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID:             1,
		Connection:     conn,
		Registry:       registry,
		InitialVersion: protocol.CurrentVersion,
		InitialState:   protocol.StatePlay,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	position := playpacket.BlockPos{X: 4, Y: 65, Z: 5}
	if err := router.Handle(s, protocol.StatePlay, &playpacket.BlockDig{
		Status: playpacket.DigStarted, Position: position, Sequence: 42,
	}); err != nil {
		t.Fatalf("handle rejected dig: %v", err)
	}
	update := <-conn.sent
	if update.ID != (&playpacket.BlockUpdate{}).ID() {
		t.Fatalf("first packet ID = %d, want block update", update.ID)
	}
	var blockUpdate playpacket.BlockUpdate
	if err := blockUpdate.Decode(protocol.WrapBuffer(update.Payload)); err != nil || blockUpdate.Position != position {
		t.Fatalf("block resync = %+v, error = %v", blockUpdate, err)
	}
	ack := <-conn.sent
	if ack.ID != (&playpacket.AcknowledgeBlockChange{}).ID() {
		t.Fatalf("second packet ID = %d, want action acknowledgment", ack.ID)
	}
	var acknowledged playpacket.AcknowledgeBlockChange
	if err := acknowledged.Decode(protocol.WrapBuffer(ack.Payload)); err != nil || acknowledged.Sequence != 42 {
		t.Fatalf("action acknowledgment = %+v, error = %v", acknowledged, err)
	}
}

// TestRegisterPlayHandlersCoversEveryProtocol769ServerboundID prevents codec-only coverage claims.
func TestRegisterPlayHandlersCoversEveryProtocol769ServerboundID(t *testing.T) {
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, nil, nil); err != nil {
		t.Fatalf("RegisterPlayHandlers: %v", err)
	}
	const serverboundPlayPacketCount = 62
	for packetID := int32(0); packetID < serverboundPlayPacketCount; packetID++ {
		if !router.Has(protocol.StatePlay, packetID) {
			t.Errorf("serverbound Play packet ID %d has no handler policy", packetID)
		}
	}
}
