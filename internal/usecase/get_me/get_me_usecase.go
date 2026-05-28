package get_me

import (
	"context"
	"errors"

	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// UseCase reads the current user's projection.
type UseCase struct {
	users user.Repository
}

// New constructs the use case.
func New(users user.Repository) *UseCase {
	return &UseCase{users: users}
}

// Execute runs the use case.
func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.TenantID.IsZero() {
		return Output{}, ErrInternal
	}

	u, err := uc.users.FindByID(ctx, in.TenantID, in.UserID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return Output{}, ErrNotFound
		}
		return Output{}, ErrInternal
	}

	return Output{
		UserID:    u.ID(),
		Email:     u.Email().String(),
		Status:    string(u.Status()),
		CreatedAt: u.CreatedAt(),
	}, nil
}
