package middleware_test

import (
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
	"github.com/dev1klas/1klas-identity/internal/usecase/internal_testkit"
)

// instrumentedRepo wraps the FakeSessions repo to count FindByTokenHash hits.
type instrumentedRepo struct {
	*internal_testkit.FakeSessions
	finds int32
}

func (r *instrumentedRepo) FindByTokenHash(ctx context.Context, h []byte) (session.Session, error) {
	atomic.AddInt32(&r.finds, 1)
	return r.FakeSessions.FindByTokenHash(ctx, h)
}

func seedSession(t *testing.T, repo session.Repository, token string) (session.Session, string) {
	t.Helper()
	sid := uuid.New()
	uid := uuid.New()
	now := time.Now().UTC()
	hash := session.HashOf(token)
	s := session.New(sid, tenant.DefaultID, uid, hash, now, now.Add(time.Hour))
	if err := repo.SaveTx(context.Background(), user.NewTx(struct{}{}), s); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	return s, hex.EncodeToString(hash)
}

// TestSession_MissingCookie_401 covers the no-cookie path.
func TestSession_MissingCookie_401(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions()}
	cache := internal_testkit.NewFakeCache()
	mw := middleware.Session(repo, cache, newSilentLogger())

	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestSession_CacheMiss_PopulatesCache covers the Postgres fall-through path
// and the write-back into Valkey on miss.
func TestSession_CacheMiss_PopulatesCache(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions()}
	cache := internal_testkit.NewFakeCache()
	token := "tokentokentokentokentokentokentokentokentoken"
	_, hashHex := seedSession(t, repo, token)

	mw := middleware.Session(repo, cache, newSilentLogger())
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	req.Header.Set("Cookie", "session="+token)

	resp, _ := dispatchFast(t, mw, req, func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !cache.Has(hashHex) {
		t.Fatal("cache should be populated after Postgres fallthrough")
	}
	if cache.SetCalls != 1 {
		t.Fatalf("want 1 cache Set on populate, got %d", cache.SetCalls)
	}
}

// TestSession_CacheHit_StillConsultsPostgres confirms the documented policy
// (cache stores only sessionID; Postgres provides expiry/user/tenant).
func TestSession_CacheHit_StillConsultsPostgres(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions()}
	cache := internal_testkit.NewFakeCache()
	token := "tokentokentokentokentokentokentokentokentoken"
	s, hashHex := seedSession(t, repo, token)
	_ = cache.Set(context.Background(), hashHex, s.ID().String(), time.Hour)

	mw := middleware.Session(repo, cache, newSilentLogger())
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	req.Header.Set("Cookie", "session="+token)

	resp, _ := dispatchFast(t, mw, req, func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(&repo.finds) != 1 {
		t.Fatalf("expected Postgres FindByTokenHash called exactly once, got %d", repo.finds)
	}
	// No second cache Set: hit path does NOT re-populate.
	if cache.SetCalls != 1 {
		t.Fatalf("want cache.SetCalls=1 (only the seed), got %d", cache.SetCalls)
	}
}

// TestSession_CacheUnavailable_FallsThroughToPostgres verifies a cache outage
// never blocks authentication.
func TestSession_CacheUnavailable_FallsThroughToPostgres(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions()}
	cache := internal_testkit.NewFakeCache()
	cache.FailGet = true
	cache.FailSet = true // repopulate also fails, but auth still succeeds
	token := "tokentokentokentokentokentokentokentokentoken"
	seedSession(t, repo, token)

	mw := middleware.Session(repo, cache, newSilentLogger())
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	req.Header.Set("Cookie", "session="+token)

	resp, _ := dispatchFast(t, mw, req, func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (cache outage must NOT block auth)", resp.StatusCode)
	}
}

func init() {
	// Silence the default logger to keep test output readable.
	_ = slog.New(slog.NewJSONHandler(io.Discard, nil))
}
