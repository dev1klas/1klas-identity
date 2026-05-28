package middleware

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	httpcookies "github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
)

// Session resolves the inbound session cookie to a (user_id, session_id,
// tenant_id) triple and injects them into the request context.
//
// Lookup order (per ADR-0008):
//  1. Valkey cache holds the FULL session payload — sessionID + userID +
//     tenantID + expiresAt. A cache hit with a non-expired entry serves the
//     request WITHOUT a Postgres round-trip. This is the hot path.
//  2. On ErrCacheMiss, or on a cached-but-expired entry, fall through to
//     Postgres SELECT by token_hash. On hit we re-populate Valkey with the
//     REMAINING TTL (time.Until(s.ExpiresAt())) so a re-populated entry never
//     outlives the session row.
//  3. On any OTHER cache error (transient infra outage), log WARN and fall
//     through to Postgres — auth is NEVER blocked on cache outage.
//
// Returns 401 with the appropriate stable error code if the cookie is
// missing, malformed, expired (per Postgres), or revoked.
func Session(repo session.Repository, cache session.Cache, logger *slog.Logger) Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			raw, ok := httpcookies.ReadSessionCookieFast(ctx)
			if !ok || raw == "" {
				httperr.WriteFast(ctx, httperr.SessionRequired())
				return
			}

			hash := session.HashOf(raw)
			tokenHashHex := hex.EncodeToString(hash)

			// Cache probe — the hot path. A hit on a non-expired entry skips
			// Postgres entirely (this is the whole point of ADR-0008).
			if cache != nil {
				cached, err := cache.Get(ctx, tokenHashHex)
				switch {
				case err == nil:
					now := nowFn()
					if cached.ExpiresAt.After(now) {
						// True hot path: serve from cache.
						ctx.SetUserValue(UVUserID, cached.UserID)
						ctx.SetUserValue(UVSessionID, cached.SessionID)
						ctx.SetUserValue(UVTenantID, cached.TenantID)
						ctx.SetUserValue(UVTokenHashHex, tokenHashHex)
						next(ctx)
						return
					}
					// Cached entry exists but is past its self-reported
					// expiry — treat as stale and fall through to Postgres,
					// which is the authoritative source of expiry/revocation.
				case errors.Is(err, session.ErrCacheMiss):
					// expected: fall through to Postgres
				default:
					logger.WarnContext(ctx, "session cache get failed; falling through to postgres",
						slog.String("err", err.Error()),
					)
				}
			}

			s, err := repo.FindByTokenHash(ctx, hash)
			if err != nil {
				if errors.Is(err, session.ErrSessionNotFound) {
					httperr.WriteFast(ctx, httperr.SessionInvalid())
					return
				}
				httperr.WriteFast(ctx, httperr.Internal())
				return
			}
			now := nowFn()
			if !s.IsActive(now) {
				httperr.WriteFast(ctx, httperr.SessionInvalid())
				return
			}

			// Re-populate the cache using the REMAINING TTL so the cache
			// entry never outlives the session row.
			if cache != nil {
				remaining := time.Until(s.ExpiresAt())
				if remaining > 0 {
					payload := session.CachedSession{
						SessionID: s.ID(),
						UserID:    s.UserID(),
						TenantID:  s.TenantID(),
						ExpiresAt: s.ExpiresAt(),
					}
					if err := cache.Set(ctx, tokenHashHex, payload, remaining); err != nil {
						logger.WarnContext(ctx, "session cache repopulate failed",
							slog.String("err", err.Error()),
						)
					}
				}
			}

			ctx.SetUserValue(UVUserID, s.UserID())
			ctx.SetUserValue(UVSessionID, s.ID())
			ctx.SetUserValue(UVTenantID, s.TenantID())
			ctx.SetUserValue(UVTokenHashHex, tokenHashHex)
			next(ctx)
		}
	}
}

// SessionUserID extracts the authenticated user id stored by the Session
// middleware. uuid.Nil if missing.
func SessionUserID(ctx *fasthttp.RequestCtx) uuid.UUID {
	v, _ := ctx.UserValue(UVUserID).(uuid.UUID)
	return v
}

// SessionID extracts the authenticated session id. uuid.Nil if missing.
func SessionID(ctx *fasthttp.RequestCtx) uuid.UUID {
	v, _ := ctx.UserValue(UVSessionID).(uuid.UUID)
	return v
}

// SessionTokenHashHex extracts the hex-encoded SHA-256 of the session token,
// stashed by the Session middleware so use cases (sign-out) can invalidate
// the cache.
func SessionTokenHashHex(ctx *fasthttp.RequestCtx) string {
	v, _ := ctx.UserValue(UVTokenHashHex).(string)
	return v
}
