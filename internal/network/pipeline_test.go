package network

import (
	"errors"
	"testing"
)

func TestTryDecodeFrameComplete(t *testing.T) {
	frame := appendVarInt(nil, 3)
	frame = append(frame, 0x11, 0x22, 0x33)

	length, header, complete, err := TryDecodeFrame(frame, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !complete {
		t.Fatalf("expected complete frame")
	}
	if length != 3 {
		t.Fatalf("unexpected frame length: %d", length)
	}
	if header != 1 {
		t.Fatalf("unexpected header size: %d", header)
	}
}

func TestTryDecodeFrameFragmentedPayload(t *testing.T) {
	fragment := appendVarInt(nil, 3)
	fragment = append(fragment, 0xAA)

	length, header, complete, err := TryDecodeFrame(fragment, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if complete {
		t.Fatalf("expected incomplete frame")
	}
	if length != 3 {
		t.Fatalf("unexpected frame length: %d", length)
	}
	if header != 1 {
		t.Fatalf("unexpected header size: %d", header)
	}
}

func TestTryDecodeFrameMalformedVarInt(t *testing.T) {
	_, _, _, err := TryDecodeFrame([]byte{0x80, 0x80, 0x80, 0x80, 0x80}, 1024)
	if !IsProtocolError(err) {
		t.Fatalf("expected protocol error, got: %v", err)
	}
	if !errors.Is(err, ErrMalformedVarInt) {
		t.Fatalf("expected malformed varint, got: %v", err)
	}
}

func TestTryDecodeFrameTooLarge(t *testing.T) {
	frame := appendVarInt(nil, 300)
	frame = append(frame, make([]byte, 300)...)

	_, _, _, err := TryDecodeFrame(frame, 64)
	if !IsProtocolError(err) {
		t.Fatalf("expected protocol error, got: %v", err)
	}
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected frame-too-large error, got: %v", err)
	}
}

func appendVarInt(dst []byte, value int32) []byte {
	buf := make([]byte, 5)
	n := writeVarInt(buf, value)
	return append(dst, buf[:n]...)
}
