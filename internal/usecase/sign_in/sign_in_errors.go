package sign_in

import "errors"

// ErrInvalidCredentials covers both unknown email and wrong password.
// Use cases MUST NOT distinguish between the two to the caller.
var ErrInvalidCredentials = errors.New("sign_in: invalid credentials")

// ErrInvalidEmail surfaces from email validation. Treated as invalid creds at
// the HTTP layer to avoid revealing email existence.
var ErrInvalidEmail = errors.New("sign_in: invalid email")

// ErrInternal wraps any other failure.
var ErrInternal = errors.New("sign_in: internal")
