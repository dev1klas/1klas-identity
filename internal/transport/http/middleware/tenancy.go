package middleware

import (
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Tenancy injects the default single-tenant ID into the request context.
// In a multi-tenant world this would resolve tenant from host header,
// JWT claim, or an internal header from the gateway.
func Tenancy(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.SetUserValue(UVTenantID, tenant.DefaultID)
		next(ctx)
	}
}
