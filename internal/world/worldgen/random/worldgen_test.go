package random

import "testing"

// TestWorldgenSeedVectors verifies stage-specific seed derivation.
func TestWorldgenSeedVectors(t *testing.T) {
	random := NewWorldgen(NewLegacy(0))
	decoration := random.SetDecorationSeed(123456789, 32, -48)
	if decoration != 5820568250569446373 {
		t.Fatalf("decoration seed = %d", decoration)
	}
	if got := random.NextInt(); got != 1152357561 {
		t.Fatalf("decoration int = %d", got)
	}
	if got := random.NextLong(); got != 5202357591059255139 {
		t.Fatalf("decoration long = %d", got)
	}
	random.SetLargeFeatureWithSalt(123456789, 7, -11, 10387320)
	if got := random.NextInt(); got != -711677854 {
		t.Fatalf("salted structure int = %d", got)
	}
	random.SetFeatureSeed(123456789, 17, 6)
	if got := random.NextInt(); got != 1933469776 {
		t.Fatalf("feature int = %d", got)
	}
	if got := SlimeChunkSource(5, -9, 123456789, 987234911).NextIntBound(10); got != 4 {
		t.Fatalf("slime roll = %d", got)
	}
}
