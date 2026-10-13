package chunk

import (
	"errors"
	"testing"
)

// TestLoaderDrainReleasesReferences clears vacated slots without altering pending work.
func TestLoaderDrainReleasesReferences(t *testing.T) {
	loader := &Loader{results: NewLoadResultQueue(1), completionOverflow: make([]LoadResult, 5, 8)}
	loader.results.Enqueue(LoadResult{X: -1, Chunk: New(-1, 0)})
	for i := range loader.completionOverflow {
		loader.completionOverflow[i] = LoadResult{X: int32(i), Chunk: New(int32(i), 0), Err: errors.New("failed write")}
	}
	for _, count := range []int{3, 1, 8} {
		results := loader.DrainCompletions(nil, count)
		if len(results) == 0 || results[0].Chunk == nil {
			t.Fatal("drain cleared returned results")
		}
		for i, result := range loader.completionOverflow {
			if result.Chunk == nil || result.Err == nil || i > 0 && result.X != loader.completionOverflow[i-1].X+1 {
				t.Fatal("drain changed pending overflow order")
			}
		}
		for _, result := range loader.completionOverflow[len(loader.completionOverflow):cap(loader.completionOverflow)] {
			if result.Chunk != nil || result.Err != nil || result.X != 0 || result.Z != 0 {
				t.Fatal("vacated overflow retained a chunk or error")
			}
		}
	}
}

// TestManagerCompletionsReleaseReferences covers applied, ignored and failed results.
func TestManagerCompletionsReleaseReferences(t *testing.T) {
	loader := &Loader{results: NewLoadResultQueue(4)}
	manager := &Manager{loader: loader, storage: NewStorage(4), completionScratch: make([]LoadResult, 0, 1), completionApplyBatch: 4}
	for x := int32(0); x < 4; x++ {
		current := New(x, 0)
		current.SetState(StateLoading)
		if x == 1 {
			current.SetState(StateLoaded)
		}
		if x != 2 {
			manager.storage.Set(current)
		}
		result := LoadResult{X: x, Chunk: New(x, 0)}
		if x == 3 {
			result.Err = errors.New("generation failed")
		}
		loader.results.Enqueue(result)
	}
	if got := manager.ApplyLoadCompletions(4); got != 1 {
		t.Fatalf("applied %d completions, want 1", got)
	}
	if loaded, ok := manager.storage.Get(0, 0); !ok || loaded.State() != StateLoaded {
		t.Fatal("clearing scratch discarded the applied chunk")
	}
	if len(manager.completionScratch) != 0 || cap(manager.completionScratch) < 4 {
		t.Fatal("grown completion scratch was not reused")
	}
	for _, result := range manager.completionScratch[:cap(manager.completionScratch)] {
		if result.Chunk != nil || result.Err != nil || result.X != 0 || result.Z != 0 {
			t.Fatal("completion scratch retained a processed result")
		}
	}
}
