package user

import (
	"context"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Tx is an opaque transaction handle. The concrete pgx.Tx is wrapped in
// infrastructure; domain only ever forwards the value to repository methods.
//
// To preserve the layering rule (no bare empty-interface in domain), Tx is
// a concrete struct holding an unexported value. Only infrastructure can
// read the inner value, via NewTx + InnerTx.
type Tx struct {
	inner any
}

// NewTx is a constructor used only by the infrastructure layer to wrap a
// concrete transaction (e.g. pgx.Tx) as a domain Tx. It is exported because
// Go has no friend-package mechanism; callers outside infrastructure MUST
// NOT use it.
func NewTx(inner any) Tx { return Tx{inner: inner} }

// InnerTx returns the wrapped value. Infrastructure-only.
func InnerTx(t Tx) any { return t.inner }

// Repository is the persistence port for the User aggregate.
type Repository interface {
	// SaveTx persists a brand-new User inside the given transaction.
	// MUST return ErrEmailTaken on a (tenant_id, email) unique violation.
	SaveTx(ctx context.Context, tx Tx, u User) error

	// FindByEmail looks up a user by tenant + normalised email.
	// MUST return ErrUserNotFound if absent.
	FindByEmail(ctx context.Context, t tenant.ID, email Email) (User, error)

	// FindByID looks up a user by primary key, scoped to tenant.
	// MUST return ErrUserNotFound if absent.
	FindByID(ctx context.Context, t tenant.ID, id uuid.UUID) (User, error)
}
