package session

import "errors"

// ErrSessionNotFound indicates the requested session row was not present.
var ErrSessionNotFound = errors.New("session: not found")

// ErrSessionExpired indicates the session row exists but has passed expires_at.
var ErrSessionExpired = errors.New("session: expired")

// ErrSessionRevoked indicates the session row exists but is revoked.
var ErrSessionRevoked = errors.New("session: revoked")
