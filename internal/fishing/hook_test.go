package fishing

import (
	"math/rand"
	"testing"

	"github.com/vitismc/vitis/internal/loot"
)

type testRoller struct{ calls int }

// Roll returns deterministic fishing loot for state-machine tests.
func (r *testRoller) Roll(tableID string, context *loot.Context) []loot.Drop {
	r.calls++
	if tableID != "minecraft:gameplay/fishing" || context == nil {
		return nil
	}
	return []loot.Drop{{ItemID: "minecraft:cod", Count: 1}}
}

// TestHookBiteAndRetrieve verifies timing, loot, XP, and terminal state.
func TestHookBiteAndRetrieve(t *testing.T) {
	hook, err := NewHook(rand.New(rand.NewSource(4)), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	environment := Environment{InWater: true, Raining: true, SeesSky: true, OpenWater: true}
	for ticks := 0; hook.State() != StateBiting && ticks < 1000; ticks++ {
		hook.Tick(environment)
	}
	if hook.State() != StateBiting {
		t.Fatal("hook never entered bite window")
	}
	roller := &testRoller{}
	result := hook.Retrieve(roller)
	if roller.calls != 1 || len(result.Drops) != 1 || result.Experience < 1 || result.Experience > 6 || result.RodDamage != 1 || hook.State() != StateExpired {
		t.Fatalf("catch = %+v, calls=%d state=%d", result, roller.calls, hook.State())
	}
}

// TestHookMissedBiteReturnsToWaiting verifies the finite bite window.
func TestHookMissedBiteReturnsToWaiting(t *testing.T) {
	hook, err := NewHook(rand.New(rand.NewSource(7)), 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	environment := Environment{InWater: true, SeesSky: true, OpenWater: true}
	seenBite := false
	seenEnd := false
	for range 1000 {
		event := hook.Tick(environment)
		seenBite = seenBite || event.Bite
		if event.BiteEnded {
			seenEnd = true
			break
		}
	}
	if !seenBite || !seenEnd || hook.State() != StateWaiting {
		t.Fatalf("bite=%v end=%v state=%d", seenBite, seenEnd, hook.State())
	}
}
