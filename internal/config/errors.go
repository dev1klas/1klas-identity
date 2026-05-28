package config

import "errors"

// ErrMissingPostgresURL is returned when POSTGRES_URL is unset.
var ErrMissingPostgresURL = errors.New("config: POSTGRES_URL is required")

// ErrInvalidInt is returned when a numeric env var fails to parse.
var ErrInvalidInt = errors.New("config: invalid integer env var")

// ErrMissingAllowedOrigins is returned when ALLOWED_ORIGINS resolves to an
// empty list. CSRF protection relies on it; refusing to start is intentional.
var ErrMissingAllowedOrigins = errors.New("config: ALLOWED_ORIGINS is required (CSRF allow-list)")

// ErrMissingValkeyURL is returned when VALKEY_URL is unset. The session
// cache is mandatory per ADR-0008.
var ErrMissingValkeyURL = errors.New("config: VALKEY_URL is required (session cache)")
