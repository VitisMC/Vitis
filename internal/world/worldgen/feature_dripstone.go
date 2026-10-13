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

var dripstoneHorizontal = [...]blockPosition{{z: -1}, {x: 1}, {z: 1}, {x: -1}}
var dripstoneDirections = [...]blockPosition{{y: -1}, {y: 1}, {z: -1}, {z: 1}, {x: -1}, {x: 1}}

// pointedDripstoneFeature retains tag membership and resolved direction/thickness states.
type pointedDripstoneFeature struct {
	spread, radius2, radius3, taller float32
	base                             int32
	replaceable                      [genblock.BlockCount]bool
	states                           [2][5][2]int32
}

// decodePointedDripstoneFeature compiles the nested feature and versioned replaceable tag.
func decodePointedDripstoneFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*pointedDripstoneFeature, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("pointed dripstone requires block tags")
	}
	var value struct {
		Config struct {
			Spread  *float32 `json:"chance_of_directional_spread"`
			Radius2 *float32 `json:"chance_of_spread_radius2"`
			Radius3 *float32 `json:"chance_of_spread_radius3"`
			Taller  *float32 `json:"chance_of_taller_dripstone"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	f := &pointedDripstoneFeature{spread: .7, radius2: .5, radius3: .5, taller: .2}
	for _, chance := range []struct {
		input  *float32
		target *float32
	}{
		{value.Config.Spread, &f.spread},
		{value.Config.Radius2, &f.radius2},
		{value.Config.Radius3, &f.radius3},
		{value.Config.Taller, &f.taller},
	} {
		if chance.input == nil {
			continue
		}
		if *chance.input < 0 || *chance.input > 1 {
			return nil, fmt.Errorf("pointed dripstone probability out of range")
		}
		*chance.target = *chance.input
	}
	names, err := snapshot.ResolveTag("block", "minecraft:dripstone_replaceable_blocks")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		info := block.InfoByName(normalizeResourceID(name))
		if info == nil {
			return nil, fmt.Errorf("unknown dripstone replaceable block %s", name)
		}
		f.replaceable[info.ID] = true
	}
	f.base = block.DefaultStateID("minecraft:dripstone_block")
	if f.base < 0 {
		return nil, fmt.Errorf("missing dripstone block")
	}
	for direction, name := range [...]string{"down", "up"} {
		for thickness, thicknessName := range [...]string{"base", "middle", "frustum", "tip", "tip_merge"} {
			for wet, wetName := range [...]string{"false", "true"} {
				state, err := block.BlockStateFromProperties("minecraft:pointed_dripstone", map[string]string{
					"vertical_direction": name, "thickness": thicknessName, "waterlogged": wetName,
				})
				if err != nil {
					return nil, err
				}
				f.states[direction][thickness][wet] = state.ID()
			}
		}
	}
	return f, nil
}

// dripstoneBase reports the replaceable substrate or an already converted base.
func (f *pointedDripstoneFeature) dripstoneBase(state int32) bool {
	return state == f.base || f.replaceable[block.BlockIDFromState(state)]
}

// placeBase converts eligible stone without consuming randomness on rejected blocks.
func (f *pointedDripstoneFeature) placeBase(chunk *section.Chunk, position blockPosition) {
	if f.replaceable[block.BlockIDFromState(getChunkBlock(chunk, position))] {
		setChunkBlock(chunk, position, f.base)
	}
}

// placeFeature grows a small stalactite or stalagmite with its branching base patch.
func (f *pointedDripstoneFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	above := blockPosition{origin.x, origin.y + 1, origin.z}
	below := blockPosition{origin.x, origin.y - 1, origin.z}
	ceiling := f.dripstoneBase(getChunkBlock(chunk, above))
	floor := f.dripstoneBase(getChunkBlock(chunk, below))
	if !ceiling && !floor {
		return false
	}
	up := !ceiling
	if ceiling && floor {
		up = !random.NextBoolean()
	}
	base := above
	step := -1
	if up {
		base, step = below, 1
	}
	f.placeBase(chunk, base)
	for _, offset := range dripstoneHorizontal {
		if random.NextFloat() > f.spread {
			continue
		}
		neighbor := blockPosition{base.x + offset.x, base.y, base.z + offset.z}
		f.placeBase(chunk, neighbor)
		if random.NextFloat() > f.radius2 {
			continue
		}
		direction := dripstoneDirections[random.NextIntBound(6)]
		neighbor = blockPosition{neighbor.x + direction.x, neighbor.y + direction.y, neighbor.z + direction.z}
		f.placeBase(chunk, neighbor)
		if random.NextFloat() > f.radius3 {
			continue
		}
		direction = dripstoneDirections[random.NextIntBound(6)]
		neighbor = blockPosition{neighbor.x + direction.x, neighbor.y + direction.y, neighbor.z + direction.z}
		f.placeBase(chunk, neighbor)
	}
	length := 1
	next := blockPosition{origin.x, origin.y + step, origin.z}
	if random.NextFloat() < f.taller && dripstoneEmptyOrWater(getChunkBlock(chunk, next)) {
		length = 2
	}
	f.grow(chunk, origin, up, length, false)
	return true
}

// grow follows the base-to-tip thickness sequence and preserves waterlogging.
func (f *pointedDripstoneFeature) grow(chunk *section.Chunk, origin blockPosition, up bool, length int, merged bool) {
	step, direction := -1, 0
	if up {
		step, direction = 1, 1
	}
	if !f.dripstoneBase(getChunkBlock(chunk, blockPosition{origin.x, origin.y - step, origin.z})) {
		return
	}
	for index := 0; index < length; index++ {
		thickness := 3
		if index == length-1 {
			if merged {
				thickness = 4
			}
		} else if length >= 3 && index == 0 {
			thickness = 0
		} else if length >= 3 && index < length-2 {
			thickness = 1
		} else {
			thickness = 2
		}
		position := blockPosition{origin.x, origin.y + index*step, origin.z}
		wet := 0
		if generationBlockTraits(getChunkBlock(chunk, position))&featureWaterTrait != 0 {
			wet = 1
		}
		setChunkBlock(chunk, position, f.states[direction][thickness][wet])
	}
}

// dripstoneEmptyOrWater matches the support scan's air and pure-water states.
func dripstoneEmptyOrWater(state int32) bool {
	return block.IsAir(state) || block.NameFromState(state) == "minecraft:water"
}
