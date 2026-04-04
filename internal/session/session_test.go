package session

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	handshakepacket "github.com/vitismc/vitis/internal/protocol/packets/handshake"
	statuspacket "github.com/vitismc/vitis/internal/protocol/packets/status"
	"github.com/vitismc/vitis/internal/protocol/states"
)

type testPayloadPacket struct {
	packetID int32
	value    int32
}

func (p *testPayloadPacket) ID() int32 {
	return p.packetID
}

func (p *testPayloadPacket) Decode(buf *protocol.Buffer) error {
	value, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.value = value
	return nil
}

func (p *testPayloadPacket) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.value)
	return nil
}

type testTransitionPacket struct {
	packetID int32
	next     protocol.State
}

func (p *testTransitionPacket) ID() int32 {
	return p.packetID
}

func (p *testTransitionPacket) Decode(_ *protocol.Buffer) error {
	return nil
}

func (p *testTransitionPacket) Encode(_ *protocol.Buffer) error {
	return nil
}

func (p *testTransitionPacket) InboundStateTransition() (protocol.State, bool) {
	return p.next, true
}

type mockConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	sent chan network.Packet

	closed atomic.Bool
}

func newMockConn() *mockConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockConn{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		sent:   make(chan network.Packet, 16),
	}
}

func (c *mockConn) Context() context.Context {
	return c.ctx
}

func (c *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 25565}
}

func (c *mockConn) Send(packet *network.Packet) error {
	if c.closed.Load() {
		return ErrSessionClosing
	}
	payload := make([]byte, len(packet.Payload))
	copy(payload, packet.Payload)
	c.sent <- network.Packet{ID: packet.ID, Payload: payload}
	return nil
}

func (c *mockConn) Close() error {
	c.once.Do(func() {
		c.closed.Store(true)
		c.cancel()
		close(c.done)
	})
	return nil
}

func (c *mockConn) ForceClose(_ error) error {
	return c.Close()
}

func (c *mockConn) Done() <-chan struct{} {
	return c.done
}

func (c *mockConn) EnableCompression(_ int) {}

func (c *mockConn) EnableEncryption(_, _ interface{}) {}

func (m *mockConn) enableNetworkEncryption(_, _ interface{}) {}

func TestCreateSession(t *testing.T) {
	registry := protocol.NewRegistry()
	conn := newMockConn()

	s, err := New(Config{
		ID:               1,
		NetworkSessionID: 11,
		Connection:       conn,
		Registry:         registry,
		InitialVersion:   protocol.AnyVersion,
		InitialState:     protocol.StateHandshake,
	})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if s.ID() != 1 {
		t.Fatalf("unexpected session id: %d", s.ID())
	}
	if s.NetworkSessionID() != 11 {
		t.Fatalf("unexpected network session id: %d", s.NetworkSessionID())
	}
	if s.ProtocolState() != protocol.StateHandshake {
		t.Fatalf("unexpected protocol state: %s", s.ProtocolState())
	}
	if s.Lifecycle() != LifecycleActive {
		t.Fatalf("unexpected lifecycle: %s", s.Lifecycle())
	}
}

