package handlers

import (
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
)

// mustTenant pulls the tenant id off the request ctx. The tenancy middleware
// guarantees it is set, so a missing entry is a wiring bug — fall back to
// the default tenant rather than panicking (defensive).
func mustTenant(ctx *fasthttp.RequestCtx) tenant.ID {
	v := ctx.UserValue(middleware.UVTenantID)
	if v == nil {
		return tenant.DefaultID
	}
	if t, ok := v.(tenant.ID); ok {
		return t
	}
	return tenant.DefaultID
}

// userIDOrNil pulls the authenticated user id off the request ctx, or
// uuid.Nil if none is set (caller is responsible for treating that as 401).
func userIDOrNil(ctx *fasthttp.RequestCtx) uuid.UUID {
	v, _ := ctx.UserValue(middleware.UVUserID).(uuid.UUID)
	return v
}

// sessionIDOrNil pulls the authenticated session id off the request ctx.
func sessionIDOrNil(ctx *fasthttp.RequestCtx) uuid.UUID {
	v, _ := ctx.UserValue(middleware.UVSessionID).(uuid.UUID)
	return v
}

// tokenHashHex returns the hex token hash stashed by the Session middleware.
func tokenHashHex(ctx *fasthttp.RequestCtx) string {
	v, _ := ctx.UserValue(middleware.UVTokenHashHex).(string)
	return v
}
