// Package valkey wires the session cache adapter against a Valkey (Redis
// wire-compatible) server. The package name matches the operational vendor
// name; the wire protocol is Redis RESP 3.
package valkey

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
)

// keyPrefix namespaces all keys so we can share a Valkey instance with other
// 1klas services later (gateway rate-limit, comms throttles, etc).
const keyPrefix = "identity:session:"

// SessionCache implements session.Cache against a Redis/Valkey client.
type SessionCache struct {
	client *redis.Client
	// opTimeout bounds each per-call deadline. Callers usually pass an
	// already-scoped ctx, but on the SessionAuth hot path we want a strict
	// upper bound so a slow cache cannot stall an authenticated request.
	opTimeout time.Duration
}

// Config drives the cache adapter.
type Config struct {
	// URL is a redis:// or rediss:// URL. rediss:// enables TLS.
	URL string
	// DialTimeout is the connection establishment deadline.
	DialTimeout time.Duration
	// OpTimeout is the per-operation deadline applied on Set/Get/Delete.
	OpTimeout time.Duration
}

// New constructs a SessionCache. It does NOT perform a connectivity check —
// call Ping after New if you need fail-fast boot semantics (cmd/server does).
func New(cfg Config) (*SessionCache, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, err
	}
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	}
	if cfg.OpTimeout > 0 {
		opts.ReadTimeout = cfg.OpTimeout
		opts.WriteTimeout = cfg.OpTimeout
	}
	// rediss:// already sets TLSConfig; this guard reaffirms a sane minimum
	// in case opts.TLSConfig is non-nil but empty.
	if opts.TLSConfig != nil && opts.TLSConfig.MinVersion == 0 {
		opts.TLSConfig.MinVersion = tls.VersionTLS12
	}
	client := redis.NewClient(opts)
	return &SessionCache{
		client:    client,
		opTimeout: cfg.OpTimeout,
	}, nil
}

// Ping does a single PING. Used at boot to fail fast on misconfiguration.
func (c *SessionCache) Ping(ctx context.Context) error {
	if c.opTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opTimeout)
		defer cancel()
	}
	return c.client.Ping(ctx).Err()
}

// Close releases the underlying pool. Safe to call multiple times.
func (c *SessionCache) Close() error { return c.client.Close() }

// Set writes the (tokenHash -> sessionID) mapping with the given TTL.
func (c *SessionCache) Set(ctx context.Context, tokenHash, sessionID string, ttl time.Duration) error {
	if c.opTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opTimeout)
		defer cancel()
	}
	return c.client.Set(ctx, keyPrefix+tokenHash, sessionID, ttl).Err()
}

// Get retrieves the sessionID for a token hash. Returns session.ErrCacheMiss
// on a clean miss; any other error indicates a transient infra problem.
func (c *SessionCache) Get(ctx context.Context, tokenHash string) (string, error) {
	if c.opTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opTimeout)
		defer cancel()
	}
	v, err := c.client.Get(ctx, keyPrefix+tokenHash).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", session.ErrCacheMiss
		}
		return "", err
	}
	return v, nil
}

// Delete removes the (tokenHash -> sessionID) mapping. Idempotent.
func (c *SessionCache) Delete(ctx context.Context, tokenHash string) error {
	if c.opTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opTimeout)
		defer cancel()
	}
	// DEL returns the count of deleted keys; missing keys are not errors.
	return c.client.Del(ctx, keyPrefix+tokenHash).Err()
}

// Compile-time guard so a port-signature change here fails fast.
var _ session.Cache = (*SessionCache)(nil)
