package inventory

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// FuzzDecodeItemStack verifies malformed component patches never panic.
func FuzzDecodeItemStack(f *testing.F) {
	f.Add([]byte{0})
	f.Add([]byte{1, 1, 0, 0})
	f.Add([]byte{1, 1, 1, 0, 3, 7})
	f.Add([]byte{1, 1, 1, 0, 23, 1, 2, 0, 0})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})
	for _, fixtures := range []map[string]string{componentPersistentFixtures(), inlineComponentFixtures()} {
		for name, fixture := range fixtures {
			tag, value, err := nbt.FromJSON([]byte(fixture))
			if err != nil {
				f.Fatal(err)
			}
			component, err := componentFromNBT(dc.DataComponentIDByName("minecraft:"+name), tag, value, 0)
			if err != nil {
				f.Fatal(err)
			}
			stack := NewSlot(1, 1)
			stack.SetComponent(dc.DataComponentIDByName("minecraft:"+name), component)
			buffer := protocol.NewBuffer(256)
			if err := EncodeItemStack(buffer, stack); err != nil {
				f.Fatal(err)
			}
			f.Add(buffer.Bytes())
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		stack, err := DecodeItemStack(protocol.WrapBuffer(data))
		if err != nil || stack.Empty() {
			return
		}
		encoded := protocol.NewBuffer(len(data) + 16)
		EncodeSlot(encoded, stack)
		if _, err := DecodeItemStack(protocol.WrapBuffer(encoded.Bytes())); err != nil {
			t.Fatalf("encoded decoded stack is invalid: %v", err)
		}
	})
}
