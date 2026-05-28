package sign_out

import (
	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Input is the use case command for sign-out. Session id comes from the
// session middleware, not the request body.
type Input struct {
	TenantID  tenant.ID
	SessionID uuid.UUID
	UserID    uuid.UUID
}
