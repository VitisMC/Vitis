package worldgen

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// largeDripstoneFeature retains the sampled shape and wind configuration.
type largeDripstoneFeature struct {
	pointed              *pointedDripstoneFeature
	baseStone            [genblock.BlockCount]bool
	radiusMin, radiusMax int
	heightScale          floatProvider
	ceilingBluntness     floatProvider
	floorBluntness       floatProvider
	windSpeed            floatProvider
	searchRange          int
	maxRadiusHeightRatio float32
	minWindRadius        int
	minWindBluntness     float32
	chunkRadius          int
}

// decodeLargeDripstoneFeature validates the shipped providers and support tag.
func decodeLargeDripstoneFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*largeDripstoneFeature, error) {
	var value struct {
		Config struct {
			SearchRange          int             `json:"floor_to_ceiling_search_range"`
			Radius               json.RawMessage `json:"column_radius"`
			HeightScale          json.RawMessage `json:"height_scale"`
			RadiusHeightRatio    float32         `json:"max_column_radius_to_cave_height_ratio"`
			CeilingBluntness     json.RawMessage `json:"stalactite_bluntness"`
			FloorBluntness       json.RawMessage `json:"stalagmite_bluntness"`
			WindSpeed            json.RawMessage `json:"wind_speed"`
			MinimumWindRadius    int             `json:"min_radius_for_wind"`
			MinimumWindBluntness float32         `json:"min_bluntness_for_wind"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	c := value.Config
	if c.SearchRange < 1 || c.SearchRange > 512 || c.RadiusHeightRatio < .1 || c.RadiusHeightRatio > 1 ||
		c.MinimumWindRadius < 0 || c.MinimumWindRadius > 100 || c.MinimumWindBluntness < 0 || c.MinimumWindBluntness > 5 {
		return nil, fmt.Errorf("invalid large dripstone configuration")
	}
	f := &largeDripstoneFeature{
		searchRange: c.SearchRange, maxRadiusHeightRatio: c.RadiusHeightRatio,
		minWindRadius: c.MinimumWindRadius, minWindBluntness: c.MinimumWindBluntness,
	}
	var err error
	if f.pointed, err = decodePointedDripstoneFeature(snapshot, json.RawMessage(`{"config":{}}`)); err != nil {
		return nil, err
	}
	radiusProvider, err := decodeIntProviderRange(c.Radius, 1, 60)
	if err != nil {
		return nil, err
	}
	switch radius := radiusProvider.(type) {
	case constantInt:
		f.radiusMin, f.radiusMax = int(radius), int(radius)
	case uniformInt:
		f.radiusMin, f.radiusMax = radius.min, radius.max
	default:
		return nil, fmt.Errorf("unsupported large dripstone radius provider")
	}
	if f.heightScale, err = decodeFloatProvider(c.HeightScale); err != nil {
		return nil, err
	}
	if f.ceilingBluntness, err = decodeFloatProvider(c.CeilingBluntness); err != nil {
		return nil, err
	}
	if f.floorBluntness, err = decodeFloatProvider(c.FloorBluntness); err != nil {
		return nil, err
	}
	if f.windSpeed, err = decodeFloatProvider(c.WindSpeed); err != nil {
		return nil, err
	}
	scaleMin, scaleMax, ok := largeDripstoneFloatBounds(f.heightScale)
	if !ok || scaleMin < 0 || scaleMax > 20 {
		return nil, fmt.Errorf("large dripstone height scale out of range")
	}
	ceilingMin, ceilingMax, ok := largeDripstoneFloatBounds(f.ceilingBluntness)
	if !ok || ceilingMin < .1 || ceilingMax > 10 {
		return nil, fmt.Errorf("large dripstone stalactite bluntness out of range")
	}
	floorMin, floorMax, ok := largeDripstoneFloatBounds(f.floorBluntness)
	if !ok || floorMin < .1 || floorMax > 10 {
		return nil, fmt.Errorf("large dripstone stalagmite bluntness out of range")
	}
	windMin, windMax, ok := largeDripstoneFloatBounds(f.windSpeed)
	if !ok || windMin < 0 || windMax > 2 {
		return nil, fmt.Errorf("large dripstone wind speed out of range")
	}
	lowestBluntness := min(ceilingMin, floorMin)
	maximumHeight := (&largeDripstoneColumn{radius: f.radiusMax, bluntness: float64(lowestBluntness), heightScale: float64(scaleMax)}).heightAt(0)
	maxReach := f.radiusMax + int(math.Ceil(float64(windMax)*float64(max(maximumHeight, f.searchRange+10)))) + 2
	f.chunkRadius = max(1, (maxReach+15)/16)
	if f.chunkRadius > 32 {
		return nil, fmt.Errorf("large dripstone reach exceeds supported feature region")
	}
	copy(f.baseStone[:], f.pointed.replaceable[:])
	f.baseStone[block.BlockIDFromState(f.pointed.base)] = true
	return f, nil
}

// largeDripstoneFloatBounds exposes safe extrema for region sizing.
func largeDripstoneFloatBounds(provider floatProvider) (float32, float32, bool) {
	var minimum, maximum float32
	switch value := provider.(type) {
	case constantFloat:
		minimum, maximum = float32(value), float32(value)
	case uniformFloat:
		minimum, maximum = value.min, value.max
	case clampedNormalFloat:
		minimum, maximum = value.min, value.max
	case trapezoidFloat:
		minimum, maximum = value.min, value.max
	default:
		return 0, 0, false
	}
	if minimum > maximum || math.IsNaN(float64(minimum)) || math.IsNaN(float64(maximum)) ||
		math.IsInf(float64(minimum), 0) || math.IsInf(float64(maximum), 0) {
		return 0, 0, false
	}
	return minimum, maximum, true
}

// largeDripstoneColumn owns one stalactite or stalagmite and its sampled shape.
type largeDripstoneColumn struct {
	root        blockPosition
	up          bool
	radius      int
	bluntness   float64
	heightScale float64
}

// largeDripstoneWind shifts higher layers with a sampled horizontal vector.
type largeDripstoneWind struct {
	originY int
	x, z    float64
	active  bool
}

// offset applies the wind displacement with floor rounding for negative values.
func (wind largeDripstoneWind) offset(position blockPosition) blockPosition {
	if !wind.active {
		return position
	}
	distance := float64(wind.originY - position.y)
	return blockPosition{position.x + int(math.Floor(wind.x*distance)), position.y, position.z + int(math.Floor(wind.z*distance))}
}

// heightAt returns the integer height of a radial dripstone section.
func (column *largeDripstoneColumn) heightAt(distance float32) int {
	radius := float64(distance)
	if radius < column.bluntness {
		radius = column.bluntness
	}
	value := radius / float64(column.radius) * .384
	shape := .75*math.Pow(value, 1.3333333333333333) - math.Pow(value, .6666666666666666) -
		.3333333333333333*math.Log(value)
	return int(math.Max(column.heightScale*shape, 0) / .384 * float64(column.radius))
}

// dripstoneEmptyWaterOrLava accepts fluids within an existing cave.
func dripstoneEmptyWaterOrLava(state int32) bool {
	name := block.NameFromState(state)
	return block.IsAir(state) || name == "minecraft:water" || name == "minecraft:lava"
}

// circleInsideStone checks the root and ring before moving a column into rock.
func circleInsideStone(chunk *section.Chunk, center blockPosition, radius int) bool {
	if dripstoneEmptyWaterOrLava(getChunkBlock(chunk, center)) {
		return false
	}
	step := float32(6) / float32(radius)
	for angle := float32(0); angle < float32(math.Pi*2); angle += step {
		dx := int(mthCos(angle) * float32(radius))
		dz := int(mthSin(angle) * float32(radius))
		if dripstoneEmptyWaterOrLava(getChunkBlock(chunk, blockPosition{center.x + dx, center.y, center.z + dz})) {
			return false
		}
	}
	return true
}

// anchor backs a column into stone, halving its radius when no ring fits.
func (column *largeDripstoneColumn) anchor(chunk *section.Chunk, wind largeDripstoneWind) bool {
	for column.radius > 1 {
		position := column.root
		for step := 0; step < min(10, column.heightAt(0)); step++ {
			if block.NameFromState(getChunkBlock(chunk, position)) == "minecraft:lava" {
				return false
			}
			if circleInsideStone(chunk, wind.offset(position), column.radius) {
				column.root = position
				return true
			}
			if column.up {
				position.y--
			} else {
				position.y++
			}
		}
		column.radius /= 2
	}
	return false
}

// place writes a radial column, stopping when it reaches exposed base stone.
func (column *largeDripstoneColumn) place(chunk *section.Chunk, random worldrandom.Source, wind largeDripstoneWind, baseStone *[genblock.BlockCount]bool, state int32) {
	for dx := -column.radius; dx <= column.radius; dx++ {
		for dz := -column.radius; dz <= column.radius; dz++ {
			distance := float32(math.Sqrt(float64(float32(dx*dx + dz*dz))))
			if distance > float32(column.radius) {
				continue
			}
			height := column.heightAt(distance)
			if height <= 0 {
				continue
			}
			if float64(random.NextFloat()) < .2 {
				scale := float32(.8) + random.NextFloat()*float32(.2)
				height = int(float32(height) * scale)
			}
			position := blockPosition{column.root.x + dx, column.root.y, column.root.z + dz}
			started := false
			limit := math.MaxInt
			if column.up {
				if surface, _ := featureHeightmap(chunk, position.x, position.z, "WORLD_SURFACE_WG"); surface >= chunk.MinY() {
					limit = surface
				}
			}
			for step := 0; step < height && position.y < limit; step++ {
				shifted := wind.offset(position)
				old := getChunkBlock(chunk, shifted)
				if dripstoneEmptyWaterOrLava(old) {
					started = true
					setChunkBlock(chunk, shifted, state)
				} else if started && baseStone[block.BlockIDFromState(old)] {
					break
				}
				if column.up {
					position.y++
				} else {
					position.y--
				}
			}
		}
	}
}

// placeFeature samples paired columns and wind in the versioned RNG order.
func (f *largeDripstoneFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !dripstoneEmptyOrWater(getChunkBlock(chunk, origin)) {
		return false
	}
	column := scanDripstoneColumn(chunk, origin, f.searchRange, func(state int32) bool {
		return f.pointed.dripstoneBase(state) || block.NameFromState(state) == "minecraft:lava"
	})
	if !column.hasCeiling || !column.hasFloor || column.ceiling-column.floor-1 < 4 {
		return false
	}
	maxRadius := min(f.radiusMax, max(f.radiusMin, int(float32(column.ceiling-column.floor-1)*f.maxRadiusHeightRatio)))
	radius := f.radiusMin + int(random.NextIntBound(int32(maxRadius-f.radiusMin+1)))
	ceiling := largeDripstoneColumn{blockPosition{origin.x, column.ceiling - 1, origin.z}, false, radius,
		float64(f.ceilingBluntness.sample(random)), float64(f.heightScale.sample(random))}
	floor := largeDripstoneColumn{blockPosition{origin.x, column.floor + 1, origin.z}, true, radius,
		float64(f.floorBluntness.sample(random)), float64(f.heightScale.sample(random))}
	wind := largeDripstoneWind{}
	if radius >= f.minWindRadius && ceiling.bluntness >= float64(f.minWindBluntness) && floor.bluntness >= float64(f.minWindBluntness) {
		speed := f.windSpeed.sample(random)
		angle := random.NextFloat() * float32(math.Pi)
		wind = largeDripstoneWind{origin.y, float64(mthCos(angle) * speed), float64(mthSin(angle) * speed), true}
	}
	ceilingOK := ceiling.anchor(chunk, wind)
	floorOK := floor.anchor(chunk, wind)
	if ceilingOK {
		ceiling.place(chunk, random, wind, &f.baseStone, f.pointed.base)
	}
	if floorOK {
		floor.place(chunk, random, wind, &f.baseStone, f.pointed.base)
	}
	return true
}
