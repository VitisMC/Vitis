package random

// PositionalScratch reuses built-in sources for independently owned generation work.
// Returned sources remain valid until the next At call on the same scratch.
type PositionalScratch struct {
	xoroshiro Xoroshiro
	legacy    Legacy
}

// At resets the scratch to the factory's exact positional state.
func (s *PositionalScratch) At(factory PositionalFactory, x, y, z int32) Source {
	switch f := factory.(type) {
	case xoroshiroPositional:
		lo, hi := uint64(PositionSeed(x, y, z))^f.lo, f.hi
		if lo|hi == 0 {
			lo, hi = goldenRatio, silverRatio
		}
		s.xoroshiro = Xoroshiro{lo: lo, hi: hi}
		return &s.xoroshiro
	case legacyPositional:
		s.legacy.SetSeed(PositionSeed(x, y, z) ^ f.seed)
		return &s.legacy
	default:
		return factory.At(x, y, z)
	}
}
