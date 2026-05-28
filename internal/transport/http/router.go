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
	Recover   middleware.Middleware
	AccessLog middleware.Middleware
	Origin    middleware.Middleware
}

// NewMux builds the HTTP mux with the full middleware stack.
func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Public mutating routes: Recover + AccessLog + RequestID + Tenancy + Origin.
	// Origin enforcement is the walking-skeleton CSRF mitigation
	// (SPEC-identity §48-49). Recover is outermost so panics in any other
	// middleware are still trapped.
	publicMutatingChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		middleware.RequestID,
		middleware.Tenancy,
		d.Origin,
	)

	// Public read-only routes: no Origin check needed (SameSite=Lax + GETs are
	// safe by RFC; today we have none, but keep the chain available).
	publicReadChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		middleware.RequestID,
		middleware.Tenancy,
	)
	_ = publicReadChain // reserved for future public GETs.

	// Protected mutating routes: session + origin check.
	protectedMutatingChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		middleware.RequestID,
		middleware.Tenancy, // overwritten by Session on success
		middleware.Session(d.Sessions),
		d.Origin,
	)

	// Protected read routes: session only.
	protectedReadChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		middleware.RequestID,
		middleware.Tenancy, // overwritten by Session on success
		middleware.Session(d.Sessions),
	)

	// Infra endpoints get recover + access log (no tenant / no CSRF).
	infraChain := middleware.Chain(d.Recover, d.AccessLog)

	mux.Handle("/healthz", infraChain(http.HandlerFunc(handlers.Healthz)))
	mux.Handle("/openapi.json", infraChain(http.HandlerFunc(handlers.OpenAPI)))

	signUp := handlers.NewSignUpHandler(d.SignUp, d.Cookie)
	signIn := handlers.NewSignInHandler(d.SignIn, d.Cookie)
	signOut := handlers.NewSignOutHandler(d.SignOut, d.Cookie)
	getMe := handlers.NewGetMeHandler(d.GetMe)

	mux.Handle("POST /api/v1/crm/public/identity/sign-up", publicMutatingChain(signUp))
	mux.Handle("POST /api/v1/crm/public/identity/sessions", publicMutatingChain(signIn))
	mux.Handle("DELETE /api/v1/crm/public/identity/sessions/current", protectedMutatingChain(signOut))
	mux.Handle("GET /api/v1/crm/public/identity/profile/me", protectedReadChain(getMe))

	return mux
}
