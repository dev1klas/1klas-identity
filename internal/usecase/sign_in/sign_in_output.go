package sign_in

import (
	"time"

	"github.com/google/uuid"
)

// Output is what the use case returns on success.
type Output struct {
	UserID           uuid.UUID
	SessionToken     string
	SessionExpiresAt time.Time
}
