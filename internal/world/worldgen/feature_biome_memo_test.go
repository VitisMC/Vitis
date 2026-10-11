package worldgen

import "testing"

type memoCountingBiomeSource struct{ calls int }

// sample returns a coordinate-derived biome and records cache misses.
func (s *memoCountingBiomeSource) sample(x, y, z int) int32 {
	s.calls++
	return int32(x*31 + y*17 + z*7)
}

// TestFeatureBiomeMemoPreservesSamplesAndReset checks collisions, negative coordinates, and reuse.
func TestFeatureBiomeMemoPreservesSamplesAndReset(t *testing.T) {
	memo := &featureBiomeMemo{}
	memo.reset()
	source := &memoCountingBiomeSource{}
	positions := []biomeQuartPosition{{x: -7499996, y: -16, z: 7499996}, {x: 0, y: 0, z: 0}}
	for x := -120; x < 120; x++ {
		positions = append(positions, biomeQuartPosition{x: x, y: x & 31, z: 3*x - 1})
	}
	for _, position := range positions {
		want := int32(position.x*31 + position.y*17 + position.z*7)
		if got := memo.sample(source, position); got != want {
			t.Fatalf("first sample at %+v = %d, want %d", position, got, want)
		}
	}
	if source.calls != len(positions) {
		t.Fatalf("first-pass calls = %d, want %d", source.calls, len(positions))
	}
	for _, position := range positions {
		memo.sample(source, position)
	}
	if source.calls != len(positions) {
		t.Fatalf("cached calls = %d, want %d", source.calls, len(positions))
	}
	memo.reset()
	memo.sample(source, positions[0])
	if source.calls != len(positions)+1 {
		t.Fatalf("reset calls = %d, want %d", source.calls, len(positions)+1)
	}
	memo.epoch = ^uint32(0)
	memo.reset()
	memo.sample(source, positions[0])
	if source.calls != len(positions)+2 {
		t.Fatalf("wraparound calls = %d, want %d", source.calls, len(positions)+2)
	}
	var uncached *featureBiomeMemo
	if got := uncached.sample(source, positions[0]); got != int32(positions[0].x*31+positions[0].y*17+positions[0].z*7) {
		t.Fatalf("uncached sample = %d", got)
	}
	memo.reset()
	for x := 0; x <= featureBiomeMemoSize; x++ {
		position := biomeQuartPosition{x: x, y: 4, z: -7}
		want := int32(position.x*31 + position.y*17 + position.z*7)
		if got := memo.sample(source, position); got != want {
			t.Fatalf("full-table sample at %d = %d, want %d", x, got, want)
		}
	}
}
