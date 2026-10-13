package noise

import (
	"math"
	"testing"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestPerlinSimplexReference(t *testing.T) {
	tests := []struct {
		seed    int64
		octaves []int
		want    []float64
	}{
		{1234, []int{0}, []float64{0, 0.014395297445173923, 0.21595838684185040, -0.054278642363964780}},
		{3456, []int{-2, -1, 0}, []float64{0, 0.61661824040793150, -0.10733886046653371, 0.094949036285088580}},
	}
	positions := [][2]float64{{0, 0}, {1.25, -8.75}, {100.5, 200.25}, {-23.1, 99.9}}
	for _, test := range tests {
		sampler, err := NewPerlinSimplex(worldrandom.NewLegacy(test.seed), test.octaves)
		if err != nil {
			t.Fatal(err)
		}
		for i, position := range positions {
			got := sampler.Sample(position[0], position[1], false)
			if math.Abs(got-test.want[i]) > 1e-15 {
				t.Errorf("seed %d position %v = %.17g, want %.17g", test.seed, position, got, test.want[i])
			}
		}
	}
}
