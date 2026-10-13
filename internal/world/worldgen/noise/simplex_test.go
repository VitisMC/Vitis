package noise

import (
	"testing"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestSimplexReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		seed  int64
		two   []float64
		three []float64
	}{
		{0, []float64{0, -0.46755237295509970, 0.43450504071499296}, []float64{0, 0.49610000000000093, -0.032358636176565190}},
		{123456789, []float64{0, 0.12794197292426338, -0.21011104606051162}, []float64{0, -0.39358437499999904, 0.032358636176565190}},
		{-1, []float64{0, 0.55638500405646160, 0.30539423411791590}, []float64{0, 0.020503125000001947, 0.45302077628345330}},
	}
	positions := [][3]float64{{0, 0, 0}, {1.25, -3.5, 9.75}, {33554433.125, 64.2, -33554431.75}}
	for _, test := range tests {
		n := NewSimplex(worldrandom.NewLegacy(test.seed))
		for index, position := range positions {
			assertClose(t, "2D sample", n.Sample2D(position[0], position[1]), test.two[index])
			assertClose(t, "3D sample", n.Sample3D(position[0], position[1], position[2]), test.three[index])
		}
	}
}
