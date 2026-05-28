package get_me_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
	"github.com/dev1klas/1klas-identity/internal/usecase/internal_testkit"
)

func TestGetMe_HappyPath(t *testing.T) {
	repo := internal_testkit.NewFakeUsers()
	id := uuid.New()
	em, _ := user.NewEmail("alice@example.com")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u := user.New(id, tenant.DefaultID, em, user.NewPasswordHash("x"), now)
	if err := repo.SaveTx(context.Background(), user.NewTx(struct{}{}), u); err != nil {
		t.Fatalf("seed: %v", err)
	}

	uc := get_me.New(repo)
	got, err := uc.Execute(context.Background(), get_me.Input{TenantID: tenant.DefaultID, UserID: id})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
	if got.Status != "active" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestGetMe_CrossTenantNotFound(t *testing.T) {
	repo := internal_testkit.NewFakeUsers()
	id := uuid.New()
	em, _ := user.NewEmail("alice@example.com")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	other := tenant.MustParse("00000000-0000-0000-0000-000000000002")
	u := user.New(id, other, em, user.NewPasswordHash("x"), now)
	if err := repo.SaveTx(context.Background(), user.NewTx(struct{}{}), u); err != nil {
		t.Fatalf("seed: %v", err)
	}

	uc := get_me.New(repo)
	_, err := uc.Execute(context.Background(), get_me.Input{TenantID: tenant.DefaultID, UserID: id})
	if !errors.Is(err, get_me.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
