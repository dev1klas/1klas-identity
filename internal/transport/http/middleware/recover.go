package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/valyala/fasthttp"

	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
)

// Recover catches any panic raised by downstream middleware or handlers, logs
// the failure with a stack trace at ERROR level, and returns a 500 to the
// client. Wire as the OUTERMOST middleware so it covers every other layer in
// the chain, including AccessLog itself.
//
// The recovered value is logged but never echoed to the client (panic
// messages can leak internals).
func Recover(logger *slog.Logger) Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			defer func() {
				if rec := recover(); rec != nil {
					// Copy ctx-derived strings into locals before logging:
					// the recover branch sits inside a deferred fn so we are
					// still on the synchronous request path, but copying is
					// the cheap, defensive habit.
					path := string(ctx.Path())
					method := string(ctx.Method())
					logger.LogAttrs(ctx, slog.LevelError, "panic recovered",
						slog.Any("panic", rec),
						slog.String("path", path),
						slog.String("method", method),
						slog.String("stack", string(debug.Stack())),
					)
					httperr.WriteFast(ctx, httperr.Internal())
				}
			}()
			next(ctx)
		}
	}
}
