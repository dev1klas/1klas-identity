package middleware

import (
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

const headerRequestID = "X-Request-Id"

// RequestID preserves or generates an X-Request-Id header.
func RequestID(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Copy the inbound header value into a local string. fasthttp returns
		// a []byte aliasing the pooled buffer; capturing it via string()
		// detaches our copy from the pool.
		id := string(ctx.Request.Header.Peek(headerRequestID))
		if id == "" {
			id = uuid.New().String()
		}
		ctx.SetUserValue(UVRequestID, id)
		ctx.Response.Header.Set(headerRequestID, id)
		next(ctx)
	}
}
