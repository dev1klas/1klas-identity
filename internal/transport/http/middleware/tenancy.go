package middleware

import (
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Tenancy injects the default single-tenant ID into the request context.
// In a multi-tenant world this would resolve tenant from host header,
// JWT claim, or an internal header from the gateway.
func Tenancy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithTenantID(r.Context(), tenant.DefaultID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
