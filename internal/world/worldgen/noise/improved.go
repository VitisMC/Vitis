// Package noise implements deterministic noise samplers used by world generation.
package noise

import (
	"math"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

var gradients = [16][3]float64{
	{1, 1, 0}, {-1, 1, 0}, {1, -1, 0}, {-1, -1, 0},
	{1, 0, 1}, {-1, 0, 1}, {1, 0, -1}, {-1, 0, -1},
	{0, 1, 1}, {0, -1, 1}, {0, 1, -1}, {0, -1, -1},
	{1, 1, 0}, {0, -1, 1}, {-1, 1, 0}, {0, -1, -1},
}

// Improved is a seeded three-dimensional gradient noise sampler.
type Improved struct {
	permutation [256]uint8
	xOffset     float64
	yOffset     float64
	zOffset     float64
	yFloor      int
	yFraction   float64
}

// NewImproved creates an improved noise sampler and consumes the source state.
func NewImproved(source worldrandom.Source) *Improved {
	n := &Improved{
		xOffset: source.NextDouble() * 256,
		yOffset: source.NextDouble() * 256,
		zOffset: source.NextDouble() * 256,
	}
	for i := range n.permutation {
		n.permutation[i] = uint8(i)
	}
	for i := 0; i < len(n.permutation); i++ {
		j := i + int(source.NextIntBound(int32(len(n.permutation)-i)))
		n.permutation[i], n.permutation[j] = n.permutation[j], n.permutation[i]
	}
	yFloor := math.Floor(n.yOffset)
	n.yFloor, n.yFraction = int(yFloor), n.yOffset-yFloor
	return n
}

// Offsets returns the random coordinate offsets of the sampler.
func (n *Improved) Offsets() (x, y, z float64) {
	return n.xOffset, n.yOffset, n.zOffset
}

// Sample returns noise at the supplied coordinates.
func (n *Improved) Sample(x, y, z float64) float64 {
	return n.SampleScaledY(x, y, z, 0, 0)
}

// SampleXZ samples the fixed Y=0 plane without recomputing its cell position.
func (n *Improved) SampleXZ(x, z float64) float64 {
	x += n.xOffset
	z += n.zOffset
	xFloor, zFloor := math.Floor(x), math.Floor(z)
	return n.sampleAndLerp(int(xFloor), n.yFloor, int(zFloor), x-xFloor, n.yFraction, z-zFloor, n.yFraction)
}

// SampleScaledY samples noise with quantized vertical coordinates.
func (n *Improved) SampleScaledY(x, y, z, yScale, yLimit float64) float64 {
	x += n.xOffset
	y += n.yOffset
	z += n.zOffset
	xFloor, yFloor, zFloor := math.Floor(x), math.Floor(y), math.Floor(z)
	xFraction, yFraction, zFraction := x-xFloor, y-yFloor, z-zFloor

	var yOffset float64
	if yScale != 0 {
		clamped := yFraction
		if yLimit >= 0 && yLimit < yFraction {
			clamped = yLimit
		}
		yOffset = math.Floor(clamped/yScale+float64(float32(1e-7))) * yScale
	}
	return n.sampleAndLerp(int(xFloor), int(yFloor), int(zFloor), xFraction, yFraction-yOffset, zFraction, yFraction)
}

// SampleWithDerivative adds the sample gradient to derivative and returns the noise value.
func (n *Improved) SampleWithDerivative(x, y, z float64, derivative *[3]float64) float64 {
	x += n.xOffset
	y += n.yOffset
	z += n.zOffset
	xFloor, yFloor, zFloor := math.Floor(x), math.Floor(y), math.Floor(z)
	return n.sampleDerivative(
		int(xFloor), int(yFloor), int(zFloor),
		x-xFloor, y-yFloor, z-zFloor, derivative,
	)
}

func (n *Improved) at(index int) int {
	return int(n.permutation[uint8(index)])
}

func (n *Improved) cornerGradients(xCell, yCell, zCell int) [8][3]float64 {
	px0, px1 := n.at(xCell), n.at(xCell+1)
	px0y0, px0y1 := n.at(px0+yCell), n.at(px0+yCell+1)
	px1y0, px1y1 := n.at(px1+yCell), n.at(px1+yCell+1)
	return [8][3]float64{
		gradients[n.at(px0y0+zCell)&15], gradients[n.at(px1y0+zCell)&15],
		gradients[n.at(px0y1+zCell)&15], gradients[n.at(px1y1+zCell)&15],
		gradients[n.at(px0y0+zCell+1)&15], gradients[n.at(px1y0+zCell+1)&15],
		gradients[n.at(px0y1+zCell+1)&15], gradients[n.at(px1y1+zCell+1)&15],
	}
}

func (n *Improved) sampleAndLerp(xCell, yCell, zCell int, x, y, z, smoothY float64) float64 {
	px0, px1 := n.at(xCell), n.at(xCell+1)
	return n.samplePrepared(px0, px1, yCell, zCell, x, y, z, smoothstep(x), smoothstep(smoothY), smoothstep(z))
}

// samplePrepared interpolates a noise cell with precomputed horizontal coordinates.
func (n *Improved) samplePrepared(px0, px1, yCell, zCell int, x, y, z, sx, sy, sz float64) float64 {
	px0y0, px0y1 := n.at(px0+yCell), n.at(px0+yCell+1)
	px1y0, px1y1 := n.at(px1+yCell), n.at(px1+yCell+1)
	v000 := dot(gradients[n.at(px0y0+zCell)&15], x, y, z)
	v100 := dot(gradients[n.at(px1y0+zCell)&15], x-1, y, z)
	v010 := dot(gradients[n.at(px0y1+zCell)&15], x, y-1, z)
	v110 := dot(gradients[n.at(px1y1+zCell)&15], x-1, y-1, z)
	v001 := dot(gradients[n.at(px0y0+zCell+1)&15], x, y, z-1)
	v101 := dot(gradients[n.at(px1y0+zCell+1)&15], x-1, y, z-1)
	v011 := dot(gradients[n.at(px0y1+zCell+1)&15], x, y-1, z-1)
	v111 := dot(gradients[n.at(px1y1+zCell+1)&15], x-1, y-1, z-1)
	x00, x10 := v000+sx*(v100-v000), v010+sx*(v110-v010)
	x01, x11 := v001+sx*(v101-v001), v011+sx*(v111-v011)
	y0, y1 := x00+sy*(x10-x00), x01+sy*(x11-x01)
	return y0 + sz*(y1-y0)
}

func (n *Improved) sampleDerivative(xCell, yCell, zCell int, x, y, z float64, derivative *[3]float64) float64 {
	g := n.cornerGradients(xCell, yCell, zCell)
	values := [8]float64{
		dot(g[0], x, y, z), dot(g[1], x-1, y, z),
		dot(g[2], x, y-1, z), dot(g[3], x-1, y-1, z),
		dot(g[4], x, y, z-1), dot(g[5], x-1, y, z-1),
		dot(g[6], x, y-1, z-1), dot(g[7], x-1, y-1, z-1),
	}
	sx, sy, sz := smoothstep(x), smoothstep(y), smoothstep(z)
	gradientXValues := gradientComponent(g, 0)
	gradientYValues := gradientComponent(g, 1)
	gradientZValues := gradientComponent(g, 2)
	gradientX := lerp3(sx, sy, sz, &gradientXValues)
	gradientY := lerp3(sx, sy, sz, &gradientYValues)
	gradientZ := lerp3(sx, sy, sz, &gradientZValues)
	changeX := lerp2(sy, sz, values[1]-values[0], values[3]-values[2], values[5]-values[4], values[7]-values[6])
	changeY := lerp2(sz, sx, values[2]-values[0], values[6]-values[4], values[3]-values[1], values[7]-values[5])
	changeZ := lerp2(sx, sy, values[4]-values[0], values[5]-values[1], values[6]-values[2], values[7]-values[3])
	derivative[0] += gradientX + smoothstepDerivative(x)*changeX
	derivative[1] += gradientY + smoothstepDerivative(y)*changeY
	derivative[2] += gradientZ + smoothstepDerivative(z)*changeZ
	return lerp3(sx, sy, sz, &values)
}

func gradientComponent(gradient [8][3]float64, component int) (result [8]float64) {
	for i := range gradient {
		result[i] = gradient[i][component]
	}
	return result
}

func dot(gradient [3]float64, x, y, z float64) float64 {
	return gradient[0]*x + gradient[1]*y + gradient[2]*z
}

func lerp(delta, start, end float64) float64 { return start + delta*(end-start) }

func lerp2(x, y, value00, value10, value01, value11 float64) float64 {
	return lerp(y, lerp(x, value00, value10), lerp(x, value01, value11))
}

func lerp3(x, y, z float64, values *[8]float64) float64 {
	return lerp(z,
		lerp2(x, y, values[0], values[1], values[2], values[3]),
		lerp2(x, y, values[4], values[5], values[6], values[7]),
	)
}

func smoothstep(value float64) float64 {
	return value * value * value * (value*(value*6-15) + 10)
}

func smoothstepDerivative(value float64) float64 {
	return 30 * value * value * (value - 1) * (value - 1)
}
