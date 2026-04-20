package behavior

import (
	"strings"

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

// IsDoor returns true if the block state is a door (not trapdoor).
func IsDoor(stateID int32) bool {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return false
	}
	info := block.Info(bid)
	if info == nil {
		return false
	}
	return strings.HasSuffix(info.Name, "_door") && !strings.Contains(info.Name, "trapdoor")
}

// IsFenceGate returns true if the block state is a fence gate.
func IsFenceGate(stateID int32) bool {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return false
	}
	info := block.Info(bid)
	if info == nil {
		return false
	}
	return strings.HasSuffix(info.Name, "_fence_gate")
}

// FindAdjacentDoor returns the position and toggled state of an adjacent door
// that should be synced when the door at (x,y,z) is toggled.
// Returns ok=false if no adjacent door needs toggling.
func FindAdjacentDoor(stateID int32, x, y, z int, getBlock func(int, int, int) int32) (ax, ay, az int, newState int32, ok bool) {
	props := block.PropertiesFromState(stateID)
	if props == nil {
		return 0, 0, 0, 0, false
	}

	facing := props["facing"]
	hinge := props["hinge"]
	openVal := props["open"]

	// Determine offset to adjacent door based on facing and hinge
	var dx, dz int
	switch facing {
	case "north":
		if hinge == "left" {
			dx = -1
		} else {
			dx = 1
		}
	case "south":
		if hinge == "left" {
			dx = 1
		} else {
			dx = -1
		}
	case "west":
		if hinge == "left" {
			dz = 1
		} else {
			dz = -1
		}
	case "east":
		if hinge == "left" {
			dz = -1
		} else {
			dz = 1
		}
	}

	nx, ny, nz := x+dx, y, z+dz
	neighborState := getBlock(nx, ny, nz)
	if neighborState <= 0 || !IsDoor(neighborState) {
		return 0, 0, 0, 0, false
	}

	neighborProps := block.PropertiesFromState(neighborState)
	if neighborProps == nil {
		return 0, 0, 0, 0, false
	}

	// Must face same direction, have opposite hinge
	if neighborProps["facing"] != facing {
		return 0, 0, 0, 0, false
	}
	oppositeHinge := "right"
	if hinge == "right" {
		oppositeHinge = "left"
	}
	if neighborProps["hinge"] != oppositeHinge {
		return 0, 0, 0, 0, false
	}

	// Toggle the neighbor to match the new open state
	neighborProps["open"] = openVal
	nInfo := block.Info(block.BlockIDFromState(neighborState))
	if nInfo == nil {
		return 0, 0, 0, 0, false
	}
	newState = block.StateID(nInfo.Name, neighborProps)
	return nx, ny, nz, newState, true
}
