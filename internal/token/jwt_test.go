package token

import (
	"github.com/golang-jwt/jwt/v4"
	"strings"
	"testing"
	"time"
)

func TestIssueVerify(t *testing.T) {
	m, err := NewManager(strings.Repeat("a", 32), 168)
	if err != nil {
		t.Fatal(err)
	}
	raw, seconds, err := m.Issue(42, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	id, err := m.Verify(raw)
	if err != nil || id != 42 || seconds != 604800 {
		t.Fatalf("id=%d seconds=%d err=%v", id, seconds, err)
	}
}
func TestRejectInvalidTokens(t *testing.T) {
	secret := strings.Repeat("a", 32)
	m, _ := NewManager(secret, 1)
	for _, tc := range []struct {
		name            string
		method          jwt.SigningMethod
		key             string
		expiry          *jwt.NumericDate
		issuer, subject string
	}{
		{"expired", jwt.SigningMethodHS256, secret, jwt.NewNumericDate(time.Now().Add(-time.Minute)), issuer, "1"},
		{"wrong signature", jwt.SigningMethodHS256, strings.Repeat("b", 32), jwt.NewNumericDate(time.Now().Add(time.Hour)), issuer, "1"},
		{"wrong algorithm", jwt.SigningMethodHS384, secret, jwt.NewNumericDate(time.Now().Add(time.Hour)), issuer, "1"},
		{"missing expiry", jwt.SigningMethodHS256, secret, nil, issuer, "1"},
		{"wrong issuer", jwt.SigningMethodHS256, secret, jwt.NewNumericDate(time.Now().Add(time.Hour)), "other", "1"},
		{"bad subject", jwt.SigningMethodHS256, secret, jwt.NewNumericDate(time.Now().Add(time.Hour)), issuer, "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := jwt.NewWithClaims(tc.method, Claims{SessionID: "session-id", RegisteredClaims: jwt.RegisteredClaims{Issuer: tc.issuer, Subject: tc.subject, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: tc.expiry}}).SignedString([]byte(tc.key))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.Verify(raw); err == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
}
