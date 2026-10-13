package random

import "fmt"

const (
	legacyMask       uint64 = (1 << 48) - 1
	legacyMultiplier uint64 = 25214903917
	legacyIncrement  uint64 = 11
)

// Legacy is the 48-bit linear congruential random source.
type Legacy struct {
	seed     uint64
	gaussian gaussian
}

// NewLegacy creates a legacy source from a signed 64-bit seed.
func NewLegacy(seed int64) *Legacy {
	random := &Legacy{}
	random.SetSeed(seed)
	return random
}

// SetSeed resets the source and Gaussian cache.
func (r *Legacy) SetSeed(seed int64) {
	r.seed = (uint64(seed) ^ legacyMultiplier) & legacyMask
	r.gaussian.reset()
}

func (r *Legacy) next(bits uint) int32 {
	r.seed = (r.seed*legacyMultiplier + legacyIncrement) & legacyMask
	return int32(r.seed >> (48 - bits))
}

// NextInt returns the next signed 32-bit value.
func (r *Legacy) NextInt() int32 { return r.next(32) }

// NextIntBound returns a value in [0, bound).
func (r *Legacy) NextIntBound(bound int32) int32 {
	if bound <= 0 {
		panic(fmt.Sprintf("random bound must be positive: %d", bound))
	}
	if bound&(bound-1) == 0 {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		bits := r.next(31)
		value := bits % bound
		if int32(uint32(bits-value)+uint32(bound-1)) >= 0 {
			return value
		}
	}
}

// NextLong returns the next signed 64-bit value.
func (r *Legacy) NextLong() int64 {
	high, low := int64(r.next(32)), int64(r.next(32))
	return (high << 32) + low
}

// NextBoolean returns one random bit.
func (r *Legacy) NextBoolean() bool { return r.next(1) != 0 }

// NextFloat returns a value in [0, 1).
func (r *Legacy) NextFloat() float32 { return float32(r.next(24)) * 5.9604645e-8 }

// NextDouble returns a value in [0, 1).
func (r *Legacy) NextDouble() float64 {
	value := (int64(r.next(26)) << 27) + int64(r.next(27))
	return float64(value) * 1.1102230246251565e-16
}

// NextGaussian returns a normally distributed value with cached pair generation.
func (r *Legacy) NextGaussian() float64 { return r.gaussian.sample(r) }

// Fork creates a child source using the next long as its seed.
func (r *Legacy) Fork() Source { return NewLegacy(r.NextLong()) }

// ForkPositional creates an order-independent positional factory.
func (r *Legacy) ForkPositional() PositionalFactory { return legacyPositional{seed: r.NextLong()} }

type legacyPositional struct{ seed int64 }

func (f legacyPositional) At(x, y, z int32) Source { return NewLegacy(PositionSeed(x, y, z) ^ f.seed) }
func (f legacyPositional) FromHashOf(name string) Source {
	return NewLegacy(int64(javaStringHash(name)) ^ f.seed)
}
func (f legacyPositional) FromSeed(seed int64) Source { return NewLegacy(seed) }

func javaStringHash(value string) int32 {
	var hash int32
	for _, codePoint := range []uint16(stringToUTF16(value)) {
		hash = hash*31 + int32(codePoint)
	}
	return hash
}
