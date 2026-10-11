package spawning

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/entity"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// moonBrightness returns the eight-day brightness cycle, including negative day times.
func moonBrightness(dayTime int64) float32 {
	phase := (dayTime/24000%8 + 8) % 8
	return [...]float32{1, .75, .5, .25, 0, .25, .5, .75}[phase]
}

// canPlaceSlime checks moon-dependent surface spawning and seeded underground slime chunks.
func canPlaceSlime(environment Environment, pos entity.Vec3, rng *rand.Rand, definitions *spawnDefinitions, brightness uint8) bool {
	settings := environment.SpawnSettings()
	if settings.Peaceful {
		return false
	}
	x, y, z := int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z))
	biome := environment.SpawnBiome(x, y, z)
	if definitions != nil && definitions.biomeTags["allows_surface_slime_spawns"][biome] && y > 50 && y < 70 && rng.Float32() < .5 && rng.Float32() < moonBrightness(settings.DayTime) && brightness <= uint8(rng.Intn(8)) {
		return true
	}
	slimeChunk := worldrandom.SlimeChunkSource(int32(x>>4), int32(z>>4), settings.Seed, 987234911).NextIntBound(10) == 0
	return rng.Intn(10) == 0 && slimeChunk && y < 40
}

// naturalSlimeSize chooses sizes one, two or four using the saved local difficulty.
func naturalSlimeSize(rng *rand.Rand, settings Settings, inhabitedTime int64) int {
	level := rng.Intn(3)
	if level < 2 && rng.Float32() < .5*specialDifficulty(settings, inhabitedTime) {
		level++
	}
	return 1 << level
}

// specialDifficulty computes the vanilla local-difficulty multiplier with bounded inputs.
func specialDifficulty(settings Settings, inhabitedTime int64) float32 {
	return max(float32(0), min(float32(1), (EffectiveDifficulty(settings, inhabitedTime)-2)/2))
}

// EffectiveDifficulty computes regional difficulty from world age and saved habitation.
func EffectiveDifficulty(settings Settings, inhabitedTime int64) float32 {
	if settings.Peaceful || settings.Difficulty <= 0 {
		return 0
	}
	global := max(float32(0), min(float32(1), (float32(settings.WorldAge)-72000)/1440000)) * .25
	local := max(float32(0), min(float32(1), float32(inhabitedTime)/3600000))
	if settings.Difficulty != 3 {
		local *= .75
	}
	local += max(float32(0), min(global, moonBrightness(settings.DayTime)*.25))
	if settings.Difficulty == 1 {
		local *= .5
	}
	return float32(min(3, settings.Difficulty)) * (.75 + global + local)
}
