package noise

import (
	"testing"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestNoiseBatchesMatchScalar verifies exact results for both SIMD and portable kernels.
func TestNoiseBatchesMatchScalar(t *testing.T) {
	for _, seed := range []int64{0, 42, -1234567} {
		level := NewImproved(worldrandom.NewXoroshiro(seed))
		for _, coordinate := range [][2]float64{{0, 0}, {-16.5, 37.25}, {29999984, -29999984}} {
			column := prepareImprovedColumn(level, coordinate[0], coordinate[1])
			for _, scale := range []float64{0, 0.02, -0.02, 1.5, 1000, 1e-12} {
				for _, ys := range [][4]float64{{-64, -56, -48, -40}, {-0.25, 0, 0.125, 1.75}, {256, 264, 272, 280}} {
					limits := [4]float64{-1, 0, 0.2, 2}
					got := column.sampleScaledY4(ys, scale, limits)
					want := column.sampleScaledY4Scalar(ys, scale, limits)
					if got != want {
						t.Fatalf("improved seed=%d xz=%v scale=%g: %v != %v", seed, coordinate, scale, got, want)
					}
				}
			}
		}
		normal, err := NewNormal(worldrandom.NewXoroshiro(seed), Parameters{FirstOctave: -5, Amplitudes: []float64{1, 0, 0.5, 1}})
		if err != nil {
			t.Fatal(err)
		}
		var normalColumn NormalColumn
		for _, xz := range [][2]float64{{0, 0}, {-13, 41}, {29999984, -29999984}, {0, 0}} {
			normal.PrepareColumn(xz[0], xz[1], &normalColumn)
			for _, heights := range [][4]float64{{-64, -56, -48, -40}, {-0.25, 0, 0.125, 1.75}, {304, 312, 320, 320}, {-30000000, -29999992, 29999992, 30000000}} {
				prepared := normalColumn.SampleY4(heights)
				for lane, y := range heights {
					if want := normal.Sample(xz[0], y, xz[1]); prepared[lane] != want {
						t.Fatalf("prepared normal seed=%d xz=%v y=%g: %.17g != %.17g", seed, xz, y, prepared[lane], want)
					}
				}
			}
			ys := [4]float64{-64, -56, -48, -40}
			values := normal.SampleY4(xz[0], ys, xz[1])
			for lane, y := range ys {
				if values[lane] != normal.Sample(xz[0], y, xz[1]) {
					t.Fatal("normal batch changed octave arithmetic")
				}
			}
		}
		blended, err := NewBlended(worldrandom.NewXoroshiro(seed), BlendedParameters{XZScale: 0.25, YScale: 0.125, XZFactor: 80, YFactor: 160, SmearScaleMultiplier: 8})
		if err != nil {
			t.Fatal(err)
		}
		var column BlendedColumn
		blended.PrepareColumn(-32, 48, &column)
		for y := -64; y <= 256; y += 32 {
			ys := [4]int{y, y + 8, y + 16, y + 24}
			got := column.Sample4(ys)
			for lane, value := range ys {
				if want := column.Sample(value); got[lane] != want {
					t.Fatalf("blended at %d: %.17g != %.17g", value, got[lane], want)
				}
			}
		}
	}
}

// BenchmarkImprovedFourScalar measures the portable four-height reference.
func BenchmarkImprovedFourScalar(b *testing.B) {
	column := prepareImprovedColumn(NewImproved(worldrandom.NewXoroshiro(42)), 16, 32)
	var result [4]float64
	for range b.N {
		result = column.sampleScaledY4Scalar([4]float64{-64, -56, -48, -40}, 0, [4]float64{})
	}
	benchmarkBatch = result
}

// BenchmarkImprovedFourVector measures scalar preparation and the selected vector kernel.
func BenchmarkImprovedFourVector(b *testing.B) {
	column := prepareImprovedColumn(NewImproved(worldrandom.NewXoroshiro(42)), 16, 32)
	var result [4]float64
	for range b.N {
		result = column.sampleScaledY4([4]float64{-64, -56, -48, -40}, 0, [4]float64{})
	}
	benchmarkBatch = result
}

var benchmarkBatch [4]float64
