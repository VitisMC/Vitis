package protocol

import (
	"bytes"
	"testing"
)

type testEchoPacket struct {
	Data []byte
}

func (p *testEchoPacket) ID() int32 {
	return 0x66
}

func (p *testEchoPacket) Decode(buf *Buffer) error {
	length, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	raw, err := buf.ReadBytes(int(length))
	if err != nil {
		return err
	}
	p.Data = append(p.Data[:0], raw...)
	return nil
}

func (p *testEchoPacket) Encode(buf *Buffer) error {
	buf.WriteVarInt(int32(len(p.Data)))
	buf.WriteBytes(p.Data)
	return nil
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	registry := NewRegistry()
	factory := func() Packet { return &testEchoPacket{} }
	if err := registry.Register(765, StatePlay, DirectionInbound, 0x66, factory); err != nil {
		t.Fatalf("register inbound failed: %v", err)
	}
	if err := registry.Register(765, StatePlay, DirectionOutbound, 0x66, factory); err != nil {
		t.Fatalf("register outbound failed: %v", err)
	}

	session := NewSessionState(765, StatePlay)
	encoder := NewEncoder(registry, 0)
	decoder := NewDecoder(registry, 0)

	source := &testEchoPacket{Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}}
	frame, err := encoder.Encode(session, source, nil)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	frameBuffer := WrapBuffer(frame)
	packetLength, err := frameBuffer.ReadVarInt()
	if err != nil {
		t.Fatalf("read frame length failed: %v", err)
	}
	payload, err := frameBuffer.ReadBytes(int(packetLength))
	if err != nil {
		t.Fatalf("read frame payload failed: %v", err)
	}

	decoded, err := decoder.Decode(session, payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	echo, ok := decoded.(*testEchoPacket)
	if !ok {
		t.Fatalf("unexpected packet type %T", decoded)
	}
	if !bytes.Equal(echo.Data, source.Data) {
		t.Fatalf("unexpected payload: got %v want %v", echo.Data, source.Data)
	}
}
