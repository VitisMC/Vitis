package fluid

import (
	"github.com/vitismc/vitis/internal/block"
	genfluid "github.com/vitismc/vitis/internal/data/generated/fluid"
)

const (
	WaterSourceState  int32 = 86
	LavaSourceState   int32 = 102
	MaxFluidLevel     int   = 8
	MinFlowingLevel   int   = 1
)

type FluidType int8

const (
	FluidTypeNone FluidType = iota
	FluidTypeWater
	FluidTypeLava
)

func GetFluidType(stateID int32) FluidType {
	if stateID <= 0 {
		return FluidTypeNone
	}
	name := block.NameFromState(stateID)
	switch name {
	case "minecraft:water":
		return FluidTypeWater
	case "minecraft:lava":
		return FluidTypeLava
	}
	return FluidTypeNone
}

func IsFluid(stateID int32) bool {
	return GetFluidType(stateID) != FluidTypeNone
}

func IsWater(stateID int32) bool {
	return GetFluidType(stateID) == FluidTypeWater
}

func IsLava(stateID int32) bool {
	return GetFluidType(stateID) == FluidTypeLava
}

func FluidLevel(stateID int32) int {
	if stateID <= 0 {
		return 0
	}
	props := block.PropertiesFromState(stateID)
	if props == nil {
		return 0
	}
	levelStr, ok := props["level"]
	if !ok {
		return 0
	}
	level := 0
	for _, c := range levelStr {
		if c >= '0' && c <= '9' {
			level = level*10 + int(c-'0')
		}
	}
	return level
}

func IsSource(stateID int32) bool {
	return FluidLevel(stateID) == 0
}

func IsFalling(stateID int32) bool {
	level := FluidLevel(stateID)
	return level >= 8
}

func EffectiveLevel(stateID int32) int {
	level := FluidLevel(stateID)
	if level >= 8 {
		return 8
	}
	return 8 - level
}

func FlowingStateID(fluidType FluidType, level int, falling bool) int32 {
	if level <= 0 || level > 8 {
		return 0
	}

	var baseName string
	switch fluidType {
	case FluidTypeWater:
		baseName = "minecraft:water"
	case FluidTypeLava:
		baseName = "minecraft:lava"
	default:
		return 0
	}

	lvl := 8 - level
	if falling {
		lvl = 8 + (8 - level)
		if lvl > 15 {
			lvl = 8
		}
	}

	props := map[string]string{
		"level": levelToString(lvl),
	}
	return block.StateID(baseName, props)
}

func SourceStateID(fluidType FluidType) int32 {
	switch fluidType {
	case FluidTypeWater:
		return WaterSourceState
	case FluidTypeLava:
		return LavaSourceState
	}
	return 0
}

func levelToString(level int) string {
	if level < 0 {
		level = 0
	}
	if level > 15 {
		level = 15
	}
	if level < 10 {
		return string(rune('0' + level))
	}
	return "1" + string(rune('0'+level-10))
}

func GetFluidID(fluidType FluidType) int32 {
	switch fluidType {
	case FluidTypeWater:
		return genfluid.FluidWater
	case FluidTypeLava:
		return genfluid.FluidLava
	}
	return genfluid.FluidEmpty
}
