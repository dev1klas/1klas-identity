// Package valkey wires the session cache adapter against a Valkey (Redis
// wire-compatible) server. The package name matches the operational vendor
// name; the wire protocol is Redis RESP 3.
package valkey

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// keyPrefix namespaces all keys so we can share a Valkey instance with other
// 1klas services later (gateway rate-limit, comms throttles, etc).
const keyPrefix = "identity:session:"

// SessionCache implements session.Cache against a Redis/Valkey client.
type SessionCache struct {
	client *redis.Client
	logger *slog.Logger
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
	// Logger receives WARN logs for corrupt cache payloads. May be nil — a
	// no-op discard logger is substituted in New.
	Logger *slog.Logger
}

// cachePayload is the wire shape persisted in Valkey. It MUST round-trip
// losslessly through json.Marshal/Unmarshal. New fields are forwards-
// compatible (json.Unmarshal silently ignores them) only if their absence is
// safe; today every field is required.
type cachePayload struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
	ExpiresAt string `json:"expires_at"`
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
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &SessionCache{
		client:    client,
		logger:    logger,
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

// Set writes the full CachedSession payload under the token-hash key with the
// given TTL.
func (c *SessionCache) Set(ctx context.Context, tokenHash string, payload session.CachedSession, ttl time.Duration) error {
	if c.opTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opTimeout)
		defer cancel()
	}
	wire := cachePayload{
		SessionID: payload.SessionID.String(),
		UserID:    payload.UserID.String(),
		TenantID:  payload.TenantID.String(),
		ExpiresAt: payload.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	b, err := json.Marshal(wire)
	if err != nil {
		// Marshal failure of a fixed schema is a programmer error — surface it.
		return err
	}
	return c.client.Set(ctx, keyPrefix+tokenHash, b, ttl).Err()
}

// Get retrieves the CachedSession for a token hash. Returns
// session.ErrCacheMiss on a clean miss OR on a corrupt payload (corrupt cache
// entries are treated as a miss; the corruption is logged WARN inside the
// adapter). Any other Redis error is returned raw so the middleware can log
// WARN and fall through to Postgres without blocking auth.
func (c *SessionCache) Get(ctx context.Context, tokenHash string) (session.CachedSession, error) {
	if c.opTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opTimeout)
		defer cancel()
	}
	b, err := c.client.Get(ctx, keyPrefix+tokenHash).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return session.CachedSession{}, session.ErrCacheMiss
		}
		return session.CachedSession{}, err
	}
	var wire cachePayload
	if err := json.Unmarshal(b, &wire); err != nil {
		c.logger.WarnContext(ctx, "session cache: corrupt JSON payload, treating as miss",
			slog.String("err", err.Error()),
		)
		return session.CachedSession{}, session.ErrCacheMiss
	}
	sid, err := uuid.Parse(wire.SessionID)
	if err != nil {
		c.logger.WarnContext(ctx, "session cache: corrupt session_id, treating as miss",
			slog.String("err", err.Error()),
		)
		return session.CachedSession{}, session.ErrCacheMiss
	}
	uid, err := uuid.Parse(wire.UserID)
	if err != nil {
		c.logger.WarnContext(ctx, "session cache: corrupt user_id, treating as miss",
			slog.String("err", err.Error()),
		)
		return session.CachedSession{}, session.ErrCacheMiss
	}
	tid, err := tenant.Parse(wire.TenantID)
	if err != nil {
		c.logger.WarnContext(ctx, "session cache: corrupt tenant_id, treating as miss",
			slog.String("err", err.Error()),
		)
		return session.CachedSession{}, session.ErrCacheMiss
	}
	exp, err := time.Parse(time.RFC3339Nano, wire.ExpiresAt)
	if err != nil {
		c.logger.WarnContext(ctx, "session cache: corrupt expires_at, treating as miss",
			slog.String("err", err.Error()),
		)
		return session.CachedSession{}, session.ErrCacheMiss
	}
	return session.CachedSession{
		SessionID: sid,
		UserID:    uid,
		TenantID:  tid,
		ExpiresAt: exp.UTC(),
	}, nil
}

// Delete removes the (tokenHash -> *) mapping. Idempotent.
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
