//go:build integration

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/argon2id"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/tokens"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/valkey"
	"github.com/dev1klas/1klas-identity/internal/observability"

	transport "github.com/dev1klas/1klas-identity/internal/transport/http"
	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

// testServer is a tiny harness wrapping a fasthttp.Server bound to an
// in-memory listener. The client side talks over the same listener via a
// fasthttp HostClient.
type testServer struct {
	url    string
	client *fasthttp.Client
	close  func()
}

func bringUpServer(t *testing.T) *testServer {
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

	mr, err := miniredis.Run()
	if err != nil {
		pool.Close()
		_ = c.Terminate(ctx)
		t.Fatalf("miniredis: %v", err)
	}
	cache, err := valkey.New(valkey.Config{
		URL:         "redis://" + mr.Addr(),
		DialTimeout: 200 * time.Millisecond,
		OpTimeout:   500 * time.Millisecond,
	})
	if err != nil {
		mr.Close()
		pool.Close()
		_ = c.Terminate(ctx)
		t.Fatalf("valkey: %v", err)
	}

	observability.InitTracing()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "identity-test")

	uow := postgres.NewUnitOfWork(pool)
	userRepo := postgres.NewUserRepository(pool)
	sessRepo := postgres.NewSessionRepository(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)
	hasher := argon2id.New(argon2id.Params{MemoryKiB: 8 * 1024, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32})
	tg := tokens.New()
	clk := clock.Real{}

	signUpUC := sign_up.New(uow, userRepo, sessRepo, outboxRepo, cache, hasher, tg, clk, time.Hour, logger)
	signInUC, err := sign_in.New(ctx, uow, userRepo, sessRepo, outboxRepo, cache, hasher, tg, clk, time.Hour, logger)
	if err != nil {
		_ = cache.Close()
		mr.Close()
		pool.Close()
		_ = c.Terminate(ctx)
		t.Fatalf("sign_in: %v", err)
	}
	signOutUC := sign_out.New(uow, sessRepo, outboxRepo, cache, clk, logger)
	getMeUC := get_me.New(userRepo)

	handler := transport.NewHandler(transport.Deps{
		SignUp:    signUpUC,
		SignIn:    signInUC,
		SignOut:   signOutUC,
		GetMe:     getMeUC,
		Sessions:  sessRepo,
		Cache:     cache,
		Cookie:    cookies.Config{Secure: false}, // local HTTP
		Recover:   middleware.Recover(logger),
		AccessLog: middleware.AccessLog(logger),
		Origin:    middleware.OriginCheck(logger, []string{testOrigin}),
		OTel:      middleware.OTelTrace,
		SessionMW: middleware.Session(sessRepo, cache, logger),
	})

	ln := fasthttputil.NewInmemoryListener()
	srv := &fasthttp.Server{Handler: handler}
	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.Serve(ln) }()

	client := &fasthttp.Client{
		Dial: func(_ string) (net.Conn, error) { return ln.Dial() },
	}

	return &testServer{
		url:    "http://localhost",
		client: client,
		close: func() {
			_ = ln.Close()
			<-srvErr
			_ = cache.Close()
			mr.Close()
			pool.Close()
			_ = c.Terminate(ctx)
		},
	}
}

const testOrigin = "http://localhost:5173"

func TestEndToEndFlow(t *testing.T) {
	t.Parallel()
	srv := bringUpServer(t)
	t.Cleanup(srv.close)

	jar := newCookieJar()

	// 1. Sign-up
	resp, body := srv.do(t, fasthttp.MethodPost, "/api/v1/crm/public/identity/sign-up",
		`{"email":"alice@example.com","password":"correct horse battery"}`, jar)
	if resp.StatusCode() != fasthttp.StatusCreated {
		t.Fatalf("sign-up status = %d, body=%s", resp.StatusCode(), body)
	}
	if !jar.has("session") {
		t.Fatalf("sign-up did not set session cookie; jar=%v", jar.names())
	}

	// 2. /profile/me
	resp, body = srv.do(t, fasthttp.MethodGet, "/api/v1/crm/public/identity/profile/me", "", jar)
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("get me status = %d, body=%s", resp.StatusCode(), body)
	}
	var me struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal([]byte(body), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.Email != "alice@example.com" {
		t.Fatalf("email = %q", me.Email)
	}

	// 3. Sign-out
	resp, body = srv.do(t, fasthttp.MethodDelete, "/api/v1/crm/public/identity/sessions/current", "", jar)
	if resp.StatusCode() != fasthttp.StatusNoContent {
		t.Fatalf("sign-out status = %d, body=%s", resp.StatusCode(), body)
	}

	// 4. /profile/me again — must be 401.
	resp, body = srv.do(t, fasthttp.MethodGet, "/api/v1/crm/public/identity/profile/me", "", jar)
	if resp.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("get me after sign-out status = %d (want 401), body=%s", resp.StatusCode(), body)
	}

	// 5. Sign-in again — must work.
	resp, body = srv.do(t, fasthttp.MethodPost, "/api/v1/crm/public/identity/sessions",
		`{"email":"alice@example.com","password":"correct horse battery"}`, jar)
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("sign-in status = %d, body=%s", resp.StatusCode(), body)
	}

	// 6. Duplicate sign-up must 409.
	resp, body = srv.do(t, fasthttp.MethodPost, "/api/v1/crm/public/identity/sign-up",
		`{"email":"alice@example.com","password":"correct horse battery"}`, jar)
	if resp.StatusCode() != fasthttp.StatusConflict {
		t.Fatalf("dup sign-up status = %d, body=%s", resp.StatusCode(), body)
	}
}

