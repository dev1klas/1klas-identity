package middleware

import (
	"log/slog"
	"net/url"

	"github.com/valyala/fasthttp"

	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
)

// OriginCheck enforces the walking-skeleton CSRF mitigation defined in
// SPEC-identity (lines 48-49): SameSite=Lax cookies + Origin header check on
// every state-changing request.
//
// Behaviour:
//   - If the Origin header is present it MUST match one of allowed exactly
//     (scheme + host + [port]). Otherwise 403.
//   - If Origin is absent, fall back to the Referer header parsed as a URL;
//     the scheme+host+port of that URL MUST match one of allowed.
//   - If both Origin and Referer are absent, 403.
//
// The allow-list is exact-string-match; build entries via originString below.
// An empty allow-list is treated as misconfiguration and every request is
// rejected (the server's config loader should refuse to start in that case).
func OriginCheck(logger *slog.Logger, allowed []string) Middleware {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		if o != "" {
			set[o] = struct{}{}
		}
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Copy ctx-bytes off the pooled buffer immediately.
			path := string(ctx.Path())

			if len(set) == 0 {
				logger.WarnContext(ctx, "origin check rejected: empty allow-list",
					slog.String("path", path),
				)
				httperr.WriteFast(ctx, httperr.Forbidden())
				return
			}

			origin := string(ctx.Request.Header.Peek("Origin"))
			if origin != "" {
				if _, ok := set[origin]; !ok {
					logger.WarnContext(ctx, "origin check rejected",
						slog.String("origin", origin),
						slog.String("path", path),
					)
					httperr.WriteFast(ctx, httperr.Forbidden())
					return
				}
				next(ctx)
				return
			}

			referer := string(ctx.Request.Header.Peek("Referer"))
			if referer == "" {
				logger.WarnContext(ctx, "origin check rejected: missing Origin and Referer",
					slog.String("path", path),
				)
				httperr.WriteFast(ctx, httperr.Forbidden())
				return
			}

			refOrigin, ok := originFromReferer(referer)
			if !ok {
				logger.WarnContext(ctx, "origin check rejected: malformed Referer",
					slog.String("referer", referer),
					slog.String("path", path),
				)
				httperr.WriteFast(ctx, httperr.Forbidden())
				return
			}
			if _, ok := set[refOrigin]; !ok {
				logger.WarnContext(ctx, "origin check rejected via Referer",
					slog.String("referer_origin", refOrigin),
					slog.String("path", path),
				)
				httperr.WriteFast(ctx, httperr.Forbidden())
				return
			}

			next(ctx)
		}
	}
}

// originFromReferer parses a Referer URL and returns its origin
// (scheme://host[:port]). Returns false if the URL is malformed, lacks a
// scheme, or lacks a host.
func originFromReferer(referer string) (string, bool) {
	u, err := url.Parse(referer)
	if err != nil {
		return "", false
	}
	if u.Scheme == "" || u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}
