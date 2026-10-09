package item

import (
	"math"
	"math/rand"
)

// DurabilityLoss samples Unbreaking without work proportional to large damage amounts.
func DurabilityLoss(points, unbreaking int32, armor bool) int32 {
	if points <= 0 {
		return 0
	}
	if unbreaking <= 0 {
		return points
	}
	probability := 1 / (float64(unbreaking) + 1)
	if armor {
		probability = .6 + .4*probability
	}
	return sampleDurabilityLoss(points, probability, rand.Float64, rand.NormFloat64)
}

// sampleDurabilityLoss uses beta splitting to halve large binomial trial counts.
func sampleDurabilityLoss(points int32, probability float64, uniform, normal func() float64) int32 {
	var damage int32
	for points > 64 && probability > 0 && probability < 1 {
		middle := points/2 + 1
		left := durabilityGamma(float64(middle), uniform, normal)
		right := durabilityGamma(float64(points-middle)+1, uniform, normal)
		split := left / (left + right)
		if split >= probability {
			points, probability = middle-1, probability/split
		} else {
			damage += middle
			points, probability = points-middle, (probability-split)/(1-split)
		}
	}
	if probability >= 1 {
		return damage + points
	}
	if probability <= 0 {
		return damage
	}
	for range points {
		if uniform() < probability {
			damage++
		}
	}
	return damage
}

// durabilityGamma samples a gamma variate for shape >= 1 using normal rejection.
func durabilityGamma(shape float64, uniform, normal func() float64) float64 {
	d := shape - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := normal()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := uniform()
		if u < 1-.0331*x*x*x*x || math.Log(u) < .5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
