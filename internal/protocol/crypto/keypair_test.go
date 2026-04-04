package protocrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if key == nil {
		t.Fatal("key is nil")
	}
	if key.N.BitLen() != rsaKeyBits {
		t.Fatalf("expected %d-bit key, got %d", rsaKeyBits, key.N.BitLen())
	}
}

func TestEncodePublicKey(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	der, err := EncodePublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("EncodePublicKey: %v", err)
	}
	if len(der) == 0 {
		t.Fatal("DER is empty")
	}
}

func TestDecryptSharedSecretRoundtrip(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, &key.PublicKey, secret)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := DecryptSharedSecret(key, encrypted)
	if err != nil {
		t.Fatalf("DecryptSharedSecret: %v", err)
	}

	if string(decrypted) != string(secret) {
		t.Fatalf("mismatch: got %x, want %x", decrypted, secret)
	}
}

func TestGenerateVerifyToken(t *testing.T) {
	token, err := GenerateVerifyToken()
	if err != nil {
		t.Fatalf("GenerateVerifyToken: %v", err)
	}
	if len(token) != 4 {
		t.Fatalf("expected 4-byte token, got %d", len(token))
	}
}
