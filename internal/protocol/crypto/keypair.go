package protocrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
)

const rsaKeyBits = 1024

// GenerateKeyPair generates a 1024-bit RSA keypair for Minecraft protocol encryption.
func GenerateKeyPair() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate rsa keypair: %w", err)
	}
	return key, nil
}

// EncodePublicKey encodes an RSA public key to ASN.1 DER format.
func EncodePublicKey(pub *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("encode public key: %w", err)
	}
	return der, nil
}

// DecryptSharedSecret decrypts client-encrypted shared secret using PKCS1v15.
func DecryptSharedSecret(priv *rsa.PrivateKey, encrypted []byte) ([]byte, error) {
	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, priv, encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt shared secret: %w", err)
	}
	return plaintext, nil
}

// DecryptVerifyToken decrypts client-encrypted verify token using PKCS1v15.
func DecryptVerifyToken(priv *rsa.PrivateKey, encrypted []byte) ([]byte, error) {
	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, priv, encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt verify token: %w", err)
	}
	return plaintext, nil
}

// GenerateVerifyToken generates a random 4-byte verify token.
func GenerateVerifyToken() ([]byte, error) {
	token := make([]byte, 4)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("generate verify token: %w", err)
	}
	return token, nil
}
