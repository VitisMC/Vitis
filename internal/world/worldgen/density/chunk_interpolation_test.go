package density

import "testing"

// TestChunkInterpolationMatchesScalar verifies channel separation, outer operations and reset.
func TestChunkInterpolationMatchesScalar(t *testing.T) {
	var calls [2]int
	channels := []*interpolated{
		{id: 1, width: 4, height: 8, function: functionFunc(func(p Position) float64 {
			calls[0]++
			return float64(p.X*p.Y-p.Z*p.Z) / 43
		})},
		{id: 3, width: 4, height: 8, function: functionFunc(func(p Position) float64 {
			calls[1]++
			return float64(p.X*p.X+p.Y*p.Z) / 91
		})},
	}
	plan := &ChunkInterpolation{channels: channels, width: 4, height: 8, valueCount: 4}
	evaluation := NewChunkEvaluation(-16, -16, 16, 32)
	for _, origin := range [][3]int{{-16, -16, 16}, {16, 0, -16}} {
		evaluation.ResetChunk(origin[0], origin[1], origin[2], 32)
		visited := 0
		if !plan.Fill(evaluation, func(position Position) {
			visited++
			for _, channel := range channels {
				got := channel.Sample(position)
				scalar := position
				scalar.Evaluation = nil
				want := channel.Sample(scalar)
				if got != want {
					t.Fatalf("channel %d at (%d,%d,%d): got %.17g, want %.17g", channel.id, position.X, position.Y, position.Z, got, want)
				}
			}
			outside := position
			outside.X += 16
			outside.Evaluation = nil
			want := channels[0].Sample(outside)
			outside.Evaluation = evaluation
			if got := channels[0].Sample(outside); got != want {
				t.Fatal("out-of-chunk sample reused active interpolation")
			}
		}) {
			t.Fatal("aligned chunk did not use interpolation")
		}
		if visited != 16*16*32 || evaluation.interpolation.active {
			t.Fatalf("visited=%d active=%v", visited, evaluation.interpolation.active)
		}
	}
}

// TestChunkInterpolationSamplesCornersOnce protects the cell-level work bound.
func TestChunkInterpolationSamplesCornersOnce(t *testing.T) {
	calls := 0
	channel := &interpolated{id: 0, width: 4, height: 8, function: functionFunc(func(p Position) float64 {
		calls++
		return float64(p.X + p.Y + p.Z)
	})}
	plan := &ChunkInterpolation{channels: []*interpolated{channel}, width: 4, height: 8, valueCount: 1}
	evaluation := NewChunkEvaluation(0, -64, 0, 384)
	plan.Fill(evaluation, func(p Position) { channel.Sample(p); channel.Sample(p) })
	if calls != 5*5*49 {
		t.Fatalf("corner calls=%d, want %d", calls, 5*5*49)
	}
	evaluation.ResetChunk(0, -63, 0, 384)
	if plan.Fill(evaluation, func(Position) { t.Fatal("unaligned bounds entered fast path") }) {
		t.Fatal("unaligned bounds accepted")
	}
}
