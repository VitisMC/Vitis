package worldgen

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/worldgen/noise"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

type noiseStateProvider struct {
	states []int32
	noise  *noise.Normal
	scale  float32
}

// state selects a block from the noise value at a world position.
func (p *noiseStateProvider) state(_ *featureSystem, _ *section.Chunk, _ worldrandom.Source, position blockPosition) int32 {
	return noiseSelectedState(p.states, p.noise.Sample(float64(position.x)*float64(p.scale), float64(position.y)*float64(p.scale), float64(position.z)*float64(p.scale)))
}

type dualNoiseStateProvider struct {
	noiseStateProvider
	slow      *noise.Normal
	slowScale float32
	minimum   int
	maximum   int
}

type thresholdNoiseStateProvider struct {
	noise      *noise.Normal
	scale      float32
	threshold  float32
	highChance float32
	defaultID  int32
	lowStates  []int32
	highStates []int32
}

// state draws from the palette selected by the local noise threshold.
func (p *thresholdNoiseStateProvider) state(_ *featureSystem, _ *section.Chunk, random worldrandom.Source, position blockPosition) int32 {
	value := p.noise.Sample(float64(position.x)*float64(p.scale), float64(position.y)*float64(p.scale), float64(position.z)*float64(p.scale))
	if value < float64(p.threshold) {
		return p.lowStates[random.NextIntBound(int32(len(p.lowStates)))]
	}
	if random.NextFloat() < p.highChance {
		return p.highStates[random.NextIntBound(int32(len(p.highStates)))]
	}
	return p.defaultID
}

// state selects a local palette using slow noise before selecting its block.
func (p *dualNoiseStateProvider) state(_ *featureSystem, _ *section.Chunk, _ worldrandom.Source, position blockPosition) int32 {
	slowValue := p.sampleSlow(position)
	variety := int(math.Max(float64(p.minimum), math.Min(float64(p.maximum+1), float64(p.minimum)+(slowValue+1)*float64(p.maximum+1-p.minimum)/2)))
	var storage [65]int32
	palette := storage[:variety]
	for i := range palette {
		shifted := blockPosition{x: position.x + i*54545, y: position.y, z: position.z + i*34234}
		palette[i] = noiseSelectedState(p.states, p.sampleSlow(shifted))
	}
	return noiseSelectedState(palette, p.noise.Sample(float64(position.x)*float64(p.scale), float64(position.y)*float64(p.scale), float64(position.z)*float64(p.scale)))
}

func (p *dualNoiseStateProvider) sampleSlow(position blockPosition) float64 {
	return p.slow.Sample(float64(float32(position.x)*p.slowScale), float64(float32(position.y)*p.slowScale), float64(float32(position.z)*p.slowScale))
}

func noiseSelectedState(states []int32, value float64) int32 {
	index := int(math.Max(0, math.Min(0.9999, (value+1)/2)) * float64(len(states)))
	return states[index]
}

// decodeNoiseStateProvider validates and builds a seeded noise state provider.
func decodeNoiseStateProvider(raw json.RawMessage, dual bool) (stateProvider, error) {
	var value struct {
		Seed      int64            `json:"seed"`
		Noise     noise.Parameters `json:"noise"`
		Scale     float32          `json:"scale"`
		States    []BlockState     `json:"states"`
		SlowNoise noise.Parameters `json:"slow_noise"`
		SlowScale float32          `json:"slow_scale"`
		Variety   []int            `json:"variety"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if len(value.States) == 0 || value.Scale <= 0 || math.IsInf(float64(value.Scale), 0) || validateNoiseParameters("state provider", value.Noise) != nil {
		return nil, fmt.Errorf("invalid noise state provider")
	}
	states := make([]int32, 0, len(value.States))
	for _, definition := range value.States {
		state, err := block.BlockStateFromProperties(definition.Name, definition.Properties)
		if err != nil {
			return nil, err
		}
		states = append(states, state.ID())
	}
	mainNoise, err := noise.NewNormal(worldrandom.NewWorldgen(worldrandom.NewLegacy(value.Seed)), value.Noise)
	if err != nil {
		return nil, err
	}
	base := noiseStateProvider{states: states, noise: mainNoise, scale: value.Scale}
	if !dual {
		return &base, nil
	}
	if len(value.Variety) != 2 || value.Variety[0] < 1 || value.Variety[1] > 64 || value.Variety[0] > value.Variety[1] || validateNoiseParameters("slow state provider", value.SlowNoise) != nil || value.SlowScale <= 0 || math.IsInf(float64(value.SlowScale), 0) {
		return nil, fmt.Errorf("invalid dual noise state provider")
	}
	slow, err := noise.NewNormal(worldrandom.NewWorldgen(worldrandom.NewLegacy(value.Seed)), value.SlowNoise)
	if err != nil {
		return nil, err
	}
	return &dualNoiseStateProvider{noiseStateProvider: base, slow: slow, slowScale: value.SlowScale, minimum: value.Variety[0], maximum: value.Variety[1]}, nil
}

// decodeThresholdNoiseStateProvider builds a seeded, weighted threshold provider.
func decodeThresholdNoiseStateProvider(raw json.RawMessage) (stateProvider, error) {
	var value struct {
		Seed         int64            `json:"seed"`
		Noise        noise.Parameters `json:"noise"`
		Scale        float32          `json:"scale"`
		Threshold    float32          `json:"threshold"`
		HighChance   float32          `json:"high_chance"`
		DefaultState BlockState       `json:"default_state"`
		LowStates    []BlockState     `json:"low_states"`
		HighStates   []BlockState     `json:"high_states"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if validateNoiseParameters("threshold state provider", value.Noise) != nil || value.Scale <= 0 || math.IsInf(float64(value.Scale), 0) || value.Threshold < -1 || value.Threshold > 1 || value.HighChance < 0 || value.HighChance > 1 || len(value.LowStates) == 0 || len(value.HighStates) == 0 {
		return nil, fmt.Errorf("invalid threshold noise state provider")
	}
	defaultState, err := block.BlockStateFromProperties(value.DefaultState.Name, value.DefaultState.Properties)
	if err != nil {
		return nil, err
	}
	decodeStates := func(definitions []BlockState) ([]int32, error) {
		states := make([]int32, 0, len(definitions))
		for _, definition := range definitions {
			state, err := block.BlockStateFromProperties(definition.Name, definition.Properties)
			if err != nil {
				return nil, err
			}
			states = append(states, state.ID())
		}
		return states, nil
	}
	low, err := decodeStates(value.LowStates)
	if err != nil {
		return nil, err
	}
	high, err := decodeStates(value.HighStates)
	if err != nil {
		return nil, err
	}
	sampler, err := noise.NewNormal(worldrandom.NewWorldgen(worldrandom.NewLegacy(value.Seed)), value.Noise)
	if err != nil {
		return nil, err
	}
	return &thresholdNoiseStateProvider{noise: sampler, scale: value.Scale, threshold: value.Threshold, highChance: value.HighChance, defaultID: defaultState.ID(), lowStates: low, highStates: high}, nil
}
