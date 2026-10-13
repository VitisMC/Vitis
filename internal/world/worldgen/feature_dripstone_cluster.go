package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// dripstoneClusterFeature places small paired dripstone columns in cave cavities.
type dripstoneClusterFeature struct {
	pointed                          *pointedDripstoneFeature
	height, radius, layer            intProvider
	density, wetness                 floatProvider
	searchRange, maxHeightDifference int
	heightDeviation, heightDistance  int
	edgeDistance                     int
	columnChance                     float32
	baseStone                        [genblock.BlockCount]bool
	water                            int32
}

// decodeDripstoneClusterFeature resolves configuration providers and snapshot tags.
func decodeDripstoneClusterFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*dripstoneClusterFeature, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("dripstone cluster requires block tags")
	}
	var value struct {
		Config struct {
			SearchRange      int             `json:"floor_to_ceiling_search_range"`
			Height           json.RawMessage `json:"height"`
			Radius           json.RawMessage `json:"radius"`
			Layer            json.RawMessage `json:"dripstone_block_layer_thickness"`
			Density          json.RawMessage `json:"density"`
			Wetness          json.RawMessage `json:"wetness"`
			HeightDifference int             `json:"max_stalagmite_stalactite_height_diff"`
			HeightDeviation  int             `json:"height_deviation"`
			HeightDistance   int             `json:"max_distance_from_center_affecting_height_bias"`
			EdgeDistance     int             `json:"max_distance_from_edge_affecting_chance_of_dripstone_column"`
			ColumnChance     float32         `json:"chance_of_dripstone_column_at_max_distance_from_center"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	c := value.Config
	if c.SearchRange < 1 || c.SearchRange > 512 || c.HeightDifference < 0 || c.HeightDifference > 64 ||
		c.HeightDeviation < 1 || c.HeightDeviation > 64 || c.HeightDistance < 1 || c.HeightDistance > 64 ||
		c.EdgeDistance < 1 || c.EdgeDistance > 64 || c.ColumnChance < 0 || c.ColumnChance > 1 {
		return nil, fmt.Errorf("invalid dripstone cluster configuration")
	}
	f := &dripstoneClusterFeature{
		searchRange: c.SearchRange, maxHeightDifference: c.HeightDifference,
		heightDeviation: c.HeightDeviation, heightDistance: c.HeightDistance,
		edgeDistance: c.EdgeDistance, columnChance: c.ColumnChance,
		water: block.DefaultStateID("minecraft:water"),
	}
	var err error
	if f.pointed, err = decodePointedDripstoneFeature(snapshot, json.RawMessage(`{"config":{}}`)); err != nil {
		return nil, err
	}
	if f.height, err = decodeIntProviderRange(c.Height, 1, 128); err != nil {
		return nil, err
	}
	if f.radius, err = decodeIntProviderRange(c.Radius, 1, 128); err != nil {
		return nil, err
	}
	if f.layer, err = decodeIntProviderRange(c.Layer, 0, 128); err != nil {
		return nil, err
	}
	if f.density, err = decodeFloatProvider(c.Density); err != nil {
		return nil, err
	}
	if f.wetness, err = decodeFloatProvider(c.Wetness); err != nil {
		return nil, err
	}
	if f.water < 0 {
		return nil, fmt.Errorf("missing water block")
	}
	names, err := snapshot.ResolveTag("block", "minecraft:base_stone_overworld")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		info := block.InfoByName(normalizeResourceID(name))
		if info == nil {
			return nil, fmt.Errorf("unknown base stone block %s", name)
		}
		f.baseStone[info.ID] = true
	}
	return f, nil
}

// dripstoneColumn stores the scanned boundaries of one empty or water-filled column.
type dripstoneColumn struct {
	ceiling, floor       int
	hasCeiling, hasFloor bool
}

// scanDripstoneColumn follows the versioned search distance and boundary predicates.
func scanDripstoneColumn(chunk *section.Chunk, origin blockPosition, distance int, boundary func(int32) bool) dripstoneColumn {
	var result dripstoneColumn
	if !dripstoneEmptyOrWater(getChunkBlock(chunk, origin)) {
		return result
	}
	for _, direction := range []int{1, -1} {
		position := origin
		for step := 1; step < distance && dripstoneEmptyOrWater(getChunkBlock(chunk, position)); step++ {
			position.y += direction
		}
		if !boundary(getChunkBlock(chunk, position)) {
			continue
		}
		if direction > 0 {
			result.ceiling, result.hasCeiling = position.y, true
		} else {
			result.floor, result.hasFloor = position.y, true
		}
	}
	return result
}

// dripstoneClampedMapFloat preserves the float arithmetic used for cluster edge chance.
func dripstoneClampedMapFloat(value, end, startValue, endValue float32) float32 {
	delta := value / end
	if delta < 0 {
		return startValue
	}
	if delta > 1 {
		return endValue
	}
	return startValue + delta*(endValue-startValue)
}

// dripstoneClampedMapDouble preserves double arithmetic used for height bias.
func dripstoneClampedMapDouble(value, end, startValue float64) float64 {
	delta := value / end
	if delta < 0 {
		return startValue
	}
	if delta > 1 {
		return 0
	}
	return startValue + delta*(0-startValue)
}

// placeFeature samples cluster parameters once and processes columns in X/Z order.
func (f *dripstoneClusterFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !dripstoneEmptyOrWater(getChunkBlock(chunk, origin)) {
		return false
	}
	height := f.height.sample(random)
	wetness := f.wetness.sample(random)
	density := f.density.sample(random)
	radiusX := f.radius.sample(random)
	radiusZ := f.radius.sample(random)
	for dx := -radiusX; dx <= radiusX; dx++ {
		for dz := -radiusZ; dz <= radiusZ; dz++ {
			edge := min(radiusX-absInt(dx), radiusZ-absInt(dz))
			chance := dripstoneClampedMapFloat(float32(edge), float32(f.edgeDistance), f.columnChance, 1)
			position := blockPosition{origin.x + dx, origin.y, origin.z + dz}
			f.placeColumn(chunk, random, position, dx, dz, wetness, float64(chance), height, density)
		}
	}
	return true
}

// placeColumn handles pools, stone layers, tip overlap, and merge states in RNG order.
func (f *dripstoneClusterFeature) placeColumn(chunk *section.Chunk, random worldrandom.Source, origin blockPosition, dx, dz int, wetness float32, chance float64, height int, density float32) {
	column := scanDripstoneColumn(chunk, origin, f.searchRange, func(state int32) bool { return !dripstoneEmptyOrWater(state) })
	if !column.hasCeiling && !column.hasFloor {
		return
	}
	pool := random.NextFloat() < wetness
	if pool && column.hasFloor {
		position := blockPosition{origin.x, column.floor, origin.z}
		if f.canPlacePool(chunk, position) {
			setChunkBlock(chunk, position, f.water)
			column.floor--
		}
	}
	ceilingLength := 0
	if random.NextDouble() < chance && column.hasCeiling && block.NameFromState(getChunkBlock(chunk, blockPosition{origin.x, column.ceiling, origin.z})) != "minecraft:lava" {
		f.replaceLayer(chunk, random, blockPosition{origin.x, column.ceiling, origin.z}, true)
		maxHeight := height
		if column.hasFloor {
			maxHeight = min(height, column.ceiling-column.floor)
		}
		ceilingLength = f.tipHeight(random, dx, dz, density, maxHeight)
	}
	floorLength := 0
	if random.NextDouble() < chance && column.hasFloor && block.NameFromState(getChunkBlock(chunk, blockPosition{origin.x, column.floor, origin.z})) != "minecraft:lava" {
		f.replaceLayer(chunk, random, blockPosition{origin.x, column.floor, origin.z}, false)
		if column.hasCeiling {
			floorLength = max(0, ceilingLength+int(random.NextIntBound(int32(2*f.maxHeightDifference+1)))-f.maxHeightDifference)
		} else {
			floorLength = f.tipHeight(random, dx, dz, density, height)
		}
	}
	if column.hasCeiling && column.hasFloor && column.ceiling-ceilingLength <= column.floor+floorLength {
		lower := max(column.ceiling-ceilingLength, column.floor+1)
		upper := min(column.floor+floorLength, column.ceiling-1)
		split := lower + int(random.NextIntBound(int32(upper-lower+2)))
		ceilingLength = column.ceiling - split
		floorLength = split - 1 - column.floor
	}
	merged := random.NextBoolean() && ceilingLength > 0 && floorLength > 0 && column.hasCeiling && column.hasFloor && ceilingLength+floorLength == column.ceiling-column.floor-1
	if column.hasCeiling {
		f.pointed.grow(chunk, blockPosition{origin.x, column.ceiling - 1, origin.z}, false, ceilingLength, merged)
	}
	if column.hasFloor {
		f.pointed.grow(chunk, blockPosition{origin.x, column.floor + 1, origin.z}, true, floorLength, merged)
	}
}

// tipHeight samples the density gate and biased Gaussian tip length.
func (f *dripstoneClusterFeature) tipHeight(random worldrandom.Source, dx, dz int, density float32, maxHeight int) int {
	if random.NextFloat() > density {
		return 0
	}
	distance := absInt(dx) + absInt(dz)
	bias := float32(dripstoneClampedMapDouble(float64(distance), float64(f.heightDistance), float64(maxHeight)/2))
	value := bias + float32(random.NextGaussian())*float32(f.heightDeviation)
	return int(max(float32(0), min(float32(maxHeight), value)))
}

// replaceLayer extends a dripstone base until an ineligible state stops it.
func (f *dripstoneClusterFeature) replaceLayer(chunk *section.Chunk, random worldrandom.Source, origin blockPosition, up bool) {
	thickness := f.layer.sample(random)
	step := -1
	if up {
		step = 1
	}
	for index := 0; index < thickness; index++ {
		position := blockPosition{origin.x, origin.y + index*step, origin.z}
		state := getChunkBlock(chunk, position)
		if !f.pointed.replaceable[block.BlockIDFromState(state)] {
			return
		}
		setChunkBlock(chunk, position, f.pointed.base)
	}
}

// canPlacePool checks the five enclosing cells and water above a possible pool.
func (f *dripstoneClusterFeature) canPlacePool(chunk *section.Chunk, origin blockPosition) bool {
	state := getChunkBlock(chunk, origin)
	name := block.NameFromState(state)
	if name == "minecraft:water" || name == "minecraft:dripstone_block" || name == "minecraft:pointed_dripstone" {
		return false
	}
	if generationBlockTraits(getChunkBlock(chunk, blockPosition{origin.x, origin.y + 1, origin.z}))&featureWaterTrait != 0 {
		return false
	}
	for _, offset := range dripstoneHorizontal {
		if !f.poolNeighbor(chunk, blockPosition{origin.x + offset.x, origin.y, origin.z + offset.z}) {
			return false
		}
	}
	return f.poolNeighbor(chunk, blockPosition{origin.x, origin.y - 1, origin.z})
}

// poolNeighbor accepts overworld base stone or a water fluid state.
func (f *dripstoneClusterFeature) poolNeighbor(chunk *section.Chunk, position blockPosition) bool {
	state := getChunkBlock(chunk, position)
	return f.baseStone[block.BlockIDFromState(state)] || generationBlockTraits(state)&featureWaterTrait != 0
}
