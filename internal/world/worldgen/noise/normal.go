package noise

import worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"

const normalInputFactor = 1.0181268882175227

// Parameters configures octave amplitudes for a normal noise sampler.
type Parameters struct {
	FirstOctave int       `json:"firstOctave"`
	Amplitudes  []float64 `json:"amplitudes"`
}

// Normal combines two octave samplers to approximate a normal distribution.
type Normal struct {
	first        *Perlin
	second       *Perlin
	parameters   Parameters
	valueFactor  float64
	maximumValue float64
}

// NewNormal creates a modern normal noise sampler.
func NewNormal(source worldrandom.Source, parameters Parameters) (*Normal, error) {
	return newNormal(source, parameters, false)
}

// NewLegacyNormal creates a sequential normal sampler for legacy generators.
func NewLegacyNormal(source worldrandom.Source, parameters Parameters) (*Normal, error) {
	return newNormal(source, parameters, true)
}

func newNormal(source worldrandom.Source, parameters Parameters, legacy bool) (*Normal, error) {
	firstNonZero, lastNonZero := -1, -1
	for index, amplitude := range parameters.Amplitudes {
		if amplitude != 0 {
			if firstNonZero < 0 {
				firstNonZero = index
			}
			lastNonZero = index
		}
	}
	constructor := NewPerlin
	if legacy {
		constructor = NewLegacyPerlin
	}
	first, err := constructor(source, parameters.FirstOctave, parameters.Amplitudes)
	if err != nil {
		return nil, err
	}
	second, err := constructor(source, parameters.FirstOctave, parameters.Amplitudes)
	if err != nil {
		return nil, err
	}
	octaveSpan := lastNonZero - firstNonZero
	if firstNonZero < 0 {
		// Integer subtraction wraps to one for the empty range in the reference implementation.
		octaveSpan = 1
	}
	valueFactor := (1.0 / 6.0) / expectedDeviation(octaveSpan)
	return &Normal{
		first: first, second: second,
		parameters: Parameters{
			FirstOctave: parameters.FirstOctave,
			Amplitudes:  append([]float64(nil), parameters.Amplitudes...),
		},
		valueFactor:  valueFactor,
		maximumValue: (first.MaxValue() + second.MaxValue()) * valueFactor,
	}, nil
}

// Sample returns normal noise at the supplied coordinates.
func (n *Normal) Sample(x, y, z float64) float64 {
	return (n.first.Sample(x, y, z) + n.second.Sample(x*normalInputFactor, y*normalInputFactor, z*normalInputFactor)) * n.valueFactor
}

// SampleXZ samples the fixed Y=0 plane for surface and horizontal noise.
func (n *Normal) SampleXZ(x, z float64) float64 {
	return (n.first.SampleXZ(x, z) + n.second.SampleXZ(x*normalInputFactor, z*normalInputFactor)) * n.valueFactor
}

// MaxValue returns the maximum estimated absolute sampler value.
func (n *Normal) MaxValue() float64 { return n.maximumValue }

// Parameters returns an independent copy of the sampler parameters.
func (n *Normal) Parameters() Parameters {
	return Parameters{FirstOctave: n.parameters.FirstOctave, Amplitudes: append([]float64(nil), n.parameters.Amplitudes...)}
}

func expectedDeviation(octaveSpan int) float64 {
	return 0.1 * (1 + 1/float64(octaveSpan+1))
}