func TestHealthzAndOpenAPI(t *testing.T) {
	t.Parallel()
	srv := bringUpServer(t)
	t.Cleanup(srv.close)

	jar := newCookieJar()
	resp, body := srv.do(t, fasthttp.MethodGet, "/healthz", "", jar)
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode())
	}
	_ = body

	resp, body = srv.do(t, fasthttp.MethodGet, "/openapi.json", "", jar)
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("openapi status = %d", resp.StatusCode())
	}
	if !strings.Contains(body, "1klas Identity API") {
		t.Fatalf("openapi body unexpected: %s", body[:min(200, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// do issues a request through the in-memory listener. It writes Origin +
// Cookie + Content-Type as appropriate.
func (s *testServer) do(t *testing.T, method, path, body string, jar *cookieJar) (*fasthttp.Response, string) {
	t.Helper()
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(s.url + path)
	req.Header.SetMethod(method)
	req.Header.Set("Origin", testOrigin)
	if body != "" {
		req.Header.SetContentType("application/json")
		req.SetBodyString(body)
	}
	for _, c := range jar.snapshot() {
		req.Header.SetCookie(c.name, c.value)
	}

	if err := s.client.DoTimeout(req, resp, 30*time.Second); err != nil {
		t.Fatalf("client do: %v", err)
	}
	// Absorb Set-Cookie response headers into the jar.
	resp.Header.VisitAllCookie(func(k, v []byte) {
		cc := fasthttp.AcquireCookie()
		defer fasthttp.ReleaseCookie(cc)
		if err := cc.ParseBytes(v); err != nil {
			return
		}
		jar.set(string(cc.Key()), string(cc.Value()), cc.MaxAge())
		_ = k
	})

	// Read body into a local string before releasing resp.
	bodyBuf := bytes.NewBuffer(nil)
	_, _ = bodyBuf.Write(resp.Body())
	out := bodyBuf.String()
	// We return the response object for status; release after caller looks at it.
	// Use a copy of the status code to avoid keeping resp around past release.
	statusCopy := resp.StatusCode()
	fasthttp.ReleaseResponse(resp)
	// Build a shim response just for status.
	shim := &fasthttp.Response{}
	shim.SetStatusCode(statusCopy)
	return shim, out
}

// cookieJar is a tiny name->value store with MaxAge tracking. Set-Cookie with
// MaxAge<0 deletes the entry.
type cookieJar struct {
	byName map[string]string
}

type cookiePair struct {
	name  string
	value string
}

func newCookieJar() *cookieJar { return &cookieJar{byName: map[string]string{}} }

func (j *cookieJar) set(name, value string, maxAge int) {
	if maxAge < 0 || value == "" {
		delete(j.byName, name)
		return
	}
	j.byName[name] = value
}

func (j *cookieJar) snapshot() []cookiePair {
	out := make([]cookiePair, 0, len(j.byName))
	for k, v := range j.byName {
		out = append(out, cookiePair{name: k, value: v})
	}
	return out
}

func (j *cookieJar) has(name string) bool {
	_, ok := j.byName[name]
	return ok
}

func (j *cookieJar) names() []string {
	out := make([]string, 0, len(j.byName))
	for k := range j.byName {
		out = append(out, k)
	}
	return out
}

// Silence unused imports in some build matrices.
var (
	_ = io.Discard
	_ = http.MethodGet
	_ = errors.New
)
