package fluid

import (
	"github.com/vitismc/vitis/internal/block"
)

func CanFluidFlowInto(stateID int32, fluidType FluidType) bool {
	if stateID == 0 {
		return true
	}

	name := block.NameFromState(stateID)
	switch name {
	case "minecraft:air", "minecraft:cave_air", "minecraft:void_air":
		return true
	}

	existingFluid := GetFluidType(stateID)
	if existingFluid == fluidType {
		return true
	}

	info := block.Info(block.BlockIDFromState(stateID))
	if info == nil {
		return false
	}

	if !info.Solid {
		return true
	}

	return false
}

func CanBeReplacedByFluid(stateID int32, fluidType FluidType) bool {
	if stateID == 0 {
		return true
	}

	name := block.NameFromState(stateID)
	switch name {
	case "minecraft:air", "minecraft:cave_air", "minecraft:void_air":
		return true
	case "minecraft:fire", "minecraft:soul_fire":
		return true
	case "minecraft:grass", "minecraft:tall_grass", "minecraft:seagrass", "minecraft:tall_seagrass":
		return true
	case "minecraft:kelp", "minecraft:kelp_plant":
		return fluidType == FluidTypeWater
	}

	if IsFluid(stateID) {
		return true
	}

	info := block.Info(block.BlockIDFromState(stateID))
	if info == nil {
		return false
	}

	if !info.Solid {
		return true
	}

	return false
}

func BlocksFluidFlow(stateID int32) bool {
	if stateID == 0 {
		return false
	}

	info := block.Info(block.BlockIDFromState(stateID))
	if info == nil {
		return true
	}

	return info.Solid
}
