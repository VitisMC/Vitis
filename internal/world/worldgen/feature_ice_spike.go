package worldgen

import (
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type iceSpikeFeature struct {
	dirt                 map[int32]struct{}
	snow, ice, packedIce int32
}

// newIceSpikeFeature resolves the states used by ice spike placement.
func newIceSpikeFeature(snapshot *datapack.Snapshot) (*iceSpikeFeature, error) {
	names, err := snapshot.ResolveTag("block", "minecraft:dirt")
	if err != nil {
		return nil, err
	}
	dirt := make(map[int32]struct{})
	for _, name := range names {
		info := block.InfoByName(name)
		if info == nil {
			return nil, fmt.Errorf("unknown dirt block %s", name)
		}
		for id := info.MinStateID; id <= info.MaxStateID; id++ {
			dirt[id] = struct{}{}
		}
	}
	stateID := func(name string) (int32, error) {
		state, err := block.DefaultBlockState(name)
		if err != nil {
			return 0, err
		}
		return state.ID(), nil
	}
	snow, err := stateID("minecraft:snow_block")
	if err != nil {
		return nil, err
	}
	ice, err := stateID("minecraft:ice")
	if err != nil {
		return nil, err
	}
	packed, err := stateID("minecraft:packed_ice")
	if err != nil {
		return nil, err
	}
	return &iceSpikeFeature{dirt: dirt, snow: snow, ice: ice, packedIce: packed}, nil
}

// replaceable reports whether a spike can replace the existing block.
func (f *iceSpikeFeature) replaceable(state int32, includePacked bool) bool {
	if block.IsAir(state) || state == f.snow || state == f.ice || (includePacked && state == f.packedIce) {
		return true
	}
	_, ok := f.dirt[state]
	return ok
}

// placeFeature grows an ice spike and its roots from a snow block.
func (f *iceSpikeFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	for isAirAt(chunk, origin) && origin.y > chunk.MinY()+2 {
		origin.y--
	}
	if getChunkBlock(chunk, origin) != f.snow {
		return false
	}
	origin.y += int(random.NextIntBound(4))
	height := int(random.NextIntBound(4)) + 7
	radius := height/4 + int(random.NextIntBound(2))
	if radius > 1 && random.NextIntBound(60) == 0 {
		origin.y += 10 + int(random.NextIntBound(30))
	}

	for layer := 0; layer < height; layer++ {
		scaledRadius := (1 - float32(layer)/float32(height)) * float32(radius)
		extent := int(math.Ceil(float64(scaledRadius)))
		for dx := -extent; dx <= extent; dx++ {
			xDistance := float32(abs(dx)) - .25
			for dz := -extent; dz <= extent; dz++ {
				zDistance := float32(abs(dz)) - .25
				if (dx != 0 || dz != 0) && xDistance*xDistance+zDistance*zDistance > scaledRadius*scaledRadius {
					continue
				}
				if (dx == -extent || dx == extent || dz == -extent || dz == extent) && random.NextFloat() > .75 {
					continue
				}
				position := blockPosition{x: origin.x + dx, y: origin.y + layer, z: origin.z + dz}
				if f.replaceable(getChunkBlock(chunk, position), false) {
					setChunkBlock(chunk, position, f.packedIce)
				}
				if layer != 0 && extent > 1 {
					position.y = origin.y - layer
					if f.replaceable(getChunkBlock(chunk, position), false) {
						setChunkBlock(chunk, position, f.packedIce)
					}
				}
			}
		}
	}

	rootRadius := min(max(radius-1, 0), 1)
	for dx := -rootRadius; dx <= rootRadius; dx++ {
		for dz := -rootRadius; dz <= rootRadius; dz++ {
			position := blockPosition{x: origin.x + dx, y: origin.y - 1, z: origin.z + dz}
			remaining := 50
			if abs(dx) == 1 && abs(dz) == 1 {
				remaining = int(random.NextIntBound(5))
			}
			for position.y > 50 && f.replaceable(getChunkBlock(chunk, position), true) {
				setChunkBlock(chunk, position, f.packedIce)
				position.y--
				remaining--
				if remaining <= 0 {
					position.y -= int(random.NextIntBound(5)) + 1
					remaining = int(random.NextIntBound(5))
				}
			}
		}
	}
	return true
}
