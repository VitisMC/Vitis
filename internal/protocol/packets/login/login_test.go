package login

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestLoginStartEncodeDecode(t *testing.T) {
	source := &LoginStart{
		Name:       "Notch",
		PlayerUUID: protocol.UUID{0x0123456789ABCDEF, 0xFEDCBA9876543210},
	}

	buf := protocol.NewBuffer(64)
	if err := source.Encode(buf); err != nil {
		t.Fatalf("encode login start failed: %v", err)
	}

	decoded := &LoginStart{}
	if err := decoded.Decode(protocol.WrapBuffer(buf.Bytes())); err != nil {
		t.Fatalf("decode login start failed: %v", err)
	}

	if decoded.Name != source.Name {
		t.Fatalf("unexpected name: got %q want %q", decoded.Name, source.Name)
	}
	if decoded.PlayerUUID != source.PlayerUUID {
		t.Fatalf("unexpected uuid: got %v want %v", decoded.PlayerUUID, source.PlayerUUID)
	}
}

func TestLoginSuccessEncodeDecodeNoProperties(t *testing.T) {
	source := &LoginSuccess{
		UUID:       protocol.UUID{0xAAAABBBBCCCCDDDD, 0x1111222233334444},
		Name:       "Steve",
		Properties: nil,
	}

	buf := protocol.NewBuffer(128)
	if err := source.Encode(buf); err != nil {
		t.Fatalf("encode login success failed: %v", err)
	}

	decoded := &LoginSuccess{}
	if err := decoded.Decode(protocol.WrapBuffer(buf.Bytes())); err != nil {
		t.Fatalf("decode login success failed: %v", err)
	}

	if decoded.UUID != source.UUID {
		t.Fatalf("unexpected uuid: got %v want %v", decoded.UUID, source.UUID)
	}
	if decoded.Name != source.Name {
		t.Fatalf("unexpected name: got %q want %q", decoded.Name, source.Name)
	}
	if len(decoded.Properties) != 0 {
		t.Fatalf("unexpected properties count: %d", len(decoded.Properties))
	}
}

func TestLoginSuccessEncodeDecodeWithProperties(t *testing.T) {
	source := &LoginSuccess{
		UUID: protocol.UUID{0x1234, 0x5678},
		Name: "Alex",
		Properties: []LoginProperty{
			{Name: "textures", Value: "base64data", HasSig: true, Signature: "sig123"},
			{Name: "other", Value: "val", HasSig: false},
		},
	}

	buf := protocol.NewBuffer(256)
	if err := source.Encode(buf); err != nil {
		t.Fatalf("encode login success failed: %v", err)
	}

	decoded := &LoginSuccess{}
	if err := decoded.Decode(protocol.WrapBuffer(buf.Bytes())); err != nil {
		t.Fatalf("decode login success failed: %v", err)
	}

	if decoded.UUID != source.UUID {
		t.Fatalf("unexpected uuid: got %v want %v", decoded.UUID, source.UUID)
	}
	if decoded.Name != source.Name {
		t.Fatalf("unexpected name: got %q want %q", decoded.Name, source.Name)
	}
	if len(decoded.Properties) != 2 {
		t.Fatalf("unexpected properties count: %d", len(decoded.Properties))
	}

	p0 := decoded.Properties[0]
	if p0.Name != "textures" || p0.Value != "base64data" || !p0.HasSig || p0.Signature != "sig123" {
		t.Fatalf("unexpected property[0]: %+v", p0)
	}

	p1 := decoded.Properties[1]
	if p1.Name != "other" || p1.Value != "val" || p1.HasSig {
		t.Fatalf("unexpected property[1]: %+v", p1)
	}
}

func TestDisconnectEncodeDecode(t *testing.T) {
	source := &Disconnect{Reason: `{"text":"You are banned"}`}

	buf := protocol.NewBuffer(64)
	if err := source.Encode(buf); err != nil {
		t.Fatalf("encode disconnect failed: %v", err)
	}

	decoded := &Disconnect{}
	if err := decoded.Decode(protocol.WrapBuffer(buf.Bytes())); err != nil {
		t.Fatalf("decode disconnect failed: %v", err)
	}

	if decoded.Reason != source.Reason {
		t.Fatalf("unexpected reason: got %q want %q", decoded.Reason, source.Reason)
	}
}

func TestLoginAcknowledgedEncodeDecode(t *testing.T) {
	source := &LoginAcknowledged{}

	buf := protocol.NewBuffer(8)
	if err := source.Encode(buf); err != nil {
		t.Fatalf("encode login acknowledged failed: %v", err)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected empty payload, got %d bytes", buf.Len())
	}

	decoded := &LoginAcknowledged{}
	if err := decoded.Decode(protocol.WrapBuffer(buf.Bytes())); err != nil {
		t.Fatalf("decode login acknowledged failed: %v", err)
	}
}

func TestLoginAcknowledgedStateTransition(t *testing.T) {
	pkt := &LoginAcknowledged{}
	nextState, ok := pkt.InboundStateTransition()
	if !ok {
		t.Fatal("expected state transition")
	}
	if nextState != protocol.StateConfiguration {
		t.Fatalf("expected configuration state, got %s", nextState)
	}
}

func TestPacketIDs(t *testing.T) {
	tests := []struct {
		name string
		pkt  protocol.Packet
		id   int32
	}{
		{"LoginStart", &LoginStart{}, 0x00},
		{"LoginSuccess", &LoginSuccess{}, 0x02},
		{"Disconnect", &Disconnect{}, 0x00},
		{"LoginAcknowledged", &LoginAcknowledged{}, 0x03},
	}

	for _, tt := range tests {
		if tt.pkt.ID() != tt.id {
			t.Errorf("%s: unexpected packet id: got 0x%02X want 0x%02X", tt.name, tt.pkt.ID(), tt.id)
		}
	}
}
