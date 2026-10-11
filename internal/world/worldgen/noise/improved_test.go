package noise

import (
	"math"
	"testing"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

var improvedBenchmarkResult float64

func BenchmarkImprovedSampleScaledY(b *testing.B) {
	noise := NewImproved(worldrandom.NewLegacy(123456789))
	var result float64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := float64(i&31) * 0.137
		y := float64(i>>5&31) * 0.191
		z := float64(i>>10&31) * -0.223
		result += noise.SampleScaledY(x, y, z, 0, 0)
	}
	improvedBenchmarkResult = result
}

func TestImprovedLegacyReference(t *testing.T) {
	t.Parallel()

	n := NewImproved(worldrandom.NewLegacy(0))
	xOffset, yOffset, zOffset := n.Offsets()
	assertClose(t, "x offset", xOffset, 187.12775356842420)
	assertClose(t, "y offset", yOffset, 61.577322411900380)
	assertClose(t, "z offset", zOffset, 163.17886088962770)

	derivative := [3]float64{}
	value := n.SampleWithDerivative(1.25, -3.5, 9.75, &derivative)
	assertClose(t, "value", value, -0.47924174146710974)
	assertClose(t, "dx", derivative[0], -0.59782036218365350)
	assertClose(t, "dy", derivative[1], -0.73856042108653290)
	assertClose(t, "dz", derivative[2], -0.41525364936086906)
}

// TestHorizontalNoiseMatchesThreeDimensionalSampling protects fixed-plane output.
func TestHorizontalNoiseMatchesThreeDimensionalSampling(t *testing.T) {
	positions := [][2]float64{{0, 0}, {1.25, -3.5}, {-1024.75, 4096.5}, {33554433.125, -33554431.75}}
	improved := NewImproved(worldrandom.NewLegacy(123))
	perlin, err := NewPerlin(worldrandom.NewXoroshiro(123), -3, []float64{1, 1, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	normal, err := NewNormal(worldrandom.NewXoroshiro(123), Parameters{FirstOctave: -3, Amplitudes: []float64{1, 1, 0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, position := range positions {
		x, z := position[0], position[1]
		if got, want := improved.SampleXZ(x, z), improved.Sample(x, 0, z); got != want {
			t.Fatalf("improved at %v: got %v, want %v", position, got, want)
		}
		if got, want := perlin.SampleXZ(x, z), perlin.Sample(x, 0, z); got != want {
			t.Fatalf("perlin at %v: got %v, want %v", position, got, want)
		}
		if got, want := normal.SampleXZ(x, z), normal.Sample(x, 0, z); got != want {
			t.Fatalf("normal at %v: got %v, want %v", position, got, want)
		}
	}
}

func TestImprovedLargeCoordinates(t *testing.T) {
	t.Parallel()

	n := NewImproved(worldrandom.NewLegacy(123456789))
	derivative := [3]float64{}
	value := n.SampleWithDerivative(33554433.125, 64.2, -33554431.75, &derivative)
	assertClose(t, "value", value, 0.091859695757788100)
	assertClose(t, "dx", derivative[0], -0.67121075939769440)
	assertClose(t, "dy", derivative[1], 1.4234922500769134)
	assertClose(t, "dz", derivative[2], -0.24000790506710423)
}

func assertClose(t *testing.T, name string, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 1e-14 {
		t.Fatalf("%s: got %.17g, want %.17g", name, actual, expected)
	}
}
