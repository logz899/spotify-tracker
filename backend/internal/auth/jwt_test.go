package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestManagerGenerateAndValidateToken(t *testing.T) {
	manager := newTestManager(t)
	raw, err := manager.GenerateToken(42, "listener")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := manager.ValidateToken(raw)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != 42 || claims.Username != "listener" {
		t.Fatalf("claims user = (%d, %q), want (42, listener)", claims.UserID, claims.Username)
	}
	if claims.Issuer != "test-issuer" || len(claims.Audience) != 1 || claims.Audience[0] != "test-audience" {
		t.Fatalf("registered claims issuer/audience = %q/%v", claims.Issuer, claims.Audience)
	}
	if claims.IssuedAt == nil || claims.NotBefore == nil || claims.ExpiresAt == nil {
		t.Fatal("generated token is missing required time claims")
	}
	lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	if lifetime != 12*time.Hour {
		t.Fatalf("token lifetime = %s, want 12h", lifetime)
	}
}

func TestManagerRejectsInvalidTokens(t *testing.T) {
	manager := newTestManager(t)
	validTimes := jwt.RegisteredClaims{
		Issuer:    "test-issuer",
		Audience:  jwt.ClaimStrings{"test-audience"},
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}

	tests := []struct {
		name   string
		method jwt.SigningMethod
		claims Claims
		secret []byte
	}{
		{
			name:   "expired",
			method: jwt.SigningMethodHS256,
			claims: Claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: "test-issuer", Audience: jwt.ClaimStrings{"test-audience"}, IssuedAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)), NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}},
			secret: []byte(strings.Repeat("s", 32)),
		},
		{name: "wrong issuer", method: jwt.SigningMethodHS256, claims: Claims{RegisteredClaims: withIssuer(validTimes, "other")}, secret: []byte(strings.Repeat("s", 32))},
		{name: "wrong audience", method: jwt.SigningMethodHS256, claims: Claims{RegisteredClaims: withAudience(validTimes, "other")}, secret: []byte(strings.Repeat("s", 32))},
		{name: "wrong algorithm", method: jwt.SigningMethodHS384, claims: Claims{RegisteredClaims: validTimes}, secret: []byte(strings.Repeat("s", 32))},
		{name: "wrong signature", method: jwt.SigningMethodHS256, claims: Claims{RegisteredClaims: validTimes}, secret: []byte(strings.Repeat("x", 32))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := jwt.NewWithClaims(tt.method, tt.claims).SignedString(tt.secret)
			if err != nil {
				t.Fatalf("SignedString() error = %v", err)
			}
			if _, err := manager.ValidateToken(raw); err == nil {
				t.Fatal("ValidateToken() error = nil")
			}
		})
	}

	if _, err := manager.ValidateToken("not-a-token"); err == nil {
		t.Fatal("ValidateToken(malformed) error = nil")
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	manager, err := NewManager([]byte(strings.Repeat("s", 32)), "test-issuer", "test-audience", 12*time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func withIssuer(claims jwt.RegisteredClaims, issuer string) jwt.RegisteredClaims {
	claims.Issuer = issuer
	return claims
}

func withAudience(claims jwt.RegisteredClaims, audience string) jwt.RegisteredClaims {
	claims.Audience = jwt.ClaimStrings{audience}
	return claims
}
