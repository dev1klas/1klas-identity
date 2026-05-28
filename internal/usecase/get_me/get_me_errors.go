package get_me

import "errors"

// ErrNotFound indicates the user could not be located (cross-tenant or deleted).
var ErrNotFound = errors.New("get_me: not found")

// ErrInternal wraps any unexpected failure.
var ErrInternal = errors.New("get_me: internal")
