package noise

import (
	"fmt"
	"math"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

const perlinWrap = 33554432.0

// Perlin combines improved noise samplers across configured octaves.
type Perlin struct {
	levels                []*Improved
	active                []perlinOctave
	firstOctave           int
	amplitudes            []float64
	lowestFreqValueFactor float64
	lowestFreqInputFactor float64
	maximumValue          float64
}

type perlinOctave struct {
	level       *Improved
	inputFactor float64
	valueFactor float64
	amplitude   float64
}

// NewPerlin creates a modern order-independent octave sampler.
func NewPerlin(source worldrandom.Source, firstOctave int, amplitudes []float64) (*Perlin, error) {
	return newPerlin(source, firstOctave, amplitudes, false)
}

// NewLegacyPerlin creates a sequential octave sampler for legacy generators.
func NewLegacyPerlin(source worldrandom.Source, firstOctave int, amplitudes []float64) (*Perlin, error) {
	return newPerlin(source, firstOctave, amplitudes, true)
}

func newPerlin(source worldrandom.Source, firstOctave int, amplitudes []float64, legacy bool) (*Perlin, error) {
	if source == nil {
		return nil, fmt.Errorf("create perlin noise: nil random source")
	}
	if len(amplitudes) == 0 {
		return nil, fmt.Errorf("create perlin noise: no amplitudes")
	}
	p := &Perlin{
		levels:      make([]*Improved, len(amplitudes)),
		firstOctave: firstOctave,
		amplitudes:  append([]float64(nil), amplitudes...),
	}
	zeroOctave := -firstOctave
	if legacy {
		if err := p.initLegacy(source, zeroOctave); err != nil {
			return nil, err
		}
	} else {
		factory := source.ForkPositional()
		for index, amplitude := range p.amplitudes {
			if amplitude != 0 {
				name := fmt.Sprintf("octave_%d", firstOctave+index)
				p.levels[index] = NewImproved(factory.FromHashOf(name))
			}
		}
	}
	p.lowestFreqInputFactor = math.Pow(2, float64(-zeroOctave))
	p.lowestFreqValueFactor = math.Pow(2, float64(len(amplitudes)-1)) / (math.Pow(2, float64(len(amplitudes))) - 1)
	inputFactor := p.lowestFreqInputFactor
	valueFactor := p.lowestFreqValueFactor
	for index, level := range p.levels {
		if level != nil {
			p.active = append(p.active, perlinOctave{
				level: level, inputFactor: inputFactor, valueFactor: valueFactor, amplitude: p.amplitudes[index],
			})
		}
		inputFactor *= 2
		valueFactor /= 2
	}
	p.maximumValue = p.edgeValue(2)
	return p, nil
}

func (p *Perlin) initLegacy(source worldrandom.Source, zeroOctave int) error {
	initial := NewImproved(source)
	if zeroOctave >= 0 && zeroOctave < len(p.levels) && p.amplitudes[zeroOctave] != 0 {
		p.levels[zeroOctave] = initial
	}
	for index := zeroOctave - 1; index >= 0; index-- {
		if index < len(p.levels) && p.amplitudes[index] != 0 {
			p.levels[index] = NewImproved(source)
		} else {
			consume(source, 262)
		}
	}
	for index, amplitude := range p.amplitudes {
		if amplitude != 0 && p.levels[index] == nil {
			return fmt.Errorf("create legacy perlin noise: unsupported positive octave %d", p.firstOctave+index)
		}
	}
	if zeroOctave < len(p.levels)-1 {
		return fmt.Errorf("create legacy perlin noise: positive octaves are unsupported")
	}
	return nil
}

// Sample returns the combined octave noise at a position.
func (p *Perlin) Sample(x, y, z float64) float64 {
	return p.SampleScaledY(x, y, z, 0, 0, false)
}

// SampleXZ samples the fixed Y=0 plane across all active octaves.
func (p *Perlin) SampleXZ(x, z float64) float64 {
	var result float64
	for _, octave := range p.active {
		value := octave.level.SampleXZ(wrap(x*octave.inputFactor), wrap(z*octave.inputFactor))
		result += octave.amplitude * value * octave.valueFactor
	}
	return result
}

// SampleScaledY samples with vertical quantization and an optional fixed octave Y offset.
func (p *Perlin) SampleScaledY(x, y, z, yScale, yLimit float64, fixedY bool) float64 {
	var result float64
	for _, octave := range p.active {
		sampleY := wrap(y * octave.inputFactor)
		if fixedY {
			_, yOffset, _ := octave.level.Offsets()
			sampleY = -yOffset
		}
		value := octave.level.SampleScaledY(
			wrap(x*octave.inputFactor), sampleY, wrap(z*octave.inputFactor),
			yScale*octave.inputFactor, yLimit*octave.inputFactor,
		)
		result += octave.amplitude * value * octave.valueFactor
	}
	return result
}

// MaxValue returns the maximum estimated absolute sampler value.
func (p *Perlin) MaxValue() float64 { return p.maximumValue }

// MaxBrokenValue returns the historical upper bound used by blended noise.
func (p *Perlin) MaxBrokenValue(value float64) float64 { return p.edgeValue(value + 2) }

// Octave returns the sampler for a descending octave index.
func (p *Perlin) Octave(index int) *Improved {
	level := len(p.levels) - 1 - index
	if level < 0 || level >= len(p.levels) {
		return nil
	}
	return p.levels[level]
}

func (p *Perlin) edgeValue(value float64) float64 {
	var result float64
	factor := p.lowestFreqValueFactor
	for index, level := range p.levels {
		if level != nil {
			result += p.amplitudes[index] * value * factor
		}
		factor /= 2
	}
	return result
}

func wrap(value float64) float64 {
	if value >= -perlinWrap/2 && value < perlinWrap/2 {
		return value
	}
	return value - math.Floor(value/perlinWrap+0.5)*perlinWrap
}

func consume(source worldrandom.Source, count int) {
	for range count {
		source.NextInt()
	}
}
