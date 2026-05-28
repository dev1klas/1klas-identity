// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr                = ":8080"
	defaultSessionTTLHrs       = 168
	defaultArgon2MemoryKi      = 65536
	defaultArgon2Time          = 3
	defaultArgon2Para          = 1
	defaultValkeyDialTimeoutMs = 200
	defaultValkeyOpTimeoutMs   = 100
)

// Config holds runtime configuration.
type Config struct {
	Addr            string
	PostgresURL     string
	CookieSecure    bool
	SessionTTL      time.Duration
	Argon2MemoryKiB uint32
	Argon2Time      uint32
	Argon2Parallel  uint8
	// AllowedOrigins is the CSRF allow-list for mutating routes. Sourced from
	// ALLOWED_ORIGINS (comma-separated). Must contain at least one origin —
	// Load refuses to start otherwise.
	AllowedOrigins []string
	// RunMigrationsOnBoot causes the server binary to apply pending goose
	// migrations during startup. Defaults to false because production
	// migrations are applied by the cmd/migrate pre-deploy job; the flag
	// exists for local docker-compose convenience where running a separate
	// binary is friction.
	RunMigrationsOnBoot bool
	// ValkeyURL is the redis:// or rediss:// URL of the Valkey instance used
	// for the write-through session cache. Required: Load refuses to start
	// otherwise (see ADR-0008).
	ValkeyURL string
	// ValkeyDialTimeout is the connection establishment deadline.
	ValkeyDialTimeout time.Duration
	// ValkeyOpTimeout is the per-operation deadline.
	ValkeyOpTimeout time.Duration
}

// Load reads configuration from env. Returns ErrMissingPostgresURL if
// POSTGRES_URL is unset, or ErrInvalidInt for malformed numeric vars.
func Load() (Config, error) {
	cfg := Config{
		Addr:                envOr("ADDR", defaultAddr),
		PostgresURL:         os.Getenv("POSTGRES_URL"),
		CookieSecure:        envBool("COOKIE_SECURE", true),
		SessionTTL:          time.Duration(defaultSessionTTLHrs) * time.Hour,
		Argon2MemoryKiB:     defaultArgon2MemoryKi,
		Argon2Time:          defaultArgon2Time,
		Argon2Parallel:      defaultArgon2Para,
		RunMigrationsOnBoot: envBool("RUN_MIGRATIONS_ON_BOOT", false),
		ValkeyURL:           os.Getenv("VALKEY_URL"),
		ValkeyDialTimeout:   time.Duration(defaultValkeyDialTimeoutMs) * time.Millisecond,
		ValkeyOpTimeout:     time.Duration(defaultValkeyOpTimeoutMs) * time.Millisecond,
	}

	if cfg.PostgresURL == "" {
		return Config{}, ErrMissingPostgresURL
	}

	if cfg.ValkeyURL == "" {
		return Config{}, ErrMissingValkeyURL
	}

	if v := os.Getenv("VALKEY_DIAL_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, ErrInvalidInt
		}
		cfg.ValkeyDialTimeout = time.Duration(n) * time.Millisecond
	}
	if v := os.Getenv("VALKEY_OP_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, ErrInvalidInt
		}
		cfg.ValkeyOpTimeout = time.Duration(n) * time.Millisecond
	}

	if v := os.Getenv("SESSION_TTL_HOURS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, ErrInvalidInt
		}
		cfg.SessionTTL = time.Duration(n) * time.Hour
	}

	if v := os.Getenv("ARGON2_MEMORY_KIB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, ErrInvalidInt
		}
		cfg.Argon2MemoryKiB = uint32(n)
	}
	if v := os.Getenv("ARGON2_TIME"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, ErrInvalidInt
		}
		cfg.Argon2Time = uint32(n)
	}
	if v := os.Getenv("ARGON2_PARALLELISM"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 255 {
			return Config{}, ErrInvalidInt
		}
		cfg.Argon2Parallel = uint8(n)
	}

	raw := os.Getenv("ALLOWED_ORIGINS")
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, p)
		}
	}
	if len(cfg.AllowedOrigins) == 0 {
		return Config{}, ErrMissingAllowedOrigins
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	case "0", "false", "FALSE", "no", "NO":
		return false
	}
	return fallback
}
