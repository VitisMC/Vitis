package noise

import (
	"testing"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestBlendedReference(t *testing.T) {
	t.Parallel()

	parameters := BlendedParameters{
		XZScale: 0.25, YScale: 0.125, XZFactor: 80, YFactor: 160, SmearScaleMultiplier: 8,
	}
	tests := []struct {
		name   string
		source worldrandom.Source
		values []float64
	}{
		{
			name:   "xoroshiro source",
			source: worldrandom.NewXoroshiro(123456789),
			values: []float64{0.13749548965531366, 0.16523996443498270, -0.29703882444458707, 0.53753265157513510},
		},
		{
			name:   "legacy source",
			source: worldrandom.NewLegacy(123456789),
			values: []float64{-0.11996240999744690, 0.10684984567737184, -0.042991208186003604, -0.22802914708808760},
		},
	}
	positions := [][3]int{{0, 0, 0}, {1, -64, 1}, {100, 64, -200}, {33554433, 255, -33554431}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n, err := NewBlended(test.source, parameters)
			if err != nil {
				t.Fatal(err)
			}
			assertClose(t, "maximum", n.MaxValue(), 87.551500000000020)
			for index, position := range positions {
				assertClose(t, "sample", n.Sample(position[0], position[1], position[2]), test.values[index])
			}
		})
	}
}

// TestBlendedPreparedColumnsMatchScalar preserves octave arithmetic across reused columns.
func TestBlendedPreparedColumnsMatchScalar(t *testing.T) {
	for _, source := range []worldrandom.Source{worldrandom.NewLegacy(123), worldrandom.NewXoroshiro(123)} {
		sampler, err := NewBlended(source, BlendedParameters{XZScale: 0.25, YScale: 0.125, XZFactor: 80, YFactor: 160, SmearScaleMultiplier: 8})
		if err != nil {
			t.Fatal(err)
		}
		var column BlendedColumn
		for _, p := range [][2]int{{0, 0}, {-16, 32}, {16, -32}, {29999984, -29999984}} {
			sampler.PrepareColumn(p[0], p[1], &column)
			for y := -64; y <= 320; y += 8 {
				if got, want := column.Sample(y), sampler.Sample(p[0], y, p[1]); got != want {
					t.Fatalf("column %v y=%d: got %.17g want %.17g", p, y, got, want)
				}
			}
		}
	}
}

func TestBlendedRejectsInvalidParameters(t *testing.T) {
	t.Parallel()

	if _, err := NewBlended(worldrandom.NewXoroshiro(0), BlendedParameters{}); err == nil {
		t.Fatal("expected invalid parameters error")
	}
	if _, err := NewBlended(nil, BlendedParameters{XZScale: 1, YScale: 1, XZFactor: 1, YFactor: 1, SmearScaleMultiplier: 1}); err == nil {
		t.Fatal("expected nil source error")
	}
}
