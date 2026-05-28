// Package domain exposes the cross-aggregate Unit of Work abstraction.
//
// The Unit of Work is the only way a use case opens a database transaction
// without importing pgx directly. Concrete implementation lives in
// internal/infrastructure/postgres.
package domain

import (
	"context"

	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// UnitOfWork begins a database transaction and gives use cases a Tx token to
// pass to repositories. Use case code looks like:
//
//	tx, err := uow.Begin(ctx)
//	if err != nil { return out, err }
//	defer func() { _ = uow.Rollback(ctx, tx) }()
//	... repo calls using tx ...
//	if err := uow.Commit(ctx, tx); err != nil { return out, err }
type UnitOfWork interface {
	Begin(ctx context.Context) (user.Tx, error)
	Commit(ctx context.Context, tx user.Tx) error
	Rollback(ctx context.Context, tx user.Tx) error
}
