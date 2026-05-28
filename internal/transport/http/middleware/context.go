package middleware

import (
	"context"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	tenantIDKey
	userIDKey
	sessionIDKey
)

// WithRequestID stores a request ID in ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFrom retrieves the request ID. Empty string if unset.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithTenantID stores a TenantID in ctx.
func WithTenantID(ctx context.Context, t tenant.ID) context.Context {
	return context.WithValue(ctx, tenantIDKey, t)
}

// TenantIDFrom retrieves the tenant. Returns DefaultID if unset.
func TenantIDFrom(ctx context.Context) tenant.ID {
	t, ok := ctx.Value(tenantIDKey).(tenant.ID)
	if !ok {
		return tenant.DefaultID
	}
	return t
}

// WithUserID stores the authenticated user id in ctx.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFrom retrieves the authenticated user id. uuid.Nil if unset.
func UserIDFrom(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey).(uuid.UUID)
	return id
}

// WithSessionID stores the authenticated session id in ctx.
func WithSessionID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// SessionIDFrom retrieves the authenticated session id. uuid.Nil if unset.
func SessionIDFrom(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(sessionIDKey).(uuid.UUID)
	return id
}
