// Package internal_testkit provides in-memory fakes shared across use case
// unit tests. NOT a runtime dependency.
package internal_testkit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain"
	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// FakeClock is a deterministic clock.Clock.
type FakeClock struct{ T time.Time }

// Now returns the fixed time.
func (c *FakeClock) Now() time.Time { return c.T }

// FakeHasher is a fake user.PasswordHasher.
type FakeHasher struct {
	HashFn   func(raw string) string
	VerifyFn func(stored, raw string) bool
}

// Hash returns a deterministic encoded hash.
func (h *FakeHasher) Hash(_ context.Context, raw string) (user.PasswordHash, error) {
	if h.HashFn != nil {
		return user.NewPasswordHash(h.HashFn(raw)), nil
	}
	return user.NewPasswordHash("fakehash:" + raw), nil
}

// Verify compares the stored hash against a fresh hash of raw.
func (h *FakeHasher) Verify(_ context.Context, stored user.PasswordHash, raw string) (bool, error) {
	if h.VerifyFn != nil {
		return h.VerifyFn(stored.String(), raw), nil
	}
	return stored.String() == "fakehash:"+raw, nil
}

// FakeTokenGen is a deterministic token generator.
type FakeTokenGen struct {
	Next int
}

// NewToken returns sequential opaque tokens.
func (g *FakeTokenGen) NewToken() (session.Token, error) {
	g.Next++
	pad := make([]byte, 40)
	for i := range pad {
		pad[i] = 'a'
	}
	tok := string(pad) + "-" + intToString(g.Next)
	return session.NewToken(tok)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// FakeUoW is a no-op UnitOfWork backing the in-memory repositories.
type FakeUoW struct{}

// Begin returns a zero Tx token.
func (FakeUoW) Begin(context.Context) (user.Tx, error) { return user.NewTx(struct{}{}), nil }

// Commit is a no-op.
func (FakeUoW) Commit(context.Context, user.Tx) error { return nil }

// Rollback is a no-op.
func (FakeUoW) Rollback(context.Context, user.Tx) error { return nil }

// Compile-time assertion.
var _ domain.UnitOfWork = FakeUoW{}

// FakeUsers is an in-memory user.Repository.
type FakeUsers struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]user.User
	byMail map[string]uuid.UUID // tenant|email -> id
}

// NewFakeUsers constructs the repo.
func NewFakeUsers() *FakeUsers {
	return &FakeUsers{
		byID:   map[uuid.UUID]user.User{},
		byMail: map[string]uuid.UUID{},
	}
}

// SaveTx persists.
func (r *FakeUsers) SaveTx(_ context.Context, _ user.Tx, u user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := u.TenantID().String() + "|" + u.Email().String()
	if _, exists := r.byMail[key]; exists {
		return user.ErrEmailTaken
	}
	r.byID[u.ID()] = u
	r.byMail[key] = u.ID()
	return nil
}

// FindByEmail looks up.
func (r *FakeUsers) FindByEmail(_ context.Context, t tenant.ID, e user.Email) (user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byMail[t.String()+"|"+e.String()]
	if !ok {
		return user.User{}, user.ErrUserNotFound
	}
	return r.byID[id], nil
}

// NewListIDs returns all known user IDs. Test helper.
func (r *FakeUsers) NewListIDs() []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]uuid.UUID, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	return out
}

// FindByID looks up.
func (r *FakeUsers) FindByID(_ context.Context, t tenant.ID, id uuid.UUID) (user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok || u.TenantID().String() != t.String() {
		return user.User{}, user.ErrUserNotFound
	}
	return u, nil
}

// FakeSessions is an in-memory session.Repository.
type FakeSessions struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]session.Session
	byHash map[string]uuid.UUID
}

// NewFakeSessions constructs the repo.
func NewFakeSessions() *FakeSessions {
	return &FakeSessions{
		byID:   map[uuid.UUID]session.Session{},
		byHash: map[string]uuid.UUID{},
	}
}

// SaveTx persists.
func (r *FakeSessions) SaveTx(_ context.Context, _ user.Tx, s session.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[s.ID()] = s
	r.byHash[string(s.TokenHash())] = s.ID()
	return nil
}

