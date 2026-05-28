//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
)

func TestUserRepository_CRUD(t *testing.T) {
	t.Parallel()

	pool, cleanup := startPostgres(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	uow := postgres.NewUnitOfWork(pool)
	users := postgres.NewUserRepository(pool)

	em, err := user.NewEmail("alice@example.com")
	if err != nil {
		t.Fatalf("email: %v", err)
	}
	u := user.New(uuid.New(), tenant.DefaultID, em, user.NewPasswordHash("hashvalue"), time.Now().UTC())

	tx, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := users.SaveTx(ctx, tx, u); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := uow.Commit(ctx, tx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got, err := users.FindByEmail(ctx, tenant.DefaultID, em)
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if got.ID() != u.ID() {
		t.Fatalf("id mismatch: got %s want %s", got.ID(), u.ID())
	}

	// Duplicate insert must surface ErrEmailTaken.
	tx2, err := uow.Begin(ctx)
	if err != nil {
		t.Fatalf("begin2: %v", err)
	}
	dup := user.New(uuid.New(), tenant.DefaultID, em, user.NewPasswordHash("other"), time.Now().UTC())
	err = users.SaveTx(ctx, tx2, dup)
	_ = uow.Rollback(ctx, tx2)
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}
