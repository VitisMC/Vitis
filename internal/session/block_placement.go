package session

import (
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/item"
)

// resolveDirectionalBlockState maps an item ID to a block state ID,
// applying directional properties based on the placement face and player yaw.
func resolveDirectionalBlockState(itemID int32, face int32, yaw float32) int32 {
	itemName := item.NameByID(itemID)
	if itemName == "" {
		return 0
	}

	info := block.InfoByName(itemName)
	if info == nil {
		return 0
	}
	if info.DefaultState < 0 {
		return 0
	}

	if len(info.Properties) == 0 {
		return info.DefaultState
	}

	props := make(map[string]string)

	hasFacing := false
	hasAxis := false
	hasHalf := false
	for _, p := range info.Properties {
		switch p.Name {
		case "facing":
			hasFacing = true
		case "axis":
			hasAxis = true
		case "half", "type":
			hasHalf = true
		}
	}

	if hasFacing {
		props["facing"] = yawToFacing(yaw)
	}

	if hasAxis {
		props["axis"] = faceToAxis(face)
	}

	if hasHalf {
		if face == 0 {
			props["half"] = "top"
		}
	}

	stateID := block.StateID(itemName, props)
	if stateID < 0 {
		return info.DefaultState
	}
	return stateID
}

func yawToFacing(yaw float32) string {
	y := float64(yaw)
	for y < 0 {
		y += 360
	}
	for y >= 360 {
		y -= 360
	}
	switch {
	case y >= 315 || y < 45:
		return "south"
	case y >= 45 && y < 135:
		return "west"
	case y >= 135 && y < 225:
		return "north"
	default:
		return "east"
	}
}

func faceToAxis(face int32) string {
	switch face {
	case 0, 1:
		return "y"
	case 2, 3:
		return "z"
	case 4, 5:
		return "x"
	default:
		return "y"
	}
}

// isWoodBlock returns true if the block name suggests wood material.
func isWoodBlock(name string) bool {
	return strings.Contains(name, "wood") || strings.Contains(name, "planks") || strings.Contains(name, "log")
}
