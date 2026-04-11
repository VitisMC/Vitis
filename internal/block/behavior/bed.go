package behavior

import (
	"strings"

	"github.com/vitismc/vitis/internal/block"
)

// BedBehavior handles right-click on bed blocks.
// OnUse sets ctx.StateID to -2 to signal bed interaction to the session handler.
type BedBehavior struct {
	DefaultBehavior
}

func (b *BedBehavior) OnUse(ctx *Context) bool {
	ctx.StateID = -2
	return true
}

func (b *BedBehavior) OnBreak(ctx *Context) []Drop {
	if ctx.PlayerGM == 1 {
		return nil
	}
	bid := block.BlockIDFromState(ctx.StateID)
	if bid <= 0 {
		return nil
	}
	info := block.Info(bid)
	if info == nil {
		return nil
	}
	itemID := itemIDFromBlockName(info.Name)
	if itemID <= 0 {
		return nil
	}
	return []Drop{{ItemID: itemID, Count: 1}}
}

// IsBed returns true if the block state is a bed, along with the bed's
// head position offsets based on facing and part properties.
func IsBed(stateID int32) bool {
	bid := block.BlockIDFromState(stateID)
	if bid < 0 {
		return false
	}
	info := block.Info(bid)
	if info == nil {
		return false
	}
	return strings.HasSuffix(info.Name, "_bed")
}

// BedHeadPos returns the head position of a bed given the clicked position
// and the block state. If the clicked part is already the head, it returns
// the same position. Otherwise it offsets based on facing direction.
func BedHeadPos(stateID int32, x, y, z int) (hx, hy, hz int) {
	props := block.PropertiesFromState(stateID)
	if props == nil {
		return x, y, z
	}
	if props["part"] == "head" {
		return x, y, z
	}
	// This is the foot part; offset toward the head based on facing
	switch props["facing"] {
	case "north":
		return x, y, z - 1
	case "south":
		return x, y, z + 1
	case "west":
		return x - 1, y, z
	case "east":
		return x + 1, y, z
	}
	return x, y, z
}

// BedOccupiedState returns a new state ID with occupied=true for the given bed state.
func BedOccupiedState(stateID int32, occupied bool) int32 {
	props := block.PropertiesFromState(stateID)
	if props == nil {
		return stateID
	}
	if occupied {
		props["occupied"] = "true"
	} else {
		props["occupied"] = "false"
	}
	bid := block.BlockIDFromState(stateID)
	info := block.Info(bid)
	if info == nil {
		return stateID
	}
	return block.StateID(info.Name, props)
}

// BedOtherHalfPos returns the position of the other half of the bed.
func BedOtherHalfPos(stateID int32, x, y, z int) (ox, oy, oz int) {
	props := block.PropertiesFromState(stateID)
	if props == nil {
		return x, y, z
	}
	facing := props["facing"]
	dx, dz := 0, 0
	switch facing {
	case "north":
		dz = -1
	case "south":
		dz = 1
	case "west":
		dx = -1
	case "east":
		dx = 1
	}
	if props["part"] == "head" {
		// Head → foot is opposite direction
		return x - dx, y, z - dz
	}
	// Foot → head
	return x + dx, y, z + dz
}

func init() {
	bedBehavior := &BedBehavior{}
	bedNames := []string{
		"minecraft:white_bed", "minecraft:orange_bed", "minecraft:magenta_bed",
		"minecraft:light_blue_bed", "minecraft:yellow_bed", "minecraft:lime_bed",
		"minecraft:pink_bed", "minecraft:gray_bed", "minecraft:light_gray_bed",
		"minecraft:cyan_bed", "minecraft:purple_bed", "minecraft:blue_bed",
		"minecraft:brown_bed", "minecraft:green_bed", "minecraft:red_bed",
		"minecraft:black_bed",
	}
	for _, name := range bedNames {
		registerByName(name, bedBehavior)
	}
}
