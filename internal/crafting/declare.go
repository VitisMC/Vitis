package crafting

import (
	"slices"

	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// DeclarePacket provides synchronized ingredient properties and stonecutter displays.
func (r *Runtime) DeclarePacket() (*play.DeclareRecipes, error) {
	set := r.current()
	buffer := protocol.NewBuffer(2048)
	properties := map[string][]int32{"minecraft:furnace_input": {}, "minecraft:blast_furnace_input": {}, "minecraft:smoker_input": {}, "minecraft:campfire_input": {}, "minecraft:stonecutter_input": {}}
	if set != nil {
		for _, recipe := range set.cooking {
			key := "minecraft:furnace_input"
			switch recipe.Type {
			case "minecraft:blasting":
				key = "minecraft:blast_furnace_input"
			case "minecraft:smoking":
				key = "minecraft:smoker_input"
			case "minecraft:campfire_cooking":
				key = "minecraft:campfire_input"
			}
			properties[key] = append(properties[key], recipe.Ingredient.ids...)
		}
		for _, recipe := range set.stonecutting {
			properties["minecraft:stonecutter_input"] = append(properties["minecraft:stonecutter_input"], recipe.Ingredient.ids...)
		}
	}
	names := []string{"minecraft:furnace_input", "minecraft:blast_furnace_input", "minecraft:smoker_input", "minecraft:campfire_input", "minecraft:stonecutter_input"}
	buffer.WriteVarInt(int32(len(names)))
	for _, name := range names {
		ids := properties[name]
		slices.Sort(ids)
		ids = slices.Compact(ids)
		buffer.WriteString(name)
		buffer.WriteVarInt(int32(len(ids)))
		for _, id := range ids {
			buffer.WriteVarInt(id)
		}
	}
	if set == nil {
		buffer.WriteVarInt(0)
	} else {
		buffer.WriteVarInt(int32(len(set.stonecutting)))
		for _, recipe := range set.stonecutting {
			buffer.WriteVarInt(int32(len(recipe.Ingredient.ids) + 1))
			for _, id := range recipe.Ingredient.ids {
				buffer.WriteVarInt(id)
			}
			display := play.SlotDisplay{Type: 3, Stack: recipe.OutputStack()}
			if err := display.Encode(buffer); err != nil {
				return nil, err
			}
		}
	}
	return &play.DeclareRecipes{Data: buffer.Bytes()}, nil
}
