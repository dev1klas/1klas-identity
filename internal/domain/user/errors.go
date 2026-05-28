package user

import "errors"

// ErrUserNotFound indicates the requested user could not be located in the
// repository.
var ErrUserNotFound = errors.New("user: not found")

// ErrEmailTaken indicates a unique-constraint violation on (tenant_id, email).
var ErrEmailTaken = errors.New("user: email already taken")

// ErrInvalidEmail indicates the provided email failed validation.
var ErrInvalidEmail = errors.New("user: invalid email")

// ErrWeakPassword indicates the provided password failed policy.
var ErrWeakPassword = errors.New("user: weak password")
