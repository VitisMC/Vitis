package play

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

// TestBoundedPlayPacketDecodersRejectOversizedFields verifies packet-local limits.
func TestBoundedPlayPacketDecodersRejectOversizedFields(t *testing.T) {
	tests := []struct {
		name   string
		packet protocol.Packet
		build  func(*protocol.Buffer)
	}{
		{
			name:   "chat public key",
			packet: &ChatSessionUpdate{},
			build: func(buf *protocol.Buffer) {
				buf.WriteUUID(protocol.UUID{})
				buf.WriteInt64(0)
				buf.WriteVarInt(maxChatKeyMaterialLength + 1)
			},
		},
		{
			name:   "cookie payload",
			packet: &ServerboundCookieResponse{},
			build: func(buf *protocol.Buffer) {
				buf.WriteString("minecraft:test")
				buf.WriteBool(true)
				buf.WriteVarInt(maxCookiePayloadLength + 1)
			},
		},
		{
			name:   "custom payload",
			packet: &ServerboundPlayCustomPayload{},
			build: func(buf *protocol.Buffer) {
				buf.WriteString("minecraft:test")
				buf.WriteBytes(make([]byte, maxPlayCustomPayloadLength+1))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := protocol.NewBuffer(64)
			test.build(buf)
			if err := test.packet.Decode(buf); err == nil {
				t.Fatal("oversized field was accepted")
			}
		})
	}
}

// TestValidatedEnumPacketsRejectUnknownValues verifies enum bounds before handlers run.
func TestValidatedEnumPacketsRejectUnknownValues(t *testing.T) {
	tests := []protocol.Packet{
		&MessageAcknowledgement{},
		&ResourcePackReceive{},
		&AdvancementTab{},
		&DebugSampleSubscription{},
	}
	for _, packet := range tests {
		buf := protocol.NewBuffer(32)
		if _, ok := packet.(*ResourcePackReceive); ok {
			buf.WriteUUID(protocol.UUID{})
		}
		buf.WriteVarInt(1000)
		if err := packet.Decode(buf); err == nil {
			t.Fatalf("%T accepted an unknown value", packet)
		}
	}
}
