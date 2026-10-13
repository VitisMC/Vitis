package density

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/world/worldgen/noise"
)

func TestCompilerArithmeticAndSpline(t *testing.T) {
	t.Parallel()

	definitions := map[string]json.RawMessage{
		"minecraft:coordinate": json.RawMessage(`{"type":"minecraft:y_clamped_gradient","from_y":-2,"to_y":2,"from_value":-2,"to_value":2}`),
		"minecraft:spline": json.RawMessage(`{
			"type":"minecraft:spline",
			"spline":{"coordinate":"minecraft:coordinate","points":[
				{"location":-1,"value":2,"derivative":1},
				{"location":1,"value":4,"derivative":0}
			]}
		}`),
	}
	compiler := testCompiler(t, definitions)
	function, err := compiler.Compile("minecraft:spline")
	if err != nil {
		t.Fatal(err)
	}
	if actual := function.Sample(Position{Y: 0}); math.Abs(actual-3.25) > 1e-7 {
		t.Fatalf("spline sample = %.17g, want 3.25", actual)
	}
	if actual := function.Sample(Position{Y: -10}); math.Abs(actual-1) > 1e-7 {
		t.Fatalf("extended spline sample = %.17g, want 1", actual)
	}
}

func TestCompilerInterpolationAndNegativeCells(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(t, map[string]json.RawMessage{"minecraft:unused": json.RawMessage(`0`)})
	function, err := compiler.CompileInline(json.RawMessage(`{
		"type":"minecraft:interpolated",
		"argument":{"type":"minecraft:square","argument":{"type":"minecraft:y_clamped_gradient","from_y":-8,"to_y":8,"from_value":0,"to_value":1}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if actual := function.Sample(Position{Y: -4}); math.Abs(actual-0.125) > 1e-15 {
		t.Fatalf("negative-cell interpolation = %.17g, want 0.125", actual)
	}
	if actual := function.Sample(Position{Y: 4}); math.Abs(actual-0.625) > 1e-15 {
		t.Fatalf("positive-cell interpolation = %.17g, want 0.625", actual)
	}
}

func TestEvaluationCachesNegativeCoordinatesWithoutCollisions(t *testing.T) {
	evaluation := NewEvaluation()
	calls := 0
	function := functionFunc(func(position Position) float64 {
		calls++
		return float64(position.X + position.Y + position.Z)
	})
	positions := []Position{
		{X: -1, Y: -64, Z: -1, Evaluation: evaluation},
		{X: 1, Y: -64, Z: 1, Evaluation: evaluation},
		{X: -30_000_000, Y: 319, Z: 30_000_000, Evaluation: evaluation},
	}
	for _, position := range positions {
		first := evaluation.sample(0, position, function)
		second := evaluation.sample(0, position, function)
		if first != second {
			t.Fatalf("cached value at %+v changed from %v to %v", position, first, second)
		}
	}
	if calls != len(positions) {
		t.Fatalf("function calls = %d, want %d", calls, len(positions))
	}
}

func TestChunkEvaluationCachesInterpolationGrid(t *testing.T) {
	calls := 0
	input := functionFunc(func(position Position) float64 {
		calls++
		return float64(position.X*3 + position.Y*5 - position.Z*7)
	})
	function := &interpolated{id: 0, function: input, width: 4, height: 8}
	evaluation := NewChunkEvaluation(-16, -64, -16, 384)
	for x := -16; x < 0; x++ {
		for z := -16; z < 0; z++ {
			for y := -64; y < 320; y++ {
				position := Position{X: x, Y: y, Z: z, Evaluation: evaluation}
				if got, want := function.Sample(position), float64(x*3+y*5-z*7); got != want {
					t.Fatalf("sample (%d,%d,%d) = %v, want %v", x, y, z, got, want)
				}
			}
		}
	}
	if calls != 5*49*5 {
		t.Fatalf("corner samples = %d, want %d", calls, 5*49*5)
	}
}

func TestChunkEvaluationResetClearsCachedSamples(t *testing.T) {
	input := functionFunc(func(position Position) float64 {
		return float64(position.X*3 + position.Y*5 - position.Z*7)
	})
	function := &interpolated{id: 0, function: input, width: 4, height: 8}
	evaluation := NewChunkEvaluation(0, -64, 0, 384)
	first := Position{X: 1, Y: 0, Z: 1, Evaluation: evaluation}
	if got, want := function.Sample(first), float64(1*3-1*7); got != want {
		t.Fatalf("first sample = %v, want %v", got, want)
	}
	evaluation.ResetChunk(16, -64, 16, 384)
	second := Position{X: 17, Y: 0, Z: 17, Evaluation: evaluation}
	if got, want := function.Sample(second), float64(17*3-17*7); got != want {
		t.Fatalf("reused sample = %v, want %v", got, want)
	}
}

func TestHorizontalMarkerCacheAndReset(t *testing.T) {
	calls := 0
	input := functionFunc(func(position Position) float64 {
		calls++
		return float64(position.X*31 + position.Z)
	})
	marker := &Marker{ID: 2, Kind: "cache_2d", Function: input}
	evaluation := NewChunkEvaluation(0, -64, 0, 384)
	for _, point := range [][2]int{{0, 0}, {16, 16}, {-1, 0}} {
		position := Position{X: point[0], Z: point[1], Evaluation: evaluation}
		if first, second := marker.Sample(position), marker.Sample(position); first != second {
			t.Fatalf("sample at %v changed from %v to %v", point, first, second)
		}
	}
	if calls != 3 {
		t.Fatalf("calls before reset = %d, want 3", calls)
	}
	evaluation.ResetChunk(16, -64, 16, 384)
	position := Position{X: 16, Z: 16, Evaluation: evaluation}
	if got := marker.Sample(position); got != float64(16*31+16) {
		t.Fatalf("sample after reset = %v", got)
	}
	if calls != 4 {
		t.Fatalf("calls after reset = %d, want 4", calls)
	}
}

func TestInterpolatedChunkEvaluationPreservesSamples(t *testing.T) {
	input := functionFunc(func(position Position) float64 {
		return math.Sin(float64(position.X)*0.17 + float64(position.Y)*0.11 - float64(position.Z)*0.23)
	})
	function := &interpolated{id: 0, function: input, width: 4, height: 8, powerOfTwo: true, widthMask: 3, heightMask: 7, widthInverse: 0.25, heightInverse: 0.125}
	evaluation := NewChunkEvaluation(0, -64, 0, 384)
	for x := 0; x < 8; x++ {
		for z := 0; z < 8; z++ {
			for y := -16; y < 16; y++ {
				position := Position{X: x, Y: y, Z: z}
				want := function.Sample(position)
				position.Evaluation = evaluation
				if got := function.Sample(position); math.Float64bits(got) != math.Float64bits(want) {
					t.Fatalf("sample (%d,%d,%d) = %.17g, want %.17g", x, y, z, got, want)
				}
			}
		}
	}
}

// TestInterpolatedPartialsResetAndBounds verifies cached interpolation across columns and chunk reuse.
func TestInterpolatedPartialsResetAndBounds(t *testing.T) {
	input := functionFunc(func(position Position) float64 {
		return math.Sin(float64(position.X)*0.17 + float64(position.Y)*0.11 - float64(position.Z)*0.23)
	})
	function := &interpolated{id: 0, function: input, width: 4, height: 8, powerOfTwo: true, widthMask: 3, heightMask: 7, widthInverse: 0.25, heightInverse: 0.125}
	evaluation := NewChunkEvaluation(0, -64, 0, 384)
	check := func(x, y, z int) {
		t.Helper()
		position := Position{X: x, Y: y, Z: z}
		want := function.Sample(position)
		position.Evaluation = evaluation
		if got := function.Sample(position); math.Float64bits(got) != math.Float64bits(want) {
			t.Fatalf("sample (%d,%d,%d) = %.17g, want %.17g", x, y, z, got, want)
		}
	}
	for z := 0; z < 8; z++ {
		for y := -64; y < -48; y++ {
			check(1, y, z)
		}
	}
	check(-1, -64, 1)
	evaluation.ResetChunk(16, -16, 16, 32)
	for z := 16; z < 24; z++ {
		for y := -16; y < 16; y++ {
			check(17, y, z)
		}
	}
}

// TestFixedPlaneNoiseCompilationPreservesSamples checks compiled Y=0 noise against the general sampler.
func TestFixedPlaneNoiseCompilationPreservesSamples(t *testing.T) {
	compiler := testCompiler(t, map[string]json.RawMessage{"minecraft:unused": json.RawMessage(`0`)})
	tests := []struct {
		definition string
		expected   func(*noise.Normal, Position) float64
	}{
		{`{"type":"minecraft:noise","noise":"minecraft:test","xz_scale":0.25,"y_scale":0}`,
			func(n *noise.Normal, p Position) float64 { return n.Sample(float64(p.X)*0.25, 0, float64(p.Z)*0.25) }},
		{`{"type":"minecraft:shift_a","argument":"minecraft:test"}`,
			func(n *noise.Normal, p Position) float64 {
				return n.Sample(float64(p.X)*0.25, 0, float64(p.Z)*0.25) * 4
			}},
		{`{"type":"minecraft:shift_b","argument":"minecraft:test"}`,
			func(n *noise.Normal, p Position) float64 {
				return n.Sample(float64(p.Z)*0.25, float64(p.X)*0.25, 0) * 4
			}},
		{`{"type":"minecraft:shifted_noise","noise":"minecraft:test","shift_x":1.25,"shift_y":0,"shift_z":-2.5,"xz_scale":0.25,"y_scale":0}`,
			func(n *noise.Normal, p Position) float64 {
				return n.Sample(float64(p.X)*0.25+1.25, 0, float64(p.Z)*0.25-2.5)
			}},
	}
	for _, test := range tests {
		function, err := compiler.CompileInline(json.RawMessage(test.definition))
		if err != nil {
			t.Fatal(err)
		}
		sampler, err := compiler.resolveNoise("minecraft:test")
		if err != nil {
			t.Fatal(err)
		}
		for _, position := range []Position{{X: 0, Y: 0, Z: 0}, {X: -123, Y: -64, Z: 45}, {X: 123456, Y: 319, Z: -456789}} {
			want, got := test.expected(sampler, position), function.Sample(position)
			if math.Float64bits(got) != math.Float64bits(want) {
				t.Fatalf("%s at %+v = %.17g, want %.17g", test.definition, position, got, want)
			}
		}
	}
}

func TestCompilerRejectsCyclesAndUnknownReferences(t *testing.T) {
	t.Parallel()

	compiler := testCompiler(t, map[string]json.RawMessage{
		"minecraft:a": json.RawMessage(`"minecraft:b"`),
		"minecraft:b": json.RawMessage(`"minecraft:a"`),
	})
	if _, err := compiler.Compile("minecraft:a"); err == nil {
		t.Fatal("expected density reference cycle error")
	}
	if _, err := compiler.Compile("minecraft:missing"); err == nil {
		t.Fatal("expected unknown density reference error")
	}
}

func TestCompilerCopiesRegistryData(t *testing.T) {
	t.Parallel()

	definition := json.RawMessage(`1`)
	definitions := map[string]json.RawMessage{"minecraft:value": definition}
	parameters := noise.Parameters{FirstOctave: -1, Amplitudes: []float64{1}}
	registry := Registry{
		Definitions: definitions,
		Noises:      map[string]noise.Parameters{"minecraft:test": parameters},
		CellWidth:   4, CellHeight: 8,
	}
	compiler, err := NewCompiler(registry)
	if err != nil {
		t.Fatal(err)
	}
	definition[0] = '9'
	parameters.Amplitudes[0] = 9
	function, err := compiler.Compile("minecraft:value")
	if err != nil {
		t.Fatal(err)
	}
	if actual := function.Sample(Position{}); actual != 1 {
		t.Fatalf("mutated registry changed compiled value to %v", actual)
	}
}

func testCompiler(t *testing.T, definitions map[string]json.RawMessage) *Compiler {
	t.Helper()
	compiler, err := NewCompiler(Registry{
		Definitions: definitions,
		Noises:      map[string]noise.Parameters{"minecraft:test": {FirstOctave: -1, Amplitudes: []float64{1}}},
		CellWidth:   4,
		CellHeight:  8,
	})
	if err != nil {
		t.Fatal(err)
	}
	return compiler
}
