package main

import "github.com/vitismc/vitis/internal/world/worldgen"

// dimensionSeaLevelReader exposes existing dimension settings for aquatic spawn checks.
func dimensionSeaLevelReader(runtime *worldgen.Runtime, dimensionID string) func() (int, bool) {
	return func() (int, bool) {
		if runtime == nil {
			return 0, false
		}
		return runtime.DimensionSeaLevel(dimensionID)
	}
}
