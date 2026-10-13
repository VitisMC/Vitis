package chunk

import "math"

// InhabitedTime returns the saved number of ticks this chunk spent near players.
func (c *Chunk) InhabitedTime() int64 {
	if c == nil {
		return 0
	}
	return c.inhabitedTime.Load()
}

// AddInhabitedTime advances local difficulty on the owning world tick without invalidating chunk packets.
func (c *Chunk) AddInhabitedTime(ticks int64) {
	if c == nil || ticks <= 0 {
		return
	}
	current := c.inhabitedTime.Load()
	next := current + min(ticks, math.MaxInt64-current)
	if next == current {
		return
	}
	c.inhabitedTime.Store(next)
	c.revision.Add(1)
	c.dirty.Store(true)
}
