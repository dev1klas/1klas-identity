package valkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
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

func TestSessionCache_SetGetDelete(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "hash-a", "session-1", 60*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := c.Get(ctx, "hash-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "session-1" {
		t.Fatalf("got %q, want %q", got, "session-1")
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

	if err := c.Set(ctx, "hash-ttl", "session-2", 1*time.Second); err != nil {
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
