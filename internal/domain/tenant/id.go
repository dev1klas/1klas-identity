// Package tenant exposes the TenantID value object and the default
// single-tenant constant used at MVP.
package tenant

import (
	"errors"

	"github.com/google/uuid"
)

// ID is a typed tenant identifier. Zero value is invalid.
type ID struct {
	value uuid.UUID
}

// ErrInvalidTenantID is returned when a string cannot be parsed as a tenant ID.
var ErrInvalidTenantID = errors.New("tenant: invalid id")

// DefaultID is the single-tenant constant used during MVP.
// See SPEC-identity.md §"Tenancy".
var DefaultID = MustParse("00000000-0000-0000-0000-000000000001")

// NewID wraps a uuid.UUID.
func NewID(u uuid.UUID) ID { return ID{value: u} }

// Parse parses a string into a TenantID.
func Parse(s string) (ID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return ID{}, ErrInvalidTenantID
	}
	return ID{value: u}, nil
}

// MustParse panics if s is not a valid UUID. Use for constants only.
func MustParse(s string) ID {
	id, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

// UUID returns the underlying uuid.UUID.
func (t ID) UUID() uuid.UUID { return t.value }

// String returns the canonical UUID string form.
func (t ID) String() string { return t.value.String() }

// IsZero reports whether t is the zero value.
func (t ID) IsZero() bool { return t.value == uuid.Nil }
