package sign_up

import "errors"

// ErrInvalidEmail surfaces from email validation.
var ErrInvalidEmail = errors.New("sign_up: invalid email")

// ErrWeakPassword surfaces from password policy.
var ErrWeakPassword = errors.New("sign_up: weak password")

// ErrEmailTaken surfaces on (tenant, email) unique violation.
var ErrEmailTaken = errors.New("sign_up: email taken")

// ErrInternal wraps any other failure.
var ErrInternal = errors.New("sign_up: internal")
