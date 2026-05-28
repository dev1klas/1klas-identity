package sign_up_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/usecase/internal_testkit"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

func newUC(t *testing.T) (*sign_up.UseCase, *internal_testkit.FakeUsers, *internal_testkit.FakeSessions, *internal_testkit.FakeOutbox, *internal_testkit.FakeCache) {
	t.Helper()
	users := internal_testkit.NewFakeUsers()
	sessions := internal_testkit.NewFakeSessions()
	out := internal_testkit.NewFakeOutbox()
	cache := internal_testkit.NewFakeCache()
	clk := &internal_testkit.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	silent := slog.New(slog.NewJSONHandler(io.Discard, nil))
	uc := sign_up.New(
		internal_testkit.FakeUoW{},
		users, sessions, out, cache,
		&internal_testkit.FakeHasher{},
		&internal_testkit.FakeTokenGen{},
		clk,
		7*24*time.Hour,
		silent,
	)
	return uc, users, sessions, out, cache
}

func TestSignUp_HappyPath(t *testing.T) {
	uc, users, sessions, out, cache := newUC(t)
	ctx := context.Background()

	got, err := uc.Execute(ctx, sign_up.Input{
		TenantID: tenant.DefaultID,
		Email:    "alice@example.com",
		Password: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.UserID.String() == "" || got.SessionToken == "" {
		t.Fatal("missing output fields")
	}
	if len(users.NewListIDs()) != 1 {
		t.Fatalf("want 1 user, got %d", len(users.NewListIDs()))
	}
	if len(sessions.NewListIDs()) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions.NewListIDs()))
	}
	if len(out.Events) != 2 {
		t.Fatalf("want 2 outbox events, got %d", len(out.Events))
	}
	if out.Events[0].EventType() != outbox.TopicUserCreated {
		t.Fatalf("first event = %q", out.Events[0].EventType())
	}
	if out.Events[1].EventType() != outbox.TopicSessionCreated {
		t.Fatalf("second event = %q", out.Events[1].EventType())
	}
	if cache.SetCalls != 1 {
		t.Fatalf("want 1 cache Set after sign-up, got %d", cache.SetCalls)
	}
}

func TestSignUp_EmailTaken(t *testing.T) {
	uc, _, _, _, _ := newUC(t)
	ctx := context.Background()
	in := sign_up.Input{TenantID: tenant.DefaultID, Email: "a@b.com", Password: "correct horse battery"}
	if _, err := uc.Execute(ctx, in); err != nil {
		t.Fatalf("first sign-up: %v", err)
	}
	_, err := uc.Execute(ctx, in)
	if !errors.Is(err, sign_up.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func TestSignUp_InvalidEmail(t *testing.T) {
	uc, _, _, _, _ := newUC(t)
	_, err := uc.Execute(context.Background(), sign_up.Input{
		TenantID: tenant.DefaultID,
		Email:    "not-an-email",
		Password: "correct horse battery",
	})
	if !errors.Is(err, sign_up.ErrInvalidEmail) {
		t.Fatalf("want ErrInvalidEmail, got %v", err)
	}
}

func TestSignUp_WeakPassword(t *testing.T) {
	uc, _, _, _, _ := newUC(t)
	_, err := uc.Execute(context.Background(), sign_up.Input{
		TenantID: tenant.DefaultID,
		Email:    "a@b.com",
		Password: "short",
	})
	if !errors.Is(err, sign_up.ErrWeakPassword) {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
}

// TestSignUp_CacheFailureIsNonFatal asserts that the use case returns OK even
// when the write-through cache.Set fails — Postgres is the source of truth.
func TestSignUp_CacheFailureIsNonFatal(t *testing.T) {
	uc, _, _, _, cache := newUC(t)
	cache.FailSet = true
	got, err := uc.Execute(context.Background(), sign_up.Input{
		TenantID: tenant.DefaultID,
		Email:    "alice@example.com",
		Password: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("execute should not fail on cache.Set error: %v", err)
	}
	if got.SessionToken == "" {
		t.Fatal("missing session token")
	}
}
