package inventory

import (
	"testing"

	datacomponent "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

func TestDecodeSlotAcceptsTypedComponentPatch(t *testing.T) {
	buf := protocol.NewBuffer(32)
	buf.WriteVarInt(1)
	buf.WriteVarInt(1)
	buf.WriteVarInt(1)
	buf.WriteVarInt(0)
	buf.WriteVarInt(datacomponent.ComponentDamage)
	buf.WriteVarInt(42)

	stack, err := DecodeSlot(buf)
	if err != nil {
		t.Fatalf("DecodeSlot: %v", err)
	}
	if stack.Damage != 42 || stack.ComponentsAdd != 1 || len(stack.ComponentPatch.Added) != 1 {
		t.Fatalf("decoded stack = %#v", stack)
	}
}

func TestDecodeSlotWithoutComponentsReadsSimpleStack(t *testing.T) {
	buf := protocol.NewBuffer(16)
	buf.WriteVarInt(3)
	buf.WriteVarInt(17)
	buf.WriteVarInt(0)
	buf.WriteVarInt(0)

	stack, err := DecodeSlot(buf)
	if err != nil {
		t.Fatalf("decode slot: %v", err)
	}
	if stack.ItemCount != 3 || stack.ItemID != 17 {
		t.Fatalf("stack = %#v", stack)
	}
}

func TestEncodeSlotNeverWritesUnsupportedComponentPayload(t *testing.T) {
	buf := protocol.NewBuffer(32)
	EncodeSlot(buf, Slot{
		ItemCount:     1,
		ItemID:        5,
		ComponentsAdd: 1,
		ComponentsRem: 1,
		RawComponents: []byte{0x7f, 0x00},
	})
	EncodeSlot(buf, NewSlot(6, 2))

	first, err := DecodeSlot(buf)
	if err != nil {
		t.Fatalf("decode first slot: %v", err)
	}
	if first.ItemID != 5 || first.ComponentsAdd != 0 || first.ComponentsRem != 0 {
		t.Fatalf("first slot = %#v", first)
	}
	second, err := DecodeSlot(buf)
	if err != nil {
		t.Fatalf("decode second slot: %v", err)
	}
	if second.ItemID != 6 || second.ItemCount != 2 {
		t.Fatalf("second slot = %#v", second)
	}
}

func TestContainerDoesNotExposeMutableSlotFields(t *testing.T) {
	container := NewContainer(1)
	container.Set(0, Slot{
		ItemCount:     1,
		ItemID:        4,
		RawComponents: []byte{1, 2},
		Enchantments:  map[int32]int32{3: 4},
	})

	got := container.Get(0)
	got.RawComponents[0] = 99
	got.Enchantments[3] = 99
	got.SetEnchant(5, 6)

	stored := container.Get(0)
	if stored.RawComponents[0] != 1 {
		t.Fatalf("stored raw components mutated: %v", stored.RawComponents)
	}
	if stored.EnchantLevel(3) != 4 || stored.EnchantLevel(5) != 0 {
		t.Fatalf("stored enchantments mutated: %v", stored.Enchantments)
	}

	snapshot := container.Slots()
	snapshot[0].RawComponents[1] = 99
	snapshot[0].Enchantments[3] = 99
	stored = container.Get(0)
	if stored.RawComponents[1] != 2 || stored.EnchantLevel(3) != 4 {
		t.Fatalf("slots snapshot mutated storage: %#v", stored)
	}
}

func TestWindowCursorDoesNotExposeMutableSlotFields(t *testing.T) {
	wm := NewWindowManager()
	wm.SetCursorItem(Slot{ItemCount: 1, ItemID: 4, Enchantments: map[int32]int32{3: 4}})

	cursor := wm.CursorItem()
	cursor.SetEnchant(3, 99)
	cursor.SetEnchant(5, 6)

	stored := wm.CursorItem()
	if stored.EnchantLevel(3) != 4 || stored.EnchantLevel(5) != 0 {
		t.Fatalf("cursor state was mutated externally: %#v", stored)
	}
}

func TestItemStackComponentPatchRoundTrip(t *testing.T) {
	stack := ItemStack{
		ItemCount: 1,
		ItemID:    5,
		ComponentPatch: ComponentPatch{
			Added: []DataComponent{
				{TypeID: datacomponent.ComponentDamage, Value: VarIntComponent(7)},
				{TypeID: datacomponent.ComponentUnbreakable, Value: BoolComponent(true)},
				{TypeID: datacomponent.ComponentEnchantments, Value: EnchantmentsComponent{
					Levels: map[int32]int32{2: 3}, ShowInTooltip: true,
				}},
			},
			Removed: []int32{datacomponent.ComponentCustomName},
		},
	}
	buf := protocol.NewBuffer(128)
	EncodeSlot(buf, stack)
	decoded, err := DecodeSlot(buf)
	if err != nil {
		t.Fatalf("DecodeSlot: %v", err)
	}
	if decoded.Damage != 7 || decoded.EnchantLevel(2) != 3 {
		t.Fatalf("derived stack fields = %#v", decoded)
	}

	reencoded := protocol.NewBuffer(128)
	EncodeSlot(reencoded, decoded)
	if string(reencoded.Bytes()) != string(buf.Bytes()) {
		t.Fatalf("wire round-trip mismatch:\n got %x\nwant %x", reencoded.Bytes(), buf.Bytes())
	}
}

func TestDecodeComponentPatchRejectsDuplicateTypes(t *testing.T) {
	buf := protocol.NewBuffer(32)
	buf.WriteVarInt(1)
	buf.WriteVarInt(1)
	buf.WriteVarInt(datacomponent.ComponentDamage)
	buf.WriteVarInt(1)
	buf.WriteVarInt(datacomponent.ComponentDamage)
	if _, err := DecodeComponentPatch(buf); err == nil {
		t.Fatal("duplicate component type was accepted")
	}
}

func TestContainerInsertMergesBeforeUsingEmptySlots(t *testing.T) {
	container := NewContainer(4)
	container.Set(0, NewSlot(5, 60))

	remainder, changed := container.Insert(NewSlot(5, 10), 64, []int{0, 1, 2})
	if !remainder.Empty() {
		t.Fatalf("remainder = %#v", remainder)
	}
	if len(changed) != 2 || changed[0] != 0 || changed[1] != 1 {
		t.Fatalf("changed slots = %v", changed)
	}
	if got := container.Get(0).ItemCount; got != 64 {
		t.Fatalf("slot 0 count = %d, want 64", got)
	}
	if got := container.Get(1).ItemCount; got != 6 {
		t.Fatalf("slot 1 count = %d, want 6", got)
	}
}

func TestContainerInsertPreservesComponentIdentity(t *testing.T) {
	container := NewContainer(2)
	first := NewSlot(5, 1)
	first.ComponentPatch.Removed = []int32{datacomponent.ComponentCustomName}
	second := NewSlot(5, 1)
	second.ComponentPatch.Removed = []int32{datacomponent.ComponentLore}
	container.Set(0, first)

	remainder, changed := container.Insert(second, 64, []int{0, 1})
	if !remainder.Empty() || len(changed) != 1 || changed[0] != 1 {
		t.Fatalf("remainder=%#v changed=%v", remainder, changed)
	}
	if got := container.Get(0).ItemCount; got != 1 {
		t.Fatalf("different component patches merged, slot 0 count = %d", got)
	}
}

func TestContainerInsertReturnsRemainderWhenFull(t *testing.T) {
	container := NewContainer(1)
	container.Set(0, NewSlot(5, 63))

	remainder, changed := container.Insert(NewSlot(5, 3), 64, nil)
	if remainder.ItemCount != 2 || len(changed) != 1 || changed[0] != 0 {
		t.Fatalf("remainder=%#v changed=%v", remainder, changed)
	}
}

func TestChatLoreAndFireworksComponentsRoundTrip(t *testing.T) {
	textNBT := []byte{nbt.TagString, 0, 4, 't', 'e', 's', 't'}
	stack := ItemStack{
		ItemCount: 1,
		ItemID:    5,
		ComponentPatch: ComponentPatch{Added: []DataComponent{
			{TypeID: datacomponent.ComponentCustomName, Value: RawNBTComponent{Data: textNBT}},
			{TypeID: datacomponent.ComponentLore, Value: LoreComponent{{Data: textNBT}}},
			{TypeID: datacomponent.ComponentFireworks, Value: FireworksComponent{
				FlightDuration: 2,
				Explosions: []FireworkExplosionComponent{{
					Shape: 1, Colors: []int32{0x112233}, FadeColors: []int32{0x445566}, Trail: true,
				}},
			}},
		}},
	}
	encoded := protocol.NewBuffer(256)
	EncodeSlot(encoded, stack)
	decoded, err := DecodeSlot(encoded)
	if err != nil {
		t.Fatalf("DecodeSlot: %v", err)
	}
	if len(decoded.ComponentPatch.Added) != 3 {
		t.Fatalf("component count = %d", len(decoded.ComponentPatch.Added))
	}
	reencoded := protocol.NewBuffer(256)
	EncodeSlot(reencoded, decoded)
	if string(reencoded.Bytes()) != string(encoded.Bytes()) {
		t.Fatalf("wire round-trip mismatch:\n got %x\nwant %x", reencoded.Bytes(), encoded.Bytes())
	}
}

func TestEncodeItemStackRejectsMismatchedComponentValue(t *testing.T) {
	buf := protocol.NewBuffer(32)
	err := EncodeItemStack(buf, ItemStack{
		ItemCount: 1,
		ItemID:    5,
		ComponentPatch: ComponentPatch{Added: []DataComponent{{
			TypeID: datacomponent.ComponentDamage,
			Value:  BoolComponent(true),
		}}},
	})
	if err == nil {
		t.Fatal("mismatched component value was encoded")
	}
	if buf.Len() != 0 {
		t.Fatalf("partial item stack was written: %x", buf.Bytes())
	}
}

func TestLegacyDamageAndEnchantmentsAreEncodedAsComponents(t *testing.T) {
	stack := NewSlot(5, 1)
	stack.Damage = 4
	stack.SetEnchant(2, 3)
	buf := protocol.NewBuffer(64)
	if err := EncodeItemStack(buf, stack); err != nil {
		t.Fatalf("EncodeItemStack: %v", err)
	}
	decoded, err := DecodeItemStack(buf)
	if err != nil {
		t.Fatalf("DecodeItemStack: %v", err)
	}
	if decoded.Damage != 4 || decoded.EnchantLevel(2) != 3 {
		t.Fatalf("decoded stack = %#v", decoded)
	}
}
