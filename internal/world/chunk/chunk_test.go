package chunk

import "testing"

func TestChunkKeyPackingUniqueness(t *testing.T) {
	testCases := []struct {
		x int32
		z int32
	}{
		{x: 0, z: 0},
		{x: 1, z: 0},
		{x: 0, z: 1},
		{x: -1, z: -1},
		{x: 12345, z: -54321},
	}

	seen := make(map[int64]struct{}, len(testCases))
	for _, testCase := range testCases {
		key := ChunkKey(testCase.x, testCase.z)
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate key for x=%d z=%d", testCase.x, testCase.z)
		}
		seen[key] = struct{}{}
	}
}

func TestChunkStateAndAccessors(t *testing.T) {
	loaded := New(2, 3)
	if loaded.X() != 2 || loaded.Z() != 3 {
		t.Fatalf("unexpected coordinates x=%d z=%d", loaded.X(), loaded.Z())
	}
	if loaded.State() != StateLoaded {
		t.Fatalf("expected loaded state, got %d", loaded.State())
	}

	loaded.Touch(42)
	if loaded.LastAccessTick() != 42 {
		t.Fatalf("unexpected last access tick %d", loaded.LastAccessTick())
	}

	loaded.SetState(StateUnloading)
	if loaded.State() != StateUnloading {
		t.Fatalf("expected unloading state, got %d", loaded.State())
	}

	loading := NewLoading(-7, 9)
	if loading.State() != StateLoading {
		t.Fatalf("expected loading state, got %d", loading.State())
	}
	if loading.Key() != ChunkKey(-7, 9) {
		t.Fatalf("unexpected key %d", loading.Key())
	}
}
