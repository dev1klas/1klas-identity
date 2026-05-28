package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// ErrCacheMiss is returned by Cache.Get when the key is not present. This
// SHOULD NOT be treated as an error by callers — it is a normal miss and the
// caller is expected to fall back to the Postgres source of truth and then
// re-populate the cache.
var ErrCacheMiss = errors.New("session: cache miss")

// CachedSession is the full payload the write-through cache stores against
// the SHA-256 hash of an opaque session cookie token. It contains exactly the
// fields SessionAuth needs to authorise a request without a Postgres
// round-trip: identity + tenant + expiry.
//
// CreatedAt and RevokedAt are intentionally omitted — no middleware or handler
// consumes them on the hot path. Revocation invalidates the cache entry via
// sign-out's Delete; expiry is enforced by ExpiresAt.
type CachedSession struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	TenantID  tenant.ID
	ExpiresAt time.Time
}

// Cache is the persistence-port for the write-through session cache.
// Postgres remains the source of truth; this cache exists to short-circuit
// the most frequent session-lookup path (authenticated requests).
//
// Per ADR-0008 the cache stores the FULL session payload (CachedSession),
// keyed by the SHA-256 hash of the opaque cookie token, so a cache hit on the
// authenticated read path skips Postgres entirely.
//
// Implementations MUST NOT mask transient infra failures as ErrCacheMiss
// arbitrarily; the contract is:
//   - key absent  -> ErrCacheMiss
//   - corrupt payload (unmarshal fails) -> ErrCacheMiss (logged WARN inside
//     the adapter — corrupt cache entries are equivalent to a miss; Postgres
//     is the source of truth)
//   - any other infra error -> that error, raw, so the caller can log WARN
//     and fall through to Postgres without blocking auth.
type Cache interface {
	// Set writes the (tokenHash -> CachedSession) mapping with the given TTL.
	// TTL should be the remaining lifetime of the session (write-through on
	// create uses absolute TTL; refresh on miss uses remaining TTL).
	Set(ctx context.Context, tokenHash string, payload CachedSession, ttl time.Duration) error

	// Get retrieves the cached session for a token hash. MUST return
	// ErrCacheMiss (not a generic error) on a clean miss or on a corrupt
	// payload.
	Get(ctx context.Context, tokenHash string) (CachedSession, error)

	// Delete removes the (tokenHash -> *) mapping. Idempotent: a missing key
	// MUST NOT be reported as an error.
	Delete(ctx context.Context, tokenHash string) error
}
