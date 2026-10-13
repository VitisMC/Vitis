package worldgen

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// geodeFeature holds validated layer settings and snapshot-scoped block tags.
type geodeFeature struct {
	filling, inner, alternate, middle, outer stateProvider
	placements                               [][6][2]int32
	cannotReplace, invalid                   [genblock.BlockCount]bool
	distribution, wallDistance, pointOffset  intProvider
	distributionMax, wallDistanceMax         int
	minOffset, maxOffset, invalidThreshold   int
	chunkRadius                              int
	layers                                   [4]float64
	crackSize, crackChance                   float64
	crackOffset                              int
	alternateChance, placementChance         float64
	noiseMultiplier                          float64
	requireAlternate                         bool
	samplers                                 sync.Map
}

// geodeIntMaximum extracts a bounded provider maximum for region sizing.
func geodeIntMaximum(provider intProvider) (int, bool) {
	switch value := provider.(type) {
	case constantInt:
		return int(value), true
	case uniformInt:
		return value.max, true
	case biasedInt:
		return value.max, true
	case clampedInt:
		return value.max, true
	case clampedNormalInt:
		return value.max, true
	case *weightedInt:
		maximum := 0
		for _, entry := range value.entries {
			bound, ok := geodeIntMaximum(entry.provider)
			if !ok {
				return 0, false
			}
			maximum = max(maximum, bound)
		}
		return maximum, len(value.entries) != 0
	default:
		return 0, false
	}
}

