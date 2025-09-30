package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

type Crypter struct {
	key []byte
}

func NewCrypter(key string) (*Crypter, error) {
	k := []byte(key)
	l := len(k)
	if l < 32 {
		return nil, fmt.Errorf("key length must be >= 32 bytes, got %d", l)
	}
	k = k[:32]
	return &Crypter{key: k}, nil
}

func (c *Crypter) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return aesgcm.Seal(nonce, nonce, data, nil), nil
}

// Decrypt accepts raw ciphertext ([]byte)
func (c *Crypter) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesgcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return aesgcm.Open(nil, nonce, ciphertext, nil)
}
