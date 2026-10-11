package worldgen

import "testing"

// TestSurfaceColumnConditionCachesOnlyOneColumn protects height-independent predicate reuse.
func TestSurfaceColumnConditionCachesOnlyOneColumn(t *testing.T) {
	system := &surfaceSystem{}
	calls := 0
	condition := system.cacheColumnCondition(conditionFunc(func(context *surfaceContext) bool { calls++; return context.x < 0 }))
	context := &surfaceContext{x: -1, columnConditions: make([]uint8, system.columnConditionCount)}
	for y := -64; y < 320; y++ {
		context.y = y
		if !condition.test(context) {
			t.Fatal("cached predicate changed with height")
		}
	}
	if calls != 1 {
		t.Fatalf("column predicate evaluated %d times", calls)
	}
	clear(context.columnConditions)
	context.x = 1
	for range 3 {
		if condition.test(context) {
			t.Fatal("predicate reused another column's value")
		}
	}
	if calls != 2 {
		t.Fatalf("second column predicate evaluated %d times", calls)
	}
	context.columnConditions = nil
	condition.test(context)
	if calls != 3 {
		t.Fatal("standalone surface context did not use generic evaluation")
	}
}
