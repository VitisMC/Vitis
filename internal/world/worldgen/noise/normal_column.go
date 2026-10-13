package noise

// perlinColumn owns mutable horizontal octave preparation for one independent evaluation.
type perlinColumn struct {
	batch   improvedBatchInput
	sampler *Perlin
	octaves []improvedColumn
}

// prepare resets octave columns while retaining their reusable storage.
func (c *perlinColumn) prepare(sampler *Perlin, x, z float64) {
	c.sampler = sampler
	if cap(c.octaves) < len(sampler.active) {
		c.octaves = make([]improvedColumn, len(sampler.active))
	} else {
		c.octaves = c.octaves[:len(sampler.active)]
	}
	for index, octave := range sampler.active {
		c.octaves[index] = prepareImprovedColumn(octave.level, wrap(x*octave.inputFactor), wrap(z*octave.inputFactor))
	}
}

// sampleY4 retains the scalar evaluator's octave and multiplication order.
func (c *perlinColumn) sampleY4(ys [4]float64) (result [4]float64) {
	for index, octave := range c.sampler.active {
		var scaled [4]float64
		for lane, y := range ys {
			scaled[lane] = wrap(y * octave.inputFactor)
		}
		values := c.octaves[index].sampleScaledY4WithScratch(scaled, 0, [4]float64{}, &c.batch)
		for lane, value := range values {
			result[lane] += octave.amplitude * value * octave.valueFactor
		}
	}
	return result
}

// NormalColumn reuses prepared octave inputs across four-height batches at one X/Z.
// A column belongs to one worker and must be prepared again when coordinates change.
type NormalColumn struct {
	sampler       *Normal
	first, second perlinColumn
}

// PrepareColumn resolves both normal-noise octave sets without changing their scaling.
func (n *Normal) PrepareColumn(x, z float64, column *NormalColumn) {
	column.sampler = n
	column.first.prepare(n.first, x, z)
	column.second.prepare(n.second, x*normalInputFactor, z*normalInputFactor)
}

// SampleY4 samples four heights using the prepared immutable horizontal inputs.
func (c *NormalColumn) SampleY4(ys [4]float64) (result [4]float64) {
	first := c.first.sampleY4(ys)
	var scaled [4]float64
	for lane, y := range ys {
		scaled[lane] = y * normalInputFactor
	}
	second := c.second.sampleY4(scaled)
	for lane := range result {
		result[lane] = (first[lane] + second[lane]) * c.sampler.valueFactor
	}
	return result
}
