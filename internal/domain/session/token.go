package session

import (
	"crypto/sha256"
	"errors"
)

// ErrInvalidToken is returned when a token string is not a valid opaque token.
var ErrInvalidToken = errors.New("session: invalid token")

// Token is the plaintext opaque session token. It is never persisted —
// only its SHA-256 hash lives in the database.
type Token struct {
	value string
}

// NewToken wraps an opaque token string (base64url, ~43 chars).
func NewToken(s string) (Token, error) {
	if len(s) < 32 || len(s) > 128 {
		return Token{}, ErrInvalidToken
	}
	return Token{value: s}, nil
}

// String returns the raw token value. Callers MUST treat this as a secret —
// no logging, no error messages.
func (t Token) String() string { return t.value }

// Hash returns the SHA-256 hash of the token. Stable, deterministic, and
// safe to store + index.
func (t Token) Hash() []byte {
	sum := sha256.Sum256([]byte(t.value))
	return sum[:]
}

// HashOf returns the SHA-256 hash of an arbitrary token string without
// constructing a Token. Used by middleware on inbound cookies.
func HashOf(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// TokenGenerator produces fresh opaque tokens. The implementation lives in
// internal/infrastructure/tokens.
type TokenGenerator interface {
	NewToken() (Token, error)
}
