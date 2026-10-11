package random

import "testing"

// TestFirstPositionalDrawMatchesSource verifies allocation-free built-in draws and fallback.
func TestFirstPositionalDrawMatchesSource(t *testing.T) {
	for _, seed := range []int64{0, 42, -987654321} {
		for _, source := range []Source{NewLegacy(seed), NewXoroshiro(seed)} {
			factory := source.ForkPositional()
			for _, position := range [][3]int32{{}, {-1, -64, -17}, {30000000, 320, -30000000}} {
				x, y, z := position[0], position[1], position[2]
				for _, candidate := range []PositionalFactory{factory, wrappedFactory{factory}} {
					if got, want := FirstFloatAt(candidate, x, y, z), factory.At(x, y, z).NextFloat(); got != want {
						t.Fatalf("seed %d position %v float: %v != %v", seed, position, got, want)
					}
					if got, want := FirstDoubleAt(candidate, x, y, z), factory.At(x, y, z).NextDouble(); got != want {
						t.Fatalf("seed %d position %v double: %v != %v", seed, position, got, want)
					}
				}
			}
		}
	}
}

// TestPositionalScratchPreservesSequence checks source reset and independent ownership.
func TestPositionalScratchPreservesSequence(t *testing.T) {
	var scratch PositionalScratch
	for _, source := range []Source{NewLegacy(-42), NewXoroshiro(12345)} {
		factory := source.ForkPositional()
		for _, p := range [][3]int32{{0, 0, 0}, {-17, -64, 31}, {30000000, 320, -30000000}, {0, 0, 0}} {
			for _, candidate := range []PositionalFactory{factory, wrappedFactory{factory}} {
				got, want := scratch.At(candidate, p[0], p[1], p[2]), factory.At(p[0], p[1], p[2])
				for range 16 {
					if got.NextIntBound(17) != want.NextIntBound(17) || got.NextFloat() != want.NextFloat() || got.NextDouble() != want.NextDouble() || got.NextGaussian() != want.NextGaussian() {
						t.Fatalf("position %v: positional sequence differs", p)
					}
				}
			}
		}
	}
}

// wrappedFactory exercises the public factory fallback without a built-in concrete type.
type wrappedFactory struct{ PositionalFactory }
