package worldgen

import (
	"math"

	"github.com/vitismc/vitis/internal/world/worldgen/density"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

const wayBelowMinimumY = -4064

type fluidStatus struct {
	level int
	state int32
}

func (s fluidStatus) at(y int) int32 {
	if y < s.level {
		return s.state
	}
	return 0
}

type aquiferConfig struct {
	enabled      bool
	seed         int64
	legacy       bool
	minY         int
	height       int
	seaLevel     int
	defaultFluid int32
	lava         int32

	barrier, floodedness, spread, lavaNoise density.Function
	erosion, depth, initialDensity          density.Function
	preliminarySurface                      func(int, int, *density.Evaluation) int
}

type gridPosition struct{ x, y, z int }
type blockPosition struct{ x, y, z int }

var aquiferNeighbourOffsets = [...]gridPosition{
	{0, -1, 0}, {0, -1, 1}, {0, 0, 0}, {0, 0, 1}, {0, 1, 0}, {0, 1, 1},
	{1, -1, 0}, {1, -1, 1}, {1, 0, 0}, {1, 0, 1}, {1, 1, 0}, {1, 1, 1},
}

type chunkAquifer struct {
	config               aquiferConfig
	random               worldrandom.PositionalFactory
	locations            map[uint64]blockPosition
	candidates           map[uint64][12]blockPosition
	lastBase             gridPosition
	lastCenters          [12]blockPosition
	hasLastBase          bool
	distanceBase         gridPosition
	distanceX, distanceZ int
	horizontalDistances  [12]int
	centerYs             [12]int
	hasDistanceBase      bool
	statuses             map[uint64]fluidStatus
	preliminary          map[[2]int]int
	evaluation           *density.Evaluation
}

func newChunkAquifer(config aquiferConfig) *chunkAquifer {
	var source worldrandom.Source = worldrandom.NewXoroshiro(config.seed)
	if config.legacy {
		source = worldrandom.NewLegacy(config.seed)
	}
	root := source.ForkPositional()
	return &chunkAquifer{
		config:      config,
		random:      root.FromHashOf("minecraft:aquifer").ForkPositional(),
		locations:   make(map[uint64]blockPosition),
		candidates:  make(map[uint64][12]blockPosition),
		statuses:    make(map[uint64]fluidStatus),
		preliminary: make(map[[2]int]int),
		evaluation:  density.NewEvaluation(),
	}
}

// reset clears position-dependent caches before pool reuse.
func (a *chunkAquifer) reset() {
	clear(a.locations)
	clear(a.candidates)
	clear(a.statuses)
	clear(a.preliminary)
	a.hasLastBase = false
	a.hasDistanceBase = false
	a.evaluation.ResetChunk(0, 0, 0, 0)
}

func (a *chunkAquifer) substance(position density.Position, substanceDensity float64) int32 {
	if substanceDensity > 0 {
		return -1
	}
	// All fluid levels are bounded by sea level or the global lava level.
	// Five blocks above both, pressure is below -2 and cannot sample the barrier.
	if position.Y >= max(-54, a.config.seaLevel)+5 {
		return 0
	}
	return a.substanceBelowCeiling(position, substanceDensity)
}

// substanceBelowCeiling evaluates the nearest aquifer centers and fluid pressure.
func (a *chunkAquifer) substanceBelowCeiling(position density.Position, substanceDensity float64) int32 {
	global := a.globalFluid(position.Y)
	if !a.config.enabled {
		return global.at(position.Y)
	}
	if global.at(position.Y) == a.config.lava {
		return a.config.lava
	}

	base := gridPosition{
		x: (position.X - 5) >> 4,
		y: floorDiv(position.Y+1, 12),
		z: (position.Z - 5) >> 4,
	}
	distances := [3]int{math.MaxInt, math.MaxInt, math.MaxInt}
	var closest [3]int
	if !a.hasDistanceBase || a.distanceBase != base || a.distanceX != position.X || a.distanceZ != position.Z {
		candidates := a.candidateLocations(base)
		for index, center := range candidates {
			dx, dz := center.x-position.X, center.z-position.Z
			a.horizontalDistances[index] = dx*dx + dz*dz
			a.centerYs[index] = center.y
		}
		a.distanceBase, a.distanceX, a.distanceZ, a.hasDistanceBase = base, position.X, position.Z, true
	}
	for index, horizontal := range a.horizontalDistances {
		dy := a.centerYs[index] - position.Y
		distance := horizontal + dy*dy
		if distance <= distances[0] {
			distances[2], closest[2] = distances[1], closest[1]
			distances[1], closest[1] = distances[0], closest[0]
			distances[0], closest[0] = distance, index
		} else if distance <= distances[1] {
			distances[2], closest[2] = distances[1], closest[1]
			distances[1], closest[1] = distance, index
		} else if distance <= distances[2] {
			distances[2], closest[2] = distance, index
		}
	}
	first := a.status(offsetGridPosition(base, aquiferNeighbourOffsets[closest[0]]))
	similarityFirstSecond := aquiferSimilarity(distances[0], distances[1])
	state := first.at(position.Y)
	if similarityFirstSecond <= 0 {
		return state
	}
	if state == a.config.defaultFluid && a.globalFluid(position.Y-1).at(position.Y-1) == a.config.lava {
		return state
	}

	second := a.status(offsetGridPosition(base, aquiferNeighbourOffsets[closest[1]]))
	barrier := math.NaN()
	pressure := similarityFirstSecond * a.pressure(position, &barrier, first, second)
	if substanceDensity+pressure > 0 {
		return -1
	}
	third := a.status(offsetGridPosition(base, aquiferNeighbourOffsets[closest[2]]))
	similarityFirstThird := aquiferSimilarity(distances[0], distances[2])
	if similarityFirstThird > 0 {
		pressure = similarityFirstSecond * similarityFirstThird * a.pressure(position, &barrier, first, third)
		if substanceDensity+pressure > 0 {
			return -1
		}
	}
	similaritySecondThird := aquiferSimilarity(distances[1], distances[2])
	if similaritySecondThird > 0 {
		pressure = similarityFirstSecond * similaritySecondThird * a.pressure(position, &barrier, second, third)
		if substanceDensity+pressure > 0 {
			return -1
		}
	}
	return state
}

func insertClosestIndex(distances *[3]int, positions *[3]int, distance, position int) {
	if distance <= distances[0] {
		distances[2], positions[2] = distances[1], positions[1]
		distances[1], positions[1] = distances[0], positions[0]
		distances[0], positions[0] = distance, position
	} else if distance <= distances[1] {
		distances[2], positions[2] = distances[1], positions[1]
		distances[1], positions[1] = distance, position
	} else if distance <= distances[2] {
		distances[2], positions[2] = distance, position
	}
}

func offsetGridPosition(base, offset gridPosition) gridPosition {
	return gridPosition{x: base.x + offset.x, y: base.y + offset.y, z: base.z + offset.z}
}

func (a *chunkAquifer) candidateLocations(base gridPosition) *[12]blockPosition {
	if a.hasLastBase && a.lastBase == base {
		return &a.lastCenters
	}
	key := aquiferGridKey(base)
	if candidates, ok := a.candidates[key]; ok {
		a.lastBase, a.lastCenters, a.hasLastBase = base, candidates, true
		return &a.lastCenters
	}
	var candidates [12]blockPosition
	for index, offset := range aquiferNeighbourOffsets {
		candidates[index] = a.location(offsetGridPosition(base, offset))
	}
	a.candidates[key] = candidates
	a.lastBase, a.lastCenters, a.hasLastBase = base, candidates, true
	return &a.lastCenters
}

func (a *chunkAquifer) location(grid gridPosition) blockPosition {
	key := aquiferGridKey(grid)
	if position, ok := a.locations[key]; ok {
		return position
	}
	source := a.random.At(int32(grid.x), int32(grid.y), int32(grid.z))
	position := blockPosition{
		x: grid.x*16 + int(source.NextIntBound(10)),
		y: grid.y*12 + int(source.NextIntBound(9)),
		z: grid.z*16 + int(source.NextIntBound(10)),
	}
	a.locations[key] = position
	return position
}

func (a *chunkAquifer) status(grid gridPosition) fluidStatus {
	key := aquiferGridKey(grid)
	if status, ok := a.statuses[key]; ok {
		return status
	}
	position := a.location(grid)
	status := a.computeFluid(position.x, position.y, position.z)
	a.statuses[key] = status
	return status
}

func aquiferGridKey(grid gridPosition) uint64 {
	return uint64(uint32(grid.x)&0x3fffff)<<42 |
		uint64(uint32(grid.z)&0x3fffff)<<20 |
		uint64(uint32(grid.y)&0xfffff)
}

func (a *chunkAquifer) computeFluid(x, y, z int) fluidStatus {
	global := a.globalFluid(y)
	minimumSurface := math.MaxInt32
	above := y + 12
	below := y - 12
	hasSurfaceFluid := false
	offsets := [...][2]int{{0, 0}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {-3, 0}, {-2, 0}, {-1, 0}, {1, 0}, {-2, 1}, {-1, 1}, {0, 1}, {1, 1}}
	for _, offset := range offsets {
		sampleX, sampleZ := x+offset[0]*16, z+offset[1]*16
		surface := a.preliminarySurface(sampleX, sampleZ)
		surfacePlusEight := int(int32(surface + 8))
		center := offset == [2]int{0, 0}
		if center && below > surfacePlusEight {
			return global
		}
		if above > surfacePlusEight || center {
			status := a.globalFluid(surfacePlusEight)
			if status.at(surfacePlusEight) != 0 {
				if center {
					hasSurfaceFluid = true
				}
				if above > surfacePlusEight {
					return status
				}
			}
		}
		if surface < minimumSurface {
			minimumSurface = surface
		}
	}
	level := a.surfaceLevel(x, y, z, global, minimumSurface, hasSurfaceFluid)
	return fluidStatus{level: level, state: a.fluidType(x, y, z, global, level)}
}

func (a *chunkAquifer) preliminarySurface(x, z int) int {
	x, z = floorMultiple(x, 4), floorMultiple(z, 4)
	if a.config.preliminarySurface != nil {
		return a.config.preliminarySurface(x, z, a.evaluation)
	}
	key := [2]int{x, z}
	if value, ok := a.preliminary[key]; ok {
		return value
	}
	minimum := a.config.minY
	for y := minimum + a.config.height; y >= minimum; y -= 8 {
		if a.config.initialDensity.Sample(density.Position{X: x, Y: y, Z: z, Evaluation: a.evaluation}) > 0.390625 {
			a.preliminary[key] = y
			return y
		}
	}
	a.preliminary[key] = math.MaxInt32
	return math.MaxInt32
}

func (a *chunkAquifer) surfaceLevel(x, y, z int, global fluidStatus, minimumSurface int, hasSurfaceFluid bool) int {
	position := density.Position{X: x, Y: y, Z: z}
	var lower, upper float64
	if a.config.erosion.Sample(position) < float64(float32(-0.225)) && a.config.depth.Sample(position) > float64(float32(0.9)) {
		lower, upper = -1, -1
	} else {
		distance := minimumSurface + 8 - y
		surfaceInfluence := 0.0
		if hasSurfaceFluid {
			surfaceInfluence = clampMap(float64(distance), 0, 64, 1, 0)
		}
		floodedness := math.Max(-1, math.Min(1, a.config.floodedness.Sample(position)))
		lower = floodedness - linearMap(surfaceInfluence, 1, 0, -0.8, 0.4)
		upper = floodedness - linearMap(surfaceInfluence, 1, 0, -0.3, 0.8)
	}
	if upper > 0 {
		return global.level
	}
	if lower > 0 {
		cellY := floorDiv(y, 40)
		base := cellY*40 + 20
		spread := a.config.spread.Sample(density.Position{X: floorDiv(x, 16), Y: cellY, Z: floorDiv(z, 16)}) * 10
		quantized := int(math.Floor(spread/3)) * 3
		return min(global.level, base+quantized)
	}
	return wayBelowMinimumY
}

func (a *chunkAquifer) fluidType(x, y, z int, global fluidStatus, level int) int32 {
	state := global.state
	if level <= -10 && level != wayBelowMinimumY && state != a.config.lava {
		value := a.config.lavaNoise.Sample(density.Position{X: floorDiv(x, 64), Y: floorDiv(y, 40), Z: floorDiv(z, 64)})
		if math.Abs(value) > 0.3 {
			state = a.config.lava
		}
	}
	return state
}

func (a *chunkAquifer) pressure(position density.Position, barrier *float64, first, second fluidStatus) float64 {
	firstState, secondState := first.at(position.Y), second.at(position.Y)
	if firstState == a.config.lava && secondState == a.config.defaultFluid || firstState == a.config.defaultFluid && secondState == a.config.lava {
		return 2
	}
	difference := abs(first.level - second.level)
	if difference == 0 {
		return 0
	}
	middle := 0.5 * float64(first.level+second.level)
	offset := float64(position.Y) + 0.5 - middle
	halfDifference := float64(difference) / 2
	distance := halfDifference - math.Abs(offset)
	var pressure float64
	if offset > 0 {
		value := distance
		if value > 0 {
			pressure = value / 1.5
		} else {
			pressure = value / 2.5
		}
	} else {
		value := 3 + distance
		if value > 0 {
			pressure = value / 3
		} else {
			pressure = value / 10
		}
	}
	barrierValue := 0.0
	if pressure >= -2 && pressure <= 2 {
		if math.IsNaN(*barrier) {
			*barrier = a.config.barrier.Sample(position)
		}
		barrierValue = *barrier
	}
	return 2 * (barrierValue + pressure)
}

func (a *chunkAquifer) globalFluid(y int) fluidStatus {
	if y < min(-54, a.config.seaLevel) {
		return fluidStatus{level: -54, state: a.config.lava}
	}
	return fluidStatus{level: a.config.seaLevel, state: a.config.defaultFluid}
}

func aquiferSimilarity(first, second int) float64 { return 1 - float64(abs(second-first))/25 }

func floorDiv(value, divisor int) int {
	quotient := value / divisor
	if value%divisor < 0 {
		quotient--
	}
	return quotient
}

func floorMultiple(value, size int) int { return floorDiv(value, size) * size }

func clampMap(value, fromLow, fromHigh, toLow, toHigh float64) float64 {
	if value <= fromLow {
		return toLow
	}
	if value >= fromHigh {
		return toHigh
	}
	return linearMap(value, fromLow, fromHigh, toLow, toHigh)
}

func linearMap(value, fromLow, fromHigh, toLow, toHigh float64) float64 {
	return toLow + (value-fromLow)/(fromHigh-fromLow)*(toHigh-toLow)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
