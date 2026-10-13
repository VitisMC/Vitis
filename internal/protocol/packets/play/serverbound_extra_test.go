package play

import (
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestServerboundCustomPayloadRejectsInvalidChannel(t *testing.T) {
	buffer := protocol.NewBuffer(64)
	if err := buffer.WriteString("invalid channel"); err != nil {
		t.Fatalf("encode channel: %v", err)
	}

	packet := NewServerboundPlayCustomPayload().(*ServerboundPlayCustomPayload)
	if err := packet.Decode(protocol.WrapBuffer(buffer.Bytes())); err == nil || !strings.Contains(err.Error(), "invalid custom payload channel") {
		t.Fatalf("expected invalid channel error, got %v", err)
	}
}

func TestServerboundCustomPayloadDecodesResourceLocation(t *testing.T) {
	buffer := protocol.NewBuffer(64)
	if err := buffer.WriteString("minecraft:brand"); err != nil {
		t.Fatalf("encode channel: %v", err)
	}
	buffer.WriteBytes([]byte{1, 2, 3})

	packet := NewServerboundPlayCustomPayload().(*ServerboundPlayCustomPayload)
	if err := packet.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if packet.Channel != "minecraft:brand" || len(packet.Data) != 3 {
		t.Fatalf("unexpected decoded payload: %#v", packet)
	}
}
