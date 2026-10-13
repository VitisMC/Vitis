package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// lakeFeature stores the resolved vanilla lava lake providers and block protections.
type lakeFeature struct {
	fluid, barrier  stateProvider
	caveAir         int32
	protected       [genblock.BlockCount]bool
	barrierExcluded [genblock.BlockCount]bool
}

// decodeLakeFeature compiles the versioned lake configuration and its replacement tags.
func decodeLakeFeature(snapshot *datapack.Snapshot, raw json.RawMessage) (*lakeFeature, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("lake requires block tags")
	}
	var value struct {
		Config struct {
			Fluid   json.RawMessage `json:"fluid"`
			Barrier json.RawMessage `json:"barrier"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	fluid, err := decodeStateProvider(value.Config.Fluid)
	if err != nil {
		return nil, fmt.Errorf("lake fluid: %w", err)
	}
	constant, ok := fluid.(simpleStateProvider)
	if !ok || blockfluid.GetFluidType(int32(constant)) != blockfluid.FluidTypeLava {
		return nil, fmt.Errorf("lake fluid must be a fixed lava state")
	}
	barrier, err := decodeStateProvider(value.Config.Barrier)
	if err != nil {
		return nil, fmt.Errorf("lake barrier: %w", err)
	}
	states, err := resolveFeatureBlocks("minecraft:cave_air")
	if err != nil {
		return nil, err
	}
	f := &lakeFeature{fluid: fluid, barrier: barrier, caveAir: states[0]}
	for _, tag := range []struct {
		name   string
		target *[genblock.BlockCount]bool
	}{
		{"minecraft:features_cannot_replace", &f.protected},
		{"minecraft:lava_pool_stone_cannot_replace", &f.barrierExcluded},
	} {
		names, err := snapshot.ResolveTag("block", tag.name)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			info := block.InfoByName(normalizeResourceID(name))
			if info == nil {
				return nil, fmt.Errorf("lake tag %s references unknown block %s", tag.name, name)
			}
			tag.target[info.ID] = true
		}
	}
	return f, nil
}

// lakeMaskIndex follows the feature's X, Z, Y traversal order.
func lakeMaskIndex(x, z, y int) int { return (x*16+z)*8 + y }

// lakeShell reports an empty cell sharing a face with the carved lake.
func lakeShell(mask *[2048]bool, x, z, y int) bool {
	if mask[lakeMaskIndex(x, z, y)] {
		return false
	}
	return x < 15 && mask[lakeMaskIndex(x+1, z, y)] ||
		x > 0 && mask[lakeMaskIndex(x-1, z, y)] ||
		z < 15 && mask[lakeMaskIndex(x, z+1, y)] ||
		z > 0 && mask[lakeMaskIndex(x, z-1, y)] ||
		y < 7 && mask[lakeMaskIndex(x, z, y+1)] ||
		y > 0 && mask[lakeMaskIndex(x, z, y-1)]
}

// placeFeature validates the entire shell before changing terrain and preserves random draw order.
func (f *lakeFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	if origin.y <= chunk.MinY()+4 {
		return false
	}
	origin.y -= 4
	var mask [2048]bool
	count := int(random.NextIntBound(4)) + 4
	for range count {
		radiusX := random.NextDouble()*6 + 3
		radiusY := random.NextDouble()*4 + 2
		radiusZ := random.NextDouble()*6 + 3
		centerX := random.NextDouble()*(16-radiusX-2) + 1 + radiusX/2
		centerY := random.NextDouble()*(8-radiusY-4) + 2 + radiusY/2
		centerZ := random.NextDouble()*(16-radiusZ-2) + 1 + radiusZ/2
		for x := 1; x < 15; x++ {
			for z := 1; z < 15; z++ {
				for y := 1; y < 7; y++ {
					dx := (float64(x) - centerX) / (radiusX / 2)
					dy := (float64(y) - centerY) / (radiusY / 2)
					dz := (float64(z) - centerZ) / (radiusZ / 2)
					if dx*dx+dy*dy+dz*dz < 1 {
						mask[lakeMaskIndex(x, z, y)] = true
					}
				}
			}
		}
	}
	fluid := f.fluid.state(system, chunk, random, origin)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 0; y < 8; y++ {
				if !lakeShell(&mask, x, z, y) {
					continue
				}
				p := blockPosition{origin.x + x, origin.y + y, origin.z + z}
				state := getChunkBlock(chunk, p)
				if y >= 4 && generationBlockTraits(state)&featureLiquidTrait != 0 ||
					y < 4 && generationBlockTraits(state)&featureSolidTrait == 0 && state != fluid {
					return false
				}
			}
		}
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 0; y < 8; y++ {
				if !mask[lakeMaskIndex(x, z, y)] {
					continue
				}
				p := blockPosition{origin.x + x, origin.y + y, origin.z + z}
				if !f.protected[block.BlockIDFromState(getChunkBlock(chunk, p))] {
					state := fluid
					if y >= 4 {
						state = f.caveAir
					}
					setChunkBlock(chunk, p, state)
				}
			}
		}
	}
	barrier := f.barrier.state(system, chunk, random, origin)
	if !block.IsAir(barrier) {
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				for y := 0; y < 8; y++ {
					if !lakeShell(&mask, x, z, y) || y >= 4 && random.NextIntBound(2) == 0 {
						continue
					}
					p := blockPosition{origin.x + x, origin.y + y, origin.z + z}
					state := getChunkBlock(chunk, p)
					if generationBlockTraits(state)&featureSolidTrait != 0 && !f.barrierExcluded[block.BlockIDFromState(state)] {
						setChunkBlock(chunk, p, barrier)
					}
				}
			}
		}
	}
	return true
}
