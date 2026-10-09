package inventory

import (
	"bytes"
	"strings"
	"testing"

	datacomponent "github.com/vitismc/vitis/internal/data/generated/data_component"
	itemdata "github.com/vitismc/vitis/internal/data/generated/item"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestUseRemainderRoundTrip checks nested item data is preserved on the wire.
func TestUseRemainderRoundTrip(t *testing.T) {
	remainder := NewSlot(42, 1)
	remainder.SetDamage(7)
	stack := NewSlot(15, 1)
	stack.ComponentPatch.Added = []DataComponent{{
		TypeID: datacomponent.ComponentUseRemainder,
		Value:  UseRemainderComponent{Stack: remainder},
	}}
	encoded := protocol.NewBuffer(64)
	if err := EncodeItemStack(encoded, stack); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeItemStack(protocol.WrapBuffer(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	value, ok := decoded.ComponentPatch.Added[0].Value.(UseRemainderComponent)
	if !ok || value.Stack.ItemID != 42 || value.Stack.Damage != 7 {
		t.Fatalf("decoded remainder = %#v", decoded.ComponentPatch.Added[0].Value)
	}
	reencoded := protocol.NewBuffer(64)
	if err := EncodeItemStack(reencoded, decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded.Bytes(), reencoded.Bytes()) {
		t.Fatalf("wire round-trip changed: %x != %x", encoded.Bytes(), reencoded.Bytes())
	}
	clone := decoded.Clone()
	cloneRemainder := clone.ComponentPatch.Added[0].Value.(UseRemainderComponent)
	cloneRemainder.Stack.ComponentPatch.Added[0].Raw[0] = 9
	if value.Stack.ComponentPatch.Added[0].Raw[0] == 9 {
		t.Fatal("cloning a nested stack changed the original")
	}
}

// TestUseRemainderRejectsEmptyAndExcessiveDepth checks bounded decoding.
func TestUseRemainderRejectsEmptyAndExcessiveDepth(t *testing.T) {
	bad := protocol.NewBuffer(16)
	bad.WriteVarInt(1)
	bad.WriteVarInt(1)
	bad.WriteVarInt(1)
	bad.WriteVarInt(0)
	bad.WriteVarInt(datacomponent.ComponentUseRemainder)
	bad.WriteVarInt(0)
	if _, err := DecodeItemStack(protocol.WrapBuffer(bad.Bytes())); err == nil {
		t.Fatal("empty use remainder was accepted")
	}

	stack := NewSlot(1, 1)
	for i := 0; i <= maxNestedItemStackDepth; i++ {
		stack = ItemStack{ItemCount: 1, ItemID: 1, ComponentPatch: ComponentPatch{
			Added: []DataComponent{{TypeID: datacomponent.ComponentUseRemainder, Value: UseRemainderComponent{Stack: stack}}},
		}}
	}
	encoded := protocol.NewBuffer(16)
	if err := EncodeItemStack(encoded, stack); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected nested-depth error, got %v", err)
	}
	if len(encoded.Bytes()) != 0 {
		t.Fatal("failed encoding modified the destination buffer")
	}
	malformed := protocol.NewBuffer(128)
	for i := 0; i <= maxNestedItemStackDepth; i++ {
		malformed.WriteVarInt(1)
		malformed.WriteVarInt(1)
		malformed.WriteVarInt(1)
		malformed.WriteVarInt(0)
		malformed.WriteVarInt(datacomponent.ComponentUseRemainder)
	}
	malformed.WriteVarInt(1)
	malformed.WriteVarInt(1)
	malformed.WriteVarInt(0)
	malformed.WriteVarInt(0)
	if _, err := DecodeItemStack(protocol.WrapBuffer(malformed.Bytes())); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected decode depth error, got %v", err)
	}
}

// TestItemStackRejectsNegativeCount checks malformed optional stacks.
func TestItemStackRejectsNegativeCount(t *testing.T) {
	encoded := protocol.NewBuffer(8)
	encoded.WriteVarInt(-1)
	if _, err := DecodeItemStack(protocol.WrapBuffer(encoded.Bytes())); err == nil {
		t.Fatal("negative count was accepted")
	}
	output := protocol.NewBuffer(8)
	if err := EncodeItemStack(output, ItemStack{ItemCount: -1}); err == nil {
		t.Fatal("negative count was encoded")
	}
}

// TestPotDecorationsRoundTrip checks bounded item-registry lists.
func TestPotDecorationsRoundTrip(t *testing.T) {
	stack := NewSlot(1, 1)
	stack.ComponentPatch.Added = []DataComponent{{
		TypeID: datacomponent.ComponentPotDecorations,
		Value:  PotDecorationsComponent{2, 3, 4, 5},
	}}
	encoded := protocol.NewBuffer(32)
	if err := EncodeItemStack(encoded, stack); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeItemStack(protocol.WrapBuffer(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	value, ok := decoded.ComponentPatch.Added[0].Value.(PotDecorationsComponent)
	if !ok || len(value) != 4 || value[3] != 5 {
		t.Fatalf("decoded decorations = %#v", decoded.ComponentPatch.Added[0].Value)
	}
	clone := decoded.Clone()
	clone.ComponentPatch.Added[0].Value.(PotDecorationsComponent)[0] = 99
	if value[0] != 2 {
		t.Fatal("cloning decorations changed the original")
	}

	bad := protocol.NewBuffer(16)
	bad.WriteVarInt(1)
	bad.WriteVarInt(1)
	bad.WriteVarInt(1)
	bad.WriteVarInt(0)
	bad.WriteVarInt(datacomponent.ComponentPotDecorations)
	bad.WriteVarInt(5)
	if _, err := DecodeItemStack(protocol.WrapBuffer(bad.Bytes())); err == nil {
		t.Fatal("five decorations were accepted")
	}
	stack.ComponentPatch.Added[0].Value = PotDecorationsComponent{itemdata.ItemCount}
	if err := EncodeItemStack(protocol.NewBuffer(16), stack); err == nil {
		t.Fatal("out-of-range decoration item ID was accepted")
	}
}

// TestItemStackRejectsUnknownItemID checks the current-version item registry bound.
func TestItemStackRejectsUnknownItemID(t *testing.T) {
	input := protocol.NewBuffer(16)
	input.WriteVarInt(1)
	input.WriteVarInt(itemdata.ItemCount)
	input.WriteVarInt(0)
	input.WriteVarInt(0)
	if _, err := DecodeItemStack(protocol.WrapBuffer(input.Bytes())); err == nil {
		t.Fatal("unknown item ID was accepted")
	}
	if err := EncodeItemStack(protocol.NewBuffer(16), NewSlot(itemdata.ItemCount, 1)); err == nil {
		t.Fatal("unknown item ID was encoded")
	}
}
