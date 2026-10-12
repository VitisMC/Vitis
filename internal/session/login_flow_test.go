package session

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	loginpacket "github.com/vitismc/vitis/internal/protocol/packets/login"
	"github.com/vitismc/vitis/internal/protocol/states"
)

func newLoginTestSession(t *testing.T) (*DefaultSession, *mockConn) {
	t.Helper()
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.AnyVersion); err != nil {
		t.Fatal(err)
	}
	router := NewPacketRouter()
	if err := RegisterLoginHandlers(router, LoginConfig{CompressionThreshold: -1}); err != nil {
		t.Fatal(err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID:               1,
		NetworkSessionID: 1,
		Connection:       conn,
		Registry:         registry,
		Router:           router,
		InitialVersion:   protocol.CurrentVersion,
		InitialState:     protocol.StateLogin,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, conn
}

func TestLoginAcknowledgementRequiresLoginSuccess(t *testing.T) {
	s, _ := newLoginTestSession(t)
	ack := &loginpacket.LoginAcknowledged{}
	packet := networkPacket(ack, t)
	if err := s.HandleNetworkPacket(&packet); err == nil {
		t.Fatal("expected acknowledgement before login success to fail")
	}
	if s.ProtocolState() != protocol.StateLogin {
		t.Fatalf("unexpected state transition: %s", s.ProtocolState())
	}
}

func TestOfflineLoginIgnoresClientUUID(t *testing.T) {
	s, _ := newLoginTestSession(t)
	clientUUID := protocol.UUID{0x1234, 0x5678}
	start := &loginpacket.LoginStart{Name: "Steve", PlayerUUID: clientUUID}
	packet := networkPacket(start, t)
	if err := s.HandleNetworkPacket(&packet); err != nil {
		t.Fatal(err)
	}
	value, ok := s.sessionData.Load("login_uuid")
	if !ok {
		t.Fatal("server did not store offline identity")
	}
	want := protocol.OfflinePlayerUUID("Steve")
	if got, ok := value.(protocol.UUID); !ok || got != want {
		t.Fatalf("offline UUID: got %v want %v", got, want)
	}
	if want == clientUUID {
		t.Fatal("test client UUID unexpectedly equals derived offline UUID")
	}
}

func TestLoginRejectsIPBanBeforeAuthentication(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.AnyVersion); err != nil {
		t.Fatal(err)
	}
	router := NewPacketRouter()
	checked := false
	if err := RegisterLoginHandlers(router, LoginConfig{
		CompressionThreshold: -1,
		IPBanChecker: func(remote net.Addr) (bool, string) {
			checked = true
			if remote.String() != "127.0.0.1:25565" {
				t.Fatalf("remote address = %q", remote.String())
			}
			return true, `bad "reason"`
		},
	}); err != nil {
		t.Fatal(err)
	}
	conn := newMockConn()
	s, err := New(Config{
		ID: 1, NetworkSessionID: 1, Connection: conn, Registry: registry, Router: router,
		InitialVersion: protocol.CurrentVersion, InitialState: protocol.StateLogin,
	})
	if err != nil {
		t.Fatal(err)
	}

	packet := networkPacket(&loginpacket.LoginStart{Name: "Steve"}, t)
	if err := s.HandleNetworkPacket(&packet); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("IP ban checker was not called")
	}
	select {
	case <-conn.Done():
	case <-time.After(time.Second):
		t.Fatal("banned connection was not closed")
	}
	outbound := <-conn.sent
	if outbound.ID != loginpacket.NewDisconnect().ID() {
		t.Fatalf("outbound packet ID = %d; want disconnect", outbound.ID)
	}
}

func TestDisconnectReasonEscapesUntrustedText(t *testing.T) {
	encoded := disconnectReason("line\n\"quoted\"")
	var component struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(encoded), &component); err != nil {
		t.Fatalf("invalid JSON component: %v", err)
	}
	if component.Text != "line\n\"quoted\"" {
		t.Fatalf("decoded text = %q", component.Text)
	}
}

func networkPacket(packet protocol.Packet, t *testing.T) network.Packet {
	t.Helper()
	return network.Packet{ID: packet.ID(), Payload: encodePayload(t, packet)}
}
