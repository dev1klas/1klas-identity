// Package http wires the HTTP surface of the identity service.
package http

import (
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/transport/http/handlers"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
)

// Deps bundles everything the router needs.
type Deps struct {
	SignUp    *sign_up.UseCase
	SignIn    *sign_in.UseCase
	SignOut   *sign_out.UseCase
	GetMe     *get_me.UseCase
	Sessions  session.Repository
	Cookie    cookies.Config
	AccessLog middleware.Middleware
}

// NewMux builds the HTTP mux with the full middleware stack.
func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Public (no session) routes get the public chain.
	publicChain := middleware.Chain(
		d.AccessLog,
		middleware.RequestID,
		middleware.Tenancy,
	)

	// Protected routes additionally require a valid session cookie.
	protectedChain := middleware.Chain(
		d.AccessLog,
		middleware.RequestID,
		middleware.Tenancy, // overwritten by Session on success
		middleware.Session(d.Sessions),
	)

	// Infra endpoints get only access log.
	logOnly := middleware.Chain(d.AccessLog)

	mux.Handle("/healthz", logOnly(http.HandlerFunc(handlers.Healthz)))
	mux.Handle("/openapi.json", logOnly(http.HandlerFunc(handlers.OpenAPI)))

	signUp := handlers.NewSignUpHandler(d.SignUp, d.Cookie)
	signIn := handlers.NewSignInHandler(d.SignIn, d.Cookie)
	signOut := handlers.NewSignOutHandler(d.SignOut, d.Cookie)
	getMe := handlers.NewGetMeHandler(d.GetMe)

	mux.Handle("POST /api/v1/crm/public/identity/sign-up", publicChain(signUp))
	mux.Handle("POST /api/v1/crm/public/identity/sessions", publicChain(signIn))
	mux.Handle("DELETE /api/v1/crm/public/identity/sessions/current", protectedChain(signOut))
	mux.Handle("GET /api/v1/crm/public/identity/profile/me", protectedChain(getMe))

	return mux
}
