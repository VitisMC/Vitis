package world

import (
	"fmt"
	"sync"
	"testing"
)

func TestManagerConcurrentCreateAndGet(t *testing.T) {
	manager := NewManager()

	const worlds = 32
	var wait sync.WaitGroup
	wait.Add(worlds)

	for index := 0; index < worlds; index++ {
		index := index
		go func() {
			defer wait.Done()
			name := fmt.Sprintf("world_%d", index)
			if _, err := manager.Create(name); err != nil {
				t.Errorf("create world %s failed: %v", name, err)
				return
			}
			if _, ok := manager.Get(name); !ok {
				t.Errorf("expected world %s to be retrievable", name)
			}
		}()
	}
	wait.Wait()

	if manager.Count() != worlds {
		t.Fatalf("expected %d worlds, got %d", worlds, manager.Count())
	}

	snapshot := manager.Worlds()
	if len(snapshot) != worlds {
		t.Fatalf("expected snapshot size %d, got %d", worlds, len(snapshot))
	}
}

func TestManagerWorldLifecycleOperations(t *testing.T) {
	manager := NewManager()
	world, err := manager.Create("alpha")
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	if world == nil {
		t.Fatalf("expected created world instance")
	}

	if _, err := manager.Create("alpha"); err == nil {
		t.Fatalf("expected duplicate name error")
	}

	if !manager.Remove("alpha") {
		t.Fatalf("expected remove to succeed")
	}
	if manager.Remove("alpha") {
		t.Fatalf("expected remove to fail for missing world")
	}
	if manager.Count() != 0 {
		t.Fatalf("expected manager to be empty")
	}
}
