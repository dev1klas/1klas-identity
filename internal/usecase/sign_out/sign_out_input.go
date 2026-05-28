package sign_out

import (
	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Input is the use case command for sign-out. Session id comes from the
// session middleware, not the request body.
//
// TokenHashHex is the hex-encoded SHA-256 of the cookie token; it is supplied
// by the SessionAuth middleware so the use case can invalidate the
// write-through session cache without re-reading the Postgres row.
// May be empty (e.g. older callers) — in that case the cache entry simply
// expires naturally instead of being deleted promptly.
type Input struct {
	TenantID     tenant.ID
	SessionID    uuid.UUID
	UserID       uuid.UUID
	TokenHashHex string
}
