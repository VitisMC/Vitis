package network

import (
	"bytes"
	"testing"
)

// stripOuterVarInt strips the leading VarInt length prefix from a CompressFrame output
// to produce the inner frame that DecompressFrame expects.
func stripOuterVarInt(data []byte) []byte {
	_, consumed, ok, err := decodeVarInt(data)
	if err != nil || !ok {
		return data
	}
	return data[consumed:]
}

func TestCompressDecompressRoundtrip(t *testing.T) {
	payload := []byte("hello world this is a test payload for compression roundtrip")

	compressed, err := CompressFrame(payload, 0)
	if err != nil {
		t.Fatalf("CompressFrame: %v", err)
	}
	if len(compressed) == 0 {
		t.Fatal("compressed output is empty")
	}

	inner := stripOuterVarInt(compressed)
	decompressed, err := DecompressFrame(inner)
	if err != nil {
		t.Fatalf("DecompressFrame: %v", err)
	}
	if !bytes.Equal(decompressed, payload) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decompressed, payload)
	}
}

func TestCompressBelowThreshold(t *testing.T) {
	payload := []byte("small")

	compressed, err := CompressFrame(payload, 1024)
	if err != nil {
		t.Fatalf("CompressFrame: %v", err)
	}

	inner := stripOuterVarInt(compressed)
	decompressed, err := DecompressFrame(inner)
	if err != nil {
		t.Fatalf("DecompressFrame: %v", err)
	}
	if !bytes.Equal(decompressed, payload) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decompressed, payload)
	}
}

func TestCompressLargePayload(t *testing.T) {
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	compressed, err := CompressFrame(payload, 256)
	if err != nil {
		t.Fatalf("CompressFrame: %v", err)
	}

	inner := stripOuterVarInt(compressed)
	decompressed, err := DecompressFrame(inner)
	if err != nil {
		t.Fatalf("DecompressFrame: %v", err)
	}
	if !bytes.Equal(decompressed, payload) {
		t.Fatalf("roundtrip mismatch for large payload")
	}
}

func TestDecompressEmptyFrame(t *testing.T) {
	_, err := DecompressFrame(nil)
	if err == nil {
		t.Fatal("expected error for nil frame")
	}
}
