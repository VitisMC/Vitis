package protocrypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestCFB8RoundTrip(t *testing.T) {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("Hello Minecraft protocol encryption test!")

	enc, err := NewCFB8Encrypter(secret)
	if err != nil {
		t.Fatalf("NewCFB8Encrypter: %v", err)
	}
	dec, err := NewCFB8Decrypter(secret)
	if err != nil {
		t.Fatalf("NewCFB8Decrypter: %v", err)
	}

	ciphertext := make([]byte, len(plaintext))
	enc.XORKeyStream(ciphertext, plaintext)

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted := make([]byte, len(ciphertext))
	dec.XORKeyStream(decrypted, ciphertext)

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestCFB8StreamingRoundTrip(t *testing.T) {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}

	enc, err := NewCFB8Encrypter(secret)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := NewCFB8Decrypter(secret)
	if err != nil {
		t.Fatal(err)
	}

	chunks := [][]byte{
		[]byte("chunk1"),
		[]byte("chunk2chunk2"),
		[]byte("c"),
	}

	var allPlain, allCipher []byte
	for _, chunk := range chunks {
		allPlain = append(allPlain, chunk...)
		ct := make([]byte, len(chunk))
		enc.XORKeyStream(ct, chunk)
		allCipher = append(allCipher, ct...)
	}

	decrypted := make([]byte, len(allCipher))
	dec.XORKeyStream(decrypted, allCipher)

	if !bytes.Equal(decrypted, allPlain) {
		t.Fatalf("streaming roundtrip failed")
	}
}

func TestCFB8InvalidKeySize(t *testing.T) {
	_, err := NewCFB8Encrypter([]byte("short"))
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
	_, err = NewCFB8Decrypter([]byte("short"))
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
}
