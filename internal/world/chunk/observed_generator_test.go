package chunk

import (
	"errors"
	"testing"
	"time"
)

type failingObservedGenerator struct{}

type boundedObservedGenerator struct{}

func (boundedObservedGenerator) Generate(x, z int32) (*Chunk, error) { return New(x, z), nil }
func (boundedObservedGenerator) Bounds() (int, int, error)           { return -64, 384, nil }

func (failingObservedGenerator) Generate(int32, int32) (*Chunk, error) {
	return nil, errors.New("generation failed")
}

// TestObservedGeneratorRecordsOnlyCompletedChunks verifies that failures do not inflate throughput.
func TestObservedGeneratorRecordsOnlyCompletedChunks(t *testing.T) {
	count := 0
	observed := ObservedGenerator{Generator: testGenerator{}, Record: func(elapsed time.Duration) {
		if elapsed < 0 {
			t.Fatal("negative generation duration")
		}
		count++
	}}
	if _, err := observed.Generate(1, 2); err != nil || count != 1 {
		t.Fatalf("successful generation was not recorded: count=%d err=%v", count, err)
	}
	observed.Generator = failingObservedGenerator{}
	if _, err := observed.Generate(1, 2); err == nil || count != 1 {
		t.Fatalf("failed generation affected count: count=%d err=%v", count, err)
	}
}

// TestObservedGeneratorPreservesBounds verifies persisted chunk decoding can read dimension height.
func TestObservedGeneratorPreservesBounds(t *testing.T) {
	observed := ObservedGenerator{Generator: boundedObservedGenerator{}}
	minY, height, err := observed.Bounds()
	if err != nil || minY != -64 || height != 384 {
		t.Fatalf("unexpected bounds: min=%d height=%d err=%v", minY, height, err)
	}
}

// TestFirstAcknowledgedDurationCountsGeneratedChunkOnce verifies duplicate client acknowledgements.
func TestFirstAcknowledgedDurationCountsGeneratedChunkOnce(t *testing.T) {
	generated := New(0, 0)
	if _, ok := generated.FirstAcknowledgedDuration(); ok {
		t.Fatal("unobserved chunk was counted")
	}
	generated.MarkGenerationStart(time.Now().Add(-time.Millisecond))
	if elapsed, ok := generated.FirstAcknowledgedDuration(); !ok || elapsed < time.Millisecond {
		t.Fatalf("first acknowledgement missing: elapsed=%s ok=%t", elapsed, ok)
	}
	if _, ok := generated.FirstAcknowledgedDuration(); ok {
		t.Fatal("duplicate acknowledgement was counted")
	}
}
