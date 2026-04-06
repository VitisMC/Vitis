package weather

import (
	"testing"
)

func TestNewWeather(t *testing.T) {
	w := New(42)
	if w == nil {
		t.Fatal("expected non-nil weather")
	}
	if w.CurrentState() != Clear {
		t.Errorf("expected Clear, got %v", w.CurrentState())
	}
	if w.IsRaining() {
		t.Error("expected not raining")
	}
	if w.IsThundering() {
		t.Error("expected not thundering")
	}
}

func TestSetWeatherRain(t *testing.T) {
	w := New(42)
	packets := w.SetWeather(Rain, 6000)

	if w.CurrentState() != Rain {
		t.Errorf("expected Rain, got %v", w.CurrentState())
	}
	if !w.IsRaining() {
		t.Error("expected raining")
	}
	if w.IsThundering() {
		t.Error("expected not thundering")
	}

	hasBeginRain := false
	for _, p := range packets {
		if p.Event == GameEventBeginRain {
			hasBeginRain = true
		}
	}
	if !hasBeginRain {
		t.Error("expected BeginRain packet")
	}
}

func TestSetWeatherThunder(t *testing.T) {
	w := New(42)
	packets := w.SetWeather(Thunder, 6000)

	if w.CurrentState() != Thunder {
		t.Errorf("expected Thunder, got %v", w.CurrentState())
	}
	if !w.IsRaining() {
		t.Error("expected raining during thunder")
	}
	if !w.IsThundering() {
		t.Error("expected thundering")
	}

	hasBeginRain := false
	for _, p := range packets {
		if p.Event == GameEventBeginRain {
			hasBeginRain = true
		}
	}
	if !hasBeginRain {
		t.Error("expected BeginRain packet when transitioning to thunder")
	}
}

func TestSetWeatherClear(t *testing.T) {
	w := New(42)
	w.SetWeather(Rain, 6000)
	packets := w.SetWeather(Clear, 6000)

	if w.CurrentState() != Clear {
		t.Errorf("expected Clear, got %v", w.CurrentState())
	}

	hasEndRain := false
	for _, p := range packets {
		if p.Event == GameEventEndRain {
			hasEndRain = true
		}
	}
	if !hasEndRain {
		t.Error("expected EndRain packet")
	}
}

func TestTickTransition(t *testing.T) {
	w := New(42)
	w.SetCycleEnabled(false)
	w.SetWeather(Rain, 6000)

	var gotRainLevel bool
	for i := 0; i < 5; i++ {
		packets := w.Tick()
		for _, p := range packets {
			if p.Event == GameEventRainLevelChange && p.Value > 0 {
				gotRainLevel = true
			}
		}
	}
	if !gotRainLevel {
		t.Error("expected rain level change packets during transition")
	}
	if w.RainLevel() <= 0 {
		t.Error("expected rain level > 0 after ticking")
	}
}

func TestTickFullTransition(t *testing.T) {
	w := New(42)
	w.SetCycleEnabled(false)
	w.SetWeather(Rain, 200)

	for i := 0; i < 150; i++ {
		w.Tick()
	}

	if w.RainLevel() < 0.99 {
		t.Errorf("expected rain level near 1.0, got %f", w.RainLevel())
	}
}

func TestJoinPacketsClear(t *testing.T) {
	w := New(42)
	packets := w.JoinPackets()
	if len(packets) != 0 {
		t.Errorf("expected no join packets for clear weather, got %d", len(packets))
	}
}

func TestJoinPacketsRain(t *testing.T) {
	w := New(42)
	w.SetCycleEnabled(false)
	w.SetWeather(Rain, 6000)

	for i := 0; i < 50; i++ {
		w.Tick()
	}

	packets := w.JoinPackets()
	if len(packets) == 0 {
		t.Error("expected join packets for rainy weather")
	}

	hasBeginRain := false
	hasRainLevel := false
	for _, p := range packets {
		if p.Event == GameEventBeginRain {
			hasBeginRain = true
		}
		if p.Event == GameEventRainLevelChange {
			hasRainLevel = true
		}
	}
	if !hasBeginRain {
		t.Error("expected BeginRain in join packets")
	}
	if !hasRainLevel {
		t.Error("expected RainLevelChange in join packets")
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		input    string
		expected State
	}{
		{"clear", Clear},
		{"rain", Rain},
		{"thunder", Thunder},
		{"unknown", Clear},
	}
	for _, tt := range tests {
		got := ParseState(tt.input)
		if got != tt.expected {
			t.Errorf("ParseState(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{Clear, "clear"},
		{Rain, "rain"},
		{Thunder, "thunder"},
	}
	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
