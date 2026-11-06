package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func HashToken(raw string) string { return hex.EncodeToString([]byte(raw)) }
func SignJWT(userID, orgID, secret string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": userID, "org_id": orgID, "exp": time.Now().Add(24 * time.Hour).Unix()}).SignedString([]byte(secret))
}
func ParseJWT(raw, secret string) (string, string, error) {
	t, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return "", "", errors.New("invalid token")
	}
	c, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid claims")
	}
	uid, uok := c["sub"].(string)
	oid, ook := c["org_id"].(string)
	if !uok || !ook {
		return "", "", errors.New("invalid claims")
	}
	return uid, oid, nil
}
func CleanQuestion(v string) string { return strings.TrimSpace(v) }
