package config_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dev1klas/1klas-identity/internal/config"
)

// setBaseEnv populates all required env vars. Tests that target a specific
// missing-required path overwrite the relevant one to "" via t.Setenv after.
func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("POSTGRES_URL", "postgres://x")
	t.Setenv("ALLOWED_ORIGINS", "https://app.1klasdev.com")
	t.Setenv("VALKEY_URL", "redis://localhost:6379/0")
}

func TestLoad_RejectsEmptyAllowedOrigins(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "")
	_, err := config.Load()
	if !errors.Is(err, config.ErrMissingAllowedOrigins) {
		t.Fatalf("want ErrMissingAllowedOrigins, got %v", err)
	}
}

func TestLoad_RejectsAllWhitespaceAllowedOrigins(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ALLOWED_ORIGINS", " , , ")
	_, err := config.Load()
	if !errors.Is(err, config.ErrMissingAllowedOrigins) {
		t.Fatalf("want ErrMissingAllowedOrigins, got %v", err)
	}
}

func TestLoad_RejectsMissingValkeyURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("VALKEY_URL", "")
	_, err := config.Load()
	if !errors.Is(err, config.ErrMissingValkeyURL) {
		t.Fatalf("want ErrMissingValkeyURL, got %v", err)
	}
}

func TestLoad_RunMigrationsOnBoot_DefaultsFalse(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("RUN_MIGRATIONS_ON_BOOT", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RunMigrationsOnBoot {
		t.Fatal("RunMigrationsOnBoot must default to false (pre-deploy job applies migrations in prod)")
	}
}

func TestLoad_RunMigrationsOnBoot_True(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("RUN_MIGRATIONS_ON_BOOT", "true")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.RunMigrationsOnBoot {
		t.Fatal("RunMigrationsOnBoot must be true when env=true")
	}
}

func TestLoad_ParsesAllowedOrigins(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173, https://app.1klasdev.com ")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("want 2 origins, got %v", cfg.AllowedOrigins)
	}
	if cfg.AllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("origin[0] = %q", cfg.AllowedOrigins[0])
	}
	if cfg.AllowedOrigins[1] != "https://app.1klasdev.com" {
		t.Fatalf("origin[1] = %q", cfg.AllowedOrigins[1])
	}
}

func TestLoad_ValkeyTimeoutDefaults(t *testing.T) {
	setBaseEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ValkeyDialTimeout != 200*time.Millisecond {
		t.Fatalf("dial timeout default = %v", cfg.ValkeyDialTimeout)
	}
	if cfg.ValkeyOpTimeout != 100*time.Millisecond {
		t.Fatalf("op timeout default = %v", cfg.ValkeyOpTimeout)
	}
}

func TestLoad_ValkeyTimeoutOverrides(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("VALKEY_DIAL_TIMEOUT_MS", "500")
	t.Setenv("VALKEY_OP_TIMEOUT_MS", "250")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ValkeyDialTimeout != 500*time.Millisecond {
		t.Fatalf("dial timeout override = %v", cfg.ValkeyDialTimeout)
	}
	if cfg.ValkeyOpTimeout != 250*time.Millisecond {
		t.Fatalf("op timeout override = %v", cfg.ValkeyOpTimeout)
	}
}
