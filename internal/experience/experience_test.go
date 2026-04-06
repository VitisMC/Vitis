package experience

import (
	"testing"
)

func TestXPForLevel(t *testing.T) {
	tests := []struct {
		level    int32
		expected int32
	}{
		{0, 0},
		{1, 7},
		{2, 16},
		{5, 55},
		{10, 160},
		{16, 352},
		{17, 394},
		{30, 1395},
		{31, 1507},
		{32, 1628},
	}
	for _, tt := range tests {
		got := XPForLevel(tt.level)
		if got != tt.expected {
			t.Errorf("XPForLevel(%d) = %d, want %d", tt.level, got, tt.expected)
		}
	}
}

func TestXPToNextLevel(t *testing.T) {
	tests := []struct {
		level    int32
		expected int32
	}{
		{0, 7},
		{1, 9},
		{15, 37},
		{16, 42},
		{30, 112},
		{31, 121},
	}
	for _, tt := range tests {
		got := XPToNextLevel(tt.level)
		if got != tt.expected {
			t.Errorf("XPToNextLevel(%d) = %d, want %d", tt.level, got, tt.expected)
		}
	}
}

func TestFromTotal(t *testing.T) {
	r := FromTotal(0)
	if r.Level != 0 || r.Total != 0 || r.Bar != 0 {
		t.Errorf("FromTotal(0) = %+v, want {0,0,0}", r)
	}

	r = FromTotal(7)
	if r.Level != 1 {
		t.Errorf("FromTotal(7).Level = %d, want 1", r.Level)
	}
	if r.Bar != 0 {
		t.Errorf("FromTotal(7).Bar = %f, want 0", r.Bar)
	}

	r = FromTotal(352)
	if r.Level != 16 {
		t.Errorf("FromTotal(352).Level = %d, want 16", r.Level)
	}
}

func TestAddXP(t *testing.T) {
	r := AddXP(0, 0, 100)
	if r.Total != 100 {
		t.Errorf("AddXP total = %d, want 100", r.Total)
	}
	if r.Level < 1 {
		t.Errorf("AddXP level = %d, want >= 1", r.Level)
	}
}

func TestAddXPNegativeClampsToZero(t *testing.T) {
	r := AddXP(5, 55, -1000)
	if r.Total != 0 {
		t.Errorf("expected total clamped to 0, got %d", r.Total)
	}
	if r.Level != 0 {
		t.Errorf("expected level 0, got %d", r.Level)
	}
}

func TestSetLevel(t *testing.T) {
	r := SetLevel(10)
	if r.Level != 10 {
		t.Errorf("SetLevel(10).Level = %d, want 10", r.Level)
	}
	if r.Total != XPForLevel(10) {
		t.Errorf("SetLevel(10).Total = %d, want %d", r.Total, XPForLevel(10))
	}
	if r.Bar != 0 {
		t.Errorf("SetLevel(10).Bar = %f, want 0", r.Bar)
	}
}

func TestSetLevels(t *testing.T) {
	r := SetLevels(5, 55, 3)
	if r.Level != 8 {
		t.Errorf("SetLevels(5,55,3).Level = %d, want 8", r.Level)
	}

	r = SetLevels(2, 16, -10)
	if r.Level != 0 {
		t.Errorf("expected level clamped to 0, got %d", r.Level)
	}
}

func TestBarProgress(t *testing.T) {
	total := XPForLevel(5)
	cost := XPToNextLevel(5)
	half := total + cost/2

	r := FromTotal(half)
	if r.Level != 5 {
		t.Errorf("expected level 5, got %d", r.Level)
	}
	if r.Bar < 0.4 || r.Bar > 0.6 {
		t.Errorf("expected bar near 0.5, got %f", r.Bar)
	}
}
