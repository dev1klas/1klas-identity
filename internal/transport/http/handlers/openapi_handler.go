package handlers

import (
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/transport/apispec"
)

// OpenAPI serves the embedded OpenAPI 3.0 spec.
func OpenAPI(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(apispec.Spec)
}
