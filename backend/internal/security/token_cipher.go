package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const tokenCipherVersion = "v1"

type TokenCipher struct {
	aead cipher.AEAD
}

func NewTokenCipher(key []byte) (*TokenCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("token encryption key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create token AEAD: %w", err)
	}
	return &TokenCipher{aead: aead}, nil
}

func (c *TokenCipher) Encrypt(plaintext string) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("token cipher is required")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}
	payload := c.aead.Seal(nonce, nonce, []byte(plaintext), []byte(tokenCipherVersion))
	return tokenCipherVersion + "." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *TokenCipher) Decrypt(encoded string) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("token cipher is required")
	}
	version, payloadText, ok := strings.Cut(encoded, ".")
	if !ok || version != tokenCipherVersion || payloadText == "" {
		return "", errors.New("invalid encrypted token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadText)
	if err != nil {
		return "", errors.New("invalid encrypted token encoding")
	}
	minimumLength := c.aead.NonceSize() + c.aead.Overhead()
	if len(payload) < minimumLength {
		return "", errors.New("invalid encrypted token payload")
	}
	nonce := payload[:c.aead.NonceSize()]
	ciphertext := payload[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, []byte(tokenCipherVersion))
	if err != nil {
		return "", errors.New("encrypted token authentication failed")
	}
	return string(plaintext), nil
}
