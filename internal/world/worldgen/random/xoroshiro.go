package random

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"math/bits"
)

const (
	goldenRatio uint64 = 0x9E3779B97F4A7C15
	silverRatio uint64 = 0x6A09E667F3BCC909
)

// Xoroshiro is the xoroshiro128++ random source used by modern worldgen.
type Xoroshiro struct {
	lo, hi   uint64
	gaussian gaussian
}

// NewXoroshiro creates a source using the mixed 128-bit expansion of seed.
func NewXoroshiro(seed int64) *Xoroshiro {
	lo, hi := upgradeSeed(seed)
	return newXoroshiroRaw(lo, hi)
}

func newXoroshiroRaw(lo, hi uint64) *Xoroshiro {
	if lo|hi == 0 {
		lo, hi = goldenRatio, silverRatio
	}
	return &Xoroshiro{lo: lo, hi: hi}
}

func mixStafford13(value uint64) uint64 {
	value = (value ^ value>>30) * uint64(0xBF58476D1CE4E5B9)
	value = (value ^ value>>27) * uint64(0x94D049BB133111EB)
	return value ^ value>>31
}

func upgradeSeed(seed int64) (uint64, uint64) {
	lo := uint64(seed) ^ silverRatio
	hi := lo + goldenRatio
	return mixStafford13(lo), mixStafford13(hi)
}

func (r *Xoroshiro) nextUint64() uint64 {
	lo, hi := r.lo, r.hi
	result := bits.RotateLeft64(lo+hi, 17) + lo
	hi ^= lo
	r.lo = bits.RotateLeft64(lo, 49) ^ hi ^ hi<<21
	r.hi = bits.RotateLeft64(hi, 28)
	return result
}

// SetSeed resets the source and Gaussian cache.
func (r *Xoroshiro) SetSeed(seed int64) { r.lo, r.hi = upgradeSeed(seed); r.gaussian.reset() }

// NextInt returns the low 32 bits as a signed value.
func (r *Xoroshiro) NextInt() int32 { return int32(r.nextUint64()) }

// NextIntBound returns an unbiased value in [0, bound).
func (r *Xoroshiro) NextIntBound(bound int32) int32 {
	if bound <= 0 {
		panic(fmt.Sprintf("random bound must be positive: %d", bound))
	}
	unsignedBound := uint64(uint32(bound))
	value := uint64(uint32(r.NextInt()))
	product := value * unsignedBound
	low := product & 0xffffffff
	if low < unsignedBound {
		threshold := uint64(uint32(-bound)) % unsignedBound
		for low < threshold {
			value = uint64(uint32(r.NextInt()))
			product = value * unsignedBound
			low = product & 0xffffffff
		}
	}
	return int32(product >> 32)
}

// NextLong returns the next signed 64-bit value.
func (r *Xoroshiro) NextLong() int64 { return int64(r.nextUint64()) }

// NextBoolean returns the low random bit.
func (r *Xoroshiro) NextBoolean() bool { return r.nextUint64()&1 != 0 }

// NextFloat returns a value in [0, 1).
func (r *Xoroshiro) NextFloat() float32 { return float32(r.nextUint64()>>40) * 5.9604645e-8 }

// NextDouble returns a value in [0, 1).
func (r *Xoroshiro) NextDouble() float64 { return float64(r.nextUint64()>>11) * 1.1102230246251565e-16 }

// NextGaussian returns a normally distributed value.
func (r *Xoroshiro) NextGaussian() float64 { return r.gaussian.sample(r) }

// Fork creates a child from two generated seed words.
func (r *Xoroshiro) Fork() Source { return newXoroshiroRaw(r.nextUint64(), r.nextUint64()) }

// ForkPositional creates an order-independent positional factory.
func (r *Xoroshiro) ForkPositional() PositionalFactory {
	return xoroshiroPositional{lo: r.nextUint64(), hi: r.nextUint64()}
}

type xoroshiroPositional struct{ lo, hi uint64 }

func (f xoroshiroPositional) At(x, y, z int32) Source {
	return newXoroshiroRaw(uint64(PositionSeed(x, y, z))^f.lo, f.hi)
}
func (f xoroshiroPositional) FromSeed(seed int64) Source {
	return newXoroshiroRaw(uint64(seed)^f.lo, uint64(seed)^f.hi)
}
func (f xoroshiroPositional) FromHashOf(name string) Source {
	digest := md5.Sum([]byte(name))
	lo := binary.BigEndian.Uint64(digest[:8])
	hi := binary.BigEndian.Uint64(digest[8:])
	return newXoroshiroRaw(lo^f.lo, hi^f.hi)
}
