package config

import "errors"

// ErrMissingPostgresURL is returned when POSTGRES_URL is unset.
var ErrMissingPostgresURL = errors.New("config: POSTGRES_URL is required")

// ErrInvalidInt is returned when a numeric env var fails to parse.
var ErrInvalidInt = errors.New("config: invalid integer env var")
