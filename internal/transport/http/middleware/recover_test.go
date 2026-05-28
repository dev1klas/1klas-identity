package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
)

func TestRecover_HandlerPanic_Returns500AndLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := middleware.Recover(logger)
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	resp := rec.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	logged := buf.String()
	if !strings.Contains(logged, "panic recovered") {
		t.Fatalf("expected log line for panic; got %q", logged)
	}
	if !strings.Contains(logged, "boom") {
		t.Fatalf("expected panic value in log; got %q", logged)
	}
}

func TestRecover_NoPanic_PassesThrough(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	mw := middleware.Recover(logger)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Result().StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Result().StatusCode)
	}
}
