package random

// FirstFloatAt reads a positional source's first float without allocating built-in sources.
func FirstFloatAt(factory PositionalFactory, x, y, z int32) float32 {
	switch f := factory.(type) {
	case xoroshiroPositional:
		lo, hi := uint64(PositionSeed(x, y, z))^f.lo, f.hi
		if lo|hi == 0 {
			lo, hi = goldenRatio, silverRatio
		}
		source := Xoroshiro{lo: lo, hi: hi}
		return source.NextFloat()
	case legacyPositional:
		source := Legacy{seed: (uint64(PositionSeed(x, y, z)^f.seed) ^ legacyMultiplier) & legacyMask}
		return source.NextFloat()
	default:
		return factory.At(x, y, z).NextFloat()
	}
}

// FirstDoubleAt reads a positional source's first double without allocating built-in sources.
func FirstDoubleAt(factory PositionalFactory, x, y, z int32) float64 {
	switch f := factory.(type) {
	case xoroshiroPositional:
		lo, hi := uint64(PositionSeed(x, y, z))^f.lo, f.hi
		if lo|hi == 0 {
			lo, hi = goldenRatio, silverRatio
		}
		source := Xoroshiro{lo: lo, hi: hi}
		return source.NextDouble()
	case legacyPositional:
		source := Legacy{seed: (uint64(PositionSeed(x, y, z)^f.seed) ^ legacyMultiplier) & legacyMask}
		return source.NextDouble()
	default:
		return factory.At(x, y, z).NextDouble()
	}
}
