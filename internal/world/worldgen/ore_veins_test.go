package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/worldgen/density"
)

func TestOreVeinifierReference(t *testing.T) {
	veins := newOreVeinifier(true, 123456789, false, fixedDensity(0.7), fixedDensity(-1), fixedDensity(0))
	tests := []struct {
		x, y int
		want string
	}{
		{0, 20, "minecraft:granite"},
		{0, 21, "minecraft:copper_ore"},
		{1, 22, "minecraft:copper_ore"},
		{3, 20, "minecraft:copper_ore"},
		{5, 22, "minecraft:granite"},
	}
	for _, test := range tests {
		state, ok := veins.state(density.Position{X: test.x, Y: test.y, Z: 3})
		if !ok || block.NameFromState(state) != test.want {
			t.Errorf("ore at %d,%d,3 = %s, %t; want %s", test.x, test.y, block.NameFromState(state), ok, test.want)
		}
	}
	if _, ok := veins.state(density.Position{X: 0, Y: 51, Z: 0}); ok {
		t.Fatal("copper vein generated above its range")
	}
}