// FindByTokenHash returns the matching session.
func (r *FakeSessions) FindByTokenHash(_ context.Context, hash []byte) (session.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byHash[string(hash)]
	if !ok {
		return session.Session{}, session.ErrSessionNotFound
	}
	return r.byID[id], nil
}

// NewListIDs returns all known session IDs. Test helper.
func (r *FakeSessions) NewListIDs() []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]uuid.UUID, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	return out
}

// RevokeTx marks revoked.
func (r *FakeSessions) RevokeTx(_ context.Context, _ user.Tx, t tenant.ID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok || s.TenantID().String() != t.String() {
		return session.ErrSessionNotFound
	}
	if s.RevokedAt() != nil {
		return nil
	}
	now := time.Now().UTC()
	r.byID[id] = session.Hydrate(s.ID(), s.TenantID(), s.UserID(), s.TokenHash(), s.CreatedAt(), s.ExpiresAt(), &now)
	return nil
}

// FakeOutbox is an in-memory outbox.Repository.
type FakeOutbox struct {
	mu     sync.Mutex
	Events []outbox.Event
}

// NewFakeOutbox constructs the repo.
func NewFakeOutbox() *FakeOutbox { return &FakeOutbox{} }

// WriteTx records.
func (r *FakeOutbox) WriteTx(_ context.Context, _ user.Tx, ev outbox.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, ev)
	return nil
}

// FakeCache is an in-memory session.Cache. TTL is recorded for assertions
// but does NOT cause expiry — tests that need expiry must seed an entry with
// a past ExpiresAt via Seed/Set or use miniredis.
type FakeCache struct {
	mu        sync.Mutex
	entries   map[string]session.CachedSession
	TTLs      map[string]time.Duration
	SetCalls  int
	GetCalls  int
	DelCalls  int
	FailSet   bool
	FailGet   bool
	FailDel   bool
	ForceMiss bool
}

// NewFakeCache constructs a FakeCache.
func NewFakeCache() *FakeCache {
	return &FakeCache{
		entries: map[string]session.CachedSession{},
		TTLs:    map[string]time.Duration{},
	}
}

// Set records the payload.
func (c *FakeCache) Set(_ context.Context, tokenHash string, payload session.CachedSession, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SetCalls++
	if c.FailSet {
		return errors.New("fake cache set failure")
	}
	c.entries[tokenHash] = payload
	c.TTLs[tokenHash] = ttl
	return nil
}

// Get returns the recorded CachedSession or session.ErrCacheMiss.
func (c *FakeCache) Get(_ context.Context, tokenHash string) (session.CachedSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GetCalls++
	if c.FailGet {
		return session.CachedSession{}, errors.New("fake cache get failure")
	}
	if c.ForceMiss {
		return session.CachedSession{}, session.ErrCacheMiss
	}
	v, ok := c.entries[tokenHash]
	if !ok {
		return session.CachedSession{}, session.ErrCacheMiss
	}
	return v, nil
}

// Delete removes the payload.
func (c *FakeCache) Delete(_ context.Context, tokenHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DelCalls++
	if c.FailDel {
		return errors.New("fake cache delete failure")
	}
	delete(c.entries, tokenHash)
	delete(c.TTLs, tokenHash)
	return nil
}

// Has reports whether the cache currently has an entry for tokenHash.
func (c *FakeCache) Has(tokenHash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.entries[tokenHash]
	return ok
}

// Seed installs a CachedSession directly (bypassing Set bookkeeping). Useful
// to prime the cache in middleware tests without inflating SetCalls.
func (c *FakeCache) Seed(tokenHash string, payload session.CachedSession, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[tokenHash] = payload
	c.TTLs[tokenHash] = ttl
}

// TTL returns the TTL recorded on the last Set/Seed for tokenHash. Zero
// duration if the key is absent.
func (c *FakeCache) TTL(tokenHash string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.TTLs[tokenHash]
}

// Compile-time guard.
var _ session.Cache = (*FakeCache)(nil)

// ErrUnused silences unused-import warnings on shared error references.
var ErrUnused = errors.New("unused")
