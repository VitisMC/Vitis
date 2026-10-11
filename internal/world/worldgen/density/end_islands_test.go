package density

import (
	"math"
	"testing"
)

func TestEndIslandsReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		seed   int64
		values []float64
	}{
		{0, []float64{0.5625, 0.5625, -0.52831685543060300, -0.171875, 0.14422577619552612, -0.40798473358154297}},
		{1, []float64{0.5625, 0.5625, -0.41171634197235110, 0.32100245356559753, 0.074514746665954590, -0.20682519674301147}},
		{123456789, []float64{0.5625, 0.5625, 0.47169709205627440, -0.57774841785430910, 0.17523354291915894, 0.15625}},
		{-1, []float64{0.5625, 0.5625, 0.004771232604980469, -0.30501544475555420, -0.60707521438598630, -0.84375}},
	}
	positions := [][2]int{{0, 0}, {8, 8}, {1000, -1000}, {10000, 10000}, {-10000, -10000}, {1000000, -2000000}}
	for _, test := range tests {
		sampler := NewEndIslands(test.seed)
		for index, position := range positions {
			actual := sampler.Sample(position[0], position[1])
			if math.Abs(actual-test.values[index]) > 1e-12 {
				t.Fatalf("seed %d at %v: got %.17g, want %.17g", test.seed, position, actual, test.values[index])
			}
		}
	}
}

func TestEndIslandsIsOrderIndependent(t *testing.T) {
	t.Parallel()

	sampler := NewEndIslands(42)
	first := sampler.Sample(12345, -6789)
	for x := -32; x <= 32; x++ {
		_ = sampler.Sample(x*16, x*-31)
	}
	if actual := sampler.Sample(12345, -6789); actual != first {
		t.Fatalf("sample changed with query order: %.17g != %.17g", actual, first)
	}
}
