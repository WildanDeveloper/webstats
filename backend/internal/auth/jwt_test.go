package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueUniqueSessions(t *testing.T) {
	manager := NewManager("test-only-signing-secret")
	tokens := make(map[string]bool)
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := manager.Issue("user-1", "user@example.com", "viewer")
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		if tokens[token] {
			t.Fatal("separate session issuances returned identical tokens")
		}
		tokens[token] = true
	}
	for token := range tokens {
		claims, err := manager.Parse(token)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if claims.ID == "" || ids[claims.ID] {
			t.Fatal("session token must have a unique nonempty jti")
		}
		ids[claims.ID] = true
		if claims.UserID != "user-1" || claims.Email != "user@example.com" || claims.Role != "viewer" || claims.Issuer != "webstats" {
			t.Fatal("session claims changed")
		}
	}
}

func TestIssueRejectsNoneAlgorithm(t *testing.T) {
	manager := NewManager("test-only-signing-secret")
	token, err := manager.Issue("user-1", "user@example.com", "viewer")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"uid": "user-1", "iss": "webstats", "exp": jwt.NewNumericDate(time.Now().Add(time.Hour))}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("forge: %v", err)
	}
	if _, err := manager.Parse(forged); err == nil {
		t.Fatal("alg=none token accepted")
	}
	hs384, err := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{"uid": "user-1", "iss": "webstats", "exp": jwt.NewNumericDate(time.Now().Add(time.Hour))}).SignedString([]byte("test-only-signing-secret"))
	if err != nil {
		t.Fatalf("forge hs384: %v", err)
	}
	if _, err := manager.Parse(hs384); err == nil {
		t.Fatal("HS384 token accepted")
	}
	noExp, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"uid": "user-1", "iss": "webstats"}).SignedString([]byte("test-only-signing-secret"))
	if err != nil {
		t.Fatalf("forge no-exp: %v", err)
	}
	if _, err := manager.Parse(noExp); err == nil {
		t.Fatal("token without exp accepted")
	}
	if _, err := manager.Parse(token); err != nil {
		t.Fatalf("legitimate token rejected: %v", err)
	}
}
