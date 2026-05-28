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
	finds      int32
	failIfCall bool
	t          *testing.T
}

func (r *instrumentedRepo) FindByTokenHash(ctx context.Context, h []byte) (session.Session, error) {
	atomic.AddInt32(&r.finds, 1)
	if r.failIfCall {
		r.t.Fatalf("Postgres FindByTokenHash MUST NOT be called on cache hit (ADR-0008)")
	}
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
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions(), t: t}
	cache := internal_testkit.NewFakeCache()
	mw := middleware.Session(repo, cache, newSilentLogger())

	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestSession_CacheMiss_PopulatesCache covers the Postgres fall-through path
// and the write-back into Valkey on miss. Verifies the TTL is approximately
// time.Until(session.ExpiresAt()) (within 1s drift).
func TestSession_CacheMiss_PopulatesCache(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions(), t: t}
	cache := internal_testkit.NewFakeCache()
	token := "tokentokentokentokentokentokentokentokentoken"
	s, hashHex := seedSession(t, repo, token)

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
	// TTL must be approximately time.Until(s.ExpiresAt()) — allow 1s drift
	// to absorb test scheduling latency between seed and middleware run.
	got := cache.TTL(hashHex)
	want := time.Until(s.ExpiresAt())
	diff := want - got
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Fatalf("repopulate TTL drift = %v (want ~%v, got %v)", diff, want, got)
	}
}

// TestSession_CacheHit_SkipsPostgres confirms the ADR-0008 contract: a fresh,
// non-expired cache entry serves the request with zero Postgres round-trips.
func TestSession_CacheHit_SkipsPostgres(t *testing.T) {
	repo := &instrumentedRepo{
		FakeSessions: internal_testkit.NewFakeSessions(),
		failIfCall:   true,
		t:            t,
	}
	cache := internal_testkit.NewFakeCache()
	token := "tokentokentokentokentokentokentokentokentoken"
	hash := session.HashOf(token)
	hashHex := hex.EncodeToString(hash)

	sid := uuid.New()
	uid := uuid.New()
	exp := time.Now().UTC().Add(time.Hour)
	cache.Seed(hashHex, session.CachedSession{
		SessionID: sid,
		UserID:    uid,
		TenantID:  tenant.DefaultID,
		ExpiresAt: exp,
	}, time.Hour)

	var gotUser, gotSession uuid.UUID
	var gotTenant tenant.ID
	mw := middleware.Session(repo, cache, newSilentLogger())
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	req.Header.Set("Cookie", "session="+token)

	resp, _ := dispatchFast(t, mw, req, func(ctx *fasthttp.RequestCtx) {
		gotUser, _ = ctx.UserValue(middleware.UVUserID).(uuid.UUID)
		gotSession, _ = ctx.UserValue(middleware.UVSessionID).(uuid.UUID)
		gotTenant, _ = ctx.UserValue(middleware.UVTenantID).(tenant.ID)
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(&repo.finds) != 0 {
		t.Fatalf("Postgres MUST NOT be touched on cache hit, got %d FindByTokenHash calls", repo.finds)
	}
	if gotUser != uid {
		t.Fatalf("user_id = %v, want %v", gotUser, uid)
	}
	if gotSession != sid {
		t.Fatalf("session_id = %v, want %v", gotSession, sid)
	}
	if gotTenant.String() != tenant.DefaultID.String() {
		t.Fatalf("tenant_id = %v, want %v", gotTenant, tenant.DefaultID)
	}
	// No second cache Set: hit path does NOT re-populate.
	if cache.SetCalls != 0 {
		t.Fatalf("cache.SetCalls = %d, want 0 (hit path must not Set)", cache.SetCalls)
	}
}

// TestSession_CacheHit_ExpiredFallsThrough verifies that a cached-but-expired
// entry causes a Postgres re-check. If Postgres also says expired, request is
// rejected with 401.
func TestSession_CacheHit_ExpiredFallsThrough(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions(), t: t}
	cache := internal_testkit.NewFakeCache()

	token := "tokentokentokentokentokentokentokentokentoken"
	hash := session.HashOf(token)
	hashHex := hex.EncodeToString(hash)

	// Seed Postgres with an already-expired session.
	sid := uuid.New()
	uid := uuid.New()
	past := time.Now().UTC().Add(-time.Hour)
	older := past.Add(-time.Hour)
	s := session.New(sid, tenant.DefaultID, uid, hash, older, past)
	if err := repo.SaveTx(context.Background(), user.NewTx(struct{}{}), s); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Seed cache with a payload that LOOKS expired so the middleware will
	// fall through.
	cache.Seed(hashHex, session.CachedSession{
		SessionID: sid,
		UserID:    uid,
		TenantID:  tenant.DefaultID,
		ExpiresAt: past,
	}, time.Hour)

	mw := middleware.Session(repo, cache, newSilentLogger())
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)
	req.Header.Set("Cookie", "session="+token)

	resp, _ := dispatchFast(t, mw, req, func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (expired in both cache + postgres)", resp.StatusCode)
	}
	if atomic.LoadInt32(&repo.finds) != 1 {
		t.Fatalf("expected Postgres consulted once on expired-cache fallthrough, got %d", repo.finds)
	}
}

// TestSession_CacheUnavailable_FallsThroughToPostgres verifies a cache outage
// never blocks authentication. WARN is logged; Postgres is consulted; request
// succeeds.
func TestSession_CacheUnavailable_FallsThroughToPostgres(t *testing.T) {
	repo := &instrumentedRepo{FakeSessions: internal_testkit.NewFakeSessions(), t: t}
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
	if atomic.LoadInt32(&repo.finds) != 1 {
		t.Fatalf("expected Postgres fallthrough on cache outage, got %d FindByTokenHash calls", repo.finds)
	}
}

func init() {
	// Silence the default logger to keep test output readable.
	_ = slog.New(slog.NewJSONHandler(io.Discard, nil))
}
