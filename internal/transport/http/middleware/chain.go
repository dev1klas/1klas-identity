// Package middleware holds the per-request middleware for the identity HTTP
// surface. Composition follows the gateway pattern: Chain wraps innermost-last.
package middleware

import "net/http"

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain composes multiple middlewares into one. The first middleware in the
// slice is the outermost wrapper (executes first on request, last on response).
func Chain(ms ...Middleware) Middleware {
	return func(h http.Handler) http.Handler {
		for i := len(ms) - 1; i >= 0; i-- {
			h = ms[i](h)
		}
		return h
	}
}
