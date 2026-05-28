package postgres

import (
	"context"
	"embed"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationsFS returns the embedded migrations as an fs.FS so external
// callers (notably cmd/migrate) can share a single source-of-truth with
// the server-side Migrate helper.
func MigrationsFS() fs.FS { return migrationsFS }

// Migrate runs all pending goose migrations against the pool's database.
//
// In production migrations are applied by the cmd/migrate pre-deploy job
// before the server starts. This helper remains available for local
// developer setups and test bootstraps where running a separate binary
// is friction.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, "migrations")
}
