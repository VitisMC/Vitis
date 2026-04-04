package protocrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// cfb8Stream implements AES/CFB8 encryption or decryption as required by Minecraft protocol.
type cfb8Stream struct {
	block   cipher.Block
	iv      [aes.BlockSize]byte
	encrypt bool
}

// NewCFB8Encrypter creates an AES-CFB8 encrypter stream.
func NewCFB8Encrypter(sharedSecret []byte) (cipher.Stream, error) {
	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("cfb8 encrypter: %w", err)
	}
	if len(sharedSecret) != aes.BlockSize {
		return nil, fmt.Errorf("cfb8 encrypter: shared secret must be %d bytes", aes.BlockSize)
	}
	s := &cfb8Stream{block: block, encrypt: true}
	copy(s.iv[:], sharedSecret)
	return s, nil
}

// NewCFB8Decrypter creates an AES-CFB8 decrypter stream.
func NewCFB8Decrypter(sharedSecret []byte) (cipher.Stream, error) {
	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("cfb8 decrypter: %w", err)
	}
	if len(sharedSecret) != aes.BlockSize {
		return nil, fmt.Errorf("cfb8 decrypter: shared secret must be %d bytes", aes.BlockSize)
	}
	s := &cfb8Stream{block: block, encrypt: false}
	copy(s.iv[:], sharedSecret)
	return s, nil
}

// XORKeyStream processes src one byte at a time using CFB8 mode.
func (s *cfb8Stream) XORKeyStream(dst, src []byte) {
	if len(dst) < len(src) {
		panic("cfb8: output smaller than input")
	}

	var encrypted [aes.BlockSize]byte

	for i := 0; i < len(src); i++ {
		s.block.Encrypt(encrypted[:], s.iv[:])

		if s.encrypt {
			dst[i] = src[i] ^ encrypted[0]
			s.shiftIV(dst[i])
		} else {
			cipherByte := src[i]
			dst[i] = cipherByte ^ encrypted[0]
			s.shiftIV(cipherByte)
		}
	}
}

func (s *cfb8Stream) shiftIV(b byte) {
	copy(s.iv[:], s.iv[1:])
	s.iv[aes.BlockSize-1] = b
}
