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

func TestNewManagerValidation(t *testing.T) {
	good := strings.Repeat("a", 32)
	for _, tc := range []struct {
		name   string
		secret string
		hours  int
	}{
		{"short secret", "short", 1},
		{"blank padded secret", strings.Repeat(" ", 40), 1},
		{"default secret", "supersecretjwtkeychangeinproduction", 1},
		{"zero hours", good, 0},
		{"too many hours", good, 8761},
	} {
		if _, err := NewManager(tc.secret, tc.hours); err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
	}
	if _, err := NewManager(good, 8760); err != nil {
		t.Fatal(err)
	}
}

func TestNewAccessManager(t *testing.T) {
	good := strings.Repeat("a", 32)
	for _, minutes := range []int{0, 61} {
		if _, err := NewAccessManager(good, minutes); err == nil {
			t.Fatalf("minutes=%d accepted", minutes)
		}
	}
	if _, err := NewAccessManager("short", 15); err == nil {
		t.Fatal("short secret accepted")
	}
	m, err := NewAccessManager(good, 15)
	if err != nil {
		t.Fatal(err)
	}
	raw, seconds, err := m.Issue(1, "sid")
	if err != nil || seconds != 900 {
		t.Fatalf("seconds=%d err=%v", seconds, err)
	}
	id, sid, err := m.VerifySession(raw)
	if err != nil || id != 1 || sid != "sid" {
		t.Fatalf("id=%d sid=%q err=%v", id, sid, err)
	}
}

func TestIssueRejectsInvalidInput(t *testing.T) {
	m, _ := NewManager(strings.Repeat("a", 32), 1)
	if _, _, err := m.Issue(0, "sid"); err == nil {
		t.Fatal("zero user accepted")
	}
	if _, _, err := m.Issue(1, ""); err == nil {
		t.Fatal("empty session accepted")
	}
}

func TestVerifyRejectsClaimGaps(t *testing.T) {
	secret := strings.Repeat("a", 32)
	m, _ := NewManager(secret, 1)
	exp := jwt.NewNumericDate(time.Now().Add(time.Hour))
	for _, tc := range []struct {
		name   string
		claims Claims
	}{
		{"missing sid", Claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, Subject: "1", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: exp}}},
		{"missing issued at", Claims{SessionID: "s", RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, Subject: "1", ExpiresAt: exp}}},
		{"non numeric subject", Claims{SessionID: "s", RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, Subject: "abc", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: exp}}},
	} {
		raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, tc.claims).SignedString([]byte(secret))
		if _, _, err := m.VerifySession(raw); err == nil {
			t.Fatalf("%s: accepted", tc.name)
		}
	}
	if _, err := m.Verify("not-a-jwt"); err == nil {
		t.Fatal("garbage accepted")
	}
}
