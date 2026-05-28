package valkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/valkey"
)

func newCache(t *testing.T) (*valkey.SessionCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis start: %v", err)
	}
	c, err := valkey.New(valkey.Config{
		URL:         "redis://" + mr.Addr(),
		DialTimeout: 200 * time.Millisecond,
		OpTimeout:   500 * time.Millisecond,
	})
	if err != nil {
		mr.Close()
		t.Fatalf("cache new: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close()
		mr.Close()
	})
	return c, mr
}

func samplePayload() session.CachedSession {
	return session.CachedSession{
		SessionID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		TenantID:  tenant.DefaultID,
		ExpiresAt: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestSessionCache_SetGetDelete(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	p := samplePayload()

	if err := c.Set(ctx, "hash-a", p, 60*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := c.Get(ctx, "hash-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SessionID != p.SessionID {
		t.Fatalf("session_id = %v, want %v", got.SessionID, p.SessionID)
	}
	if got.UserID != p.UserID {
		t.Fatalf("user_id = %v, want %v", got.UserID, p.UserID)
	}
	if got.TenantID.String() != p.TenantID.String() {
		t.Fatalf("tenant_id = %v, want %v", got.TenantID, p.TenantID)
	}
	if !got.ExpiresAt.Equal(p.ExpiresAt) {
		t.Fatalf("expires_at = %v, want %v", got.ExpiresAt, p.ExpiresAt)
	}

	if err := c.Delete(ctx, "hash-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := c.Get(ctx, "hash-a"); !errors.Is(err, session.ErrCacheMiss) {
		t.Fatalf("get after delete: want ErrCacheMiss, got %v", err)
	}
}

func TestSessionCache_MissReturnsErrCacheMiss(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()

	_, err := c.Get(ctx, "never-set")
	if !errors.Is(err, session.ErrCacheMiss) {
		t.Fatalf("want ErrCacheMiss, got %v", err)
	}
}

func TestSessionCache_TTLExpires(t *testing.T) {
	c, mr := newCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "hash-ttl", samplePayload(), 1*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Sanity: present before TTL elapses.
	if _, err := c.Get(ctx, "hash-ttl"); err != nil {
		t.Fatalf("get before expiry: %v", err)
	}

	mr.FastForward(2 * time.Second)

	if _, err := c.Get(ctx, "hash-ttl"); !errors.Is(err, session.ErrCacheMiss) {
		t.Fatalf("get after expiry: want ErrCacheMiss, got %v", err)
	}
}

func TestSessionCache_DeleteIdempotent(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	// Deleting a non-existent key must NOT error.
	if err := c.Delete(ctx, "ghost-key"); err != nil {
		t.Fatalf("delete missing key: %v", err)
	}
}

func TestSessionCache_PingOK(t *testing.T) {
	c, _ := newCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestSessionCache_InvalidURL(t *testing.T) {
	_, err := valkey.New(valkey.Config{URL: "not-a-redis-url"})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// TestSessionCache_CorruptJSONReturnsMiss asserts that a hand-written corrupt
// payload (raw string instead of JSON) is surfaced as ErrCacheMiss — corrupt
// cache entries are equivalent to a miss; Postgres is the source of truth.
func TestSessionCache_CorruptJSONReturnsMiss(t *testing.T) {
	c, mr := newCache(t)
	ctx := context.Background()

	// Write a non-JSON value directly under the namespaced key. The adapter's
	// keyPrefix is "identity:session:" — replicate it here to bypass Set.
	if err := mr.Set("identity:session:corrupt", "not-json"); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}

	if _, err := c.Get(ctx, "corrupt"); !errors.Is(err, session.ErrCacheMiss) {
		t.Fatalf("want ErrCacheMiss on corrupt payload, got %v", err)
	}
}

// TestSessionCache_CorruptUUIDReturnsMiss asserts that valid JSON with an
// invalid UUID inside is also surfaced as a miss.
func TestSessionCache_CorruptUUIDReturnsMiss(t *testing.T) {
	c, mr := newCache(t)
	ctx := context.Background()

	if err := mr.Set("identity:session:bad-uuid",
		`{"session_id":"not-a-uuid","user_id":"22222222-2222-2222-2222-222222222222","tenant_id":"`+tenant.DefaultID.String()+`","expires_at":"2030-01-02T03:04:05Z"}`,
	); err != nil {
		t.Fatalf("seed bad-uuid: %v", err)
	}

	if _, err := c.Get(ctx, "bad-uuid"); !errors.Is(err, session.ErrCacheMiss) {
		t.Fatalf("want ErrCacheMiss on bad UUID, got %v", err)
	}
}
