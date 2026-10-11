package random

import (
	"math"
	"testing"
)

// TestLegacyVectors verifies exact java.util.Random compatible output.
func TestLegacyVectors(t *testing.T) {
	random := NewLegacy(0)
	if got := random.NextInt(); got != -1155484576 {
		t.Fatalf("next int = %d", got)
	}
	if got := random.NextLong(); got != -3109364765729502342 {
		t.Fatalf("next long = %d", got)
	}
	if got := random.NextIntBound(100); got != 47 {
		t.Fatalf("bounded int = %d", got)
	}
	if got := random.NextDouble(); got != 0.6374174253501083 {
		t.Fatalf("double = %.17g", got)
	}
	if got := random.NextGaussian(); math.Abs(got-1.1309640497243958) > 1e-15 {
		t.Fatalf("gaussian = %.17g", got)
	}
}

// TestXoroshiroVectors verifies exact xoroshiro128++ seed expansion and output.
func TestXoroshiroVectors(t *testing.T) {
	random := NewXoroshiro(0)
	for index, want := range []int64{3038984756725240190, -3694039286755638414, 4633751808701151732} {
		if got := random.NextLong(); got != want {
			t.Fatalf("long %d = %d, want %d", index, got, want)
		}
	}
	random = NewXoroshiro(0)
	if got := random.NextIntBound(100); got != 96 {
		t.Fatalf("bounded 100 = %d", got)
	}
	if got := random.NextIntBound(7); got != 1 {
		t.Fatalf("bounded 7 = %d", got)
	}
}

// TestPositionSeedVerifiesSignedOverflow checks positional seed arithmetic.
func TestPositionSeedVerifiesSignedOverflow(t *testing.T) {
	if got := PositionSeed(1, 2, -3); got != -32767471662471 {
		t.Fatalf("position seed = %d", got)
	}
}

// TestPositionalFactoriesAreOrderIndependent verifies stable derived streams.
func TestPositionalFactoriesAreOrderIndependent(t *testing.T) {
	factory := NewXoroshiro(42).ForkPositional()
	first := factory.At(3, -2, 9).NextLong()
	_ = factory.At(100, 20, -5).NextLong()
	second := factory.At(3, -2, 9).NextLong()
	if first != second {
		t.Fatalf("positional stream changed: %d != %d", first, second)
	}
}
