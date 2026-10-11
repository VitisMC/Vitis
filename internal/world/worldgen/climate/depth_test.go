package climate

import (
	"fmt"
	"math"
	"testing"
)

// TestDepthGroupsMatchOriginalTree verifies changing depths and original tie order.
func TestDepthGroupsMatchOriginalTree(t *testing.T) {
	for _, preset := range []string{"minecraft:overworld", "minecraft:nether"} {
		index, err := NewPreset(preset)
		if err != nil {
			t.Fatal(err)
		}
		var cursor Cursor
		for x := -10; x <= 10; x++ {
			for z := -10; z <= 10; z++ {
				for depth := -5; depth <= 15; depth++ {
					target := NewTarget(float64(x)/10, float64(z)/10, float64(x+z)/20, float64(x-z)/20, float64(depth)/10, float64(x*z)/100)
					var expanded [parameterCount]int64
					copy(expanded[:], target[:])
					original, _ := index.root.search(expanded, nil, math.MaxInt64)
					if got := index.FindWithCursor(target, &cursor); got != original.biome {
						t.Fatalf("%s target %v: %q != %q", preset, target, got, original.biome)
					}
				}
			}
		}
	}
}

// TestDepthGroupsFallback checks custom interval counts and exact equal distances.
func TestDepthGroupsFallback(t *testing.T) {
	for _, count := range []int{2, 9} {
		points := make([]Point, count)
		for n := range points {
			points[n].Biome = fmt.Sprintf("minecraft:test_%d", n)
			points[n].Parameters[4] = Parameter{Min: int64(n * 2), Max: int64(n * 2)}
		}
		index, err := NewIndex(points)
		if err != nil {
			t.Fatal(err)
		}
		var cursor Cursor
		for depth := int64(-2); depth < 20; depth++ {
			target := Target{4: depth}
			var expanded [parameterCount]int64
			copy(expanded[:], target[:])
			original, _ := index.root.search(expanded, nil, math.MaxInt64)
			if got := index.FindWithCursor(target, &cursor); got != original.biome {
				t.Fatalf("count %d depth %d: %q != %q", count, depth, got, original.biome)
			}
			cursor.Reset()
		}
	}
}
