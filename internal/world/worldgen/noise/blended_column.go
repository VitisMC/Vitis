package noise

import "math"

// improvedColumn keeps the horizontal permutation and smoothing values for a noise column.
type improvedColumn struct {
	gradientCell           uint8
	gradientsSet           bool
	gradientIDs            [8]uint8
	level                  *Improved
	px0, px1, zCell        int
	x, z, smoothX, smoothZ float64
}

// prepareImprovedColumn resolves the invariant horizontal portion of a sampler.
func prepareImprovedColumn(level *Improved, x, z float64) improvedColumn {
	if level == nil {
		return improvedColumn{}
	}
	x += level.xOffset
	z += level.zOffset
	xFloor, zFloor := math.Floor(x), math.Floor(z)
	x -= xFloor
	z -= zFloor
	return improvedColumn{level: level, px0: level.at(int(xFloor)), px1: level.at(int(xFloor) + 1), zCell: int(zFloor), x: x, z: z, smoothX: smoothstep(x), smoothZ: smoothstep(z)}
}

// sampleScaledY evaluates only the height-dependent part of an improved-noise column.
func (c *improvedColumn) sampleScaledY(y, yScale, yLimit float64) float64 {
	y += c.level.yOffset
	yFloor := math.Floor(y)
	yFraction := y - yFloor
	yOffset := 0.0
	if yScale != 0 {
		clamped := yFraction
		if yLimit >= 0 && yLimit < yFraction {
			clamped = yLimit
		}
		yOffset = math.Floor(clamped/yScale+float64(float32(1e-7))) * yScale
	}
	return c.level.samplePrepared(c.px0, c.px1, int(yFloor), c.zCell, c.x, yFraction-yOffset, c.z, c.smoothX, smoothstep(yFraction), c.smoothZ)
}

// BlendedColumn owns reusable horizontal state for the blended terrain sampler.
type BlendedColumn struct {
	batch            improvedBatchInput
	sampler          *Blended
	main             [8]improvedColumn
	minimum, maximum [16]improvedColumn
}

// PrepareColumn resets horizontal state without allocating or changing octave order.
func (n *Blended) PrepareColumn(x, z int, column *BlendedColumn) {
	column.sampler = n
	worldX, worldZ := float64(x)*n.xzMultiplier, float64(z)*n.xzMultiplier
	mainX, mainZ := worldX/n.xzFactor, worldZ/n.xzFactor
	factor := 1.0
	for octave := 0; octave < 16; octave++ {
		if octave < 8 {
			column.main[octave] = prepareImprovedColumn(n.main.Octave(octave), wrap(mainX*factor), wrap(mainZ*factor))
		}
		x, z := wrap(worldX*factor), wrap(worldZ*factor)
		column.minimum[octave] = prepareImprovedColumn(n.minLimit.Octave(octave), x, z)
		column.maximum[octave] = prepareImprovedColumn(n.maxLimit.Octave(octave), x, z)
		factor /= 2
	}
}

// Sample evaluates blended noise at one height using the prepared horizontal column.
func (c *BlendedColumn) Sample(blockY int) float64 {
	n := c.sampler
	y := float64(blockY) * n.yMultiplier
	mainY := y / n.yFactor
	smear := n.yMultiplier * n.smearScaleMultiplier
	mainSmear := smear / n.yFactor
	minimum, maximum, mainValue := 0.0, 0.0, 0.0
	factor := 1.0
	for octave := 0; octave < 8; octave++ {
		if level := &c.main[octave]; level.level != nil {
			mainValue += level.sampleScaledY(wrap(mainY*factor), mainSmear*factor, mainY*factor) / factor
		}
		factor /= 2
	}
	blend := (mainValue/10 + 1) / 2
	factor = 1
	for octave := 0; octave < 16; octave++ {
		sampleY, yScale := wrap(y*factor), smear*factor
		if blend < 1 {
			if level := &c.minimum[octave]; level.level != nil {
				minimum += level.sampleScaledY(sampleY, yScale, y*factor) / factor
			}
		}
		if blend > 0 {
			if level := &c.maximum[octave]; level.level != nil {
				maximum += level.sampleScaledY(sampleY, yScale, y*factor) / factor
			}
		}
		factor /= 2
	}
	return clampedLerp(minimum/512, maximum/512, blend) / 128
}
