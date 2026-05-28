package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// http.ErrAbortHandler is the documented way to abort a
					// handler without logging — propagate it as-is.
					if rec == http.ErrAbortHandler {
						panic(rec)
					}
					logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
						slog.String("stack", string(debug.Stack())),
					)
					httperr.Write(w, httperr.Internal())
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
