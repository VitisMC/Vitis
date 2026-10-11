package worldgen

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/vitismc/vitis/internal/block"
	blockfluid "github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type seagrassFeature struct{ probability float64 }

func (f *seagrassFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	x := origin.x + int(random.NextIntBound(8)) - int(random.NextIntBound(8))
	z := origin.z + int(random.NextIntBound(8)) - int(random.NextIntBound(8))
	y, ok := featureHeightmap(chunk, x, z, "OCEAN_FLOOR")
	if !ok {
		return false
	}
	position := blockPosition{x: x, y: y, z: z}
	if !blockfluid.IsWater(getChunkBlock(chunk, position)) {
		return false
	}
	tall := random.NextDouble() < f.probability
	name := "minecraft:seagrass"
	properties := map[string]string(nil)
	if tall {
		name = "minecraft:tall_seagrass"
		properties = map[string]string{"half": "lower"}
	}
	state, err := block.BlockStateFromProperties(name, properties)
	if err != nil || !block.IsSolid(getChunkBlock(chunk, blockPosition{x: x, y: y - 1, z: z})) {
		return false
	}
	if tall {
		above := blockPosition{x: x, y: y + 1, z: z}
		if !blockfluid.IsWater(getChunkBlock(chunk, above)) {
			return false
		}
		upper, err := block.BlockStateFromProperties(name, map[string]string{"half": "upper"})
		if err != nil {
			return false
		}
		setChunkBlock(chunk, position, state.ID())
		setChunkBlock(chunk, above, upper.ID())
		return true
	}
	setChunkBlock(chunk, position, state.ID())
	return true
}

type seaPickleFeature struct{ count intProvider }

func (f *seaPickleFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	placed := false
	for count := f.count.sample(random); count > 0; count-- {
		x := origin.x + int(random.NextIntBound(8)) - int(random.NextIntBound(8))
		z := origin.z + int(random.NextIntBound(8)) - int(random.NextIntBound(8))
		y, ok := featureHeightmap(chunk, x, z, "OCEAN_FLOOR")
		if !ok {
			continue
		}
		position := blockPosition{x: x, y: y, z: z}
		pickles := int(random.NextIntBound(4)) + 1
		if !blockfluid.IsWater(getChunkBlock(chunk, position)) || !block.IsSolid(getChunkBlock(chunk, blockPosition{x: x, y: y - 1, z: z})) {
			continue
		}
		state, err := block.BlockStateFromProperties("minecraft:sea_pickle", map[string]string{"pickles": strconv.Itoa(pickles), "waterlogged": "true"})
		if err != nil {
			continue
		}
		setChunkBlock(chunk, position, state.ID())
		placed = true
	}
	return placed
}

type kelpFeature struct{}

func (*kelpFeature) placeFeature(_ *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	y, ok := featureHeightmap(chunk, origin.x, origin.z, "OCEAN_FLOOR")
	if !ok {
		return false
	}
	position := blockPosition{x: origin.x, y: y, z: origin.z}
	if !blockfluid.IsWater(getChunkBlock(chunk, position)) {
		return false
	}
	height := 1 + int(random.NextIntBound(10))
	plant := block.DefaultStateID("minecraft:kelp_plant")
	placed := false
	for index := 0; index <= height; index++ {
		above := blockPosition{x: position.x, y: position.y + 1, z: position.z}
		if blockfluid.IsWater(getChunkBlock(chunk, position)) && blockfluid.IsWater(getChunkBlock(chunk, above)) && (block.IsSolid(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z})) || block.NameFromState(getChunkBlock(chunk, blockPosition{x: position.x, y: position.y - 1, z: position.z})) == "minecraft:kelp_plant") {
			if index == height {
				state, err := block.BlockStateFromProperties("minecraft:kelp", map[string]string{"age": strconv.Itoa(20 + int(random.NextIntBound(4)))})
				if err == nil {
					setChunkBlock(chunk, position, state.ID())
					placed = true
				}
			} else {
				setChunkBlock(chunk, position, plant)
			}
		} else if index > 0 {
			tip := blockPosition{x: position.x, y: position.y - 1, z: position.z}
			below := blockPosition{x: tip.x, y: tip.y - 1, z: tip.z}
			if block.NameFromState(getChunkBlock(chunk, below)) != "minecraft:kelp" {
				state, err := block.BlockStateFromProperties("minecraft:kelp", map[string]string{"age": strconv.Itoa(20 + int(random.NextIntBound(4)))})
				if err == nil {
					setChunkBlock(chunk, tip, state.ID())
					placed = true
				}
			}
			break
		}
		position = above
	}
	return placed
}

func decodeAquaticFeature(_ *datapack.Snapshot, kind string, raw json.RawMessage) (configuredFeature, bool, error) {
	switch kind {
	case "minecraft:seagrass":
		var value struct {
			Config struct {
				Probability float64 `json:"probability"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		if value.Config.Probability < 0 || value.Config.Probability > 1 {
			return nil, true, fmt.Errorf("invalid seagrass probability")
		}
		return &seagrassFeature{probability: value.Config.Probability}, true, nil
	case "minecraft:sea_pickle":
		var value struct {
			Config struct {
				Count json.RawMessage `json:"count"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, err
		}
		count, err := decodeIntProviderRange(value.Config.Count, 0, 256)
		if err != nil {
			return nil, true, err
		}
		return &seaPickleFeature{count: count}, true, nil
	case "minecraft:kelp":
		return &kelpFeature{}, true, nil
	default:
		return nil, false, nil
	}
}
