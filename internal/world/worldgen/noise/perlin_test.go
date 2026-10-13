package noise

import (
	"testing"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

func TestPerlinReference(t *testing.T) {
	t.Parallel()

	amplitudes := []float64{1, 0.5, 0, 2}
	tests := []struct {
		name     string
		newNoise func() (*Perlin, error)
		values   []float64
	}{
		{
			name: "modern",
			newNoise: func() (*Perlin, error) {
				return NewPerlin(worldrandom.NewXoroshiro(123456789), -5, amplitudes)
			},
			values: []float64{-0.19597955607018197, -0.35675159095519365, 0.28902244413000920},
		},
		{
			name: "legacy",
			newNoise: func() (*Perlin, error) {
				return NewLegacyPerlin(worldrandom.NewLegacy(123456789), -5, amplitudes)
			},
			values: []float64{-0.31733613715372800, -0.31367199125754086, 0.20206416125612442},
		},
	}
	positions := [][3]float64{{0, 0, 0}, {1.25, -3.5, 9.75}, {33554433.125, 64.2, -33554431.75}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n, err := test.newNoise()
			if err != nil {
				t.Fatal(err)
			}
			assertClose(t, "maximum", n.MaxValue(), 1.5999999999999999)
			for index, position := range positions {
				assertClose(t, "sample", n.Sample(position[0], position[1], position[2]), test.values[index])
			}
		})
	}
}

func TestNormalReference(t *testing.T) {
	t.Parallel()

	parameters := Parameters{FirstOctave: -5, Amplitudes: []float64{1, 0.5, 0, 2}}
	tests := []struct {
		name     string
		newNoise func() (*Normal, error)
		values   []float64
	}{
		{
			name: "modern",
			newNoise: func() (*Normal, error) {
				return NewNormal(worldrandom.NewXoroshiro(123456789), parameters)
			},
			values: []float64{0.13293415464920325, -0.13168965608583250, 0.50110875195314410},
		},
		{
			name: "legacy",
			newNoise: func() (*Normal, error) {
				return NewLegacyNormal(worldrandom.NewLegacy(123456789), parameters)
			},
			values: []float64{-0.22928784879534153, -0.31139019920755595, 0.18246760278539564},
		},
	}
	positions := [][3]float64{{0, 0, 0}, {1.25, -3.5, 9.75}, {33554433.125, 64.2, -33554431.75}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n, err := test.newNoise()
			if err != nil {
				t.Fatal(err)
			}
			assertClose(t, "maximum", n.MaxValue(), 4.2666666666666660)
			for index, position := range positions {
				assertClose(t, "sample", n.Sample(position[0], position[1], position[2]), test.values[index])
			}
		})
	}
}

func TestNoiseValidationAndParameterCopy(t *testing.T) {
	t.Parallel()

	if _, err := NewPerlin(nil, -1, []float64{1}); err == nil {
		t.Fatal("expected nil source error")
	}
	zero, err := NewNormal(worldrandom.NewXoroshiro(0), Parameters{FirstOctave: 0, Amplitudes: []float64{0}})
	if err != nil {
		t.Fatal(err)
	}
	if zero.Sample(1, 2, 3) != 0 || zero.MaxValue() != 0 {
		t.Fatal("all-zero normal noise must remain zero")
	}

	parameters := Parameters{FirstOctave: -1, Amplitudes: []float64{1}}
	n, err := NewNormal(worldrandom.NewXoroshiro(0), parameters)
	if err != nil {
		t.Fatal(err)
	}
	parameters.Amplitudes[0] = 9
	copy := n.Parameters()
	copy.Amplitudes[0] = 8
	if n.Parameters().Amplitudes[0] != 1 {
		t.Fatal("normal noise parameters are mutable")
	}
}
