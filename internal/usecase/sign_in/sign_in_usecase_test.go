package sign_in_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/usecase/internal_testkit"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

func buildUC(t *testing.T) (*sign_in.UseCase, *internal_testkit.FakeUsers, *internal_testkit.FakeSessions) {
	t.Helper()
	users := internal_testkit.NewFakeUsers()
	sessions := internal_testkit.NewFakeSessions()
	out := internal_testkit.NewFakeOutbox()
	clk := &internal_testkit.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	hasher := &internal_testkit.FakeHasher{}
	tokens := &internal_testkit.FakeTokenGen{}

	// Seed a user by going through sign_up.
	suc := sign_up.New(internal_testkit.FakeUoW{}, users, sessions, out, hasher, tokens, clk, time.Hour)
	if _, err := suc.Execute(context.Background(), sign_up.Input{
		TenantID: tenant.DefaultID,
		Email:    "alice@example.com",
		Password: "correct horse battery",
	}); err != nil {
		t.Fatalf("seed sign-up: %v", err)
	}

	uc, err := sign_in.New(context.Background(), internal_testkit.FakeUoW{}, users, sessions, out, hasher, tokens, clk, time.Hour)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return uc, users, sessions
}

func TestSignIn_HappyPath(t *testing.T) {
	uc, _, _ := buildUC(t)
	got, err := uc.Execute(context.Background(), sign_in.Input{
		TenantID: tenant.DefaultID,
		Email:    "alice@example.com",
		Password: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.SessionToken == "" {
		t.Fatal("empty session token")
	}
}

func TestSignIn_WrongPassword(t *testing.T) {
	uc, _, _ := buildUC(t)
	_, err := uc.Execute(context.Background(), sign_in.Input{
		TenantID: tenant.DefaultID,
		Email:    "alice@example.com",
		Password: "wrong-password-but-long-enough",
	})
	if !errors.Is(err, sign_in.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestSignIn_UnknownEmail(t *testing.T) {
	uc, _, _ := buildUC(t)
	_, err := uc.Execute(context.Background(), sign_in.Input{
		TenantID: tenant.DefaultID,
		Email:    "bob@example.com",
		Password: "correct horse battery",
	})
	if !errors.Is(err, sign_in.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}
