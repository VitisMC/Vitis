package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

var adjacentDirections = [][3]int{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}}

type glowstoneFeature struct{ state int32 }

func (f *glowstoneFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !isAirAt(chunk, origin) {
		return false
	}
	above := block.NameFromState(getChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y + 1, z: origin.z}))
	if above != "minecraft:netherrack" && above != "minecraft:basalt" && above != "minecraft:blackstone" {
		return false
	}
	setChunkBlock(chunk, origin, f.state)
	for range 1500 {
		position := blockPosition{x: origin.x + int(random.NextIntBound(8)) - int(random.NextIntBound(8)), y: origin.y - int(random.NextIntBound(12)), z: origin.z + int(random.NextIntBound(8)) - int(random.NextIntBound(8))}
		if !isAirAt(chunk, position) {
			continue
		}
		neighbors := 0
		for _, offset := range adjacentDirections {
			if block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x + offset[0], y: position.y + offset[1], z: position.z + offset[2]})) == "minecraft:glowstone" {
				neighbors++
				if neighbors > 1 {
					break
				}
			}
		}
		if neighbors == 1 {
			setChunkBlock(chunk, position, f.state)
		}
	}
	return true
}

type blueIceFeature struct{ blue int32 }

func (f *blueIceFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if origin.y > system.seaLevel-1 {
		return false
	}
	if !blockfluid.IsWater(getChunkBlock(chunk, origin)) && !blockfluid.IsWater(getChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y - 1, z: origin.z})) {
		return false
	}
	packed := false
	for index, offset := range adjacentDirections {
		if index == 0 {
			continue
		}
		if block.NameFromState(getChunkBlock(chunk, blockPosition{x: origin.x + offset[0], y: origin.y + offset[1], z: origin.z + offset[2]})) == "minecraft:packed_ice" {
			packed = true
			break
		}
	}
	if !packed {
		return false
	}
	setChunkBlock(chunk, origin, f.blue)
	for range 200 {
		yOffset := int(random.NextIntBound(5)) - int(random.NextIntBound(6))
		bound := 3
		if yOffset < 2 {
			bound += yOffset / 2
		}
		if bound < 1 {
			continue
		}
		position := blockPosition{x: origin.x + int(random.NextIntBound(int32(bound))) - int(random.NextIntBound(int32(bound))), y: origin.y + yOffset, z: origin.z + int(random.NextIntBound(int32(bound))) - int(random.NextIntBound(int32(bound)))}
		name := block.NameFromState(getChunkBlock(chunk, position))
		if !block.IsAir(getChunkBlock(chunk, position)) && !blockfluid.IsWater(getChunkBlock(chunk, position)) && name != "minecraft:packed_ice" && name != "minecraft:ice" {
			continue
		}
		for _, offset := range adjacentDirections {
			if block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x + offset[0], y: position.y + offset[1], z: position.z + offset[2]})) == "minecraft:blue_ice" {
				setChunkBlock(chunk, position, f.blue)
				break
			}
		}
	}
	return true
}

type vinesFeature struct{ base int32 }

func (f *vinesFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, _ worldrandom.Source, origin blockPosition) bool {
	if !isAirAt(chunk, origin) {
		return false
	}
	directions := []struct {
		offset   [3]int
		property string
	}{{[3]int{0, 1, 0}, "up"}, {[3]int{0, 0, -1}, "north"}, {[3]int{0, 0, 1}, "south"}, {[3]int{-1, 0, 0}, "west"}, {[3]int{1, 0, 0}, "east"}}
	base, _ := block.BlockStateFromID(f.base)
	for _, direction := range directions {
		neighbor := blockPosition{x: origin.x + direction.offset[0], y: origin.y + direction.offset[1], z: origin.z + direction.offset[2]}
		if !block.IsSolid(getChunkBlock(chunk, neighbor)) {
			continue
		}
		state, err := base.WithProperty(direction.property, "true")
		if err == nil {
			setChunkBlock(chunk, origin, state.ID())
			return true
		}
	}
	return false
}

type bambooFeature struct {
	probability                                   float32
	plantable, dirt                               map[int32]struct{}
	trunk, finalLarge, topLarge, topSmall, podzol int32
}

