//go:build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/argon2id"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/tokens"
	"github.com/dev1klas/1klas-identity/internal/observability"
	"log/slog"
	"os"

	transport "github.com/dev1klas/1klas-identity/internal/transport/http"
	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

func bringUpServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	ctx := context.Background()

	c, err := tcpostgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		tcpostgres.WithDatabase("identity"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("dsn: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("pool: %v", err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		pool.Close()
		_ = c.Terminate(ctx)
		t.Fatalf("migrate: %v", err)
	}

	observability.InitTracing()

	uow := postgres.NewUnitOfWork(pool)
	userRepo := postgres.NewUserRepository(pool)
	sessRepo := postgres.NewSessionRepository(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)
	// Lighter argon2 for fast test runs.
	hasher := argon2id.New(argon2id.Params{MemoryKiB: 8 * 1024, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32})
	tg := tokens.New()
	clk := clock.Real{}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "identity-test")

	signUpUC := sign_up.New(uow, userRepo, sessRepo, outboxRepo, hasher, tg, clk, time.Hour)
	signInUC, err := sign_in.New(ctx, uow, userRepo, sessRepo, outboxRepo, hasher, tg, clk, time.Hour, logger)
	if err != nil {
		t.Fatalf("sign_in: %v", err)
	}
	signOutUC := sign_out.New(uow, sessRepo, outboxRepo, clk)
	getMeUC := get_me.New(userRepo)

	mux := transport.NewMux(transport.Deps{
		SignUp:    signUpUC,
		SignIn:    signInUC,
		SignOut:   signOutUC,
		GetMe:     getMeUC,
		Sessions:  sessRepo,
		Cookie:    cookies.Config{Secure: false}, // httptest uses HTTP
		Recover:   middleware.Recover(logger),
		AccessLog: middleware.AccessLog(logger),
		Origin:    middleware.OriginCheck(logger, []string{"http://localhost:5173"}),
	})

	srv := httptest.NewServer(mux)

	return srv, func() {
		srv.Close()
		pool.Close()
		_ = c.Terminate(ctx)
	}
}

func TestEndToEndFlow(t *testing.T) {
	t.Parallel()
	srv, cleanup := bringUpServer(t)
	t.Cleanup(cleanup)

	client := srv.Client()
	// httptest's default client follows redirects but doesn't keep cookies.
	jar := &simpleJar{}
	client.Jar = jar

	// 1. Sign-up
	resp, err := postJSON(client, srv.URL+"/api/v1/crm/public/identity/sign-up", `{"email":"alice@example.com","password":"correct horse battery"}`)
	if err != nil {
		t.Fatalf("sign-up: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("sign-up status = %d, body=%s", resp.StatusCode, mustRead(resp))
	}
	if !jar.has("session") {
		t.Fatalf("sign-up did not set session cookie; jar=%v", jar.cookies)
	}

	// 2. /profile/me
	resp, err = client.Get(srv.URL + "/api/v1/crm/public/identity/profile/me")
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get me status = %d, body=%s", resp.StatusCode, mustRead(resp))
	}
	var me struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.Email != "alice@example.com" {
		t.Fatalf("email = %q", me.Email)
	}

	// 3. Sign-out
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/crm/public/identity/sessions/current", nil)
	req.Header.Set("Origin", testOrigin)
	for _, c := range jar.cookies {
		req.AddCookie(c)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("sign-out: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("sign-out status = %d, body=%s", resp.StatusCode, mustRead(resp))
	}
	// Clear cookies cleared by server response.
	jar.absorb(resp)

	// 4. /profile/me again — must be 401.
	resp, err = client.Get(srv.URL + "/api/v1/crm/public/identity/profile/me")
	if err != nil {
		t.Fatalf("get me after sign-out: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("get me after sign-out status = %d (want 401), body=%s", resp.StatusCode, mustRead(resp))
	}

	// 5. Sign-in again — must work.
	resp, err = postJSON(client, srv.URL+"/api/v1/crm/public/identity/sessions", `{"email":"alice@example.com","password":"correct horse battery"}`)
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sign-in status = %d, body=%s", resp.StatusCode, mustRead(resp))
	}

	// 6. Duplicate sign-up must 409.
	resp, err = postJSON(client, srv.URL+"/api/v1/crm/public/identity/sign-up", `{"email":"alice@example.com","password":"correct horse battery"}`)
	if err != nil {
		t.Fatalf("dup sign-up: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("dup sign-up status = %d, body=%s", resp.StatusCode, mustRead(resp))
	}
}

func TestHealthzAndOpenAPI(t *testing.T) {
	t.Parallel()
	srv, cleanup := bringUpServer(t)
	t.Cleanup(cleanup)

	resp, err := srv.Client().Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}

	resp, err = srv.Client().Get(srv.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("openapi: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("openapi status = %d", resp.StatusCode)
	}
	body := mustRead(resp)
	if !strings.Contains(body, "1klas Identity API") {
		t.Fatalf("openapi body unexpected: %s", body[:200])
	}
}

const testOrigin = "http://localhost:5173"

func postJSON(c *http.Client, url, body string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	return c.Do(req)
}

func mustRead(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

// simpleJar is a deliberately tiny cookie jar — net/http/cookiejar requires a
// public-suffix list which is overkill for in-process testing.
type simpleJar struct {
	cookies []*http.Cookie
}

func (j *simpleJar) SetCookies(_ *url.URL, cs []*http.Cookie) {
	for _, c := range cs {
		j.replace(c)
	}
}

func (j *simpleJar) Cookies(_ *url.URL) []*http.Cookie { return j.activeCookies() }

func (j *simpleJar) replace(c *http.Cookie) {
	for i, ex := range j.cookies {
		if ex.Name == c.Name {
			j.cookies[i] = c
			return
		}
	}
	j.cookies = append(j.cookies, c)
}

func (j *simpleJar) activeCookies() []*http.Cookie {
	out := make([]*http.Cookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		if c.MaxAge < 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (j *simpleJar) has(name string) bool {
	for _, c := range j.activeCookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}

func (j *simpleJar) absorb(r *http.Response) {
	for _, c := range r.Cookies() {
		j.replace(c)
	}
}
