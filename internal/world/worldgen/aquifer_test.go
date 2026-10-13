package worldgen

import (
	"testing"

	"github.com/vitismc/vitis/internal/world/worldgen/density"
)

type fixedDensity float64

func (f fixedDensity) Sample(density.Position) float64 { return float64(f) }

func TestDisabledAquiferFluidPicker(t *testing.T) {
	t.Parallel()

	aquifer := newChunkAquifer(aquiferConfig{
		seaLevel: 63, defaultFluid: 86, lava: 102,
	})
	tests := []struct {
		y       int
		density float64
		state   int32
	}{
		{0, 1, -1},
		{-60, -1, 102},
		{0, -1, 86},
		{62, -1, 86},
		{63, -1, 0},
	}
	for _, test := range tests {
		actual := aquifer.substance(density.Position{Y: test.y}, test.density)
		if actual != test.state {
			t.Fatalf("y=%d density=%v: state=%d, want %d", test.y, test.density, actual, test.state)
		}
	}
}

func TestAquiferIsIndependentFromSampleOrder(t *testing.T) {
	t.Parallel()

	configuration := aquiferConfig{
		enabled: true, seed: 123456789, minY: -64, height: 384,
		seaLevel: 63, defaultFluid: 86, lava: 102,
		barrier: fixedDensity(0), floodedness: fixedDensity(1), spread: fixedDensity(0), lavaNoise: fixedDensity(0),
		erosion: fixedDensity(0), depth: fixedDensity(0), initialDensity: fixedDensity(1),
	}
	positions := []density.Position{
		{X: 0, Y: 20, Z: 0}, {X: -17, Y: -30, Z: 31}, {X: 15, Y: 62, Z: -16}, {X: 100, Y: 5, Z: 200},
	}
	forward, reverse := newChunkAquifer(configuration), newChunkAquifer(configuration)
	want := make([]int32, len(positions))
	for index, position := range positions {
		want[index] = forward.substance(position, -1)
	}
	for index := len(positions) - 1; index >= 0; index-- {
		actual := reverse.substance(positions[index], -1)
		if actual != want[index] {
			t.Fatalf("position %+v: reverse state=%d, forward state=%d", positions[index], actual, want[index])
		}
	}
	forward.reset()
	for index := len(positions) - 1; index >= 0; index-- {
		actual := forward.substance(positions[index], -1)
		if actual != want[index] {
			t.Fatalf("position %+v: reused state=%d, fresh state=%d", positions[index], actual, want[index])
		}
	}
}

func TestFloorDivision(t *testing.T) {
	t.Parallel()

	tests := map[int]int{-17: -2, -16: -1, -1: -1, 0: 0, 15: 0, 16: 1}
	for value, expected := range tests {
		if actual := floorDiv(value, 16); actual != expected {
			t.Fatalf("floorDiv(%d, 16)=%d, want %d", value, actual, expected)
		}
	}
}

// TestAquiferDryCeilingMatchesPressureSampling proves the early-air bound for custom levels.
func TestAquiferDryCeilingMatchesPressureSampling(t *testing.T) {
	for _, seaLevel := range []int{-100, -54, 0, 63, 128} {
		for _, floodedness := range []fixedDensity{-1, 0, 1} {
			configuration := aquiferConfig{
				enabled: true, seed: 123, minY: -128, height: 512, seaLevel: seaLevel,
				defaultFluid: 86, lava: 102, barrier: fixedDensity(100), floodedness: floodedness,
				spread: fixedDensity(1), lavaNoise: fixedDensity(1), erosion: fixedDensity(0), depth: fixedDensity(0), initialDensity: fixedDensity(1),
			}
			fast, generic := newChunkAquifer(configuration), newChunkAquifer(configuration)
			for y := max(-54, seaLevel) + 5; y < 384; y += 11 {
				for _, point := range [][2]int{{-17, -1}, {0, 0}, {15, 16}, {100, -100}} {
					position := density.Position{X: point[0], Y: y, Z: point[1]}
					if got, want := fast.substance(position, -0.001), generic.substanceBelowCeiling(position, -0.001); got != want || got != 0 {
						t.Fatalf("sea=%d floodedness=%v at %+v: fast=%d generic=%d", seaLevel, floodedness, position, got, want)
					}
					if got := fast.substance(position, 0.001); got != -1 {
						t.Fatal("dry ceiling removed solid terrain")
					}
				}
			}
		}
	}
}
