package noise

import "math"

// improvedBatchInput is the fixed-width scalar preparation consumed by the SIMD kernel.
type improvedBatchInput struct {
	gradients              [8][4]uint8
	x, z, smoothX, smoothZ float64
	y, fractionY           [4]float64
}

// sampleScaledY4 samples four heights with one horizontal permutation preparation.
func (c *improvedColumn) sampleScaledY4(ys [4]float64, yScale float64, yLimits [4]float64) [4]float64 {
	var input improvedBatchInput
	return c.sampleScaledY4WithScratch(ys, yScale, yLimits, &input)
}

// sampleScaledY4WithScratch overwrites caller-owned preparation without clearing it between octaves.
func (c *improvedColumn) sampleScaledY4WithScratch(ys [4]float64, yScale float64, yLimits [4]float64, input *improvedBatchInput) [4]float64 {
	if !hasAVX2 {
		return c.sampleScaledY4Scalar(ys, yScale, yLimits)
	}
	input.x, input.z, input.smoothX, input.smoothZ = c.x, c.z, c.smoothX, c.smoothZ
	var previousCell int
	for lane, y := range ys {
		y += c.level.yOffset
		yFloor := math.Floor(y)
		fraction := y - yFloor
		offset := 0.0
		if yScale != 0 {
			clamped := fraction
			if yLimits[lane] >= 0 && yLimits[lane] < fraction {
				clamped = yLimits[lane]
			}
			offset = math.Floor(clamped/yScale+float64(float32(1e-7))) * yScale
		}
		input.y[lane], input.fractionY[lane] = fraction-offset, fraction
		cell := int(yFloor)
		if lane > 0 && cell == previousCell {
			for corner := range input.gradients {
				input.gradients[corner][lane] = input.gradients[corner][lane-1]
			}
			continue
		}
		previousCell = cell
		if lane == 0 && c.gradientsSet && uint8(cell) == c.gradientCell {
			for corner, gradient := range c.gradientIDs {
				input.gradients[corner][lane] = gradient
			}
			continue
		}
		p00, p01 := c.level.at(c.px0+cell), c.level.at(c.px0+cell+1)
		p10, p11 := c.level.at(c.px1+cell), c.level.at(c.px1+cell+1)
		input.gradients[0][lane] = uint8(c.level.at(p00+c.zCell) & 15)
		input.gradients[1][lane] = uint8(c.level.at(p10+c.zCell) & 15)
		input.gradients[2][lane] = uint8(c.level.at(p01+c.zCell) & 15)
		input.gradients[3][lane] = uint8(c.level.at(p11+c.zCell) & 15)
		input.gradients[4][lane] = uint8(c.level.at(p00+c.zCell+1) & 15)
		input.gradients[5][lane] = uint8(c.level.at(p10+c.zCell+1) & 15)
		input.gradients[6][lane] = uint8(c.level.at(p01+c.zCell+1) & 15)
		input.gradients[7][lane] = uint8(c.level.at(p11+c.zCell+1) & 15)
		if lane == 0 {
			c.gradientCell, c.gradientsSet = uint8(cell), true
			for corner := range c.gradientIDs {
				c.gradientIDs[corner] = uint8(input.gradients[corner][lane])
			}
		}
	}
	var result [4]float64
	improvedBatchAVX2(input, &result)
	return result
}

// sampleScaledY4Scalar is the portable reference for the vector kernel.
func (c *improvedColumn) sampleScaledY4Scalar(ys [4]float64, yScale float64, yLimits [4]float64) (result [4]float64) {
	for lane, y := range ys {
		result[lane] = c.sampleScaledY(y, yScale, yLimits[lane])
	}
	return result
}

// SampleY4 samples four heights without changing octave accumulation order.
func (p *Perlin) SampleY4(x float64, ys [4]float64, z float64) (result [4]float64) {
	for _, octave := range p.active {
		column := prepareImprovedColumn(octave.level, wrap(x*octave.inputFactor), wrap(z*octave.inputFactor))
		var scaled [4]float64
		for lane, y := range ys {
			scaled[lane] = wrap(y * octave.inputFactor)
		}
		values := column.sampleScaledY4(scaled, 0, [4]float64{})
		for lane, value := range values {
			result[lane] += octave.amplitude * value * octave.valueFactor
		}
	}
	return result
}

// SampleY4 evaluates a normal-noise batch with the scalar path's exact scaling.
func (n *Normal) SampleY4(x float64, ys [4]float64, z float64) (result [4]float64) {
	first := n.first.SampleY4(x, ys, z)
	var scaled [4]float64
	for lane, y := range ys {
		scaled[lane] = y * normalInputFactor
	}
	second := n.second.SampleY4(x*normalInputFactor, scaled, z*normalInputFactor)
	for lane := range result {
		result[lane] = (first[lane] + second[lane]) * n.valueFactor
	}
	return result
}

// Sample4 evaluates a prepared blended column in four-height batches.
func (c *BlendedColumn) Sample4(blockYs [4]int) (result [4]float64) {
	n := c.sampler
	var y, mainY [4]float64
	for lane, value := range blockYs {
		y[lane] = float64(value) * n.yMultiplier
		mainY[lane] = y[lane] / n.yFactor
	}
	smear := n.yMultiplier * n.smearScaleMultiplier
	mainSmear := smear / n.yFactor
	var mainValue, minimum, maximum, blend [4]float64
	factor := 1.0
	for octave := 0; octave < 8; octave++ {
		if level := &c.main[octave]; level.level != nil {
			var ys, limits [4]float64
			for lane, value := range mainY {
				limits[lane] = value * factor
				ys[lane] = wrap(limits[lane])
			}
			values := level.sampleScaledY4WithScratch(ys, mainSmear*factor, limits, &c.batch)
			for lane, value := range values {
				mainValue[lane] += value / factor
			}
		}
		factor /= 2
	}
	needMinimum, needMaximum := false, false
	for lane, value := range mainValue {
		blend[lane] = (value/10 + 1) / 2
		needMinimum = needMinimum || blend[lane] < 1
		needMaximum = needMaximum || blend[lane] > 0
	}
	factor = 1
	for octave := 0; octave < 16; octave++ {
		var ys, limits [4]float64
		for lane, value := range y {
			limits[lane] = value * factor
			ys[lane] = wrap(limits[lane])
		}
		if level := &c.minimum[octave]; needMinimum && level.level != nil {
			values := level.sampleScaledY4WithScratch(ys, smear*factor, limits, &c.batch)
			for lane, value := range values {
				if blend[lane] < 1 {
					minimum[lane] += value / factor
				}
			}
		}
		if level := &c.maximum[octave]; needMaximum && level.level != nil {
			values := level.sampleScaledY4WithScratch(ys, smear*factor, limits, &c.batch)
			for lane, value := range values {
				if blend[lane] > 0 {
					maximum[lane] += value / factor
				}
			}
		}
		factor /= 2
	}
	for lane := range result {
		result[lane] = clampedLerp(minimum[lane]/512, maximum[lane]/512, blend[lane]) / 128
	}
	return result
}
