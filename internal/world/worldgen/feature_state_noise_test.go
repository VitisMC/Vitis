package worldgen

import (
	"encoding/json"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestNoiseStateProvidersAreDeterministic checks seeded selection at fixed positions.
func TestNoiseStateProvidersAreDeterministic(t *testing.T) {
	for _, raw := range []string{
		`{"type":"minecraft:noise_provider","seed":2345,"noise":{"firstOctave":0,"amplitudes":[1]},"scale":0.02,"states":[{"Name":"minecraft:stone"},{"Name":"minecraft:dirt"}]}`,
		`{"type":"minecraft:dual_noise_provider","seed":2345,"noise":{"firstOctave":-3,"amplitudes":[1]},"scale":1,"slow_noise":{"firstOctave":-10,"amplitudes":[1]},"slow_scale":1,"variety":[1,3],"states":[{"Name":"minecraft:stone"},{"Name":"minecraft:dirt"}]}`,
		`{"type":"minecraft:noise_threshold_provider","seed":2345,"noise":{"firstOctave":0,"amplitudes":[1]},"scale":0.005,"threshold":-0.8,"high_chance":0.5,"default_state":{"Name":"minecraft:stone"},"low_states":[{"Name":"minecraft:dirt"}],"high_states":[{"Name":"minecraft:grass_block"}]}`,
	} {
		first, err := decodeStateProvider(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		second, err := decodeStateProvider(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		firstRandom, secondRandom := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		for x := -32; x <= 32; x++ {
			position := blockPosition{x: x, y: 70, z: x * 3}
			left := first.state(nil, nil, firstRandom, position)
			right := second.state(nil, nil, secondRandom, position)
			if left != right || block.NameFromState(left) == "" {
				t.Fatalf("state at %v = %d, %d", position, left, right)
			}
		}
	}
}

// TestNoiseStateProviderRejectsInvalidInput checks malformed palette and range data.
func TestNoiseStateProviderRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{
		`{"type":"minecraft:noise_provider","seed":1,"noise":{"firstOctave":0,"amplitudes":[1]},"scale":1,"states":[]}`,
		`{"type":"minecraft:dual_noise_provider","seed":1,"noise":{"firstOctave":0,"amplitudes":[1]},"scale":1,"slow_noise":{"firstOctave":0,"amplitudes":[1]},"slow_scale":1,"variety":[0,65],"states":[{"Name":"minecraft:stone"}]}`,
		`{"type":"minecraft:noise_threshold_provider","seed":1,"noise":{"firstOctave":0,"amplitudes":[1]},"scale":1,"threshold":2,"high_chance":0.5,"default_state":{"Name":"minecraft:stone"},"low_states":[{"Name":"minecraft:dirt"}],"high_states":[{"Name":"minecraft:grass_block"}]}`,
	} {
		if _, err := decodeStateProvider(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid provider accepted: %s", raw)
		}
	}
}
