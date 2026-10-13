package random

import "math"

type gaussian struct {
	hasNext bool
	next    float64
}

func (g *gaussian) reset() { g.hasNext = false }

func (g *gaussian) sample(source interface{ NextDouble() float64 }) float64 {
	if g.hasNext {
		g.hasNext = false
		return g.next
	}
	for {
		x := 2*source.NextDouble() - 1
		y := 2*source.NextDouble() - 1
		radius := x*x + y*y
		if radius >= 1 || radius == 0 {
			continue
		}
		multiplier := math.Sqrt(-2 * math.Log(radius) / radius)
		g.next = y * multiplier
		g.hasNext = true
		return x * multiplier
	}
}
