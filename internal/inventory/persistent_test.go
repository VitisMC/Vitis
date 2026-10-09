package inventory

import (
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"testing"
)

// TestPersistentStackMovesMetadata verifies that item metadata follows the stack.
func TestPersistentStackMovesMetadata(t *testing.T) {
	raw := nbt.NewCompound().PutString("id", "minecraft:diamond_sword").PutInt("count", 1).PutByte("Slot", 0).PutString("CustomTag", "keep").PutCompound("components", nbt.NewCompound().PutString("minecraft:custom_name", `"Blade"`).PutInt("minecraft:damage", 7).PutCompound("!minecraft:lore", nbt.NewCompound()))
	stack, err := ItemStackFromNBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	window := NewWindowManager()
	window.Inventory().Set(36, stack)
	window.HandleClick(0, window.CurrentStateID(), 36, 0, 0, nil, EmptySlot())
	window.HandleClick(0, window.CurrentStateID(), 37, 0, 0, nil, EmptySlot())
	moved := window.Inventory().Get(37)
	encoded, err := ItemStackToNBT(moved)
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := encoded.GetString("CustomTag"); name != "keep" {
		t.Fatal("unknown tag lost")
	}
	if _, _, ok := encoded.Get("Slot"); ok {
		t.Fatal("slot identity leaked into stack")
	}
	again, err := ItemStackFromNBT(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !moved.StackableWith(again) {
		t.Fatal("round trip changed component semantics")
	}
	clone := moved.Clone()
	clone.PersistentData.PutString("CustomTag", "other")
	if moved.StackableWith(clone) {
		t.Fatal("different persistent data merged")
	}
}

// TestEffectiveStackSizeUsesPatch verifies overrides and explicit removals.
func TestEffectiveStackSizeUsesPatch(t *testing.T) {
	s := NewSlot(item.IDByName("minecraft:stone"), 1)
	s.SetComponent(dc.ComponentMaxStackSize, VarIntComponent(16))
	if s.MaxStackSize() != 16 {
		t.Fatal("override ignored")
	}
	s.ComponentPatch.Added = nil
	s.ComponentPatch.Removed = []int32{dc.ComponentMaxStackSize}
	if s.MaxStackSize() != 1 {
		t.Fatal("removal ignored")
	}
}

// TestVanillaDefaultComponentCodecs verifies every default has executable codecs.
func TestVanillaDefaultComponentCodecs(t *testing.T) {
	for name, definition := range defaultDefinitions {
		for component, raw := range definition.Components {
			t.Run(name+"/"+component, func(t *testing.T) {
				id := dc.DataComponentIDByName(component)
				tag, value, err := nbt.FromJSON(raw)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := componentFromNBT(id, tag, value, 0)
				if err != nil {
					t.Fatal(err)
				}
				buf := protocol.NewBuffer(128)
				if err := EncodeComponentPatch(buf, ComponentPatch{Added: []DataComponent{{TypeID: id, Value: decoded}}}); err != nil {
					t.Fatal(err)
				}
				patch, err := DecodeComponentPatch(buf)
				if err != nil || buf.Remaining() != 0 {
					t.Fatalf("wire round trip: %v", err)
				}
				tag, value, err = componentToNBT(id, patch.Added[0].Value, 0)
				if err != nil {
					t.Fatal(err)
				}
				assertPersistentFixture(t, string(raw), value)
				if _, err := componentFromNBT(id, tag, value, 0); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// TestUnknownNestedComponentFieldsFollowStacks preserves future fields in list entries.
func TestUnknownNestedComponentFieldsFollowStacks(t *testing.T) {
	raw := nbt.NewCompound().PutString("id", "minecraft:potion").PutInt("count", 1).PutCompound("components", nbt.NewCompound().PutCompound("minecraft:potion_contents", nbt.NewCompound().PutList("custom_effects", nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutString("id", "minecraft:speed").PutInt("duration", 100).PutString("FutureEffect", "preserved")))))
	stack, err := ItemStackFromNBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := ItemStackToNBT(stack)
	if err != nil {
		t.Fatal(err)
	}
	components, _ := encoded.GetCompound("components")
	potion, _ := components.GetCompound("minecraft:potion_contents")
	list, _ := potion.GetList("custom_effects")
	field, _ := list.Elements()[0].(*nbt.Compound).GetString("FutureEffect")
	if field != "preserved" {
		t.Fatal("unknown effect field lost")
	}
	different := stack.Clone()
	comps, _ := different.PersistentData.GetCompound("components")
	pot, _ := comps.GetCompound("minecraft:potion_contents")
	entries, _ := pot.GetList("custom_effects")
	entries.Elements()[0].(*nbt.Compound).PutString("FutureEffect", "different")
	if stack.StackableWith(different) {
		t.Fatal("unknown nested fields merged")
	}
}

// FuzzPersistentItemStack verifies bounded malformed NBT cannot panic item codecs.
func FuzzPersistentItemStack(f *testing.F) {
	for _, fixtures := range []map[string]string{componentPersistentFixtures(), inlineComponentFixtures()} {
		for name, fixture := range fixtures {
			tag, value, err := nbt.FromJSON([]byte(fixture))
			if err != nil {
				f.Fatal(err)
			}
			component := nbt.NewCompound()
			component.PutTag("minecraft:"+name, tag, value)
			root := nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", 1).PutCompound("components", component)
			encoder := nbt.NewEncoder(256)
			if err := encoder.WriteRootCompound(root); err != nil {
				f.Fatal(err)
			}
			f.Add(encoder.Bytes())
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			return
		}
		root, err := nbt.NewDecoder(data).ReadRootCompound()
		if err != nil {
			return
		}
		stack, err := ItemStackFromNBT(root)
		if err != nil {
			return
		}
		encoded, err := ItemStackToNBT(stack)
		if err != nil {
			t.Fatalf("valid persistent stack failed to encode: %v", err)
		}
		again, err := ItemStackFromNBT(encoded)
		if err != nil || !stack.StackableWith(again) {
			t.Fatalf("persistent stack changed after round trip: %v; before=%#v after=%#v", err, stack, again)
		}
	})
}

// TestRemovedDamageOverridesLegacyFields preserves explicit component removals.
func TestRemovedDamageOverridesLegacyFields(t *testing.T) {
	raw := nbt.NewCompound().PutString("id", "minecraft:diamond_sword").PutInt("Damage", 42).PutCompound("components", nbt.NewCompound().PutCompound("!minecraft:damage", nbt.NewCompound()))
	stack, err := ItemStackFromNBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stack.Component(dc.ComponentDamage); ok {
		t.Fatal("legacy damage overrode removal")
	}
	result, err := ItemStackToNBT(stack)
	if err != nil {
		t.Fatal(err)
	}
	components, _ := result.GetCompound("components")
	if _, ok := components.GetCompound("!minecraft:damage"); !ok {
		t.Fatal("removal was not persisted")
	}
}

// TestPersistentStackRejectsMalformedOwnedFields avoids silently replacing stored values.
func TestPersistentStackRejectsMalformedOwnedFields(t *testing.T) {
	for _, root := range []*nbt.Compound{
		nbt.NewCompound().PutString("id", "minecraft:stone").PutString("count", "bad"),
		nbt.NewCompound().PutString("id", "minecraft:stone").PutString("components", "bad"),
	} {
		if _, err := ItemStackFromNBT(root); err == nil {
			t.Fatal("malformed owned field accepted")
		}
	}
}
