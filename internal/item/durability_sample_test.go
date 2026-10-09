package item

import (
	"math"
	"math/rand"
	"testing"
)

// TestDurabilityLossMatchesBinomialMoments checks both direct trials and beta splitting.
func TestDurabilityLossMatchesBinomialMoments(t *testing.T) {
	for _, test := range []struct {
		points int32
		p      float64
	}{{3, .25}, {65, .6}, {10000, .1}, {10000, .7}, {math.MaxInt32, .6}} {
		random := rand.New(rand.NewSource(482))
		const samples = 10000
		var sum, squares float64
		mean := float64(test.points) * test.p
		variance := mean * (1 - test.p)
		for range samples {
			value := sampleDurabilityLoss(test.points, test.p, random.Float64, random.NormFloat64)
			if value < 0 || value > test.points {
				t.Fatal("durability sample escaped trial bounds")
			}
			delta := float64(value) - mean
			sum += delta
			squares += delta * delta
		}
		if math.Abs(sum/samples) > 6*math.Sqrt(variance/samples) || math.Abs(squares/samples-variance) > .08*variance {
			t.Fatalf("points=%d p=%g: mean error=%g variance=%g expected=%g", test.points, test.p, sum/samples, squares/samples, variance)
		}
	}
}

// TestDurabilityLossBoundsWorkAndHandlesEndpoints checks large damage and safe arithmetic.
func TestDurabilityLossBoundsWorkAndHandlesEndpoints(t *testing.T) {
	random := rand.New(rand.NewSource(18))
	if DurabilityLoss(0, 3, true) != 0 || DurabilityLoss(-1, 3, true) != 0 || DurabilityLoss(math.MaxInt32, 0, true) != math.MaxInt32 {
		t.Fatal("invalid trial count or absent Unbreaking changed damage")
	}
	for _, probability := range []float64{0, .001, .6, .999, 1} {
		calls := 0
		uniform := func() float64 { calls++; return random.Float64() }
		normal := func() float64 { calls++; return random.NormFloat64() }
		value := sampleDurabilityLoss(math.MaxInt32, probability, uniform, normal)
		if calls > 512 || value < 0 || value > math.MaxInt32 || probability == 0 && value != 0 || probability == 1 && value != math.MaxInt32 {
			t.Fatalf("p=%g value=%d random calls=%d", probability, value, calls)
		}
	}
	if value := DurabilityLoss(math.MaxInt32, math.MaxInt32, true); value < 1 || value > math.MaxInt32 {
		t.Fatal("large enchantment level overflowed sampling")
	}
}
