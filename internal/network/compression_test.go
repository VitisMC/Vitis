package network

import (
	"bytes"
	"sync"
	"testing"
)

func TestCompressFrameConcurrentReuse(t *testing.T) {
	var workers sync.WaitGroup
	for worker := 0; worker < 12; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			payload := bytes.Repeat([]byte{byte(worker)}, 4096)
			for i := 0; i < 20; i++ {
				frame, err := CompressFrame(payload, 256)
				if err != nil {
					t.Errorf("compress: %v", err)
					return
				}
				decoded, err := DecompressFrame(stripOuterVarInt(frame))
				if err != nil || !bytes.Equal(decoded, payload) {
					t.Errorf("roundtrip worker %d: %v", worker, err)
					return
				}
			}
		}(worker)
	}
	workers.Wait()
}

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

func TestDecompressRejectsUnderdeclaredSize(t *testing.T) {
	encoded, err := CompressFrame([]byte{1, 2, 3, 4}, 0)
	if err != nil {
		t.Fatal(err)
	}
	inner := stripOuterVarInt(encoded)
	inner[0] = 1
	if _, err := DecompressFrame(inner); err == nil {
		t.Fatal("expected decompressed length mismatch")
	}
}

func TestDecompressRejectsInvalidChecksum(t *testing.T) {
	encoded, err := CompressFrame([]byte{1, 2, 3, 4}, 0)
	if err != nil {
		t.Fatal(err)
	}
	encoded[len(encoded)-1] ^= 0xff
	if _, err := DecompressFrame(stripOuterVarInt(encoded)); err == nil {
		t.Fatal("expected zlib checksum error")
	}
}

func TestDecompressEnforcesThreshold(t *testing.T) {
	compressed, err := CompressFrame([]byte("payload"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecompressFrameWithThreshold(stripOuterVarInt(compressed), 1024); err == nil {
		t.Fatal("expected compressed payload below threshold to be rejected")
	}

	uncompressed, err := CompressFrame([]byte("payload"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecompressFrameWithThreshold(stripOuterVarInt(uncompressed), 4); err == nil {
		t.Fatal("expected uncompressed payload at or above threshold to be rejected")
	}
}

func FuzzDecompressFrame(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{0x80, 0x80, 0x80, 0x80, 0x80})
	f.Add([]byte{4, 0, 1, 2, 3})

	f.Fuzz(func(t *testing.T, frame []byte) {
		if len(frame) > maxDecompressedSize {
			t.Skip()
		}
		_, _ = DecompressFrame(frame)
		_, _ = DecompressFrameWithThreshold(frame, 256)
	})
}
