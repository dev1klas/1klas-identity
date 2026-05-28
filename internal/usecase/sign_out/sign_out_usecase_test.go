package sign_out_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
	"github.com/dev1klas/1klas-identity/internal/usecase/internal_testkit"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"

	"github.com/google/uuid"
)

func TestSignOut_RevokesAndEmits(t *testing.T) {
	sessions := internal_testkit.NewFakeSessions()
	out := internal_testkit.NewFakeOutbox()
	cache := internal_testkit.NewFakeCache()
	clk := &internal_testkit.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	silent := slog.New(slog.NewJSONHandler(io.Discard, nil))

	uid := uuid.New()
	sid := uuid.New()
	s := session.New(sid, tenant.DefaultID, uid, []byte("hash"), clk.T, clk.T.Add(time.Hour))
	_ = sessions.SaveTx(context.Background(), user.NewTx(struct{}{}), s)

	// Seed cache so we can confirm the delete fires.
	tokenHashHex := "deadbeefcafebabe"
	_ = cache.Set(context.Background(), tokenHashHex, session.CachedSession{
		SessionID: sid,
		UserID:    uid,
		TenantID:  tenant.DefaultID,
		ExpiresAt: clk.T.Add(time.Hour),
	}, time.Hour)

	uc := sign_out.New(internal_testkit.FakeUoW{}, sessions, out, cache, clk, silent)
	if _, err := uc.Execute(context.Background(), sign_out.Input{
		TenantID:     tenant.DefaultID,
		SessionID:    sid,
		UserID:       uid,
		TokenHashHex: tokenHashHex,
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(out.Events) != 1 || out.Events[0].EventType() != outbox.TopicSessionRevoked {
		t.Fatalf("expected one revoked event, got %v", out.Events)
	}
	if cache.Has(tokenHashHex) {
		t.Fatal("expected cache entry to be deleted after sign-out")
	}
}

// TestSignOut_CacheFailureIsNonFatal asserts that a failing cache.Delete does
// not break the sign-out flow; the Postgres revocation is the source of truth.
func TestSignOut_CacheFailureIsNonFatal(t *testing.T) {
	sessions := internal_testkit.NewFakeSessions()
	out := internal_testkit.NewFakeOutbox()
	cache := internal_testkit.NewFakeCache()
	cache.FailDel = true
	clk := &internal_testkit.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	silent := slog.New(slog.NewJSONHandler(io.Discard, nil))

	uid := uuid.New()
	sid := uuid.New()
	s := session.New(sid, tenant.DefaultID, uid, []byte("hash"), clk.T, clk.T.Add(time.Hour))
	_ = sessions.SaveTx(context.Background(), user.NewTx(struct{}{}), s)

	uc := sign_out.New(internal_testkit.FakeUoW{}, sessions, out, cache, clk, silent)
	if _, err := uc.Execute(context.Background(), sign_out.Input{
		TenantID:     tenant.DefaultID,
		SessionID:    sid,
		UserID:       uid,
		TokenHashHex: "deadbeef",
	}); err != nil {
		t.Fatalf("execute must not fail on cache.Delete error: %v", err)
	}
}
