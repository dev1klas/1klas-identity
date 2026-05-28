// Package tokens implements session.TokenGenerator using crypto/rand.
package tokens

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
)

// OpaqueGenerator emits 32-random-byte tokens, base64url encoded, no padding
// (43 chars).
type OpaqueGenerator struct{}

// New returns the default generator.
func New() *OpaqueGenerator { return &OpaqueGenerator{} }

// NewToken produces a fresh opaque token.
func (g *OpaqueGenerator) NewToken() (session.Token, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return session.Token{}, err
	}
	return session.NewToken(base64.RawURLEncoding.EncodeToString(b))
}
