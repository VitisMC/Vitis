package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type freezeTopLayerFeature struct {
	ice, snow int32
}

func (f *freezeTopLayerFeature) placeFeature(system *featureSystem, chunk *section.Chunk, _ worldrandom.Source, origin blockPosition) bool {
	biomes := newCachedBiomeSource(system.biomes)
	biomes.beginChunk(int(chunk.X)*16, int(chunk.Z)*16, chunk.MinY(), chunk.MaxY()-chunk.MinY()+1)
	return f.placeWithBiomes(chunk, origin, biomes)
}

// placeWithBiomes reuses the generator's climate samples when freezing a surface.
func (f *freezeTopLayerFeature) placeWithBiomes(chunk *section.Chunk, origin blockPosition, biomes biomeSource) bool {
	for x := origin.x; x < origin.x+16; x++ {
		for z := origin.z; z < origin.z+16; z++ {
			y, ok := featureHeightmap(chunk, x, z, "MOTION_BLOCKING")
			if !ok {
				continue
			}
			biome := generatedbiome.BiomeByID(biomes.sample(x>>2, y>>2, z>>2))
			if biome == nil || biome.Temperature >= 0.15 {
				continue
			}
			below := blockPosition{x: x, y: y - 1, z: z}
			if blockfluid.IsWater(getChunkBlock(chunk, below)) {
				setChunkBlock(chunk, below, f.ice)
			}
			position := blockPosition{x: x, y: y, z: z}
			if isAirAt(chunk, position) && block.IsSolid(getChunkBlock(chunk, below)) {
				setChunkBlock(chunk, position, f.snow)
				state, err := block.BlockStateFromID(getChunkBlock(chunk, below))
				if err == nil && state.Properties()["snowy"] != "" {
					if snowy, err := state.WithProperty("snowy", "true"); err == nil {
						setChunkBlock(chunk, below, snowy.ID())
					}
				}
			}
		}
	}
	return true
}

type endPlatformFeature struct{ obsidian int32 }

func (f *endPlatformFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, _ worldrandom.Source, origin blockPosition) bool {
	for z := origin.z - 2; z <= origin.z+2; z++ {
		for x := origin.x - 2; x <= origin.x+2; x++ {
			for y := origin.y - 1; y < origin.y+3; y++ {
				state := int32(0)
				if y == origin.y-1 {
					state = f.obsidian
				}
				setChunkBlock(chunk, blockPosition{x: x, y: y, z: z}, state)
			}
		}
	}
	return true
}

type voidStartPlatformFeature struct {
	stone, cobblestone int32
}

func (f *voidStartPlatformFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, _ worldrandom.Source, origin blockPosition) bool {
	center := blockPosition{x: 8, y: origin.y + 3, z: 8}
	chunkX, chunkZ := int(chunk.X), int(chunk.Z)
	if absInt(chunkX) > 1 || absInt(chunkZ) > 1 {
		return true
	}
	for z := chunkZ * 16; z < chunkZ*16+16; z++ {
		for x := chunkX * 16; x < chunkX*16+16; x++ {
			if maxIntValue(absInt(x-center.x), absInt(z-center.z)) > 16 {
				continue
			}
			state := f.stone
			if x == center.x && z == center.z {
				state = f.cobblestone
			}
			setChunkBlock(chunk, blockPosition{x: x, y: center.y, z: z}, state)
		}
	}
	return true
}

type underwaterMagmaFeature struct {
	floorSearchRange int
	probability      float32
	radius           int
	magma            int32
}

func (f *underwaterMagmaFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	floorY := 0
	found := false
	position := origin
	for step := 0; step <= f.floorSearchRange; step++ {
		if !blockfluid.IsWater(getChunkBlock(chunk, position)) {
			break
		}
		below := blockPosition{x: position.x, y: position.y - 1, z: position.z}
		if !blockfluid.IsWater(getChunkBlock(chunk, below)) {
			floorY, found = below.y, true
			break
		}
		position.y--
	}
	if !found {
		return false
	}
	placed := false
	for y := floorY - f.radius; y <= floorY+f.radius; y++ {
		for z := origin.z - f.radius; z <= origin.z+f.radius; z++ {
			for x := origin.x - f.radius; x <= origin.x+f.radius; x++ {
				candidate := blockPosition{x: x, y: y, z: z}
				if random.NextFloat() >= f.probability || !validMagmaFloor(chunk, candidate) {
					continue
				}
				setChunkBlock(chunk, candidate, f.magma)
				placed = true
			}
		}
	}
	return placed
}

func validMagmaFloor(chunk *section.Chunk, position blockPosition) bool {
	waterOrAir := func(position blockPosition) bool {
		state := getChunkBlock(chunk, position)
		return block.IsAir(state) || blockfluid.IsWater(state)
	}
	if waterOrAir(position) || waterOrAir(blockPosition{x: position.x, y: position.y - 1, z: position.z}) {
		return false
	}
	for _, offset := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		if waterOrAir(blockPosition{x: position.x + offset[0], y: position.y, z: position.z + offset[1]}) {
			return false
		}
	}
	return true
}

func decodeEnvironmentFeature(kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	state := func(name string) (int32, error) {
		id := block.DefaultStateID(name)
		if id < 0 {
			return 0, fmt.Errorf("unknown block %s", name)
		}
		return id, nil
	}
	switch kind {
	case "minecraft:freeze_top_layer":
		ice, err := state("minecraft:ice")
		if err != nil {
			return nil, true, err
		}
		snow, err := state("minecraft:snow")
		return &freezeTopLayerFeature{ice: ice, snow: snow}, true, err
	case "minecraft:end_platform":
		obsidian, err := state("minecraft:obsidian")
		return &endPlatformFeature{obsidian: obsidian}, true, err
	case "minecraft:void_start_platform":
		stone, err := state("minecraft:stone")
		if err != nil {
			return nil, true, err
		}
		cobblestone, err := state("minecraft:cobblestone")
		return &voidStartPlatformFeature{stone: stone, cobblestone: cobblestone}, true, err
	case "minecraft:underwater_magma":
		var value struct {
			Config struct {
				FloorSearchRange int     `json:"floor_search_range"`
				Probability      float32 `json:"placement_probability_per_valid_position"`
				Radius           int     `json:"placement_radius_around_floor"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		if value.Config.FloorSearchRange < 0 || value.Config.Radius < 0 || value.Config.Probability < 0 || value.Config.Probability > 1 {
			return nil, true, fmt.Errorf("invalid underwater magma configuration")
		}
		magma, err := state("minecraft:magma_block")
		return &underwaterMagmaFeature{floorSearchRange: value.Config.FloorSearchRange, probability: value.Config.Probability, radius: value.Config.Radius, magma: magma}, true, err
	default:
		return nil, false, nil
	}
}

func maxIntValue(a, b int) int {
	if a > b {
		return a
	}
	return b
}
