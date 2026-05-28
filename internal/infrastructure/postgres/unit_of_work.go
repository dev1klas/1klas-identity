// Package postgres holds the pgx-backed implementations of the domain ports.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// UnitOfWork begins/commits/rolls back pgx transactions.
type UnitOfWork struct {
	pool *pgxpool.Pool
}

// NewUnitOfWork returns a UnitOfWork backed by the pool.
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

// Begin opens a transaction. Returned token MUST be passed to Commit or
// Rollback exactly once.
func (u *UnitOfWork) Begin(ctx context.Context) (user.Tx, error) {
	t, err := u.pool.Begin(ctx)
	if err != nil {
		return user.Tx{}, err
	}
	return user.NewTx(t), nil
}

// Commit commits the transaction.
func (u *UnitOfWork) Commit(ctx context.Context, t user.Tx) error {
	pgxT, err := txOf(t)
	if err != nil {
		return err
	}
	return pgxT.Commit(ctx)
}

// Rollback aborts the transaction. Idempotent — silent on already-committed.
func (u *UnitOfWork) Rollback(ctx context.Context, t user.Tx) error {
	pgxT, err := txOf(t)
	if err != nil {
		return err
	}
	err = pgxT.Rollback(ctx)
	if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return err
	}
	return nil
}

// txOf extracts pgx.Tx from a user.Tx token.
func txOf(t user.Tx) (pgx.Tx, error) {
	inner := user.InnerTx(t)
	if inner == nil {
		return nil, errors.New("postgres: empty tx token")
	}
	pgxT, ok := inner.(pgx.Tx)
	if !ok {
		return nil, errors.New("postgres: foreign tx token")
	}
	return pgxT, nil
}
