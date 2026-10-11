package noise

import (
	"fmt"
	"math"

	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// BlendedParameters configures the legacy three-sampler terrain noise.
type BlendedParameters struct {
	XZScale              float64 `json:"xz_scale"`
	YScale               float64 `json:"y_scale"`
	XZFactor             float64 `json:"xz_factor"`
	YFactor              float64 `json:"y_factor"`
	SmearScaleMultiplier float64 `json:"smear_scale_multiplier"`
}

// Blended is the legacy terrain density sampler used by noise settings.
type Blended struct {
	minLimit             *Perlin
	maxLimit             *Perlin
	main                 *Perlin
	xzMultiplier         float64
	yMultiplier          float64
	xzFactor             float64
	yFactor              float64
	smearScaleMultiplier float64
	maximumValue         float64
}

// NewBlended creates a seeded blended terrain sampler.
func NewBlended(source worldrandom.Source, parameters BlendedParameters) (*Blended, error) {
	if source == nil {
		return nil, fmt.Errorf("create blended noise: nil random source")
	}
	if err := validateBlendedParameters(parameters); err != nil {
		return nil, err
	}
	limitAmplitudes := make([]float64, 16)
	for index := range limitAmplitudes {
		limitAmplitudes[index] = 1
	}
	mainAmplitudes := make([]float64, 8)
	for index := range mainAmplitudes {
		mainAmplitudes[index] = 1
	}
	minLimit, err := NewLegacyPerlin(source, -15, limitAmplitudes)
	if err != nil {
		return nil, err
	}
	maxLimit, err := NewLegacyPerlin(source, -15, limitAmplitudes)
	if err != nil {
		return nil, err
	}
	main, err := NewLegacyPerlin(source, -7, mainAmplitudes)
	if err != nil {
		return nil, err
	}
	yMultiplier := 684.412 * parameters.YScale
	return &Blended{
		minLimit: minLimit, maxLimit: maxLimit, main: main,
		xzMultiplier: 684.412 * parameters.XZScale,
		yMultiplier:  yMultiplier,
		xzFactor:     parameters.XZFactor, yFactor: parameters.YFactor,
		smearScaleMultiplier: parameters.SmearScaleMultiplier,
		maximumValue:         minLimit.MaxBrokenValue(yMultiplier),
	}, nil
}

// Sample returns terrain density for integer block coordinates.
func (n *Blended) Sample(blockX, blockY, blockZ int) float64 {
	x := float64(blockX) * n.xzMultiplier
	y := float64(blockY) * n.yMultiplier
	z := float64(blockZ) * n.xzMultiplier
	mainX, mainY, mainZ := x/n.xzFactor, y/n.yFactor, z/n.xzFactor
	smear := n.yMultiplier * n.smearScaleMultiplier
	mainSmear := smear / n.yFactor

	var minValue, maxValue, mainValue float64
	factor := 1.0
	for octave := 0; octave < 8; octave++ {
		if level := n.main.Octave(octave); level != nil {
			mainValue += level.SampleScaledY(
				wrap(mainX*factor), wrap(mainY*factor), wrap(mainZ*factor),
				mainSmear*factor, mainY*factor,
			) / factor
		}
		factor /= 2
	}
	blend := (mainValue/10 + 1) / 2
	factor = 1
	for octave := 0; octave < 16; octave++ {
		sampleX, sampleY, sampleZ := wrap(x*factor), wrap(y*factor), wrap(z*factor)
		yScale := smear * factor
		if blend < 1 {
			if level := n.minLimit.Octave(octave); level != nil {
				minValue += level.SampleScaledY(sampleX, sampleY, sampleZ, yScale, y*factor) / factor
			}
		}
		if blend > 0 {
			if level := n.maxLimit.Octave(octave); level != nil {
				maxValue += level.SampleScaledY(sampleX, sampleY, sampleZ, yScale, y*factor) / factor
			}
		}
		factor /= 2
	}
	return clampedLerp(minValue/512, maxValue/512, blend) / 128
}

// MaxValue returns the maximum estimated density value.
func (n *Blended) MaxValue() float64 { return n.maximumValue }

func validateBlendedParameters(parameters BlendedParameters) error {
	values := []float64{parameters.XZScale, parameters.YScale, parameters.XZFactor, parameters.YFactor, parameters.SmearScaleMultiplier}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("create blended noise: non-finite parameter")
		}
	}
	if parameters.XZScale < 0.001 || parameters.XZScale > 1000 || parameters.YScale < 0.001 || parameters.YScale > 1000 ||
		parameters.XZFactor < 0.001 || parameters.XZFactor > 1000 || parameters.YFactor < 0.001 || parameters.YFactor > 1000 ||
		parameters.SmearScaleMultiplier < 1 || parameters.SmearScaleMultiplier > 8 {
		return fmt.Errorf("create blended noise: parameter outside allowed range")
	}
	return nil
}

func clampedLerp(start, end, delta float64) float64 {
	if delta < 0 {
		return start
	}
	if delta > 1 {
		return end
	}
	return lerp(delta, start, end)
}
