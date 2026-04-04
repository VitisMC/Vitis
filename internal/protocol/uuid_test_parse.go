package protocol

import "testing"

func TestParseUUIDValid(t *testing.T) {
	input := "12345678-1234-1234-1234-123456789abc"
	u, err := ParseUUID(input)
	if err != nil {
		t.Fatalf("ParseUUID: %v", err)
	}
	roundtrip := UUIDToString(u)
	if roundtrip != input {
		t.Fatalf("roundtrip mismatch: got %q, want %q", roundtrip, input)
	}
}

func TestParseUUIDOfflineRoundtrip(t *testing.T) {
	original := OfflinePlayerUUID("Notch")
	str := UUIDToString(original)
	parsed, err := ParseUUID(str)
	if err != nil {
		t.Fatalf("ParseUUID: %v", err)
	}
	if parsed != original {
		t.Fatalf("roundtrip mismatch: got %v, want %v", parsed, original)
	}
}

func TestParseUUIDInvalidFormat(t *testing.T) {
	cases := []string{
		"",
		"12345678123412341234123456789abc",
		"12345678-1234-1234-1234-123456789ab",
		"ZZZZZZZZ-1234-1234-1234-123456789abc",
	}
	for _, c := range cases {
		if _, err := ParseUUID(c); err == nil {
			t.Errorf("expected error for input %q", c)
		}
	}
}
