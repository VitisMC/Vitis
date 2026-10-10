// Package fishing implements server-authoritative fishing hook timing and loot.
package fishing

import (
	"fmt"
	"math/rand"

	"github.com/vitismc/vitis/internal/loot"
)

// State is the current fishing hook phase.
type State uint8

const (
	StateWaiting State = iota
	StateApproaching
	StateBiting
	StateExpired
)

// Environment contains the world observations needed for one fishing tick.
type Environment struct {
	InWater   bool
	Raining   bool
	SeesSky   bool
	OpenWater bool
}

// Event reports a state transition that needs client effects.
type Event struct {
	Approaching bool
	Bite        bool
	BiteEnded   bool
}

// Catch is the result of retrieving a hook.
type Catch struct {
	Drops      []loot.Drop
	Experience int
	RodDamage  int
}

// Roller evaluates the active fishing loot table.
type Roller interface {
	Roll(tableID string, context *loot.Context) []loot.Drop
}

// Hook owns the timing state for one cast fishing hook.
type Hook struct {
	rng         *rand.Rand
	state       State
	waitTicks   int
	travelTicks int
	biteTicks   int
	outOfWater  int
	lureSpeed   int
	luck        int
	openWater   bool
}

// NewHook creates a hook with enchantment effects applied.
func NewHook(rng *rand.Rand, luckOfTheSeaLevel, lureLevel int) (*Hook, error) {
	if rng == nil {
		return nil, fmt.Errorf("create fishing hook: nil random source")
	}
	if luckOfTheSeaLevel < 0 || lureLevel < 0 {
		return nil, fmt.Errorf("create fishing hook: negative enchantment level")
	}
	return &Hook{rng: rng, state: StateWaiting, lureSpeed: lureLevel * 100, luck: luckOfTheSeaLevel, openWater: true}, nil
}

// State returns the current hook phase.
func (h *Hook) State() State {
	if h == nil {
		return StateExpired
	}
	return h.state
}

// Tick advances fishing timers by one server tick.
func (h *Hook) Tick(environment Environment) Event {
	if h == nil || h.state == StateExpired {
		return Event{}
	}
	if !environment.InWater {
		h.outOfWater++
		if h.outOfWater > 10 {
			h.openWater = false
		}
		return Event{}
	}
	if h.outOfWater > 0 {
		h.outOfWater--
	}
	if h.state != StateWaiting {
		h.openWater = h.openWater && h.outOfWater < 10 && environment.OpenWater
	}
	speed := 1
	if environment.Raining {
		speed++
	}
	if !environment.SeesSky && h.rng.Float64() < 0.5 {
		speed--
	}
	switch h.state {
	case StateWaiting:
		if h.waitTicks == 0 {
			h.waitTicks = 100 + h.rng.Intn(501) - h.lureSpeed
			if h.waitTicks < 1 {
				h.waitTicks = 1
			}
		}
		h.waitTicks -= speed
		if h.waitTicks <= 0 {
			h.travelTicks = 20 + h.rng.Intn(61)
			h.state = StateApproaching
			return Event{Approaching: true}
		}
	case StateApproaching:
		h.travelTicks -= speed
		if h.travelTicks <= 0 {
			h.biteTicks = 20 + h.rng.Intn(21)
			h.state = StateBiting
			return Event{Bite: true}
		}
	case StateBiting:
		h.biteTicks--
		if h.biteTicks <= 0 {
			h.waitTicks = 0
			h.travelTicks = 0
			h.state = StateWaiting
			return Event{BiteEnded: true}
		}
	}
	return Event{}
}

// Retrieve removes the hook and produces loot only during the bite window.
func (h *Hook) Retrieve(roller Roller) Catch {
	if h == nil || h.state == StateExpired {
		return Catch{}
	}
	result := Catch{}
	if h.state == StateBiting && roller != nil {
		context := loot.NewContext(h.rng).WithLuck(float64(h.luck)).WithOpenWater(h.openWater)
		result.Drops = roller.Roll("minecraft:gameplay/fishing", context)
		result.Experience = 1 + h.rng.Intn(6)
		result.RodDamage = 1
	}
	h.state = StateExpired
	return result
}
