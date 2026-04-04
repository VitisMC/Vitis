package behavior

import (
	"github.com/vitismc/vitis/internal/block"
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

func init() {
	chestBehavior := &ContainerBehavior{WindowType: 2, Title: "Chest", SlotCount: 27}
	barrelBehavior := &ContainerBehavior{WindowType: 2, Title: "Barrel", SlotCount: 27}
	dispenserBehavior := &ContainerBehavior{WindowType: 6, Title: "Dispenser", SlotCount: 9}
	dropperBehavior := &ContainerBehavior{WindowType: 6, Title: "Dropper", SlotCount: 9}
	hopperBehavior := &ContainerBehavior{WindowType: 15, Title: "Hopper", SlotCount: 5}
	shulkerBehavior := &ContainerBehavior{WindowType: 2, Title: "Shulker Box", SlotCount: 27}

	containerMap := map[string]*ContainerBehavior{
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