func (f *bambooFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if !isAirAt(chunk, origin) {
		return false
	}
	if _, ok := f.plantable[getChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y - 1, z: origin.z})]; !ok {
		return true
	}
	height := int(random.NextIntBound(12)) + 5
	if random.NextFloat() < f.probability {
		radius := int(random.NextIntBound(4)) + 1
		for x := origin.x - radius; x <= origin.x+radius; x++ {
			for z := origin.z - radius; z <= origin.z+radius; z++ {
				dx, dz := x-origin.x, z-origin.z
				if dx*dx+dz*dz > radius*radius {
					continue
				}
				surface, ok := featureHeightmap(chunk, x, z, "WORLD_SURFACE_WG")
				if !ok {
					continue
				}
				position := blockPosition{x: x, y: surface - 1, z: z}
				if _, ok := f.dirt[getChunkBlock(chunk, position)]; ok {
					setChunkBlock(chunk, position, f.podzol)
				}
			}
		}
	}
	position := origin
	for count := 0; count < height && isAirAt(chunk, position); count++ {
		setChunkBlock(chunk, position, f.trunk)
		position.y++
	}
	if position.y-origin.y >= 3 {
		setChunkBlock(chunk, position, f.finalLarge)
		position.y--
		setChunkBlock(chunk, position, f.topLarge)
		position.y--
		setChunkBlock(chunk, position, f.topSmall)
	}
	return true
}

func decodeSpecialFeature(snapshot *datapack.Snapshot, kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	switch kind {
	case "minecraft:glowstone_blob":
		state, err := block.DefaultBlockState("minecraft:glowstone")
		return &glowstoneFeature{state: state.ID()}, true, err
	case "minecraft:blue_ice":
		state, err := block.DefaultBlockState("minecraft:blue_ice")
		return &blueIceFeature{blue: state.ID()}, true, err
	case "minecraft:ice_spike":
		feature, err := newIceSpikeFeature(snapshot)
		return feature, true, err
	case "minecraft:vines":
		state, err := block.DefaultBlockState("minecraft:vine")
		return &vinesFeature{base: state.ID()}, true, err
	case "minecraft:bamboo":
		var value struct {
			Config struct {
				Probability float32 `json:"probability"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		if value.Config.Probability < 0 || value.Config.Probability > 1 {
			return nil, true, fmt.Errorf("invalid bamboo probability")
		}
		resolve := func(tag string) (map[int32]struct{}, error) {
			names, err := snapshot.ResolveTag("block", tag)
			if err != nil {
				return nil, err
			}
			states := make(map[int32]struct{})
			for _, name := range names {
				info := block.InfoByName(name)
				if info == nil {
					return nil, fmt.Errorf("unknown block %s", name)
				}
				for id := info.MinStateID; id <= info.MaxStateID; id++ {
					states[id] = struct{}{}
				}
			}
			return states, nil
		}
		plantable, err := resolve("minecraft:bamboo_plantable_on")
		if err != nil {
			return nil, true, err
		}
		dirt, err := resolve("minecraft:dirt")
		if err != nil {
			return nil, true, err
		}
		state := func(properties map[string]string) (int32, error) {
			s, err := block.BlockStateFromProperties("minecraft:bamboo", properties)
			return s.ID(), err
		}
		trunk, err := state(map[string]string{"age": "1", "leaves": "none", "stage": "0"})
		if err != nil {
			return nil, true, err
		}
		finalLarge, err := state(map[string]string{"age": "1", "leaves": "large", "stage": "1"})
		if err != nil {
			return nil, true, err
		}
		topLarge, err := state(map[string]string{"age": "1", "leaves": "large", "stage": "0"})
		if err != nil {
			return nil, true, err
		}
		topSmall, err := state(map[string]string{"age": "1", "leaves": "small", "stage": "0"})
		if err != nil {
			return nil, true, err
		}
		podzol, err := block.DefaultBlockState("minecraft:podzol")
		if err != nil {
			return nil, true, err
		}
		return &bambooFeature{probability: value.Config.Probability, plantable: plantable, dirt: dirt, trunk: trunk, finalLarge: finalLarge, topLarge: topLarge, topSmall: topSmall, podzol: podzol.ID()}, true, nil
	default:
		return nil, false, nil
	}
}
