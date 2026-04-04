package protocol

import (
	"testing"
)

func TestOfflinePlayerUUID(t *testing.T) {
	uuid := OfflinePlayerUUID("Notch")
	str := UUIDToString(uuid)

	if uuid == (UUID{}) {
		t.Fatal("UUID should not be zero")
	}

	b6 := byte(uuid[0] >> 8)
	if (b6 >> 4) != 3 {
		t.Errorf("UUID version should be 3, got %d", b6>>4)
	}

	b8 := byte(uuid[1] >> 56)
	if (b8 >> 6) != 2 {
		t.Errorf("UUID variant should be 2 (RFC 4122), got %d", b8>>6)
	}

	if len(str) != 36 {
		t.Errorf("UUID string length should be 36, got %d: %s", len(str), str)
	}
	if str[8] != '-' || str[13] != '-' || str[18] != '-' || str[23] != '-' {
		t.Errorf("UUID string format invalid: %s", str)
	}
}

func TestOfflinePlayerUUIDDeterministic(t *testing.T) {
	a := OfflinePlayerUUID("Steve")
	b := OfflinePlayerUUID("Steve")
	if a != b {
		t.Errorf("same name should produce same UUID: %s != %s", UUIDToString(a), UUIDToString(b))
	}
}

func TestOfflinePlayerUUIDDifferentNames(t *testing.T) {
	a := OfflinePlayerUUID("Alice")
	b := OfflinePlayerUUID("Bob")
	if a == b {
		t.Error("different names should produce different UUIDs")
	}
}

func TestUUIDToStringFormat(t *testing.T) {
	uuid := OfflinePlayerUUID("test")
	str := UUIDToString(uuid)

	parts := 0
	for _, c := range str {
		if c == '-' {
			parts++
		}
	}
	if parts != 4 {
		t.Errorf("expected 4 hyphens, got %d in %s", parts, str)
	}
}
