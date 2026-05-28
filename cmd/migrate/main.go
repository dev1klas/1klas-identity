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
//
// Reads POSTGRES_URL from env; exits non-zero on any failure so the
// pre-deploy job blocks the release on a migration error.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
)

const migrationsDir = "migrations"

func main() {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		log.Fatal("POSTGRES_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	// Source migrations from the embedded FS shared with the server
	// binary, so a single source-of-truth ships everywhere.
	goose.SetBaseFS(postgres.MigrationsFS())
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("setting goose dialect: %v", err)
	}

	command, args := parseArgs(os.Args[1:])

	if err := goose.RunContext(ctx, command, db, migrationsDir, args...); err != nil {
		log.Fatalf("goose %s: %v", command, err)
	}
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
