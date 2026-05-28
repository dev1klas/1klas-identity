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

// Session resolves the inbound session cookie to a (user_id, session_id) pair
// and injects them + tenant_id into the request context.
//
// Lookup order:
//  1. Valkey cache (single hash key -> sessionID). On hit we still consult
//     Postgres to load expiry / user_id / tenant_id authoritatively.
//  2. Postgres SELECT by token_hash. On hit we re-populate Valkey with the
//     REMAINING TTL (so a re-populated entry never outlives the session).
//
// Cache unavailability is never fatal — a logged WARN and a Postgres
// fallthrough is the documented policy.
//
// Returns 401 with the appropriate stable error code if the cookie is
// missing, malformed, expired, or revoked.
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

			// Cache probe — best effort. On miss / unavailable, fall through
			// to Postgres. On hit, we still need expiry and user/tenant from
			// Postgres, so the round-trip pattern is:
			//   cache.Get -> Postgres SELECT -> validate -> (re)populate cache
			cacheHit := false
			if cache != nil {
				_, err := cache.Get(ctx, tokenHashHex)
				switch {
				case err == nil:
					cacheHit = true
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

			// Re-populate the cache on a miss using the REMAINING TTL —
			// guarantees the cache entry never outlives the session row.
			if cache != nil && !cacheHit {
				remaining := time.Until(s.ExpiresAt())
				if remaining > 0 {
					if err := cache.Set(ctx, tokenHashHex, s.ID().String(), remaining); err != nil {
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
