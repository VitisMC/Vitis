package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// bonusChestFeature retains the states used by the optional spawn chest.
type bonusChestFeature struct {
	chest, torch int32
}

// decodeBonusChestFeature validates the empty configuration and resolved states.
func decodeBonusChestFeature(raw json.RawMessage) (*bonusChestFeature, error) {
	var value struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if len(value.Config) != 0 {
		return nil, fmt.Errorf("bonus chest has unexpected configuration")
	}
	f := &bonusChestFeature{chest: block.DefaultStateID("minecraft:chest"), torch: block.DefaultStateID("minecraft:torch")}
	if f.chest < 0 || f.torch < 0 {
		return nil, fmt.Errorf("bonus chest states are unavailable")
	}
	return f, nil
}

// placeFeature shuffles each chunk axis before selecting the first open surface.
func (f *bonusChestFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	baseX, baseZ := origin.x&^15, origin.z&^15
	var xOrder, zOrder [16]int
	for index := range xOrder {
		xOrder[index], zOrder[index] = baseX+index, baseZ+index
	}
	shuffleBonusChestAxis(random, &xOrder)
	shuffleBonusChestAxis(random, &zOrder)
	for _, x := range xOrder {
		for _, z := range zOrder {
			y, _ := featureHeightmap(chunk, x, z, "MOTION_BLOCKING_NO_LEAVES")
			if y < chunk.MinY() || y > chunk.MaxY() {
				continue
			}
			position := blockPosition{x, y, z}
			state := getChunkBlock(chunk, position)
			if !block.IsAir(state) && generationBlockTraits(state)&(63<<6) != 0 {
				continue
			}
			setChunkBlock(chunk, position, f.chest)
			if getChunkBlock(chunk, position) != f.chest {
				continue
			}
			root := featureBlockEntityNBT("minecraft:chest", position).
				PutString("LootTable", "minecraft:chests/spawn_bonus_chest").PutLong("LootTableSeed", random.NextLong())
			chunk.AddBlockEntity(blockentity.NewRawBlockEntity(root))
			for _, direction := range featureFaceOffsets[featureNorth:] {
				neighbor := blockPosition{x + direction.x, y, z + direction.z}
				if canStateSurvive(chunk, neighbor, f.torch) {
					setChunkBlock(chunk, neighbor, f.torch)
				}
			}
			return true
		}
	}
	return false
}

// shuffleBonusChestAxis preserves the integer Fisher-Yates draw order.
func shuffleBonusChestAxis(random worldrandom.Source, axis *[16]int) {
	for length := len(axis); length > 1; length-- {
		index := int(random.NextIntBound(int32(length)))
		axis[length-1], axis[index] = axis[index], axis[length-1]
	}
}
