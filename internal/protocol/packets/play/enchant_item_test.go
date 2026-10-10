package play

import (
	"bytes"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

// TestContainerButtonVarInts verifies fields are not truncated to single bytes.
func TestContainerButtonVarInts(t *testing.T) {
	packet := &EnchantItem{WindowID: 300, Enchantment: 65536}
	buffer := protocol.NewBuffer(16)
	if err := packet.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer.Bytes(), []byte{0xac, 0x02, 0x80, 0x80, 0x04}) {
		t.Fatalf("encoded = %x", buffer.Bytes())
	}
	var decoded EnchantItem
	if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatal(err)
	}
	if decoded != *packet {
		t.Fatalf("decoded = %+v", decoded)
	}
}

// TestContainerButtonRejectsMalformedInput verifies truncated and negative requests.
func TestContainerButtonRejectsMalformedInput(t *testing.T) {
	for _, raw := range [][]byte{{}, {1}, {1, 0x80}, {1, 2, 3}, {0xff, 0xff, 0xff, 0xff, 0x0f, 0}, {1, 0xff, 0xff, 0xff, 0xff, 0x0f}} {
		var packet EnchantItem
		if err := packet.Decode(protocol.WrapBuffer(raw)); err == nil {
			t.Fatalf("accepted %x", raw)
		}
	}
	for _, packet := range []EnchantItem{{WindowID: -1}, {WindowID: 1, Enchantment: -1}} {
		if err := packet.Encode(protocol.NewBuffer(16)); err == nil {
			t.Fatal("encoded negative field")
		}
	}
}
