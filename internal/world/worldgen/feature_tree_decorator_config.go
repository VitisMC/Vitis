package worldgen

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/datapack"
)

// decodeTreeDecorator rejects unknown decorators instead of silently dropping content.
func decodeTreeDecorator(_ *datapack.Snapshot, raw json.RawMessage) (treeDecorator, bool, error) {
	var data struct {
		Type          string          `json:"type"`
		Probability   float32         `json:"probability"`
		Provider      json.RawMessage `json:"provider"`
		BlockProvider json.RawMessage `json:"block_provider"`
		ExclusionXZ   int             `json:"exclusion_radius_xz"`
		ExclusionY    int             `json:"exclusion_radius_y"`
		RequiredEmpty int             `json:"required_empty_blocks"`
		Directions    []string        `json:"directions"`
		LeavesChance  float32         `json:"leaves_probability"`
		TrunkChance   float32         `json:"trunk_probability"`
		GroundChance  float32         `json:"ground_probability"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return treeDecorator{}, true, err
	}
	result := treeDecorator{kind: normalizeResourceID(data.Type), probability: data.Probability}
	for _, chance := range []float32{data.Probability, data.LeavesChance, data.TrunkChance, data.GroundChance} {
		if chance < 0 || chance > 1 || math.IsNaN(float64(chance)) {
			return result, true, fmt.Errorf("tree decorator probability out of range")
		}
	}
	var err error
	switch result.kind {
	case "minecraft:trunk_vine", "minecraft:leave_vine", "minecraft:cocoa", "minecraft:beehive", "minecraft:creaking_heart":
	case "minecraft:alter_ground":
		result.provider, err = decodeStateProvider(data.Provider)
	case "minecraft:attached_to_leaves":
		if data.ExclusionXZ < 0 || data.ExclusionXZ > 16 || data.ExclusionY < 0 || data.ExclusionY > 16 || data.RequiredEmpty < 1 || data.RequiredEmpty > 16 || len(data.Directions) == 0 {
			return result, true, fmt.Errorf("invalid attached-to-leaves settings")
		}
		result.exclusionXZ, result.exclusionY, result.requiredEmpty = data.ExclusionXZ, data.ExclusionY, data.RequiredEmpty
		for _, name := range data.Directions {
			direction, ok := treeDecoratorDirection(name)
			if !ok {
				return result, true, fmt.Errorf("unknown tree decorator direction %q", name)
			}
			result.directions = append(result.directions, direction)
		}
		result.provider, err = decodeStateProvider(data.BlockProvider)
	case "minecraft:pale_moss":
		result.leavesChance, result.trunkChance, result.groundChance = data.LeavesChance, data.TrunkChance, data.GroundChance
	default:
		return result, false, nil
	}
	return result, true, err
}

// treeDecoratorDirection resolves the six permitted block-relative directions.
func treeDecoratorDirection(name string) ([3]int, bool) {
	switch name {
	case "down":
		return [3]int{0, -1, 0}, true
	case "up":
		return [3]int{0, 1, 0}, true
	case "north":
		return [3]int{0, 0, -1}, true
	case "south":
		return [3]int{0, 0, 1}, true
	case "west":
		return [3]int{-1, 0, 0}, true
	case "east":
		return [3]int{1, 0, 0}, true
	default:
		return [3]int{}, false
	}
}
