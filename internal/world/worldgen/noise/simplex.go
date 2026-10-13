package noise

import (
	"math"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

var (
	simplexF2 = 0.5 * (math.Sqrt(3) - 1)
	simplexG2 = (3 - math.Sqrt(3)) / 6
)

// Simplex is a seeded two and three-dimensional simplex noise sampler.
type Simplex struct {
	permutation [256]uint8
	xOffset     float64
	yOffset     float64
	zOffset     float64
}

// NewSimplex creates a simplex sampler and consumes the source state.
func NewSimplex(source worldrandom.Source) *Simplex {
	n := &Simplex{
		xOffset: source.NextDouble() * 256,
		yOffset: source.NextDouble() * 256,
		zOffset: source.NextDouble() * 256,
	}
	for index := range n.permutation {
		n.permutation[index] = uint8(index)
	}
	for index := range n.permutation {
		target := index + int(source.NextIntBound(int32(len(n.permutation)-index)))
		n.permutation[index], n.permutation[target] = n.permutation[target], n.permutation[index]
	}
	return n
}

// Sample2D returns two-dimensional simplex noise.
func (n *Simplex) Sample2D(x, y float64) float64 {
	skew := (x + y) * simplexF2
	xCell, yCell := int(math.Floor(x+skew)), int(math.Floor(y+skew))
	unskew := float64(xCell+yCell) * simplexG2
	x0, y0 := x-(float64(xCell)-unskew), y-(float64(yCell)-unskew)
	xStep, yStep := 0, 1
	if x0 > y0 {
		xStep, yStep = 1, 0
	}
	x1, y1 := x0-float64(xStep)+simplexG2, y0-float64(yStep)+simplexG2
	x2, y2 := x0-1+2*simplexG2, y0-1+2*simplexG2
	xHash, yHash := xCell&255, yCell&255
	g0 := n.at(xHash+n.at(yHash)) % 12
	g1 := n.at(xHash+xStep+n.at(yHash+yStep)) % 12
	g2 := n.at(xHash+1+n.at(yHash+1)) % 12
	return 70 * (simplexCorner(g0, x0, y0, 0, 0.5) + simplexCorner(g1, x1, y1, 0, 0.5) + simplexCorner(g2, x2, y2, 0, 0.5))
}

// Sample3D returns three-dimensional simplex noise.
func (n *Simplex) Sample3D(x, y, z float64) float64 {
	skew := (x + y + z) / 3
	xCell, yCell, zCell := int(math.Floor(x+skew)), int(math.Floor(y+skew)), int(math.Floor(z+skew))
	unskew := float64(xCell+yCell+zCell) / 6
	x0, y0, z0 := x-(float64(xCell)-unskew), y-(float64(yCell)-unskew), z-(float64(zCell)-unskew)
	firstX, firstY, firstZ, secondX, secondY, secondZ := simplexSteps(x0, y0, z0)
	x1, y1, z1 := x0-float64(firstX)+1.0/6, y0-float64(firstY)+1.0/6, z0-float64(firstZ)+1.0/6
	x2, y2, z2 := x0-float64(secondX)+1.0/3, y0-float64(secondY)+1.0/3, z0-float64(secondZ)+1.0/3
	x3, y3, z3 := x0-0.5, y0-0.5, z0-0.5
	xHash, yHash, zHash := xCell&255, yCell&255, zCell&255
	g0 := n.at(xHash+n.at(yHash+n.at(zHash))) % 12
	g1 := n.at(xHash+firstX+n.at(yHash+firstY+n.at(zHash+firstZ))) % 12
	g2 := n.at(xHash+secondX+n.at(yHash+secondY+n.at(zHash+secondZ))) % 12
	g3 := n.at(xHash+1+n.at(yHash+1+n.at(zHash+1))) % 12
	return 32 * (simplexCorner(g0, x0, y0, z0, 0.6) + simplexCorner(g1, x1, y1, z1, 0.6) + simplexCorner(g2, x2, y2, z2, 0.6) + simplexCorner(g3, x3, y3, z3, 0.6))
}

func (n *Simplex) at(index int) int { return int(n.permutation[uint8(index)]) }

func simplexCorner(gradient int, x, y, z, radius float64) float64 {
	falloff := radius - x*x - y*y - z*z
	if falloff < 0 {
		return 0
	}
	falloff *= falloff
	return falloff * falloff * dot(gradients[gradient], x, y, z)
}

func simplexSteps(x, y, z float64) (firstX, firstY, firstZ, secondX, secondY, secondZ int) {
	if x >= y {
		if y >= z {
			return 1, 0, 0, 1, 1, 0
		}
		if x >= z {
			return 1, 0, 0, 1, 0, 1
		}
		return 0, 0, 1, 1, 0, 1
	}
	if y < z {
		return 0, 0, 1, 0, 1, 1
	}
	if x < z {
		return 0, 1, 0, 0, 1, 1
	}
	return 0, 1, 0, 1, 1, 0
}