// decodeGeodeFeature compiles the shipped geode shape without retaining JSON in the hot path.
func decodeGeodeFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*geodeFeature, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("geode requires block tags")
	}
	var definition struct {
		Config struct {
			Blocks struct {
				Filling    json.RawMessage `json:"filling_provider"`
				Inner      json.RawMessage `json:"inner_layer_provider"`
				Alternate  json.RawMessage `json:"alternate_inner_layer_provider"`
				Middle     json.RawMessage `json:"middle_layer_provider"`
				Outer      json.RawMessage `json:"outer_layer_provider"`
				Placements []BlockState    `json:"inner_placements"`
				Protected  string          `json:"cannot_replace"`
				Invalid    string          `json:"invalid_blocks"`
			} `json:"blocks"`
			Layers struct {
				Filling float64 `json:"filling"`
				Inner   float64 `json:"inner_layer"`
				Middle  float64 `json:"middle_layer"`
				Outer   float64 `json:"outer_layer"`
			} `json:"layers"`
			Crack struct {
				Size   float64 `json:"base_crack_size"`
				Offset int     `json:"crack_point_offset"`
				Chance float64 `json:"generate_crack_chance"`
			} `json:"crack"`
			Distribution     json.RawMessage `json:"distribution_points"`
			WallDistance     json.RawMessage `json:"outer_wall_distance"`
			PointOffset      json.RawMessage `json:"point_offset"`
			MinOffset        int             `json:"min_gen_offset"`
			MaxOffset        int             `json:"max_gen_offset"`
			InvalidThreshold int             `json:"invalid_blocks_threshold"`
			NoiseMultiplier  float64         `json:"noise_multiplier"`
			AlternateChance  float64         `json:"use_alternate_layer0_chance"`
			PlacementChance  float64         `json:"use_potential_placements_chance"`
			RequireAlternate bool            `json:"placements_require_layer0_alternate"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, err
	}
	c := definition.Config
	validChance := func(value float64) bool { return !math.IsNaN(value) && value >= 0 && value <= 1 }
	validLayer := func(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0 }
	if c.MinOffset > c.MaxOffset || c.MinOffset < -32 || c.MaxOffset > 32 ||
		c.InvalidThreshold < 0 || c.InvalidThreshold > 128 ||
		!validLayer(c.Layers.Filling) || !validLayer(c.Layers.Inner) ||
		!validLayer(c.Layers.Middle) || !validLayer(c.Layers.Outer) ||
		!validLayer(c.Crack.Size) || c.Crack.Offset < 0 || c.Crack.Offset > 64 ||
		!validChance(c.Crack.Chance) || !validChance(c.AlternateChance) || !validChance(c.PlacementChance) ||
		math.IsNaN(c.NoiseMultiplier) || math.IsInf(c.NoiseMultiplier, 0) || c.NoiseMultiplier < 0 || c.NoiseMultiplier > 1 {
		return nil, fmt.Errorf("invalid geode configuration")
	}
	f := &geodeFeature{
		minOffset: c.MinOffset, maxOffset: c.MaxOffset, invalidThreshold: c.InvalidThreshold,
		layers:    [4]float64{c.Layers.Filling, c.Layers.Inner, c.Layers.Middle, c.Layers.Outer},
		crackSize: c.Crack.Size, crackChance: c.Crack.Chance, crackOffset: c.Crack.Offset,
		alternateChance: c.AlternateChance, placementChance: c.PlacementChance,
		noiseMultiplier: c.NoiseMultiplier, requireAlternate: c.RequireAlternate,
	}
	var err error
	for _, item := range []struct {
		raw json.RawMessage
		out *stateProvider
	}{
		{c.Blocks.Filling, &f.filling}, {c.Blocks.Inner, &f.inner},
		{c.Blocks.Alternate, &f.alternate}, {c.Blocks.Middle, &f.middle}, {c.Blocks.Outer, &f.outer},
	} {
		if *item.out, err = decodeStateProvider(item.raw); err != nil {
			return nil, fmt.Errorf("geode state provider: %w", err)
		}
	}
	for _, item := range []struct {
		raw     json.RawMessage
		out     *intProvider
		maximum *int
	}{
		{c.Distribution, &f.distribution, &f.distributionMax},
		{c.WallDistance, &f.wallDistance, &f.wallDistanceMax},
		{c.PointOffset, &f.pointOffset, nil},
	} {
		if *item.out, err = decodeIntProviderRange(item.raw, 1, 32); err != nil {
			return nil, fmt.Errorf("geode integer provider: %w", err)
		}
		maximum, ok := geodeIntMaximum(*item.out)
		if !ok {
			return nil, fmt.Errorf("unsupported geode integer provider")
		}
		if item.maximum != nil {
			*item.maximum = maximum
		}
	}
	if f.distributionMax > 32 || f.wallDistanceMax > 32 {
		return nil, fmt.Errorf("geode point count or wall distance exceeds supported bounds")
	}
	for _, item := range []struct {
		name string
		out  *[genblock.BlockCount]bool
	}{
		{c.Blocks.Protected, &f.cannotReplace}, {c.Blocks.Invalid, &f.invalid},
	} {
		if !strings.HasPrefix(item.name, "#") {
			return nil, fmt.Errorf("geode block tag %q must start with #", item.name)
		}
		names, err := snapshot.ResolveTag("block", normalizeResourceID(strings.TrimPrefix(item.name, "#")))
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			info := block.InfoByName(normalizeResourceID(name))
			if info == nil {
				return nil, fmt.Errorf("unknown geode tag block %q", name)
			}
			item.out[info.ID] = true
		}
	}
	if len(c.Blocks.Placements) == 0 {
		return nil, fmt.Errorf("geode has no inner placements")
	}
	for _, placement := range c.Blocks.Placements {
		states, err := geodePlacementStates(placement)
		if err != nil {
			return nil, err
		}
		f.placements = append(f.placements, states)
	}
	reach := max(max(absInt(c.MinOffset), absInt(c.MaxOffset))+1, f.wallDistanceMax)
	f.chunkRadius = max(1, (reach+15)/16)
	return f, nil
}

// geodePlacementStates resolves directional and waterlogged cluster states once.
func geodePlacementStates(value BlockState) ([6][2]int32, error) {
	var states [6][2]int32
	for direction, facing := range [...]string{"down", "up", "north", "south", "west", "east"} {
		for wet := range 2 {
			properties := make(map[string]string, len(value.Properties))
			for name, property := range value.Properties {
				properties[name] = property
			}
			if _, ok := properties["facing"]; ok {
				properties["facing"] = facing
			}
			if _, ok := properties["waterlogged"]; ok {
				properties["waterlogged"] = [...]string{"false", "true"}[wet]
			}
			state, err := block.BlockStateFromProperties(value.Name, properties)
			if err != nil {
				return states, fmt.Errorf("geode cluster %s: %w", value.Name, err)
			}
			states[direction][wet] = state.ID()
		}
	}
	return states, nil
}

// sampler returns deterministic world-seed noise shared by placement workers.
func (f *geodeFeature) sampler(seed int64) (*noise.Normal, error) {
	if cached, ok := f.samplers.Load(seed); ok {
		return cached.(*noise.Normal), nil
	}
	created, err := noise.NewNormal(worldrandom.NewWorldgen(worldrandom.NewLegacy(seed)),
		noise.Parameters{FirstOctave: -4, Amplitudes: []float64{1}})
	if err != nil {
		return nil, err
	}
	actual, _ := f.samplers.LoadOrStore(seed, created)
	return actual.(*noise.Normal), nil
}

// geodePoint carries one sampled distribution center and its distance offset.
type geodePoint struct {
	position blockPosition
	offset   int
}

// geodeInvDistance preserves the layer sum's inverse-square-root arithmetic.
func geodeInvDistance(position, center blockPosition, offset int) float64 {
	dx, dy, dz := position.x-center.x, position.y-center.y, position.z-center.z
	return 1 / math.Sqrt(float64(dx*dx+dy*dy+dz*dz+offset))
}

// geodeSet replaces a block only when the configured tag permits it.
func (f *geodeFeature) geodeSet(chunk *section.Chunk, position blockPosition, state int32) {
	if !f.cannotReplace[block.BlockIDFromState(getChunkBlock(chunk, position))] {
		setChunkBlock(chunk, position, state)
	}
}

// placeFeature builds all layers before attaching buds to eligible air and water cells.
func (f *geodeFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if system == nil || chunk == nil {
		return false
	}
	count := f.distribution.sample(random)
	sampler, err := f.sampler(system.seed)
	if err != nil {
		return false
	}
	wallRatio := float64(count) / float64(f.wallDistanceMax)
	threshold := [4]float64{
		1 / math.Sqrt(f.layers[0]),
		1 / math.Sqrt(f.layers[1]+wallRatio),
		1 / math.Sqrt(f.layers[2]+wallRatio),
		1 / math.Sqrt(f.layers[3]+wallRatio),
	}
	crackThreshold := 1 / math.Sqrt(f.crackSize+random.NextDouble()/2+func() float64 {
		if count > 3 {
			return wallRatio
		}
		return 0
	}())
	cracked := float64(random.NextFloat()) < f.crackChance
	points := make([]geodePoint, 0, count)
	invalid := 0
	for range count {
		position := blockPosition{
			origin.x + f.wallDistance.sample(random),
			origin.y + f.wallDistance.sample(random),
			origin.z + f.wallDistance.sample(random),
		}
		state := getChunkBlock(chunk, position)
		if block.IsAir(state) || f.invalid[block.BlockIDFromState(state)] {
			invalid++
			if invalid > f.invalidThreshold {
				return false
			}
		}
		points = append(points, geodePoint{position, f.pointOffset.sample(random)})
	}
	var cracks [3]blockPosition
	if cracked {
		span := count*2 + 1
		switch random.NextIntBound(4) {
		case 0:
			cracks = [3]blockPosition{{origin.x + span, origin.y + 7, origin.z}, {origin.x + span, origin.y + 5, origin.z}, {origin.x + span, origin.y + 1, origin.z}}
		case 1:
			cracks = [3]blockPosition{{origin.x, origin.y + 7, origin.z + span}, {origin.x, origin.y + 5, origin.z + span}, {origin.x, origin.y + 1, origin.z + span}}
		case 2:
			cracks = [3]blockPosition{{origin.x + span, origin.y + 7, origin.z + span}, {origin.x + span, origin.y + 5, origin.z + span}, {origin.x + span, origin.y + 1, origin.z + span}}
		default:
			cracks = [3]blockPosition{{origin.x, origin.y + 7, origin.z}, {origin.x, origin.y + 5, origin.z}, {origin.x, origin.y + 1, origin.z}}
		}
	}
	potential := make([]blockPosition, 0, 256)
	for z := origin.z + f.minOffset; z <= origin.z+f.maxOffset; z++ {
		for y := origin.y + f.minOffset; y <= origin.y+f.maxOffset; y++ {
			for x := origin.x + f.minOffset; x <= origin.x+f.maxOffset; x++ {
				position := blockPosition{x, y, z}
				jitter := sampler.Sample(float64(x), float64(y), float64(z)) * f.noiseMultiplier
				layer, crack := 0.0, 0.0
				for _, point := range points {
					layer += geodeInvDistance(position, point.position, point.offset) + jitter
				}
				if layer < threshold[3] {
					continue
				}
				for _, point := range cracks {
					if cracked {
						crack += geodeInvDistance(position, point, f.crackOffset) + jitter
					}
				}
				switch {
				case cracked && crack >= crackThreshold && layer < threshold[0]:
					f.geodeSet(chunk, position, 0)
				case layer >= threshold[0]:
					f.geodeSet(chunk, position, f.filling.state(system, chunk, random, position))
				case layer >= threshold[1]:
					alternate := float64(random.NextFloat()) < f.alternateChance
					if alternate {
						f.geodeSet(chunk, position, f.alternate.state(system, chunk, random, position))
					} else {
						f.geodeSet(chunk, position, f.inner.state(system, chunk, random, position))
					}
					if (!f.requireAlternate || alternate) && float64(random.NextFloat()) < f.placementChance {
						potential = append(potential, position)
					}
				case layer >= threshold[2]:
					f.geodeSet(chunk, position, f.middle.state(system, chunk, random, position))
				default:
					f.geodeSet(chunk, position, f.outer.state(system, chunk, random, position))
				}
			}
		}
	}
	directions := [...]blockPosition{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}}
	for _, position := range potential {
		states := f.placements[random.NextIntBound(int32(len(f.placements)))]
		for direction, delta := range directions {
			neighbor := blockPosition{position.x + delta.x, position.y + delta.y, position.z + delta.z}
			previous := getChunkBlock(chunk, neighbor)
			if !block.IsAir(previous) && (blockfluid.GetFluidType(previous) != blockfluid.FluidTypeWater || !blockfluid.IsSource(previous)) {
				continue
			}
			wet := 0
			if blockfluid.GetFluidType(previous) == blockfluid.FluidTypeWater && blockfluid.IsSource(previous) {
				wet = 1
			}
			f.geodeSet(chunk, neighbor, states[direction][wet])
			break
		}
	}
	return true
}
