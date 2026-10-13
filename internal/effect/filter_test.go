package effect

import (
	"sync"
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
)

// TestEffectFilterConcurrentApplication keeps immutable species policy ahead of mutation.
func TestEffectFilterConcurrentApplication(t *testing.T) {
	manager := NewManagerWithFilter(func(instance Instance) bool { return instance.ID != effectdata.EffectPoison })
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if manager.Add(Instance{ID: effectdata.EffectPoison, Duration: 100}) {
				t.Error("filter accepted poison")
			}
			manager.Add(Instance{ID: effectdata.EffectSpeed, Duration: 100})
			manager.Active()
		}()
	}
	workers.Wait()
	if manager.Has(effectdata.EffectPoison) || !manager.Has(effectdata.EffectSpeed) {
		t.Fatal("filter did not preserve accepted effects")
	}
}
