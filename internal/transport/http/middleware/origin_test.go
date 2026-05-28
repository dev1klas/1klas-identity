package middleware_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
)

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func newOriginReq(t *testing.T, headers map[string]string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://localhost/x", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestOriginCheck_MatchingOrigin_Passes(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), []string{"http://localhost:5173"})
	req := newOriginReq(t, map[string]string{"Origin": "http://localhost:5173"})
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestOriginCheck_MismatchingOrigin_Forbidden(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), []string{"http://localhost:5173"})
	req := newOriginReq(t, map[string]string{"Origin": "https://evil.example.com"})
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestOriginCheck_RefererFallback_Pass(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), []string{"http://localhost:5173"})
	req := newOriginReq(t, map[string]string{"Referer": "http://localhost:5173/some/path"})
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestOriginCheck_RefererFallback_Mismatch_Forbidden(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), []string{"http://localhost:5173"})
	req := newOriginReq(t, map[string]string{"Referer": "https://evil.example.com/foo"})
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestOriginCheck_MalformedReferer_Forbidden(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), []string{"http://localhost:5173"})
	req := newOriginReq(t, map[string]string{"Referer": "not-a-url"})
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestOriginCheck_MissingBoth_Forbidden(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), []string{"http://localhost:5173"})
	req := newOriginReq(t, nil)
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestOriginCheck_EmptyAllowList_Forbidden(t *testing.T) {
	mw := middleware.OriginCheck(newSilentLogger(), nil)
	req := newOriginReq(t, map[string]string{"Origin": "http://localhost:5173"})
	resp, _ := dispatchFast(t, mw, req, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

// TestOriginCheck_DocumentedServerStartContract — see config tests for the
// boot-time contract. This stub keeps the BDD-style contract discoverable.
func TestOriginCheck_DocumentedServerStartContract(_ *testing.T) {}
