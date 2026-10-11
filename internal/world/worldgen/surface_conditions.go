package worldgen

import (
	"encoding/json"
	"fmt"

	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func (s *surfaceSystem) compileCondition(raw json.RawMessage, _ *ResourceSet, resolveNoise func(string) (*noise.Normal, error)) (surfaceCondition, error) {
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case "minecraft:biome":
		var value struct {
			Biomes []string `json:"biome_is"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || len(value.Biomes) == 0 {
			return nil, fmt.Errorf("invalid biome condition")
		}
		ids := make(biomeSurfaceCondition, len(generatedbiome.Biomes))
		for _, name := range value.Biomes {
			id := generatedbiome.BiomeIDByName(name)
			if id < 0 {
				return nil, fmt.Errorf("unknown biome %s", name)
			}
			if int(id) >= len(ids) {
				ids = append(ids, make([]bool, int(id)+1-len(ids))...)
			}
			ids[id] = true
		}
		return ids, nil
	case "minecraft:noise_threshold":
		var value struct {
			Noise string  `json:"noise"`
			Min   float64 `json:"min_threshold"`
			Max   float64 `json:"max_threshold"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || value.Min > value.Max {
			return nil, fmt.Errorf("invalid noise threshold")
		}
		sampler, err := resolveNoise(normalizeResourceID(value.Noise))
		if err != nil {
			return nil, err
		}
		return s.cacheColumnCondition(conditionFunc(func(context *surfaceContext) bool {
			result := sampler.SampleXZ(float64(context.x), float64(context.z))
			return result >= value.Min && result <= value.Max
		})), nil
	case "minecraft:vertical_gradient":
		var value struct {
			RandomName string          `json:"random_name"`
			TrueBelow  json.RawMessage `json:"true_at_and_below"`
			FalseAbove json.RawMessage `json:"false_at_and_above"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		lower, err := s.resolveAnchor(value.TrueBelow)
		if err != nil {
			return nil, err
		}
		upper, err := s.resolveAnchor(value.FalseAbove)
		if err != nil || lower > upper {
			return nil, fmt.Errorf("invalid vertical gradient")
		}
		factory := s.random.FromHashOf(normalizeResourceID(value.RandomName)).ForkPositional()
		return conditionFunc(func(context *surfaceContext) bool {
			if context.y <= lower {
				return true
			}
			if context.y >= upper {
				return false
			}
			chance := float64(upper-context.y) / float64(upper-lower)
			return float64(worldrandom.FirstFloatAt(factory, int32(context.x), int32(context.y), int32(context.z))) < chance
		}), nil
	case "minecraft:y_above":
		var value struct {
			Anchor          json.RawMessage `json:"anchor"`
			DepthMultiplier int             `json:"surface_depth_multiplier"`
			AddStoneDepth   bool            `json:"add_stone_depth"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || value.DepthMultiplier < -20 || value.DepthMultiplier > 20 {
			return nil, fmt.Errorf("invalid y condition")
		}
		anchor, err := s.resolveAnchor(value.Anchor)
		if err != nil {
			return nil, err
		}
		return conditionFunc(func(context *surfaceContext) bool {
			y := context.y
			if value.AddStoneDepth {
				y += context.stoneDepthAbove
			}
			return y >= anchor+context.surfaceDepth*value.DepthMultiplier
		}), nil
	case "minecraft:water":
		var value struct {
			Offset          int  `json:"offset"`
			DepthMultiplier int  `json:"surface_depth_multiplier"`
			AddStoneDepth   bool `json:"add_stone_depth"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || value.DepthMultiplier < -20 || value.DepthMultiplier > 20 {
			return nil, fmt.Errorf("invalid water condition")
		}
		return conditionFunc(func(context *surfaceContext) bool {
			if context.waterHeight == noWaterHeight {
				return true
			}
			y := context.y
			if value.AddStoneDepth {
				y += context.stoneDepthAbove
			}
			return y >= context.waterHeight+value.Offset+context.surfaceDepth*value.DepthMultiplier
		}), nil
	case "minecraft:temperature":
		return conditionFunc(func(context *surfaceContext) bool { return s.coldEnoughToSnow(context) }), nil
	case "minecraft:steep":
		return s.cacheColumnCondition(conditionFunc(func(context *surfaceContext) bool {
			previousZ, nextZ := max(context.localZ-1, 0), min(context.localZ+1, 15)
			if context.heights[context.localX][nextZ] >= context.heights[context.localX][previousZ]+4 {
				return true
			}
			previousX, nextX := max(context.localX-1, 0), min(context.localX+1, 15)
			return context.heights[previousX][context.localZ] >= context.heights[nextX][context.localZ]+4
		})), nil
	case "minecraft:not":
		var value struct {
			Invert json.RawMessage `json:"invert"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		child, err := s.compileCondition(value.Invert, nil, resolveNoise)
		if err != nil {
			return nil, err
		}
		return conditionFunc(func(context *surfaceContext) bool { return !child.test(context) }), nil
	case "minecraft:hole":
		return s.cacheColumnCondition(conditionFunc(func(context *surfaceContext) bool { return context.surfaceDepth <= 0 })), nil
	case "minecraft:above_preliminary_surface":
		return conditionFunc(func(context *surfaceContext) bool { return context.y >= context.minSurfaceLevel }), nil
	case "minecraft:stone_depth":
		var value struct {
			Offset              int    `json:"offset"`
			AddSurfaceDepth     bool   `json:"add_surface_depth"`
			SecondaryDepthRange int    `json:"secondary_depth_range"`
			SurfaceType         string `json:"surface_type"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || (value.SurfaceType != "floor" && value.SurfaceType != "ceiling") {
			return nil, fmt.Errorf("invalid stone depth condition")
		}
		return conditionFunc(func(context *surfaceContext) bool {
			depth := context.stoneDepthAbove
			if value.SurfaceType == "ceiling" {
				depth = context.stoneDepthBelow
			}
			limit := 1 + value.Offset
			if value.AddSurfaceDepth {
				limit += context.surfaceDepth
			}
			if value.SecondaryDepthRange != 0 {
				limit += int(mapRange(context.surfaceSecondary, -1, 1, 0, float64(value.SecondaryDepthRange)))
			}
			return depth <= limit
		}), nil
	default:
		return nil, fmt.Errorf("unsupported condition type %q", header.Type)
	}
}

// biomeSurfaceCondition uses registry-indexed membership for a biome rule.
type biomeSurfaceCondition []bool

// test checks a rule's immutable biome set without a map lookup.
func (c biomeSurfaceCondition) test(context *surfaceContext) bool {
	id := context.biomeID()
	return id >= 0 && int(id) < len(c) && c[id]
}

// columnSurfaceCondition caches a predicate that cannot change with block height.
type columnSurfaceCondition struct {
	id        int
	condition surfaceCondition
}

// cacheColumnCondition assigns one per-column cache slot to an immutable predicate.
func (s *surfaceSystem) cacheColumnCondition(condition surfaceCondition) surfaceCondition {
	id := s.columnConditionCount
	s.columnConditionCount++
	return columnSurfaceCondition{id: id, condition: condition}
}

// test evaluates the predicate once for each prepared column context.
func (c columnSurfaceCondition) test(context *surfaceContext) bool {
	if c.id >= len(context.columnConditions) {
		return c.condition.test(context)
	}
	value := context.columnConditions[c.id]
	if value != 0 {
		return value == 2
	}
	result := c.condition.test(context)
	context.columnConditions[c.id] = 1
	if result {
		context.columnConditions[c.id] = 2
	}
	return result
}

func (s *surfaceSystem) resolveAnchor(raw json.RawMessage) (int, error) {
	var value struct {
		Absolute    *int `json:"absolute"`
		AboveBottom *int `json:"above_bottom"`
		BelowTop    *int `json:"below_top"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	count := 0
	result := 0
	if value.Absolute != nil {
		count++
		result = *value.Absolute
	}
	if value.AboveBottom != nil {
		count++
		result = s.minY + *value.AboveBottom
	}
	if value.BelowTop != nil {
		count++
		result = s.minY + s.height - 1 - *value.BelowTop
	}
	if count != 1 {
		return 0, fmt.Errorf("invalid vertical anchor")
	}
	return result, nil
}

func mapRange(value, fromMin, fromMax, toMin, toMax float64) float64 {
	return toMin + (value-fromMin)/(fromMax-fromMin)*(toMax-toMin)
}
