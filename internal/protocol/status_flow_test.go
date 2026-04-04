package protocol_test

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/states"
)

func TestStatusFlowHandshakeToStatusAndPing(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.AnyVersion); err != nil {
		t.Fatalf("register core states failed: %v", err)
	}

	session := protocol.NewSessionState(protocol.AnyVersion, protocol.StateHandshake)
	decoder := protocol.NewDecoder(registry, 0)
	encoder := protocol.NewEncoder(registry, 0)

	handshakeFrame := encodeHandshakeFrame(t, 769, "localhost", 25565, 1)
	decodedHandshake, err := decoder.Decode(session, handshakeFrame)
	if err != nil {
		t.Fatalf("decode handshake failed: %v", err)
	}
	if decodedHandshake.ID() != 0x00 {
		t.Fatalf("unexpected handshake packet id %d", decodedHandshake.ID())
	}
	if session.State() != protocol.StateStatus {
		t.Fatalf("expected status state after handshake, got %s", session.State())
	}

	statusRequestFrame := encodeRawFrame(t, 0x00, nil)
	decodedStatusRequest, err := decoder.Decode(session, statusRequestFrame)
	if err != nil {
		t.Fatalf("decode status request failed: %v", err)
	}
	if decodedStatusRequest.ID() != 0x00 {
		t.Fatalf("unexpected status request packet id %d", decodedStatusRequest.ID())
	}

	statusResponsePayload := protocol.NewBuffer(128)
	if err := statusResponsePayload.WriteString(`{"version":{"name":"1.21.4","protocol":769},"players":{"max":200,"online":1,"sample":[]},"description":{"text":"Vitis Server"}}`); err != nil {
		t.Fatalf("encode status response payload failed: %v", err)
	}
	if _, err := encoder.Encode(session, &rawPacket{id: 0x00, payload: statusResponsePayload.Bytes()}, nil); err != nil {
		t.Fatalf("encode status response failed: %v", err)
	}

	pingRequestPayload := protocol.NewBuffer(16)
	pingRequestPayload.WriteInt64(987654321)
	pingRequestFrame := encodeRawFrame(t, 0x01, pingRequestPayload.Bytes())
	decodedPingRequest, err := decoder.Decode(session, pingRequestFrame)
	if err != nil {
		t.Fatalf("decode ping request failed: %v", err)
	}
	if decodedPingRequest.ID() != 0x01 {
		t.Fatalf("unexpected ping request packet id %d", decodedPingRequest.ID())
	}

	pingResponsePayload := protocol.NewBuffer(16)
	pingResponsePayload.WriteInt64(987654321)
	if _, err := encoder.Encode(session, &rawPacket{id: 0x01, payload: pingResponsePayload.Bytes()}, nil); err != nil {
		t.Fatalf("encode ping response failed: %v", err)
	}
}

