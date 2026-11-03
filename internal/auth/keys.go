package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	RoleAdmin = "admin"
	RoleERP   = "erp"
)

func GenerateAPIKey(role string) (string, string, string, error) {
	if role != RoleAdmin && role != RoleERP {
		return "", "", "", errors.New("papel deve ser admin ou erp")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	key := "pag_" + role + "_" + secret
	return key, HashAPIKey(key), Prefix(key), nil
}

func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

func Prefix(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 16 {
		return key
	}
	return key[:16]
}
