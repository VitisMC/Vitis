package random

// Worldgen wraps a source with the seed derivations used by generation stages.
type Worldgen struct {
	source   Source
	count    uint64
	gaussian gaussian
}

// NewWorldgen creates a counted world generation source.
func NewWorldgen(source Source) *Worldgen {
	if source == nil {
		panic("nil worldgen random source")
	}
	return &Worldgen{source: source}
}

// Count returns the number of primitive random values consumed by this wrapper.
func (r *Worldgen) Count() uint64 { return r.count }

// SetSeed resets the wrapped source.
func (r *Worldgen) SetSeed(seed int64) { r.source.SetSeed(seed); r.gaussian.reset() }

func (r *Worldgen) next(bits uint) int32 {
	r.count++
	if legacy, ok := r.source.(*Legacy); ok {
		return legacy.next(bits)
	}
	return int32(uint64(r.source.NextLong()) >> (64 - bits))
}

// NextInt returns the next signed integer.
func (r *Worldgen) NextInt() int32 { return r.next(32) }

// NextIntBound returns the next bounded integer.
func (r *Worldgen) NextIntBound(bound int32) int32 {
	if bound <= 0 {
		panic("random bound must be positive")
	}
	if bound&(bound-1) == 0 {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		bits, value := r.next(31), int32(0)
		value = bits % bound
		if int32(uint32(bits-value)+uint32(bound-1)) >= 0 {
			return value
		}
	}
}

// NextLong returns the next signed long.
func (r *Worldgen) NextLong() int64 { return (int64(r.next(32)) << 32) + int64(r.next(32)) }

// NextBoolean returns a random Boolean.
func (r *Worldgen) NextBoolean() bool { return r.next(1) != 0 }

// NextFloat returns a random float.
func (r *Worldgen) NextFloat() float32 { return float32(r.next(24)) * 5.9604645e-8 }

// NextDouble returns a random double.
func (r *Worldgen) NextDouble() float64 {
	return float64((int64(r.next(26))<<27)+int64(r.next(27))) * 1.1102230246251565e-16
}

// NextGaussian returns a Gaussian sample.
func (r *Worldgen) NextGaussian() float64 { return r.gaussian.sample(r) }

// Fork delegates child source creation.
func (r *Worldgen) Fork() Source { return r.source.Fork() }

// ForkPositional delegates positional factory creation.
func (r *Worldgen) ForkPositional() PositionalFactory { return r.source.ForkPositional() }

// SetDecorationSeed derives the population seed for a block coordinate.
func (r *Worldgen) SetDecorationSeed(worldSeed int64, blockX, blockZ int32) int64 {
	r.SetSeed(worldSeed)
	xSeed := r.NextLong() | 1
	zSeed := r.NextLong() | 1
	seed := int64(blockX)*xSeed + int64(blockZ)*zSeed ^ worldSeed
	r.SetSeed(seed)
	return seed
}

// SetFeatureSeed derives the seed for a feature index and generation step.
func (r *Worldgen) SetFeatureSeed(decorationSeed int64, featureIndex, step int32) {
	r.SetSeed(decorationSeed + int64(featureIndex) + int64(int32(10000*step)))
}

// SetLargeFeatureSeed derives the seed for a structure chunk.
func (r *Worldgen) SetLargeFeatureSeed(worldSeed int64, chunkX, chunkZ int32) {
	r.SetSeed(worldSeed)
	xSeed, zSeed := r.NextLong(), r.NextLong()
	r.SetSeed(int64(chunkX)*xSeed ^ int64(chunkZ)*zSeed ^ worldSeed)
}

// SetLargeFeatureWithSalt derives a salted structure placement seed.
func (r *Worldgen) SetLargeFeatureWithSalt(worldSeed int64, regionX, regionZ, salt int32) {
	seed := int64(regionX)*341873128712 + int64(regionZ)*132897987541 + worldSeed + int64(salt)
	r.SetSeed(seed)
}

// SlimeChunkSource creates the legacy source used by slime chunk selection.
func SlimeChunkSource(chunkX, chunkZ int32, worldSeed, salt int64) Source {
	xSquared := int64(int32(chunkX * chunkX * 4987142))
	xLinear := int64(int32(chunkX * 5947611))
	zSquared := int64(chunkZ*chunkZ) * 4392871
	zLinear := int64(int32(chunkZ * 389711))
	return NewLegacy(worldSeed + xSquared + xLinear + zSquared + zLinear ^ salt)
}
