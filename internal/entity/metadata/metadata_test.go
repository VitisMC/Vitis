package metadata

import (
	"testing"
)

func TestDefaultPlayer(t *testing.T) {
	m := DefaultPlayer()
	data := m.Encode()
	if len(data) == 0 {
		t.Fatal("expected non-empty metadata")
	}
	if data[len(data)-1] != EndMarker {
		t.Fatalf("expected end marker 0xFF, got 0x%02X", data[len(data)-1])
	}
}

func TestEmptyMetadata(t *testing.T) {
	m := New()
	data := m.Encode()
	if len(data) != 1 {
		t.Fatalf("expected 1 byte (end marker), got %d", len(data))
	}
	if data[0] != EndMarker {
		t.Fatalf("expected 0xFF, got 0x%02X", data[0])
	}
}

func TestSetAndEncode(t *testing.T) {
	m := New()
	m.SetByte(0, 0)
	m.SetFloat(9, 20.0)
	m.SetBool(3, false)
	data := m.Encode()
	if data[len(data)-1] != EndMarker {
		t.Fatal("missing end marker")
	}
	if len(data) < 10 {
		t.Fatalf("encoded data too short: %d bytes", len(data))
	}
}

func BenchmarkDefaultPlayerEncode(b *testing.B) {
	m := DefaultPlayer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Encode()
	}
}
