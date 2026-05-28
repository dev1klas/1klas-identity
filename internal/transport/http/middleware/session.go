package middleware

import (
	"errors"
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	httpcookies "github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
)

// Session resolves the inbound session cookie to a (user_id, session_id) pair
// and injects them + tenant_id into the request context. Returns 401 with the
// appropriate stable error code if the cookie is missing, malformed, expired,
// or revoked.
func Session(repo session.Repository) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := httpcookies.ReadSessionCookie(r)
			if !ok || raw == "" {
				httperr.Write(w, httperr.SessionRequired())
				return
			}

			hash := session.HashOf(raw)
			s, err := repo.FindByTokenHash(r.Context(), hash)
			if err != nil {
				if errors.Is(err, session.ErrSessionNotFound) {
					httperr.Write(w, httperr.SessionInvalid())
					return
				}
				httperr.Write(w, httperr.Internal())
				return
			}
			if !s.IsActive(nowFn()) {
				httperr.Write(w, httperr.SessionInvalid())
				return
			}

			ctx := WithUserID(r.Context(), s.UserID())
			ctx = WithSessionID(ctx, s.ID())
			ctx = WithTenantID(ctx, s.TenantID())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
