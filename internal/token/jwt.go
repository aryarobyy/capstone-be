package token

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
}

const issuer = "capstone-be"

type Manager struct {
	secret   []byte
	lifetime time.Duration
}

func NewManager(secret string, hours int) (*Manager, error) {
	if len(strings.TrimSpace(secret)) < 32 || secret == "supersecretjwtkeychangeinproduction" {
		return nil, errors.New("JWT_SECRET must contain at least 32 characters and must not use the default secret")
	}
	if hours <= 0 || hours > 8760 {
		return nil, errors.New("JWT_EXPIRATION_HOURS must be between 1 and 8760")
	}
	return &Manager{[]byte(secret), time.Duration(hours) * time.Hour}, nil
}

// NewAccessManager uses minutes for short-lived access tokens.
func NewAccessManager(secret string, minutes int)(*Manager,error){
 if minutes<1||minutes>60{return nil,errors.New("JWT_ACCESS_MINUTES must be between 1 and 60")}
 m,err:=NewManager(secret,1);if err!=nil{return nil,err};m.lifetime=time.Duration(minutes)*time.Minute;return m,nil
}

func (m *Manager) Issue(userID int64, sessionID string) (string, int64, error) {
	if userID <= 0 || sessionID == "" {
		return "", 0, errors.New("invalid user ID")
	}
	now := time.Now()
	claims := Claims{SessionID: sessionID, RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, Subject: strconv.FormatInt(userID, 10), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(m.lifetime))}}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return signed, int64(m.lifetime / time.Second), err
}

func (m *Manager) Verify(raw string) (int64, error) {
	id, _, err := m.VerifySession(raw)
	return id, err
}

func (m *Manager) VerifySession(raw string) (int64, string, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) { return m.secret, nil }, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return 0, "", err
	}
	if claims.SessionID == "" || !parsed.Valid || claims.Issuer != issuer || claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return 0, "", errors.New("invalid access token")
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", errors.New("invalid token subject")
	}
	return id, claims.SessionID, nil
}
