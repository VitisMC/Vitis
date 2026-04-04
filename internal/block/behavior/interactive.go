package behavior

import (
	"github.com/vitismc/vitis/internal/block"
)

// ToggleOpenBehavior toggles the "open" property on use (doors, trapdoors, fence gates).
type ToggleOpenBehavior struct {
	DefaultBehavior
}

func (b *ToggleOpenBehavior) OnUse(ctx *Context) bool {
	props := block.PropertiesFromState(ctx.StateID)
	if props == nil {
		return false
	}
	openVal, ok := props["open"]
	if !ok {
		return false
	}
	if openVal == "true" {
		props["open"] = "false"
	} else {
		props["open"] = "true"
	}
	info := block.Info(block.BlockIDFromState(ctx.StateID))
	if info == nil {
		return false
	}
	ctx.StateID = block.StateID(info.Name, props)
	return true
}

// TogglePoweredBehavior toggles the "powered" property on use (buttons, levers).
type TogglePoweredBehavior struct {
	DefaultBehavior
}

func (b *TogglePoweredBehavior) OnUse(ctx *Context) bool {
	props := block.PropertiesFromState(ctx.StateID)
	if props == nil {
		return false
	}
	powVal, ok := props["powered"]
	if !ok {
		return false
	}
	if powVal == "true" {
		props["powered"] = "false"
	} else {
		props["powered"] = "true"
	}
	info := block.Info(block.BlockIDFromState(ctx.StateID))
	if info == nil {
		return false
	}
	ctx.StateID = block.StateID(info.Name, props)
	return true
}

func init() {
	toggleOpen := &ToggleOpenBehavior{}
	togglePowered := &TogglePoweredBehavior{}

	doorNames := []string{
		"minecraft:oak_door", "minecraft:spruce_door", "minecraft:birch_door",
		"minecraft:jungle_door", "minecraft:acacia_door", "minecraft:cherry_door",
		"minecraft:dark_oak_door", "minecraft:pale_oak_door", "minecraft:mangrove_door",
		"minecraft:bamboo_door", "minecraft:iron_door", "minecraft:crimson_door",
		"minecraft:warped_door", "minecraft:copper_door", "minecraft:exposed_copper_door",
		"minecraft:weathered_copper_door", "minecraft:oxidized_copper_door",
		"minecraft:waxed_copper_door", "minecraft:waxed_exposed_copper_door",
		"minecraft:waxed_weathered_copper_door", "minecraft:waxed_oxidized_copper_door",
	}

	trapdoorNames := []string{
		"minecraft:oak_trapdoor", "minecraft:spruce_trapdoor", "minecraft:birch_trapdoor",
		"minecraft:jungle_trapdoor", "minecraft:acacia_trapdoor", "minecraft:cherry_trapdoor",
		"minecraft:dark_oak_trapdoor", "minecraft:pale_oak_trapdoor", "minecraft:mangrove_trapdoor",
		"minecraft:bamboo_trapdoor", "minecraft:iron_trapdoor", "minecraft:crimson_trapdoor",
		"minecraft:warped_trapdoor", "minecraft:copper_trapdoor", "minecraft:exposed_copper_trapdoor",
		"minecraft:weathered_copper_trapdoor", "minecraft:oxidized_copper_trapdoor",
		"minecraft:waxed_copper_trapdoor", "minecraft:waxed_exposed_copper_trapdoor",
		"minecraft:waxed_weathered_copper_trapdoor", "minecraft:waxed_oxidized_copper_trapdoor",
	}

	fenceGateNames := []string{
		"minecraft:oak_fence_gate", "minecraft:spruce_fence_gate", "minecraft:birch_fence_gate",
		"minecraft:jungle_fence_gate", "minecraft:acacia_fence_gate", "minecraft:cherry_fence_gate",
		"minecraft:dark_oak_fence_gate", "minecraft:pale_oak_fence_gate", "minecraft:mangrove_fence_gate",
		"minecraft:bamboo_fence_gate", "minecraft:crimson_fence_gate", "minecraft:warped_fence_gate",
	}

	for _, name := range doorNames {
		registerByName(name, toggleOpen)
	}
	for _, name := range trapdoorNames {
		registerByName(name, toggleOpen)
	}
	for _, name := range fenceGateNames {
		registerByName(name, toggleOpen)
	}

	leverName := "minecraft:lever"
	registerByName(leverName, togglePowered)

	buttonNames := []string{
		"minecraft:stone_button", "minecraft:oak_button", "minecraft:spruce_button",
		"minecraft:birch_button", "minecraft:jungle_button", "minecraft:acacia_button",
		"minecraft:cherry_button", "minecraft:dark_oak_button", "minecraft:pale_oak_button",
		"minecraft:mangrove_button", "minecraft:bamboo_button", "minecraft:crimson_button",
		"minecraft:warped_button", "minecraft:polished_blackstone_button",
	}
	for _, name := range buttonNames {
		registerByName(name, togglePowered)
	}
}

func registerByName(name string, b Behavior) {
	info := block.InfoByName(name)
	if info != nil {
		Register(info.ID, b)
	}
}
