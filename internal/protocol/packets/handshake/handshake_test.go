package handshake

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestHandshakeRejectsOversizedAddress(t *testing.T) {
	address := make([]byte, maxServerAddressLength+1)
	for i := range address {
		address[i] = 'a'
	}

	buf := protocol.NewBuffer(len(address) + 16)
	buf.WriteVarInt(protocol.CurrentVersion)
	if err := buf.WriteString(string(address)); err != nil {
		t.Fatalf("write address: %v", err)
	}
	buf.WriteUint16(25565)
	buf.WriteVarInt(int32(protocol.StateLogin))

	if err := (&Handshake{}).Decode(protocol.WrapBuffer(buf.Bytes())); err == nil {
		t.Fatal("oversized server address was accepted")
	}
}
