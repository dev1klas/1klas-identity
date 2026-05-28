package get_me

import (
	"time"

	"github.com/google/uuid"
)

// Output is the projection returned for the current user.
type Output struct {
	UserID    uuid.UUID
	Email     string
	Status    string
	CreatedAt time.Time
}
