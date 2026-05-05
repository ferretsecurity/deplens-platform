package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type TokenLookup interface {
	FindToken(ctx context.Context, tokenHash string) (tenantID string, scopes []string, err error)
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func BearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}
