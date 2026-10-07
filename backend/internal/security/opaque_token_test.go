package security

import (
	"encoding/base64"
	"testing"
)

func TestOpaqueTokenHasThirtyTwoRandomBytes(t *testing.T) {
	seen := make(map[string]struct{})
	for range 64 {
		raw, hash, err := NewOpaqueToken()
		if err != nil {
			t.Fatalf("NewOpaqueToken() error = %v", err)
		}
		if _, exists := seen[raw]; exists {
			t.Fatal("NewOpaqueToken() returned a duplicate token")
		}
		seen[raw] = struct{}{}
		if len(raw) == 0 || raw[len(raw)-1] == '=' {
			t.Fatalf("raw token %q is empty or padded", raw)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("decode raw token: %v", err)
		}
		if len(decoded) != 32 {
			t.Fatalf("decoded token length = %d, want 32", len(decoded))
		}
		if got := HashOpaqueToken(raw); got != hash {
			t.Fatal("HashOpaqueToken() does not reproduce generated hash")
		}
	}
}

func TestOpaqueTokenHashIsStableAndDistinct(t *testing.T) {
	first := HashOpaqueToken("first")
	if first != HashOpaqueToken("first") {
		t.Fatal("hash is not stable")
	}
	if first == HashOpaqueToken("second") {
		t.Fatal("distinct raw values have identical hashes")
	}
}

func TestValidateOpaqueTokenRejectsMalformedValues(t *testing.T) {
	valid, _, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	if err := ValidateOpaqueToken(valid); err != nil {
		t.Fatalf("ValidateOpaqueToken(valid) error = %v", err)
	}

	for _, raw := range []string{"", "not-base64!", "c2hvcnQ", valid + "="} {
		if err := ValidateOpaqueToken(raw); err == nil {
			t.Fatalf("ValidateOpaqueToken(%q) error = nil", raw)
		}
	}
}
