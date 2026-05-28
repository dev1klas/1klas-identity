package config_test

import (
	"errors"
	"testing"

	"github.com/dev1klas/1klas-identity/internal/config"
)

func TestLoad_RejectsEmptyAllowedOrigins(t *testing.T) {
	t.Setenv("POSTGRES_URL", "postgres://x")
	t.Setenv("ALLOWED_ORIGINS", "")
	_, err := config.Load()
	if !errors.Is(err, config.ErrMissingAllowedOrigins) {
		t.Fatalf("want ErrMissingAllowedOrigins, got %v", err)
	}
}

func TestLoad_RejectsAllWhitespaceAllowedOrigins(t *testing.T) {
	t.Setenv("POSTGRES_URL", "postgres://x")
	t.Setenv("ALLOWED_ORIGINS", " , , ")
	_, err := config.Load()
	if !errors.Is(err, config.ErrMissingAllowedOrigins) {
		t.Fatalf("want ErrMissingAllowedOrigins, got %v", err)
	}
}

func TestLoad_ParsesAllowedOrigins(t *testing.T) {
	t.Setenv("POSTGRES_URL", "postgres://x")
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
