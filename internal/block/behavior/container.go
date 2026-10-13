package behavior

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/data/generated/screen"
)

// ContainerBehavior opens an inventory screen when a container block is used.
type ContainerBehavior struct {
	DefaultBehavior
	WindowType int32
	Title      string
	SlotCount  int
}

func (b *ContainerBehavior) OnUse(ctx *Context) bool {
	ctx.StateID = -1
	return true
}

// ContainerInfo returns the window type, title, and slot count for the container.
func (b *ContainerBehavior) ContainerInfo() (windowType int32, title string, slots int) {
	return b.WindowType, b.Title, b.SlotCount
}

// IsContainer checks if a block state has container behavior and returns its info.
func IsContainer(stateID int32) (windowType int32, title string, slots int, ok bool) {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return 0, "", 0, false
	}
	b := Get(bid)
	if cb, isCont := b.(*ContainerBehavior); isCont {
		wt, t, s := cb.ContainerInfo()
		return wt, t, s, true
	}
	return 0, "", 0, false
}

// IsChest returns true if the block state is a chest or trapped chest.
func IsChest(stateID int32) bool {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return false
	}
	info := block.Info(bid)
	if info == nil {
		return false
	}
	return info.Name == "minecraft:chest" || info.Name == "minecraft:trapped_chest"
}

// FindDoubleChestHalf returns the position of the other half of a double chest,
// or ok=false if this is a single chest. Chests have a "type" property:
// "single", "left", or "right".
func FindDoubleChestHalf(stateID int32, x, y, z int) (ox, oy, oz int, ok bool) {
	props := block.PropertiesFromState(stateID)
	if props == nil {
		return 0, 0, 0, false
	}
	chestType := props["type"]
	if chestType == "single" || chestType == "" {
		return 0, 0, 0, false
	}
	facing := props["facing"]
	dx, dz := 0, 0
	switch facing {
	case "north":
		if chestType == "left" {
			dx = 1
		} else {
			dx = -1
		}
	case "south":
		if chestType == "left" {
			dx = -1
		} else {
			dx = 1
		}
	case "west":
		if chestType == "left" {
			dz = -1
		} else {
			dz = 1
		}
	case "east":
		if chestType == "left" {
			dz = 1
		} else {
			dz = -1
		}
	}
	return x + dx, y, z + dz, true
}

func init() {
	chestBehavior := &ContainerBehavior{WindowType: 2, Title: "Chest", SlotCount: 27}
	barrelBehavior := &ContainerBehavior{WindowType: 2, Title: "Barrel", SlotCount: 27}
	dispenserBehavior := &ContainerBehavior{WindowType: 6, Title: "Dispenser", SlotCount: 9}
	dropperBehavior := &ContainerBehavior{WindowType: 6, Title: "Dropper", SlotCount: 9}
	hopperBehavior := &ContainerBehavior{WindowType: 16, Title: "Hopper", SlotCount: 5}
	shulkerBehavior := &ContainerBehavior{WindowType: 20, Title: "Shulker Box", SlotCount: 27}

	containerMap := map[string]*ContainerBehavior{
		"minecraft:crafting_table":         {WindowType: 12, Title: "Crafting", SlotCount: 10},
		"minecraft:stonecutter":            {WindowType: screen.ScreenStonecutter, Title: "Stonecutter", SlotCount: 2},
		"minecraft:chest":                  chestBehavior,
		"minecraft:trapped_chest":          chestBehavior,
		"minecraft:barrel":                 barrelBehavior,
		"minecraft:dispenser":              dispenserBehavior,
		"minecraft:dropper":                dropperBehavior,
		"minecraft:hopper":                 hopperBehavior,
		"minecraft:shulker_box":            shulkerBehavior,
		"minecraft:white_shulker_box":      shulkerBehavior,
		"minecraft:orange_shulker_box":     shulkerBehavior,
		"minecraft:magenta_shulker_box":    shulkerBehavior,
		"minecraft:light_blue_shulker_box": shulkerBehavior,
		"minecraft:yellow_shulker_box":     shulkerBehavior,
		"minecraft:lime_shulker_box":       shulkerBehavior,
		"minecraft:pink_shulker_box":       shulkerBehavior,
		"minecraft:gray_shulker_box":       shulkerBehavior,
		"minecraft:light_gray_shulker_box": shulkerBehavior,
		"minecraft:cyan_shulker_box":       shulkerBehavior,
		"minecraft:purple_shulker_box":     shulkerBehavior,
		"minecraft:blue_shulker_box":       shulkerBehavior,
		"minecraft:brown_shulker_box":      shulkerBehavior,
		"minecraft:green_shulker_box":      shulkerBehavior,
		"minecraft:red_shulker_box":        shulkerBehavior,
		"minecraft:black_shulker_box":      shulkerBehavior,
	}

	for name, b := range containerMap {
		registerByName(name, b)
	}
}
