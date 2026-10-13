package worldgen

import (
	"strconv"

	"github.com/vitismc/vitis/internal/block"
	generatedblock "github.com/vitismc/vitis/internal/data/generated/block"
)

// leafDistanceStates resolves immutable registry property transitions outside tree placement.
var leafDistanceStates = buildLeafDistanceStates()

// buildLeafDistanceStates includes every registered distance-bearing block, including custom providers.
func buildLeafDistanceStates() [generatedblock.TotalStates][7]int32 {
	var values [generatedblock.TotalStates][7]int32
	for state := range values {
		for distance := range values[state] {
			values[state][distance] = -1
		}
	}
	for _, info := range generatedblock.Blocks {
		hasDistance := false
		for _, property := range info.Properties {
			hasDistance = hasDistance || property.Name == "distance"
		}
		if !hasDistance {
			continue
		}
		for id := info.MinStateID; id <= info.MaxStateID; id++ {
			state, err := block.BlockStateFromID(id)
			if err != nil {
				continue
			}
			for distance := 1; distance < 7; distance++ {
				updated, err := state.WithProperty("distance", strconv.Itoa(distance))
				if err == nil {
					values[id][distance] = updated.ID()
				}
			}
		}
	}
	return values
}
