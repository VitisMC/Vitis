package worldgen

import (
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// desertWellFeature retains resolved blocks for sandstone wells and archaeology.
type desertWellFeature struct {
	sand, stone, slab, water, suspicious int32
}

// decodeSmallStructureFeature compiles small structure-like configured features.
func decodeSmallStructureFeature(snapshot *datapack.Snapshot, kind string) (configuredFeature, bool, error) {
	switch kind {
	case "minecraft:desert_well":
		states, err := resolveFeatureBlocks("minecraft:sand", "minecraft:sandstone", "minecraft:sandstone_slab", "minecraft:water", "minecraft:suspicious_sand")
		if err != nil {
			return nil, true, err
		}
		return &desertWellFeature{states[0], states[1], states[2], states[3], states[4]}, true, nil
	case "minecraft:monster_room":
		return decodeMonsterRoomFeature(snapshot)
	default:
		return nil, false, nil
	}
}

// resolveFeatureBlocks rejects unavailable blocks before a feature enters generation.
func resolveFeatureBlocks(names ...string) ([]int32, error) {
	states := make([]int32, len(names))
	for index, name := range names {
		state := block.DefaultStateID(name)
		if state < 0 {
			return nil, fmt.Errorf("feature requires unknown block %s", name)
		}
		states[index] = state
	}
	return states, nil
}

// placeFeature validates support and builds a well without consuming RNG on rejected sites.
func (f *desertWellFeature) placeFeature(_ *featureSystem, target *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	origin.y++
	for origin.y > target.MinY()+2 && isAirAt(target, origin) {
		origin.y--
	}
	if getChunkBlock(target, origin) != f.sand {
		return false
	}
	for dx := -2; dx <= 2; dx++ {
		for dz := -2; dz <= 2; dz++ {
			if isAirAt(target, blockPosition{origin.x + dx, origin.y - 1, origin.z + dz}) && isAirAt(target, blockPosition{origin.x + dx, origin.y - 2, origin.z + dz}) {
				return false
			}
		}
	}
	for dy := -2; dy <= 0; dy++ {
		for dx := -2; dx <= 2; dx++ {
			for dz := -2; dz <= 2; dz++ {
				setChunkBlock(target, blockPosition{origin.x + dx, origin.y + dy, origin.z + dz}, f.stone)
			}
		}
	}
	basin := [...]blockPosition{origin, {origin.x + 1, origin.y, origin.z}, {origin.x, origin.y, origin.z + 1}, {origin.x - 1, origin.y, origin.z}, {origin.x, origin.y, origin.z - 1}}
	for _, position := range basin {
		setChunkBlock(target, position, f.water)
		position.y--
		setChunkBlock(target, position, f.sand)
	}
	for dx := -2; dx <= 2; dx++ {
		for dz := -2; dz <= 2; dz++ {
			if absInt(dx) == 2 || absInt(dz) == 2 {
				setChunkBlock(target, blockPosition{origin.x + dx, origin.y + 1, origin.z + dz}, f.stone)
			}
		}
	}
	for _, offset := range [][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}} {
		setChunkBlock(target, blockPosition{origin.x + offset[0], origin.y + 1, origin.z + offset[1]}, f.slab)
	}
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			state := f.slab
			if dx == 0 && dz == 0 {
				state = f.stone
			}
			setChunkBlock(target, blockPosition{origin.x + dx, origin.y + 4, origin.z + dz}, state)
		}
	}
	for dy := 1; dy <= 3; dy++ {
		for _, offset := range [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
			setChunkBlock(target, blockPosition{origin.x + offset[0], origin.y + dy, origin.z + offset[1]}, f.stone)
		}
	}
	for depth := 1; depth <= 2; depth++ {
		position := basin[random.NextIntBound(int32(len(basin)))]
		position.y -= depth
		setChunkBlock(target, position, f.suspicious)
		root := featureBlockEntityNBT("minecraft:brushable_block", position).
			PutString("LootTable", "minecraft:archaeology/desert_well").PutLong("LootTableSeed", packedBlockPosition(position))
		target.AddBlockEntity(blockentity.NewRawBlockEntity(root))
	}
	return true
}

// featureBlockEntityNBT creates the common persistent identity of generated block entities.
func featureBlockEntityNBT(kind string, position blockPosition) *nbt.Compound {
	return nbt.NewCompound().PutString("id", kind).PutInt("x", int32(position.x)).PutInt("y", int32(position.y)).PutInt("z", int32(position.z))
}

// packedBlockPosition reproduces the signed 26/12/26-bit coordinate representation.
func packedBlockPosition(position blockPosition) int64 {
	return int64((uint64(position.x)&0x3ffffff)<<38 | (uint64(position.z)&0x3ffffff)<<12 | uint64(position.y)&0xfff)
}
