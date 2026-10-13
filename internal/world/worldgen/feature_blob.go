package worldgen

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type forestRockFeature struct {
	state   int32
	support map[int32]struct{}
}

func (f *forestRockFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	for origin.y > chunk.MinY()+3 {
		below := getChunkBlock(chunk, blockPosition{x: origin.x, y: origin.y - 1, z: origin.z})
		if !block.IsAir(below) {
			if _, ok := f.support[below]; ok {
				break
			}
		}
		origin.y--
	}
	if origin.y <= chunk.MinY()+3 {
		return false
	}
	for range 3 {
		xRadius, yRadius, zRadius := int(random.NextIntBound(2)), int(random.NextIntBound(2)), int(random.NextIntBound(2))
		radius := float32(xRadius+yRadius+zRadius)*float32(0.333) + float32(0.5)
		for z := origin.z - zRadius; z <= origin.z+zRadius; z++ {
			for y := origin.y - yRadius; y <= origin.y+yRadius; y++ {
				for x := origin.x - xRadius; x <= origin.x+xRadius; x++ {
					dx, dy, dz := x-origin.x, y-origin.y, z-origin.z
					if float64(dx*dx+dy*dy+dz*dz) <= float64(radius*radius) {
						setChunkBlock(chunk, blockPosition{x: x, y: y, z: z}, f.state)
					}
				}
			}
		}
		origin.x += -1 + int(random.NextIntBound(2))
		origin.y -= int(random.NextIntBound(2))
		origin.z += -1 + int(random.NextIntBound(2))
	}
	return true
}

type replaceBlobFeature struct {
	target, replacement int32
	radius              intProvider
}

func (f *replaceBlobFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if origin.y < chunk.MinY()+1 {
		origin.y = chunk.MinY() + 1
	}
	if origin.y > chunk.MaxY() {
		origin.y = chunk.MaxY()
	}
	for origin.y > chunk.MinY()+1 && block.NameFromState(getChunkBlock(chunk, origin)) != block.NameFromState(f.target) {
		origin.y--
	}
	if block.NameFromState(getChunkBlock(chunk, origin)) != block.NameFromState(f.target) {
		return false
	}
	rx, ry, rz := f.radius.sample(random), f.radius.sample(random), f.radius.sample(random)
	maximum := rx
	if ry > maximum {
		maximum = ry
	}
	if rz > maximum {
		maximum = rz
	}
	placed := false
	for dx := -rx; dx <= rx; dx++ {
		for dy := -ry; dy <= ry; dy++ {
			for dz := -rz; dz <= rz; dz++ {
				if absInt(dx)+absInt(dy)+absInt(dz) > maximum {
					continue
				}
				position := blockPosition{x: origin.x + dx, y: origin.y + dy, z: origin.z + dz}
				if block.NameFromState(getChunkBlock(chunk, position)) == block.NameFromState(f.target) {
					setChunkBlock(chunk, position, f.replacement)
					placed = true
				}
			}
		}
	}
	return placed
}

type endIslandFeature struct{ state int32 }

func (f *endIslandFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	radius := float32(random.NextIntBound(3)) + 4
	for yOffset := 0; radius > .5; yOffset-- {
		for xOffset := int(math.Floor(float64(-radius))); xOffset <= int(math.Ceil(float64(radius))); xOffset++ {
			for zOffset := int(math.Floor(float64(-radius))); zOffset <= int(math.Ceil(float64(radius))); zOffset++ {
				if float32(xOffset*xOffset+zOffset*zOffset) <= (radius+1)*(radius+1) {
					setChunkBlock(chunk, blockPosition{x: origin.x + xOffset, y: origin.y + yOffset, z: origin.z + zOffset}, f.state)
				}
			}
		}
		radius -= float32(random.NextIntBound(2)) + .5
	}
	return true
}

func decodeBlobFeature(snapshot *datapack.Snapshot, kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	switch kind {
	case "minecraft:forest_rock":
		var value struct {
			Config struct {
				State BlockState `json:"state"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		state, err := block.BlockStateFromProperties(value.Config.State.Name, value.Config.State.Properties)
		if err != nil {
			return nil, true, err
		}
		support := make(map[int32]struct{})
		for _, tag := range []string{"minecraft:dirt", "minecraft:base_stone_overworld"} {
			names, err := snapshot.ResolveTag("block", tag)
			if err != nil {
				return nil, true, err
			}
			for _, name := range names {
				info := block.InfoByName(name)
				if info == nil {
					return nil, true, fmt.Errorf("unknown support block %s", name)
				}
				for id := info.MinStateID; id <= info.MaxStateID; id++ {
					support[id] = struct{}{}
				}
			}
		}
		return &forestRockFeature{state: state.ID(), support: support}, true, nil
	case "minecraft:netherrack_replace_blobs":
		var value struct {
			Config struct {
				Target BlockState      `json:"target"`
				State  BlockState      `json:"state"`
				Radius json.RawMessage `json:"radius"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		target, err := block.BlockStateFromProperties(value.Config.Target.Name, value.Config.Target.Properties)
		if err != nil {
			return nil, true, err
		}
		replacement, err := block.BlockStateFromProperties(value.Config.State.Name, value.Config.State.Properties)
		if err != nil {
			return nil, true, err
		}
		radius, err := decodeIntProviderRange(value.Config.Radius, 0, 12)
		if err != nil {
			return nil, true, err
		}
		return &replaceBlobFeature{target: target.ID(), replacement: replacement.ID(), radius: radius}, true, nil
	case "minecraft:end_island":
		state, err := block.DefaultBlockState("minecraft:end_stone")
		if err != nil {
			return nil, true, err
		}
		return &endIslandFeature{state: state.ID()}, true, nil
	default:
		return nil, false, nil
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
