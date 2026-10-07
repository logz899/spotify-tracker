package security

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestTokenCipherRoundTripUsesUniqueNonces(t *testing.T) {
	cipher := newTestCipher(t, "k")
	first, err := cipher.Encrypt("spotify-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := cipher.Encrypt("spotify-token")
	if err != nil {
		t.Fatalf("Encrypt() second error = %v", err)
	}
	if first == second {
		t.Fatal("Encrypt() reused a nonce for identical plaintext")
	}
	if !strings.HasPrefix(first, "v1.") {
		t.Fatalf("ciphertext = %q, want v1 prefix", first)
	}

	plaintext, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "spotify-token" {
		t.Fatalf("Decrypt() = %q, want spotify-token", plaintext)
	}
}

func TestTokenCipherSupportsEmptyPlaintext(t *testing.T) {
	cipher := newTestCipher(t, "k")
	encrypted, err := cipher.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt(empty) error = %v", err)
	}
	plaintext, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt(empty) error = %v", err)
	}
	if plaintext != "" {
		t.Fatalf("Decrypt(empty) = %q", plaintext)
	}
}

func TestTokenCipherRejectsWrongKeyAndTampering(t *testing.T) {
	cipher := newTestCipher(t, "k")
	encrypted, err := cipher.Encrypt("spotify-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	wrongCipher := newTestCipher(t, "x")
	if _, err := wrongCipher.Decrypt(encrypted); err == nil {
		t.Fatal("Decrypt() with wrong key error = nil")
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encrypted, "v1."))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	payload[len(payload)-1] ^= 0xff
	tampered := "v1." + base64.RawURLEncoding.EncodeToString(payload)
	if _, err := cipher.Decrypt(tampered); err == nil {
		t.Fatal("Decrypt() tampered ciphertext error = nil")
	}
}

func TestTokenCipherRejectsInvalidInput(t *testing.T) {
	if _, err := NewTokenCipher([]byte("short")); err == nil {
		t.Fatal("NewTokenCipher(short key) error = nil")
	}

	cipher := newTestCipher(t, "k")
	for _, raw := range []string{"", "v2.payload", "v1.", "v1.not-base64!", "v1.YQ"} {
		if _, err := cipher.Decrypt(raw); err == nil {
			t.Fatalf("Decrypt(%q) error = nil", raw)
		}
	}
}

func newTestCipher(t *testing.T, keyByte string) *TokenCipher {
	t.Helper()
	cipher, err := NewTokenCipher([]byte(strings.Repeat(keyByte, 32)))
	if err != nil {
		t.Fatalf("NewTokenCipher() error = %v", err)
	}
	return cipher
}
