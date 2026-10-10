package play

import (
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestSetPlayerPositionAndRotationDecodesMovementFlags(t *testing.T) {
	buffer := protocol.NewBuffer(34)
	buffer.WriteFloat64(1)
	buffer.WriteFloat64(2)
	buffer.WriteFloat64(3)
	buffer.WriteFloat32(4)
	buffer.WriteFloat32(5)
	buffer.WriteByte(0x03)

	packet := NewSetPlayerPositionAndRotation().(*SetPlayerPositionAndRotation)
	if err := packet.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !packet.OnGround || !packet.HorizontalCollision {
		t.Fatalf("movement flags = onGround:%t horizontalCollision:%t, want both true", packet.OnGround, packet.HorizontalCollision)
	}
}

func TestSetPlayerPositionRejectsUnknownMovementFlag(t *testing.T) {
	buffer := protocol.NewBuffer(25)
	buffer.WriteFloat64(1)
	buffer.WriteFloat64(2)
	buffer.WriteFloat64(3)
	buffer.WriteByte(0x04)

	packet := NewSetPlayerPosition().(*SetPlayerPosition)
	err := packet.Decode(protocol.WrapBuffer(buffer.Bytes()))
	if err == nil || !strings.Contains(err.Error(), "invalid flags") {
		t.Fatalf("expected invalid movement flags error, got %v", err)
	}
}

func TestSetPlayerOnGroundEncodesMovementFlags(t *testing.T) {
	packet := &SetPlayerOnGround{OnGround: true, HorizontalCollision: true}
	buffer := protocol.NewBuffer(1)
	if err := packet.Encode(buffer); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := buffer.Bytes(); len(got) != 1 || got[0] != 0x03 {
		t.Fatalf("encoded flags = %v, want [3]", got)
	}
}
