package session

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss is returned by Cache.Get when the key is not present. This
// SHOULD NOT be treated as an error by callers — it is a normal miss and the
// caller is expected to fall back to the Postgres source of truth and then
// re-populate the cache.
var ErrCacheMiss = errors.New("session: cache miss")

// Cache is the persistence-port for the write-through session cache.
// Postgres remains the source of truth; this cache exists to short-circuit
// the most frequent session-lookup path (authenticated requests).
//
// The cache stores a minimal payload — only the canonical session id — keyed
// by the SHA-256 hash of the opaque cookie token. Callers receive the id and
// then load the full Session row from Postgres when they need expiry / user
// id / tenant. This deliberately keeps the cache schema flat and avoids the
// risk of stale projection drift if the Postgres row changes shape.
//
// Implementations MUST NOT mask transient failures as ErrCacheMiss. Callers
// distinguish "key absent" (ErrCacheMiss) from "cache unreachable" (any other
// error) and fall through to Postgres in either case, logging only the latter.
type Cache interface {
	// Set writes the (tokenHash -> sessionID) mapping with the given TTL.
	// TTL should be the remaining lifetime of the session (write-through on
	// create uses absolute TTL; refresh on miss uses remaining TTL).
	Set(ctx context.Context, tokenHash string, sessionID string, ttl time.Duration) error

	// Get retrieves the sessionID for a token hash. MUST return ErrCacheMiss
	// (not a generic error) on a clean miss.
	Get(ctx context.Context, tokenHash string) (sessionID string, err error)

	// Delete removes the (tokenHash -> sessionID) mapping. Idempotent: a
	// missing key MUST NOT be reported as an error.
	Delete(ctx context.Context, tokenHash string) error
}
