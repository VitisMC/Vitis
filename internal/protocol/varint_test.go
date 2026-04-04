package protocol

import (
	"errors"
	"testing"
)

func TestVarIntEncodeDecode(t *testing.T) {
	values := []int32{0, 1, 2, 127, 128, 255, 2097151, -1, -2147483648, 2147483647}

	for _, value := range values {
		buf := [maxVarIntBytes]byte{}
		n := EncodeVarInt(buf[:], value)
		decoded, consumed, err := DecodeVarInt(buf[:n])
		if err != nil {
			t.Fatalf("decode failed for %d: %v", value, err)
		}
		if consumed != n {
			t.Fatalf("unexpected consumed bytes for %d: got %d want %d", value, consumed, n)
		}
		if decoded != value {
			t.Fatalf("unexpected value: got %d want %d", decoded, value)
		}
	}
}

func TestVarIntSpecSamples(t *testing.T) {
	tests := []struct {
		value int32
		bytes []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{2, []byte{0x02}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{25565, []byte{0xdd, 0xc7, 0x01}},
		{2097151, []byte{0xff, 0xff, 0x7f}},
		{2147483647, []byte{0xff, 0xff, 0xff, 0xff, 0x07}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0x08}},
	}

	for _, tt := range tests {
		buf := [maxVarIntBytes]byte{}
		n := EncodeVarInt(buf[:], tt.value)
		if n != len(tt.bytes) {
			t.Errorf("value %d: encode length got %d want %d", tt.value, n, len(tt.bytes))
			continue
		}
		for i := 0; i < n; i++ {
			if buf[i] != tt.bytes[i] {
				t.Errorf("value %d: byte[%d] got 0x%02x want 0x%02x", tt.value, i, buf[i], tt.bytes[i])
			}
		}

		decoded, consumed, err := DecodeVarInt(tt.bytes)
		if err != nil {
			t.Errorf("value %d: decode failed: %v", tt.value, err)
			continue
		}
		if consumed != len(tt.bytes) {
			t.Errorf("value %d: consumed got %d want %d", tt.value, consumed, len(tt.bytes))
		}
		if decoded != tt.value {
			t.Errorf("value %d: decoded got %d want %d", tt.value, decoded, tt.value)
		}
	}
}

func TestVarIntSize(t *testing.T) {
	tests := []struct {
		value int32
		size  int
	}{
		{0, 1},
		{1, 1},
		{127, 1},
		{128, 2},
		{25565, 3},
		{2097151, 3},
		{2147483647, 5},
		{-1, 5},
		{-2147483648, 5},
	}

	for _, tt := range tests {
		got := VarIntSize(tt.value)
		if got != tt.size {
			t.Errorf("VarIntSize(%d) = %d, want %d", tt.value, got, tt.size)
		}
	}
}

func TestDecodeVarIntIncomplete(t *testing.T) {
	_, _, err := DecodeVarInt([]byte{0x80})
	if !errors.Is(err, ErrVarIntIncomplete) {
		t.Fatalf("expected ErrVarIntIncomplete, got %v", err)
	}
}

func TestDecodeVarIntTooBig(t *testing.T) {
	_, _, err := DecodeVarInt([]byte{0x80, 0x80, 0x80, 0x80, 0x80})
	if !errors.Is(err, ErrVarIntTooBig) {
		t.Fatalf("expected ErrVarIntTooBig, got %v", err)
	}
}
