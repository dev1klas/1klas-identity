// Package middleware AccessLog — connection-pool safety notes:
//
// fasthttp.RequestCtx is taken from a sync.Pool and reused after the handler
// returns. Methods on ctx.Request / ctx.QueryArgs() return []byte slices that
// alias the pooled buffer. Capturing those slices into a goroutine, a
// defer-closure that outlives the handler, or any structure persisted past
// the handler return is a USE-AFTER-FREE bug.
//
// Rule for any future maintainer adding to this file:
//
//  1. Read the primitives you need (method, path, request_id, client_ip,
//     user_agent) at the TOP of the middleware, coerce them to string
//     immediately so the bytes are copied off the pooled buffer.
//  2. Log them at the bottom by referencing those LOCAL string vars.
//  3. NEVER capture ctx itself in a goroutine. NEVER hold a slice returned
//     by ctx.Request.Header.Peek / ctx.Path / ctx.RemoteAddr past the
//     handler boundary.
package middleware

import (
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

// AccessLog emits one JSON line per request with stable fields. Never logs
// PII; only references user_id.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()

			// Copy primitives off the pooled buffer immediately. These local
			// strings are detached from RequestCtx and safe to use anywhere
			// downstream — including after next(ctx) returns.
			method := string(ctx.Method())
			path := string(ctx.Path())
			userAgent := string(ctx.Request.Header.UserAgent())
			clientIPStr := clientIP(ctx)

			next(ctx)

			// User-values come from middleware that already copied strings
			// (RequestID stores a string; Session stores typed uuid.UUID).
			// Safe to read post-handler.
			status := ctx.Response.StatusCode()

			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}

			reqID, _ := ctx.UserValue(UVRequestID).(string)

			tenantField := ""
			if t := ctxTenantString(ctx); t != "" {
				tenantField = t
			}

			userIDField := ""
			if uid := ctxUserIDString(ctx); uid != "" && uid != "00000000-0000-0000-0000-000000000000" {
				userIDField = uid
			}

			logger.LogAttrs(ctx, level, "request",
				slog.String("request_id", reqID),
				slog.String("tenant_id", tenantField),
				slog.String("user_id", userIDField),
				slog.String("method", method),
				slog.String("path", path),
				slog.Int("status", status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("client_ip", clientIPStr),
				slog.String("user_agent", userAgent),
			)
		}
	}
}

// clientIP picks the public-most IP. Honours X-Forwarded-For when present
// (gateway prepends it); falls back to ctx.RemoteAddr().String().
//
// Returns a freshly-allocated string — safe to pass anywhere.
func clientIP(ctx *fasthttp.RequestCtx) string {
	if xff := string(ctx.Request.Header.Peek("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	addr := ctx.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// ctxTenantString returns the stringified tenant id stored on the ctx, or "".
func ctxTenantString(ctx *fasthttp.RequestCtx) string {
	v := ctx.UserValue(UVTenantID)
	if v == nil {
		return ""
	}
	type stringer interface{ String() string }
	if s, ok := v.(stringer); ok {
		return s.String()
	}
	return ""
}

// ctxUserIDString returns the stringified user id stored on the ctx, or "".
func ctxUserIDString(ctx *fasthttp.RequestCtx) string {
	v := ctx.UserValue(UVUserID)
	if v == nil {
		return ""
	}
	type stringer interface{ String() string }
	if s, ok := v.(stringer); ok {
		return s.String()
	}
	return ""
}
