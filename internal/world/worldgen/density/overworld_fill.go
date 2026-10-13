package density

import "math"

// overworldFillDensity evaluates the standard terrain and noodle combination after interpolation.
type overworldFillDensity struct {
	generic Function
	ids     [5]int
}

// OverworldFillDensity specializes a validated standard final graph after its compilation.
// Callers must validate the 1.21.4 definitions before requesting this specialization.
func (c *Compiler) OverworldFillDensity(generic Function) Function {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.interpolators) != 5 {
		return generic
	}
	function := &overworldFillDensity{generic: generic}
	for index, channel := range c.interpolators {
		function.ids[index] = channel.id
	}
	return function
}

// Sample applies outer operations directly during filling and retains generic external sampling.
func (f *overworldFillDensity) Sample(p Position) float64 {
	if p.Evaluation == nil {
		return f.generic.Sample(p)
	}
	work := &p.Evaluation.interpolation
	if !work.active || work.x != p.X || work.y != p.Y || work.z != p.Z {
		return f.generic.Sample(p)
	}
	value := 0.64 * work.channel(f.ids[0])
	if value < -1 {
		value = -1
	} else if value > 1 {
		value = 1
	}
	value = value/2 - value*value*value/24
	if work.channel(f.ids[1]) < 0 {
		return value
	}
	thickness := work.channel(f.ids[2])
	ridgeA, ridgeB := math.Abs(work.channel(f.ids[3])), math.Abs(work.channel(f.ids[4]))
	if ridgeB > ridgeA {
		ridgeA = ridgeB
	}
	noodle := thickness + 1.5*ridgeA
	if noodle < value {
		return noodle
	}
	return value
}

// channel interpolates one channel from the current block's cached partials.
func (s *interpolationScratch) channel(id int) float64 {
	pair := s.row + s.pairs[id]
	low, high := s.partials[pair], s.partials[pair+1]
	return low + s.fraction*(high-low)
}
