package weather

import (
	"math/rand"
	"sync"
)

const (
	rainDelayMin     = 12000
	rainDelayMax     = 180000
	rainDurationMin  = 12000
	rainDurationMax  = 24000
	thunderDelayMin  = 12000
	thunderDelayMax  = 180000
	thunderDurationMin = 3600
	thunderDurationMax = 15600

	transitionSpeed = 0.01
)

const (
	GameEventBeginRain       byte = 1
	GameEventEndRain         byte = 2
	GameEventRainLevelChange byte = 7
	GameEventThunderLevel    byte = 8
)

// State represents the current weather type.
type State int

const (
	Clear   State = iota
	Rain
	Thunder
)

// String returns the weather state name.
func (s State) String() string {
	switch s {
	case Rain:
		return "rain"
	case Thunder:
		return "thunder"
	default:
		return "clear"
	}
}

// ParseState converts a string to a weather State.
func ParseState(s string) State {
	switch s {
	case "rain":
		return Rain
	case "thunder":
		return Thunder
	default:
		return Clear
	}
}

// Packet represents a weather-related GameEvent to broadcast.
type Packet struct {
	Event byte
	Value float32
}

// Weather manages the world weather simulation.
type Weather struct {
	mu sync.RWMutex

	clearWeatherTime int32
	raining          bool
	rainTime         int32
	thundering       bool
	thunderTime      int32

	rainLevel      float32
	oldRainLevel   float32
	thunderLevel   float32
	oldThunderLevel float32

	cycleEnabled bool
	rng          *rand.Rand
}

// New creates a weather system with natural cycling enabled.
func New(seed int64) *Weather {
	return &Weather{
		cycleEnabled: true,
		rng:          rand.New(rand.NewSource(seed)),
	}
}

// Tick advances the weather simulation by one tick and returns packets to broadcast.
func (w *Weather) Tick() []Packet {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cycleEnabled {
		w.advanceCycle()
	}

	w.oldRainLevel = w.rainLevel
	w.oldThunderLevel = w.thunderLevel

	if w.raining {
		w.rainLevel = min32(w.rainLevel+transitionSpeed, 1.0)
	} else {
		w.rainLevel = max32(w.rainLevel-transitionSpeed, 0.0)
	}

	if w.thundering {
		w.thunderLevel = min32(w.thunderLevel+transitionSpeed, 1.0)
	} else {
		w.thunderLevel = max32(w.thunderLevel-transitionSpeed, 0.0)
	}

	var packets []Packet

	if abs32(w.oldRainLevel-w.rainLevel) > 1e-6 {
		packets = append(packets, Packet{
			Event: GameEventRainLevelChange,
			Value: w.rainLevel,
		})
	}

	if abs32(w.oldThunderLevel-w.thunderLevel) > 1e-6 {
		packets = append(packets, Packet{
			Event: GameEventThunderLevel,
			Value: w.thunderLevel,
		})
	}

	return packets
}

// SetWeather forces a weather change with the given duration in ticks.
func (w *Weather) SetWeather(state State, duration int32) []Packet {
	w.mu.Lock()
	defer w.mu.Unlock()

	wasRaining := w.raining

	switch state {
	case Clear:
		w.clearWeatherTime = duration
		w.rainTime = 0
		w.thunderTime = 0
		w.raining = false
		w.thundering = false
	case Rain:
		w.clearWeatherTime = 0
		w.rainTime = duration
		w.thunderTime = 0
		w.raining = true
		w.thundering = false
	case Thunder:
		w.clearWeatherTime = 0
		w.rainTime = duration
		w.thunderTime = duration
		w.raining = true
		w.thundering = true
	}

	var packets []Packet
	if wasRaining != w.raining {
		if w.raining {
			packets = append(packets, Packet{Event: GameEventBeginRain, Value: 0})
		} else {
			packets = append(packets, Packet{Event: GameEventEndRain, Value: 0})
		}
	}
	return packets
}

// CurrentState returns the current weather state.
func (w *Weather) CurrentState() State {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.thundering && w.raining {
		return Thunder
	}
	if w.raining {
		return Rain
	}
	return Clear
}

// IsRaining returns true if it is currently raining.
func (w *Weather) IsRaining() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.raining
}

// IsThundering returns true if it is currently thundering.
func (w *Weather) IsThundering() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.thundering
}

// RainLevel returns the current rain intensity (0.0–1.0).
func (w *Weather) RainLevel() float32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.rainLevel
}

// ThunderLevel returns the current thunder intensity (0.0–1.0).
func (w *Weather) ThunderLevel() float32 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.thunderLevel
}

// JoinPackets returns the GameEvent packets a player needs on join to sync weather state.
func (w *Weather) JoinPackets() []Packet {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var packets []Packet
	if w.raining {
		packets = append(packets, Packet{Event: GameEventBeginRain, Value: 0})
		packets = append(packets, Packet{Event: GameEventRainLevelChange, Value: w.rainLevel})
	}
	if w.thundering {
		packets = append(packets, Packet{Event: GameEventThunderLevel, Value: w.thunderLevel})
	}
	return packets
}

// SetCycleEnabled enables or disables the natural weather cycle.
func (w *Weather) SetCycleEnabled(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cycleEnabled = enabled
}

func (w *Weather) advanceCycle() {
	if w.clearWeatherTime > 0 {
		w.clearWeatherTime--
		w.thunderTime = boolToInt32(!w.thundering)
		w.rainTime = boolToInt32(!w.raining)
		w.thundering = false
		w.raining = false
		return
	}

	if w.thunderTime > 0 {
		w.thunderTime--
		if w.thunderTime == 0 {
			w.thundering = !w.thundering
		}
	} else if w.thundering {
		w.thunderTime = w.randRange(thunderDurationMin, thunderDurationMax)
	} else {
		w.thunderTime = w.randRange(thunderDelayMin, thunderDelayMax)
	}

	if w.rainTime > 0 {
		w.rainTime--
		if w.rainTime == 0 {
			w.raining = !w.raining
		}
	} else if w.raining {
		w.rainTime = w.randRange(rainDurationMin, rainDurationMax)
	} else {
		w.rainTime = w.randRange(rainDelayMin, rainDelayMax)
	}
}

func (w *Weather) randRange(min, max int32) int32 {
	return min + w.rng.Int31n(max-min+1)
}

func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
