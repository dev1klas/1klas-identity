package outbox

import (
	"context"

	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// Repository is the persistence port for outbox events. Writes MUST happen
// inside the same pgx.Tx as the state change being recorded.
type Repository interface {
	WriteTx(ctx context.Context, tx user.Tx, ev Event) error
}
