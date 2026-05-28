// Package middleware holds the per-request middleware for the identity HTTP
// surface. Composition follows the gateway pattern: Chain wraps innermost-last.
//
// Connection-pool safety (fasthttp): the *fasthttp.RequestCtx is taken from a
// sync.Pool and reused after the handler returns. Middleware (and handlers)
// MUST treat every []byte returned by methods on RequestCtx / Request as
// invalid the moment they hand control off to a goroutine — copy the bytes
// into a local string first. See access_log.go for the canonical example.
package middleware

import "github.com/valyala/fasthttp"

// Middleware is a function that wraps a fasthttp.RequestHandler.
type Middleware func(fasthttp.RequestHandler) fasthttp.RequestHandler

// Chain composes multiple middlewares into one. The first middleware in the
// slice is the outermost wrapper (executes first on request, last on response).
func Chain(ms ...Middleware) Middleware {
	return func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
		for i := len(ms) - 1; i >= 0; i-- {
			h = ms[i](h)
		}
		return h
	}
}
