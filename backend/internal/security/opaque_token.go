package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const opaqueTokenBytes = 32

func NewOpaqueToken() (raw string, hash [32]byte, err error) {
	bytes := make([]byte, opaqueTokenBytes)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate opaque token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(bytes)
	return raw, HashOpaqueToken(raw), nil
}

func HashOpaqueToken(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}

func ValidateOpaqueToken(raw string) error {
	if raw == "" {
		return errors.New("opaque token is required")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return errors.New("opaque token must use unpadded base64url")
	}
	if len(decoded) != opaqueTokenBytes {
		return fmt.Errorf("opaque token must contain %d bytes", opaqueTokenBytes)
	}
	return nil
}
