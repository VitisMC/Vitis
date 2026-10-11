package chunk

import (
	"errors"
	"time"
)

// ObservedGenerator reports successful terrain generation durations.
type ObservedGenerator struct {
	Generator Generator
	Record    func(time.Duration)
}

// Generate delegates generation and records the time of a completed chunk.
func (g ObservedGenerator) Generate(x, z int32) (*Chunk, error) {
	start := time.Now()
	generated, err := g.Generator.Generate(x, z)
	if err == nil && generated != nil {
		generated.MarkGenerationStart(start)
		if g.Record != nil {
			g.Record(time.Since(start))
		}
	}
	return generated, err
}

// Bounds forwards world-height metadata when the underlying generator provides it.
func (g ObservedGenerator) Bounds() (int, int, error) {
	bounded, ok := g.Generator.(interface{ Bounds() (int, int, error) })
	if !ok {
		return 0, 0, errors.New("generator bounds unavailable")
	}
	return bounded.Bounds()
}
