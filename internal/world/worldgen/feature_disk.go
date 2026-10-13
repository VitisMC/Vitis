package worldgen

import (
	"encoding/json"
	"fmt"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type stateProviderRule struct {
	predicate blockPredicate
	provider  stateProvider
}

type ruleStateProvider struct {
	fallback stateProvider
	rules    []stateProviderRule
}

func (p *ruleStateProvider) state(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, position blockPosition) int32 {
	for _, rule := range p.rules {
		if rule.predicate.test(system, chunk, position) {
			return rule.provider.state(system, chunk, random, position)
		}
	}
	return p.fallback.state(system, chunk, random, position)
}

type diskFeature struct {
	provider   *ruleStateProvider
	target     blockPredicate
	radius     intProvider
	halfHeight int
}

func (f *diskFeature) placeFeature(system *featureSystem, chunk *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	radius := f.radius.sample(random)
	placed := false
	for z := origin.z - radius; z <= origin.z+radius; z++ {
		for x := origin.x - radius; x <= origin.x+radius; x++ {
			dx, dz := x-origin.x, z-origin.z
			if dx*dx+dz*dz > radius*radius {
				continue
			}
			for y := origin.y + f.halfHeight; y > origin.y-f.halfHeight-1; y-- {
				position := blockPosition{x: x, y: y, z: z}
				if !f.target.test(system, chunk, position) {
					continue
				}
				setChunkBlock(chunk, position, f.provider.state(system, chunk, random, position))
				placed = true
			}
		}
	}
	return placed
}

func decodeDiskFeature(_ *datapack.Snapshot, raw json.RawMessage) (configuredFeature, bool, error) {
	var value struct {
		Config struct {
			HalfHeight int             `json:"half_height"`
			Radius     json.RawMessage `json:"radius"`
			Target     json.RawMessage `json:"target"`
			Provider   struct {
				Fallback json.RawMessage `json:"fallback"`
				Rules    []struct {
					IfTrue json.RawMessage `json:"if_true"`
					Then   json.RawMessage `json:"then"`
				} `json:"rules"`
			} `json:"state_provider"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	if value.Config.HalfHeight < 0 || value.Config.HalfHeight > 4 {
		return nil, true, fmt.Errorf("invalid disk half height")
	}
	radius, err := decodeIntProviderRange(value.Config.Radius, 0, 8)
	if err != nil {
		return nil, true, fmt.Errorf("radius: %w", err)
	}
	target, err := decodeBlockPredicate(value.Config.Target)
	if err != nil {
		return nil, true, fmt.Errorf("target: %w", err)
	}
	fallback, err := decodeStateProvider(value.Config.Provider.Fallback)
	if err != nil {
		return nil, true, fmt.Errorf("fallback: %w", err)
	}
	provider := &ruleStateProvider{fallback: fallback}
	for index, definition := range value.Config.Provider.Rules {
		predicate, err := decodeBlockPredicate(definition.IfTrue)
		if err != nil {
			return nil, true, fmt.Errorf("rule %d predicate: %w", index, err)
		}
		then, err := decodeStateProvider(definition.Then)
		if err != nil {
			return nil, true, fmt.Errorf("rule %d provider: %w", index, err)
		}
		provider.rules = append(provider.rules, stateProviderRule{predicate: predicate, provider: then})
	}
	return &diskFeature{provider: provider, target: target, radius: radius, halfHeight: value.Config.HalfHeight}, true, nil
}