func TestSendPacket(t *testing.T) {
	registry := protocol.NewRegistry()
	factory := func() protocol.Packet { return &testPayloadPacket{packetID: 0x21} }
	if err := registry.Register(protocol.AnyVersion, protocol.StateHandshake, protocol.DirectionOutbound, 0x21, factory); err != nil {
		t.Fatalf("register outbound packet failed: %v", err)
	}

	conn := newMockConn()
	s, err := New(Config{
		ID:               1,
		NetworkSessionID: 9,
		Connection:       conn,
		Registry:         registry,
		InitialVersion:   protocol.AnyVersion,
		InitialState:     protocol.StateHandshake,
	})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	out := &testPayloadPacket{packetID: 0x21, value: 321}
	if err := s.Send(out); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	select {
	case packet := <-conn.sent:
		if packet.ID != 0x21 {
			t.Fatalf("unexpected packet id: %d", packet.ID)
		}
		buf := protocol.WrapBuffer(packet.Payload)
		value, err := buf.ReadVarInt()
		if err != nil {
			t.Fatalf("decode payload failed: %v", err)
		}
		if value != 321 {
			t.Fatalf("unexpected payload value: %d", value)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timed out waiting for outbound packet")
	}
}

func TestInboundStateTransition(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := registry.Register(protocol.AnyVersion, protocol.StateHandshake, protocol.DirectionInbound, 0x01, func() protocol.Packet {
		return &testTransitionPacket{packetID: 0x01, next: protocol.StateLogin}
	}); err != nil {
		t.Fatalf("register inbound packet failed: %v", err)
	}

	router := NewPacketRouter()
	handled := make(chan struct{}, 1)
	if err := router.Register(protocol.StateHandshake, 0x01, func(_ Session, _ protocol.Packet) error {
		handled <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("register packet handler failed: %v", err)
	}

	conn := newMockConn()
	s, err := New(Config{
		ID:               1,
		NetworkSessionID: 7,
		Connection:       conn,
		Registry:         registry,
		Router:           router,
		InitialVersion:   protocol.AnyVersion,
		InitialState:     protocol.StateHandshake,
	})
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if err := s.HandleNetworkPacket(&network.Packet{ID: 0x01}); err != nil {
		t.Fatalf("handle network packet failed: %v", err)
	}

	select {
	case <-handled:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timed out waiting for handler dispatch")
	}

	if s.ProtocolState() != protocol.StateLogin {
		t.Fatalf("expected login state, got %s", s.ProtocolState())
	}
}

func TestStatusFlowHandshakeRequestPing(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.AnyVersion); err != nil {
		t.Fatalf("register core states failed: %v", err)
	}

	router := NewPacketRouter()
	manager, err := NewManager(ManagerConfig{Registry: registry, Router: router})
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}

	provider := &DefaultStatusInfoProvider{
		VersionName:     "1.21.4",
		ProtocolVersion: 767,
		MaxPlayers:      200,
		Description:     "Vitis Server",
		SessionCounter:  manager,
	}
	if err := RegisterStatusHandlers(router, provider); err != nil {
		t.Fatalf("register status handlers failed: %v", err)
	}

	conn := newMockConn()
	s, err := manager.CreateWithConnection(conn, 77)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	handshake := &handshakepacket.Handshake{
		ProtocolVersion: 767,
		ServerAddress:   "localhost",
		ServerPort:      25565,
		NextState:       protocol.StateStatus,
	}
	handshakePayload := encodePayload(t, handshake)
	if err := s.HandleNetworkPacket(&network.Packet{ID: handshake.ID(), Payload: handshakePayload}); err != nil {
		t.Fatalf("handle handshake packet failed: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if s.ProtocolState() == protocol.StateStatus {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s.ProtocolState() != protocol.StateStatus {
		t.Fatalf("expected status state after handshake, got %s", s.ProtocolState())
	}

	if err := s.HandleNetworkPacket(&network.Packet{ID: (&statuspacket.StatusRequest{}).ID()}); err != nil {
		t.Fatalf("handle status request failed: %v", err)
	}

	select {
	case outbound := <-conn.sent:
		if outbound.ID != (&statuspacket.StatusResponse{}).ID() {
			t.Fatalf("unexpected status response id: %d", outbound.ID)
		}
		buf := protocol.WrapBuffer(outbound.Payload)
		jsonPayload, err := buf.ReadString()
		if err != nil {
			t.Fatalf("decode status response json failed: %v", err)
		}
		if !json.Valid([]byte(jsonPayload)) {
			t.Fatalf("invalid status response json: %s", jsonPayload)
		}

		var response struct {
			Version struct {
				Name     string `json:"name"`
				Protocol int32  `json:"protocol"`
			} `json:"version"`
			Players struct {
				Max    int `json:"max"`
				Online int `json:"online"`
			} `json:"players"`
			Description struct {
				Text string `json:"text"`
			} `json:"description"`
		}
		if err := json.Unmarshal([]byte(jsonPayload), &response); err != nil {
			t.Fatalf("unmarshal status response failed: %v", err)
		}
		if response.Version.Protocol != 767 {
			t.Fatalf("unexpected protocol version: %d", response.Version.Protocol)
		}
		if response.Players.Max != 200 {
			t.Fatalf("unexpected max players: %d", response.Players.Max)
		}
		if response.Players.Online != 1 {
			t.Fatalf("unexpected online players: %d", response.Players.Online)
		}
		if response.Description.Text != "Vitis Server" {
			t.Fatalf("unexpected description: %s", response.Description.Text)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for status response")
	}

	ping := &statuspacket.PingRequest{Payload: 123456789}
	if err := s.HandleNetworkPacket(&network.Packet{ID: ping.ID(), Payload: encodePayload(t, ping)}); err != nil {
		t.Fatalf("handle ping request failed: %v", err)
	}

	select {
	case outbound := <-conn.sent:
		if outbound.ID != (&statuspacket.PingResponse{}).ID() {
			t.Fatalf("unexpected ping response id: %d", outbound.ID)
		}
		buf := protocol.WrapBuffer(outbound.Payload)
		value, err := buf.ReadInt64()
		if err != nil {
			t.Fatalf("decode ping response failed: %v", err)
		}
		if value != ping.Payload {
			t.Fatalf("unexpected ping payload: got %d want %d", value, ping.Payload)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for ping response")
	}
}

func TestManagerDisconnectCleanup(t *testing.T) {
	registry := protocol.NewRegistry()
	manager, err := NewManager(ManagerConfig{Registry: registry})
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}

	conn := newMockConn()
	s, err := manager.CreateWithConnection(conn, 42)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if manager.Count() != 1 {
		t.Fatalf("unexpected session count: %d", manager.Count())
	}

	if err := s.Close(); err != nil && !errors.Is(err, ErrSessionClosing) {
		t.Fatalf("close failed: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if manager.Count() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected manager cleanup after disconnect, count=%d", manager.Count())
}

func encodePayload(t *testing.T, packet protocol.Packet) []byte {
	t.Helper()
	buffer := protocol.NewBuffer(64)
	if err := packet.Encode(buffer); err != nil {
		t.Fatalf("encode payload for packet id=%d failed: %v", packet.ID(), err)
	}
	out := make([]byte, len(buffer.Bytes()))
	copy(out, buffer.Bytes())
	return out
}
