package inventory

import (
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/nbt"
)

// TestPersistentContainerRejectsMalformedEntries guards sparse slot validation.
func TestPersistentContainerRejectsMalformedEntries(t *testing.T) {
	valid := nbt.NewCompound().PutInt("slot", 2).PutCompound("item", nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", 1))
	for _, test := range []struct {
		name  string
		value any
		tag   byte
	}{
		{"wrong_root", nbt.NewCompound(), nbt.TagCompound},
		{"wrong_entry", nbt.NewList(nbt.TagString).Add("item"), nbt.TagList},
		{"missing_slot", nbt.NewList(nbt.TagCompound).Add(nbt.NewCompound().PutCompound("item", valid.Clone())), nbt.TagList},
		{"negative_slot", nbt.NewList(nbt.TagCompound).Add(valid.Clone().PutInt("slot", -1)), nbt.TagList},
		{"large_slot", nbt.NewList(nbt.TagCompound).Add(valid.Clone().PutInt("slot", 256)), nbt.TagList},
		{"empty_item", nbt.NewList(nbt.TagCompound).Add(valid.Clone().PutCompound("item", nbt.NewCompound())), nbt.TagList},
		{"duplicate_slot", nbt.NewList(nbt.TagCompound).Add(valid.Clone()).Add(valid.Clone()), nbt.TagList},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", 1).PutCompound("components", nbt.NewCompound().PutTag("minecraft:container", test.tag, test.value))
			if _, err := ItemStackFromNBT(root); err == nil {
				t.Fatal("malformed container component was accepted")
			}
		})
	}
}

// TestNestedStacksRetainOwnedPersistentFields checks extraction from components.
func TestNestedStacksRetainOwnedPersistentFields(t *testing.T) {
	for _, test := range []struct {
		name  string
		id    int32
		field string
		index int
	}{
		{"container", dc.ComponentContainer, "contents", 2},
		{"bundle_contents", dc.ComponentBundleContents, "contents", 0},
		{"charged_projectiles", dc.ComponentChargedProjectiles, "projectiles", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			nested := nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", 2).PutString("FutureItemField", "keep")
			list := nbt.NewList(nbt.TagCompound)
			if test.id == dc.ComponentContainer {
				list.Add(nbt.NewCompound().PutInt("slot", 2).PutCompound("item", nested))
			} else {
				list.Add(nested)
			}
			root := nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", 1).PutCompound("components", nbt.NewCompound().PutList("minecraft:"+test.name, list))
			parent, err := ItemStackFromNBT(root)
			if err != nil {
				t.Fatal(err)
			}
			component, ok := parent.Component(test.id)
			if !ok {
				t.Fatal("nested component missing")
			}
			contents := component.(StructuredComponent).Data.(map[string]any)[test.field].([]any)
			stack := contents[test.index].(ItemStack)
			encoded, err := ItemStackToNBT(stack)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := encoded.GetString("FutureItemField"); value != "keep" {
				t.Fatal("extracted stack lost its persistent fields")
			}
			stack.PersistentData.PutString("FutureItemField", "changed")
			encoded, err = ItemStackToNBT(parent)
			if err != nil {
				t.Fatal(err)
			}
			again, err := ItemStackFromNBT(encoded)
			if err != nil || !parent.StackableWith(again) {
				t.Fatalf("parent round trip changed after extracting a clone: %v", err)
			}
		})
	}
}
