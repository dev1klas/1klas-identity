// Package main is the goose migration runner. It is built into a
// standalone binary (bin/migrate) and invoked as a DO App Platform
// pre-deploy job so each deployment applies pending migrations
// atomically before the server starts. The migrations directory is
// embedded into the binary so the runtime image needs no extra files.
//
// Usage:
//
//	migrate              # equivalent to "migrate up"
//	migrate up
//	migrate up-to <ver>
//	migrate down
//	migrate status
//	migrate version
//	migrate -h | --help  # print this usage to stderr and exit 0
//
// Reads POSTGRES_URL from env; exits non-zero on any failure so the
// pre-deploy job blocks the release on a migration error. Operations
// are bounded by a 5-minute deadline so a hung migration tears the
// pre-deploy job down rather than blocking the deployment forever.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
)

const (
	migrationsDir  = "migrations"
	migrateTimeout = 5 * time.Minute

	// schemaName is the Postgres schema owned by the identity_app role.
	// The identity_app user does NOT have CREATE on schema public (DO
	// managed Postgres locks public down to the cluster admin), so all
	// objects — including goose's bookkeeping table — live under this
	// schema.
	schemaName = "identity"

	// gooseVersionTable is the fully-qualified name of goose's version
	// table. Setting it here keeps goose entirely out of the public
	// schema, which the runtime DB user has no privileges on.
	gooseVersionTable = schemaName + ".goose_db_version"
)

// usage describes the supported subcommands. Printed on -h / --help and on
// any flag-parse-style error before delegating to goose.
const usage = `migrate — 1klas-identity migration runner

Usage:
  migrate              # equivalent to "migrate up"
  migrate up           # apply all pending migrations
  migrate up-to <ver>  # apply through the given version
  migrate down         # roll back the most recent migration
  migrate status       # print applied / pending state
  migrate version      # print current schema version

Env:
  POSTGRES_URL  required, pgx-compatible DSN

Exits non-zero on any failure (so DO pre-deploy job blocks the release).
A 5-minute deadline is applied to the whole operation.
`

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil)).With(
		"service", "1klas-identity",
		"component", "migrate",
	)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			fmt.Fprint(os.Stderr, usage)
			return
		}
	}

	if err := run(logger); err != nil {
		logger.Error("migration failed", "error", err.Error())
		os.Exit(1)
	}
}

// run encapsulates the migration logic so main() can keep a single exit
// point and the logger / context wiring is testable in isolation.
func run(logger *slog.Logger) error {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		return errors.New("POSTGRES_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Bound the whole migrate operation. If goose hangs (lock contention on
	// goose_db_version, slow DDL, network drop) the pre-deploy job must
	// surface failure rather than block the deployment indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	// Bootstrap the identity schema before goose touches the version table.
	// The version table is qualified into this schema below, so the schema
	// must exist on the very first run. CREATE SCHEMA IF NOT EXISTS is
	// idempotent and safe on every subsequent migrate.
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+schemaName); err != nil {
		return fmt.Errorf("bootstrapping schema %q: %w", schemaName, err)
	}

	// Source migrations from the embedded FS shared with the server binary,
	// so a single source-of-truth ships everywhere.
	goose.SetBaseFS(postgres.MigrationsFS())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}
	// Keep goose's bookkeeping out of the public schema (the runtime DB
	// user has no privileges there in DO managed Postgres).
	goose.SetTableName(gooseVersionTable)

	command, args := parseArgs(os.Args[1:])
	logger.Info("starting migration", "command", command, "args", args, "timeout", migrateTimeout.String())

	if err := goose.RunContext(ctx, command, db, migrationsDir, args...); err != nil {
		return fmt.Errorf("goose %s: %w", command, err)
	}

	logger.Info("migration completed", "command", command)
	return nil
}

// parseArgs picks the command and forwards any remaining positional args to
// goose (e.g. "up-to 20260528000003"). Defaults to "up" when no command is
// supplied, which is the pre-deploy job path.
func parseArgs(in []string) (string, []string) {
	if len(in) == 0 {
		return "up", nil
	}
	cmd := in[0]
	if cmd == "" {
		return "up", nil
	}
	return cmd, append([]string(nil), in[1:]...)
}
