package protocol

import (
	"errors"
	"testing"
)

func TestBufferReadWriteRoundtrip(t *testing.T) {
	buf := NewBuffer(64)
	buf.WriteVarInt(12345)
	if err := buf.WriteString("vitis"); err != nil {
		t.Fatalf("write string failed: %v", err)
	}
	buf.WriteUint16(25565)
	buf.WriteInt64(99887766)
	buf.WriteBytes([]byte{0xAA, 0xBB})

	value, err := buf.ReadVarInt()
	if err != nil {
		t.Fatalf("read varint failed: %v", err)
	}
	if value != 12345 {
		t.Fatalf("unexpected varint value: %d", value)
	}

	text, err := buf.ReadString()
	if err != nil {
		t.Fatalf("read string failed: %v", err)
	}
	if text != "vitis" {
		t.Fatalf("unexpected string: %q", text)
	}

	port, err := buf.ReadUint16()
	if err != nil {
		t.Fatalf("read uint16 failed: %v", err)
	}
	if port != 25565 {
		t.Fatalf("unexpected uint16: %d", port)
	}

	keepalive, err := buf.ReadInt64()
	if err != nil {
		t.Fatalf("read int64 failed: %v", err)
	}
	if keepalive != 99887766 {
		t.Fatalf("unexpected int64: %d", keepalive)
	}

	raw, err := buf.ReadBytes(2)
	if err != nil {
		t.Fatalf("read bytes failed: %v", err)
	}
	if len(raw) != 2 || raw[0] != 0xAA || raw[1] != 0xBB {
		t.Fatalf("unexpected bytes: %v", raw)
	}

	if !buf.Exhausted() {
		t.Fatal("expected exhausted buffer")
	}
}

func TestBufferReadUnderflow(t *testing.T) {
	buf := WrapBuffer([]byte{0x01})
	_, err := buf.ReadBytes(2)
	if !errors.Is(err, ErrBufferUnderflow) {
		t.Fatalf("expected ErrBufferUnderflow, got %v", err)
	}
}

func TestBufferWriteStringTooLarge(t *testing.T) {
	buf := NewBuffer(0)
	err := buf.WriteString(string(make([]byte, 0)))
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
}

func TestBufferBoolRoundtrip(t *testing.T) {
	buf := NewBuffer(4)
	buf.WriteBool(true)
	buf.WriteBool(false)

	v1, err := buf.ReadBool()
	if err != nil {
		t.Fatalf("read bool failed: %v", err)
	}
	if !v1 {
		t.Fatal("expected true")
	}

	v2, err := buf.ReadBool()
	if err != nil {
		t.Fatalf("read bool failed: %v", err)
	}
	if v2 {
		t.Fatal("expected false")
	}

	if !buf.Exhausted() {
		t.Fatal("expected exhausted buffer")
	}
}

func TestBufferUUIDRoundtrip(t *testing.T) {
	original := UUID{0x0123456789ABCDEF, 0xFEDCBA9876543210}
	buf := NewBuffer(32)
	buf.WriteUUID(original)

	decoded, err := buf.ReadUUID()
	if err != nil {
		t.Fatalf("read uuid failed: %v", err)
	}
	if decoded != original {
		t.Fatalf("unexpected uuid: got %v want %v", decoded, original)
	}
	if !buf.Exhausted() {
		t.Fatal("expected exhausted buffer")
	}
}

func TestBufferUUIDZero(t *testing.T) {
	buf := NewBuffer(16)
	buf.WriteUUID(UUID{})

	decoded, err := buf.ReadUUID()
	if err != nil {
		t.Fatalf("read uuid failed: %v", err)
	}
	if decoded != (UUID{}) {
		t.Fatalf("expected zero uuid, got %v", decoded)
	}
}
