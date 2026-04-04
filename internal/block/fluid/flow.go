package fluid

type FlowConfig struct {
	FluidType          FluidType
	LevelDecreasePerBlock int
	FlowSpeed          int
	MaxFlowDistance    int
	CanFormSource      bool
}

var WaterConfig = FlowConfig{
	FluidType:          FluidTypeWater,
	LevelDecreasePerBlock: 1,
	FlowSpeed:          5,
	MaxFlowDistance:    4,
	CanFormSource:      true,
}

var LavaOverworldConfig = FlowConfig{
	FluidType:          FluidTypeLava,
	LevelDecreasePerBlock: 2,
	FlowSpeed:          30,
	MaxFlowDistance:    3,
	CanFormSource:      false,
}

var LavaNetherConfig = FlowConfig{
	FluidType:          FluidTypeLava,
	LevelDecreasePerBlock: 1,
	FlowSpeed:          10,
	MaxFlowDistance:    5,
	CanFormSource:      false,
}

type FlowResult struct {
	NewStates    map[[3]int]int32
	BlockUpdates [][3]int
}

func ComputeFlow(x, y, z int, config FlowConfig, getBlock BlockGetter) *FlowResult {
	result := &FlowResult{
		NewStates:    make(map[[3]int]int32),
		BlockUpdates: make([][3]int, 0, 8),
	}

	currentState := getBlock(x, y, z)
	if !IsFluid(currentState) {
		return result
	}

	currentLevel := EffectiveLevel(currentState)
	isSource := IsSource(currentState)

	belowState := getBlock(x, y-1, z)
	if CanBeReplacedByFluid(belowState, config.FluidType) {
		fallingState := FlowingStateID(config.FluidType, 8, true)
		if fallingState > 0 {
			pos := [3]int{x, y - 1, z}
			result.NewStates[pos] = fallingState
			result.BlockUpdates = append(result.BlockUpdates, pos)
		}

		if isSource && config.CanFormSource {
			flowToSides(x, y, z, currentLevel, config, getBlock, result)
		}
		return result
	}

	flowToSides(x, y, z, currentLevel, config, getBlock, result)
	return result
}

func flowToSides(x, y, z int, currentLevel int, config FlowConfig, getBlock BlockGetter, result *FlowResult) {
	newLevel := currentLevel - config.LevelDecreasePerBlock
	if newLevel <= 0 {
		return
	}

	directions := FindFlowDirections(x, y, z, config.FluidType, config.MaxFlowDistance, getBlock)

	if len(directions) == 0 {
		for _, dir := range AllHorizontalDirs {
			dx, _, dz := dir.Offset()
			nx, nz := x+dx, z+dz

			sideState := getBlock(nx, y, nz)
			if !CanBeReplacedByFluid(sideState, config.FluidType) {
				continue
			}

			existingFluid := GetFluidType(sideState)
			if existingFluid == config.FluidType {
				existingLevel := EffectiveLevel(sideState)
				if existingLevel >= newLevel {
					continue
				}
			}

			flowState := FlowingStateID(config.FluidType, newLevel, false)
			if flowState > 0 {
				pos := [3]int{nx, y, nz}
				result.NewStates[pos] = flowState
				result.BlockUpdates = append(result.BlockUpdates, pos)
			}
		}
		return
	}

	for _, dir := range directions {
		dx, _, dz := dir.Offset()
		nx, nz := x+dx, z+dz

		sideState := getBlock(nx, y, nz)
		existingFluid := GetFluidType(sideState)
		if existingFluid == config.FluidType {
			existingLevel := EffectiveLevel(sideState)
			if existingLevel >= newLevel {
				continue
			}
		}

		flowState := FlowingStateID(config.FluidType, newLevel, false)
		if flowState > 0 {
			pos := [3]int{nx, y, nz}
			result.NewStates[pos] = flowState
			result.BlockUpdates = append(result.BlockUpdates, pos)
		}
	}
}

func ComputeNewFluidState(x, y, z int, config FlowConfig, getBlock BlockGetter) int32 {
	currentState := getBlock(x, y, z)
	if IsSource(currentState) {
		return currentState
	}

	aboveState := getBlock(x, y+1, z)
	if GetFluidType(aboveState) == config.FluidType {
		return FlowingStateID(config.FluidType, 8, true)
	}

	highestLevel := 0
	sourceCount := 0

	for _, dir := range AllHorizontalDirs {
		dx, _, dz := dir.Offset()
		nx, nz := x+dx, z+dz

		neighborState := getBlock(nx, y, nz)
		if GetFluidType(neighborState) != config.FluidType {
			continue
		}

		if IsSource(neighborState) {
			sourceCount++
			highestLevel = 8
		} else {
			level := EffectiveLevel(neighborState)
			if IsFalling(neighborState) {
				level = 8
			}
			if level > highestLevel {
				highestLevel = level
			}
		}
	}

	if config.CanFormSource && sourceCount >= 2 {
		belowState := getBlock(x, y-1, z)
		belowFluid := GetFluidType(belowState)
		belowIsSource := belowFluid == config.FluidType && IsSource(belowState)
		belowIsSolid := BlocksFluidFlow(belowState)

		if belowIsSource || belowIsSolid {
			return SourceStateID(config.FluidType)
		}
	}

	newLevel := highestLevel - config.LevelDecreasePerBlock
	if newLevel <= 0 {
		return 0
	}

	return FlowingStateID(config.FluidType, newLevel, false)
}

func CheckLavaWaterInteraction(x, y, z int, getBlock BlockGetter) (resultState int32, hasInteraction bool) {
	currentState := getBlock(x, y, z)
	currentFluid := GetFluidType(currentState)

	if currentFluid == FluidTypeLava {
		for _, dir := range AllHorizontalDirs {
			dx, _, dz := dir.Offset()
			nx, nz := x+dx, z+dz
			neighborState := getBlock(nx, y, nz)
			if GetFluidType(neighborState) == FluidTypeWater {
				if IsSource(currentState) {
					return 2270, true
				}
				return 21, true
			}
		}

		aboveState := getBlock(x, y+1, z)
		if GetFluidType(aboveState) == FluidTypeWater {
			return 2270, true
		}

		belowState := getBlock(x, y-1, z)
		if GetFluidType(belowState) == FluidTypeWater {
			if IsFalling(currentState) {
				return 1, true
			}
		}
	}

	if currentFluid == FluidTypeWater {
		for _, dir := range AllHorizontalDirs {
			dx, _, dz := dir.Offset()
			nx, nz := x+dx, z+dz
			neighborState := getBlock(nx, y, nz)
			if GetFluidType(neighborState) == FluidTypeLava {
				if IsSource(neighborState) {
					return 0, false
				}
			}
		}
	}

	return 0, false
}
