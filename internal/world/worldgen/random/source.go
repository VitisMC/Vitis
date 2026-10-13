// Package random implements deterministic random sources used by world generation.
package random

// Source is a seedable deterministic world generation random source.
type Source interface {
	SetSeed(seed int64)
	NextInt() int32
	NextIntBound(bound int32) int32
	NextLong() int64
	NextBoolean() bool
	NextFloat() float32
	NextDouble() float64
	NextGaussian() float64
	Fork() Source
	ForkPositional() PositionalFactory
}

// PositionalFactory derives order-independent sources for positions and names.
type PositionalFactory interface {
	At(x, y, z int32) Source
	FromHashOf(name string) Source
	FromSeed(seed int64) Source
}

// PositionSeed returns the deterministic hash used for a block position.
func PositionSeed(x, y, z int32) int64 {
	value := int64(int32(x*3129871)) ^ int64(z)*116129781 ^ int64(y)
	value = value*value*42317861 + value*11
	return value >> 16
}
