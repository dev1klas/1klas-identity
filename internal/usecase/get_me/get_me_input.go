package get_me

import (
	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Input is the use case query for /profile/me.
type Input struct {
	TenantID tenant.ID
	UserID   uuid.UUID
}
