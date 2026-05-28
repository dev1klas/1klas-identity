package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
)

func TestRecover_HandlerPanic_Returns500AndLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mw := middleware.Recover(logger)
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)

	resp, _ := dispatchFast(t, mw, req, func(_ *fasthttp.RequestCtx) {
		panic("boom")
	})

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
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/x", nil)

	resp, _ := dispatchFast(t, mw, req, func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusTeapot)
	})

	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", resp.StatusCode)
	}
}
