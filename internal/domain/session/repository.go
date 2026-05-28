package session

import (
	"context"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// Repository is the persistence port for the Session aggregate.
type Repository interface {
	// SaveTx persists a new Session inside the given transaction.
	SaveTx(ctx context.Context, tx user.Tx, s Session) error

	// FindByTokenHash loads a session by its SHA-256 token hash. Tenant is
	// not required because the token hash is globally unique and high-entropy;
	// the returned Session carries its tenant for downstream checks.
	// MUST return ErrSessionNotFound if absent.
	FindByTokenHash(ctx context.Context, tokenHash []byte) (Session, error)

	// RevokeTx marks the session id as revoked. Idempotent: returns nil if
	// the session is already revoked. MUST return ErrSessionNotFound if no
	// such row.
	RevokeTx(ctx context.Context, tx user.Tx, t tenant.ID, id uuid.UUID) error
}
