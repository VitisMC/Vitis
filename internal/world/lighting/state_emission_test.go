package lighting

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
)

// TestBlockEmissionUsesStateProperties covers emitters whose default registry state is unlit.
func TestBlockEmissionUsesStateProperties(t *testing.T) {
	for _, test := range []struct {
		name       string
		properties map[string]string
		want       uint8
	}{
		{"minecraft:glow_lichen", nil, 0},
		{"minecraft:glow_lichen", map[string]string{"east": "true"}, 7},
		{"minecraft:glow_lichen", map[string]string{"south": "true", "waterlogged": "true"}, 7},
		{"minecraft:sculk_vein", map[string]string{"east": "true"}, 0},
		{"minecraft:furnace", map[string]string{"lit": "false"}, 0},
		{"minecraft:furnace", map[string]string{"lit": "true"}, 13},
		{"minecraft:redstone_lamp", map[string]string{"lit": "true"}, 15},
		{"minecraft:redstone_lamp", map[string]string{"lit": "false"}, 0},
		{"minecraft:candle", map[string]string{"lit": "true", "candles": "1"}, 3},
		{"minecraft:candle", map[string]string{"lit": "true", "candles": "4"}, 12},
		{"minecraft:sea_pickle", map[string]string{"waterlogged": "true", "pickles": "1"}, 6},
		{"minecraft:sea_pickle", map[string]string{"waterlogged": "false", "pickles": "1"}, 0},
	} {
		state, err := block.BlockStateFromProperties(test.name, test.properties)
		if err != nil {
			t.Fatal(err)
		}
		if got := blockEmission(state.ID()); got != test.want {
			t.Errorf("%s %v emits %d, want %d", test.name, test.properties, got, test.want)
		}
	}
	for _, state := range []int32{-1, 1 << 30} {
		if blockEmission(state) != 0 {
			t.Fatal("invalid state emits light")
		}
	}
}
