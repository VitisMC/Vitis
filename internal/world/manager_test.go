package world

import (
	"fmt"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"sync"
	"testing"
)

// TestManagerSharesEntityIDsAcrossDimensions prevents collisions after a player transfer.
func TestManagerSharesEntityIDsAcrossDimensions(t *testing.T) {
	m := NewManager()
	first, err := m.CreateWithConfig(Config{Name: "overworld", Dimension: "minecraft:overworld"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.CreateWithConfig(Config{Name: "nether", Dimension: "minecraft:the_nether"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	one, two := first.entities.AllocateID(), second.entities.AllocateID()
	if one <= 0 || two <= one {
		t.Fatal("worlds reused protocol entity IDs")
	}
	if !second.entities.Add(entity.NewEntity(100, protocol.UUID{1, 2}, entity.EntityTypeMob, entity.Vec3{}, entity.Vec2{})) {
		t.Fatal("explicit entity registration failed")
	}
	if id := first.entities.AllocateID(); id <= 100 {
		t.Fatalf("allocated ID %d collides with registered state", id)
	}
}

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
