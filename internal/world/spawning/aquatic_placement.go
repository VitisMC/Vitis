package spawning

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
)

// darkEnough checks dimension-specific block, sky and local brightness requirements.
func darkEnough(block, sky, brightness uint8, rules DimensionRules, rng *rand.Rand) bool {
	if rules.SkyLight && sky > uint8(rng.Intn(32)) {
		return false
	}
	return int(block) <= rules.BlockLightLimit && int(brightness) <= rules.LightMin+rng.Intn(rules.LightMax-rules.LightMin+1)
}

// canPlaceDrowned checks water placement, darkness, biome frequency and sea depth.
func canPlaceDrowned(environment Environment, pos entity.Vec3, rules DimensionRules, rng *rand.Rand, definitions *spawnDefinitions, block, sky, brightness uint8) bool {
	x, y, z := int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z))
	if fluid.ContainedFluidType(environment.GetBlockStateAt(x, y, z)) != fluid.FluidTypeWater || fluid.ContainedFluidType(environment.GetBlockStateAt(x, y-1, z)) != fluid.FluidTypeWater || !darkEnough(block, sky, brightness, rules, rng) {
		return false
	}
	biome := environment.SpawnBiome(x, y, z)
	if definitions != nil && definitions.biomeTags["more_frequent_drowned_spawns"][biome] {
		return rng.Intn(15) == 0
	}
	settings := environment.SpawnSettings()
	return settings.SeaLevelKnown && y < settings.SeaLevel-5 && rng.Intn(40) == 0
}
