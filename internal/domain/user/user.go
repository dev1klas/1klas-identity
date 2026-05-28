package user

import (
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
)

// Status enumerates the allowed user lifecycle states.
type Status string

const (
	StatusActive            Status = "active"
	StatusPendingActivation Status = "pending_activation"
	StatusSuspended         Status = "suspended"
	StatusRejected          Status = "rejected"
)

// User is the Identity aggregate root.
type User struct {
	id           uuid.UUID
	tenantID     tenant.ID
	email        Email
	passwordHash PasswordHash
	status       Status
	createdAt    time.Time
	updatedAt    time.Time
}

// New builds a new active User. ID generation is the caller's responsibility
// so that the use case controls the transactional ordering.
func New(id uuid.UUID, t tenant.ID, email Email, hash PasswordHash, now time.Time) User {
	return User{
		id:           id,
		tenantID:     t,
		email:        email,
		passwordHash: hash,
		status:       StatusActive,
		createdAt:    now,
		updatedAt:    now,
	}
}

// Hydrate rebuilds a User from persistence. Used by repositories only.
func Hydrate(id uuid.UUID, t tenant.ID, email Email, hash PasswordHash, status Status, createdAt, updatedAt time.Time) User {
	return User{
		id:           id,
		tenantID:     t,
		email:        email,
		passwordHash: hash,
		status:       status,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

// Accessors. No mutation on the walking skeleton.

func (u User) ID() uuid.UUID              { return u.id }
func (u User) TenantID() tenant.ID        { return u.tenantID }
func (u User) Email() Email               { return u.email }
func (u User) PasswordHash() PasswordHash { return u.passwordHash }
func (u User) Status() Status             { return u.status }
func (u User) CreatedAt() time.Time       { return u.createdAt }
func (u User) UpdatedAt() time.Time       { return u.updatedAt }
