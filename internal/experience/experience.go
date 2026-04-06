package experience

// XPForLevel returns the total XP required to reach the given level from level 0.
func XPForLevel(level int32) int32 {
	switch {
	case level <= 0:
		return 0
	case level <= 16:
		return level*level + 6*level
	case level <= 31:
		return (5*level*level-81*level)/2 + 360
	default:
		return (9*level*level-325*level)/2 + 2220
	}
}

// XPToNextLevel returns the XP cost to advance from the given level to level+1.
func XPToNextLevel(level int32) int32 {
	switch {
	case level <= 0:
		return 7
	case level <= 15:
		return 2*level + 7
	case level <= 30:
		return 5*level - 38
	default:
		return 9*level - 158
	}
}

// Result holds the computed XP state after adding or setting experience.
type Result struct {
	Level int32
	Total int32
	Bar   float32
}

// AddXP computes the new level/total/bar after adding xpAmount to the current state.
func AddXP(currentLevel, currentTotal int32, xpAmount int32) Result {
	newTotal := currentTotal + xpAmount
	if newTotal < 0 {
		newTotal = 0
	}
	return FromTotal(newTotal)
}

// FromTotal computes level and bar from a total XP value.
func FromTotal(total int32) Result {
	if total <= 0 {
		return Result{Level: 0, Total: 0, Bar: 0}
	}

	level := int32(0)
	for {
		cost := XPToNextLevel(level)
		if total < XPForLevel(level+1) {
			break
		}
		level++
		if level > 21863 {
			break
		}
		_ = cost
	}

	base := XPForLevel(level)
	remaining := total - base
	cost := XPToNextLevel(level)
	bar := float32(0)
	if cost > 0 {
		bar = float32(remaining) / float32(cost)
	}
	if bar < 0 {
		bar = 0
	}
	if bar > 1 {
		bar = 1
	}

	return Result{Level: level, Total: total, Bar: bar}
}

// SetLevel computes the XP state for an exact level with zero progress.
func SetLevel(level int32) Result {
	if level < 0 {
		level = 0
	}
	total := XPForLevel(level)
	return Result{Level: level, Total: total, Bar: 0}
}

// SetLevels adds levels to the current level.
func SetLevels(currentLevel, currentTotal int32, levels int32) Result {
	newLevel := currentLevel + levels
	if newLevel < 0 {
		newLevel = 0
	}
	return SetLevel(newLevel)
}
