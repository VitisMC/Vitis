package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type blockColumnLayer struct {
	height   intProvider
	provider stateProvider
}

type blockColumnFeature struct {
	layers        []blockColumnLayer
	direction     [3]int
	allowed       blockPredicate
	prioritizeTip bool
}

func (f *blockColumnFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	heights := make([]int, len(f.layers))
	total := 0
	for i, layer := range f.layers {
		heights[i] = layer.height.sample(random)
		total += heights[i]
	}
	if total == 0 {
		return false
	}
	probe := moveBlockPosition(origin, f.direction)
	for step := 0; step < total; step++ {
		if !f.allowed.test(system, chunk, probe) {
			truncateColumn(heights, total-step, f.prioritizeTip)
			break
		}
		probe = moveBlockPosition(probe, f.direction)
	}
	position := origin
	for index, layer := range f.layers {
		for count := 0; count < heights[index]; count++ {
			setChunkBlock(chunk, position, layer.provider.state(system, chunk, random, position))
			position = moveBlockPosition(position, f.direction)
		}
	}
	return true
}

func truncateColumn(heights []int, remove int, prioritizeTip bool) {
	index, increment, end := len(heights)-1, -1, -1
	if prioritizeTip {
		index, increment, end = 0, 1, len(heights)
	}
	for index != end && remove > 0 {
		amount := heights[index]
		if amount > remove {
			amount = remove
		}
		heights[index] -= amount
		remove -= amount
		index += increment
	}
}

func moveBlockPosition(position blockPosition, direction [3]int) blockPosition {
	return blockPosition{x: position.x + direction[0], y: position.y + direction[1], z: position.z + direction[2]}
}

func decodeBlockColumnFeature(_ *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var value struct {
		Config struct {
			Allowed       json.RawMessage `json:"allowed_placement"`
			Direction     string          `json:"direction"`
			PrioritizeTip bool            `json:"prioritize_tip"`
			Layers        []struct {
				Height   json.RawMessage `json:"height"`
				Provider json.RawMessage `json:"provider"`
			} `json:"layers"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	directions := map[string][3]int{"up": {0, 1, 0}, "down": {0, -1, 0}, "north": {0, 0, -1}, "south": {0, 0, 1}, "west": {-1, 0, 0}, "east": {1, 0, 0}}
	direction, ok := directions[value.Config.Direction]
	if !ok {
		return nil, true, fmt.Errorf("invalid column direction %q", value.Config.Direction)
	}
	allowed, err := decodeBlockPredicate(value.Config.Allowed)
	if err != nil {
		return nil, true, err
	}
	if len(value.Config.Layers) == 0 {
		return nil, true, fmt.Errorf("empty block column")
	}
	feature := &blockColumnFeature{direction: direction, allowed: allowed, prioritizeTip: value.Config.PrioritizeTip}
	for index, definition := range value.Config.Layers {
		height, err := decodeIntProviderRange(definition.Height, 0, 1000000)
		if err != nil {
			return nil, true, fmt.Errorf("layer %d height: %w", index, err)
		}
		provider, err := decodeStateProvider(definition.Provider)
		if err != nil {
			return nil, true, fmt.Errorf("layer %d provider: %w", index, err)
		}
		feature.layers = append(feature.layers, blockColumnLayer{height: height, provider: provider})
	}
	return feature, true, nil
}
