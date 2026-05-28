package session

import (
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Session is the authenticated-session aggregate. Token hash, not plaintext,
// is stored.
type Session struct {
	id        uuid.UUID
	tenantID  tenant.ID
	userID    uuid.UUID
	tokenHash []byte
	createdAt time.Time
	expiresAt time.Time
	revokedAt *time.Time
}

// New constructs a fresh, unrevoked session.
func New(id uuid.UUID, t tenant.ID, userID uuid.UUID, tokenHash []byte, now, expiresAt time.Time) Session {
	return Session{
		id:        id,
		tenantID:  t,
		userID:    userID,
		tokenHash: tokenHash,
		createdAt: now,
		expiresAt: expiresAt,
	}
}

// Hydrate rebuilds a Session from persistence.
func Hydrate(id uuid.UUID, t tenant.ID, userID uuid.UUID, tokenHash []byte, createdAt, expiresAt time.Time, revokedAt *time.Time) Session {
	return Session{
		id:        id,
		tenantID:  t,
		userID:    userID,
		tokenHash: tokenHash,
		createdAt: createdAt,
		expiresAt: expiresAt,
		revokedAt: revokedAt,
	}
}

func (s Session) ID() uuid.UUID         { return s.id }
func (s Session) TenantID() tenant.ID   { return s.tenantID }
func (s Session) UserID() uuid.UUID     { return s.userID }
func (s Session) TokenHash() []byte     { return s.tokenHash }
func (s Session) CreatedAt() time.Time  { return s.createdAt }
func (s Session) ExpiresAt() time.Time  { return s.expiresAt }
func (s Session) RevokedAt() *time.Time { return s.revokedAt }

// IsActive reports whether the session is usable at the given instant.
func (s Session) IsActive(now time.Time) bool {
	if s.revokedAt != nil {
		return false
	}
	return now.Before(s.expiresAt)
}
