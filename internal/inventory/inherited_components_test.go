package inventory

import (
	"reflect"
	"testing"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/item"
)

// TestPersistentEmptyInheritedEnchantmentsDoNotCreatePatches preserves implicit and explicit stack identity.
func TestPersistentEmptyInheritedEnchantmentsDoNotCreatePatches(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		stack := NewSlot(item.IDByName("minecraft:arrow"), 1)
		if explicit {
			stack.SetComponent(dc.ComponentEnchantments, EnchantmentsComponent{Levels: map[int32]int32{}, ShowInTooltip: true})
		}
		raw, err := ItemStackToNBT(stack)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := ItemStackFromNBT(raw)
		if err != nil {
			t.Fatal(err)
		}
		again, err := ItemStackToNBT(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(raw, again) || !stack.StackableWith(decoded) {
			t.Fatalf("inherited enchantments became an explicit patch: explicit=%v", explicit)
		}
	}
}
