package loot

import (
	"encoding/json"
	"fmt"
	"strings"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
)

// UnmarshalJSON compiles component patches with the existing persistent stack codec.
func (fn *Function) UnmarshalJSON(data []byte) error {
	type definition Function
	var value definition
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*fn = Function(value)
	if fn.Function != "minecraft:set_components" {
		return nil
	}
	for name := range fn.Components {
		if dc.DataComponentIDByName(strings.TrimPrefix(name, "!")) < 0 {
			return fmt.Errorf("unknown loot component %q", name)
		}
	}
	raw, err := json.Marshal(fn.Components)
	if err != nil {
		return err
	}
	tag, components, err := nbt.FromJSON(raw)
	if err != nil {
		return err
	}
	if tag != nbt.TagCompound {
		return fmt.Errorf("loot components must be an object")
	}
	root := nbt.NewCompound()
	root.PutString("id", "minecraft:stone")
	root.PutCompound("components", components.(*nbt.Compound))
	fn.componentPatch, err = inventory.ItemStackFromNBT(root)
	return err
}