func TestLoginFlowHandshakeToConfiguration(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.AnyVersion); err != nil {
		t.Fatalf("register core states failed: %v", err)
	}

	session := protocol.NewSessionState(protocol.AnyVersion, protocol.StateHandshake)
	decoder := protocol.NewDecoder(registry, 0)
	encoder := protocol.NewEncoder(registry, 0)

	handshakeFrame := encodeHandshakeFrame(t, 769, "localhost", 25565, 2)
	_, err := decoder.Decode(session, handshakeFrame)
	if err != nil {
		t.Fatalf("decode handshake failed: %v", err)
	}
	if session.State() != protocol.StateLogin {
		t.Fatalf("expected login state after handshake, got %s", session.State())
	}

	loginStartFrame := encodeLoginStartFrame(t, "Steve", protocol.UUID{0x1234, 0x5678})
	decodedLoginStart, err := decoder.Decode(session, loginStartFrame)
	if err != nil {
		t.Fatalf("decode login start failed: %v", err)
	}
	if decodedLoginStart.ID() != 0x00 {
		t.Fatalf("unexpected login start packet id 0x%02X", decodedLoginStart.ID())
	}

	loginSuccessPayload := protocol.NewBuffer(64)
	loginSuccessPayload.WriteUUID(protocol.UUID{0xAAAA, 0xBBBB})
	if err := loginSuccessPayload.WriteString("Steve"); err != nil {
		t.Fatalf("encode login success name failed: %v", err)
	}
	loginSuccessPayload.WriteVarInt(0)
	if _, err := encoder.Encode(session, &rawPacket{id: 0x02, payload: loginSuccessPayload.Bytes()}, nil); err != nil {
		t.Fatalf("encode login success failed: %v", err)
	}
	if session.State() != protocol.StateLogin {
		t.Fatalf("expected login state after login success (before ack), got %s", session.State())
	}

	loginAckFrame := encodeRawFrame(t, 0x03, nil)
	decodedAck, err := decoder.Decode(session, loginAckFrame)
	if err != nil {
		t.Fatalf("decode login acknowledged failed: %v", err)
	}
	if decodedAck.ID() != 0x03 {
		t.Fatalf("unexpected login acknowledged packet id 0x%02X", decodedAck.ID())
	}
	if session.State() != protocol.StateConfiguration {
		t.Fatalf("expected configuration state after login acknowledged, got %s", session.State())
	}
}

func TestDisconnectLoginEncode(t *testing.T) {
	registry := protocol.NewRegistry()
	if err := states.RegisterCore(registry, protocol.AnyVersion); err != nil {
		t.Fatalf("register core states failed: %v", err)
	}

	session := protocol.NewSessionState(protocol.AnyVersion, protocol.StateLogin)
	encoder := protocol.NewEncoder(registry, 0)

	disconnectPayload := protocol.NewBuffer(64)
	if err := disconnectPayload.WriteString(`{"text":"Server full"}`); err != nil {
		t.Fatalf("encode disconnect reason failed: %v", err)
	}
	frame, err := encoder.Encode(session, &rawPacket{id: 0x00, payload: disconnectPayload.Bytes()}, nil)
	if err != nil {
		t.Fatalf("encode disconnect failed: %v", err)
	}
	if len(frame) == 0 {
		t.Fatal("expected non-empty frame")
	}
}

func encodeLoginStartFrame(t *testing.T, name string, uuid protocol.UUID) []byte {
	t.Helper()
	buf := protocol.NewBuffer(64)
	buf.WriteVarInt(0x00)
	if err := buf.WriteString(name); err != nil {
		t.Fatalf("encode login start name failed: %v", err)
	}
	buf.WriteUUID(uuid)
	out := make([]byte, len(buf.Bytes()))
	copy(out, buf.Bytes())
	return out
}

func encodeRawFrame(t *testing.T, packetID int32, payload []byte) []byte {
	t.Helper()
	buf := protocol.NewBuffer(64)
	buf.WriteVarInt(packetID)
	if len(payload) > 0 {
		buf.WriteBytes(payload)
	}
	out := make([]byte, len(buf.Bytes()))
	copy(out, buf.Bytes())
	return out
}

type rawPacket struct {
	id      int32
	payload []byte
}

func (p *rawPacket) ID() int32 {
	return p.id
}

func (p *rawPacket) Decode(_ *protocol.Buffer) error {
	return nil
}

func (p *rawPacket) Encode(buf *protocol.Buffer) error {
	buf.WriteBytes(p.payload)
	return nil
}

func encodeHandshakeFrame(t *testing.T, version int32, address string, port uint16, nextState int32) []byte {
	t.Helper()
	buf := protocol.NewBuffer(64)
	buf.WriteVarInt(0x00)
	buf.WriteVarInt(version)
	if err := buf.WriteString(address); err != nil {
		t.Fatalf("encode handshake address failed: %v", err)
	}
	buf.WriteUint16(port)
	buf.WriteVarInt(nextState)
	out := make([]byte, len(buf.Bytes()))
	copy(out, buf.Bytes())
	return out
}
