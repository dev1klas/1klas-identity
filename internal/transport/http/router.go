// Package http wires the HTTP surface of the identity service on fasthttp.
package http

import (
	"github.com/valyala/fasthttp"

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
	Cache     session.Cache
	Cookie    cookies.Config
	Recover   middleware.Middleware
	AccessLog middleware.Middleware
	Origin    middleware.Middleware
	OTel      middleware.Middleware
	SessionMW middleware.Middleware
}

// NewHandler returns a single fasthttp.RequestHandler that dispatches on
// method + path. fasthttp has no built-in ServeMux so we route by hand —
// the route table is small enough at walking skeleton to keep here.
func NewHandler(d Deps) fasthttp.RequestHandler {
	// Public mutating routes: Recover + AccessLog + OTel + RequestID + Tenancy + Origin.
	// Origin enforcement is the walking-skeleton CSRF mitigation
	// (SPEC-identity §48-49). Recover is outermost so panics in any other
	// middleware are still trapped.
	publicMutatingChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		d.OTel,
		middleware.RequestID,
		middleware.Tenancy,
		d.Origin,
	)

	// Public read-only routes: no Origin check needed (SameSite=Lax + GETs are
	// safe by RFC; today we have none, but keep the chain available).
	publicReadChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		d.OTel,
		middleware.RequestID,
		middleware.Tenancy,
	)
	_ = publicReadChain // reserved for future public GETs.

	// Protected mutating routes: session + origin check.
	protectedMutatingChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		d.OTel,
		middleware.RequestID,
		middleware.Tenancy, // injects DefaultID; Session overrides it only on successful auth
		d.SessionMW,
		d.Origin,
	)

	// Protected read routes: session only.
	protectedReadChain := middleware.Chain(
		d.Recover,
		d.AccessLog,
		d.OTel,
		middleware.RequestID,
		middleware.Tenancy, // injects DefaultID; Session overrides it only on successful auth
		d.SessionMW,
	)

	// Infra endpoints get recover + access log (no tenant / no CSRF).
	infraChain := middleware.Chain(d.Recover, d.AccessLog)

	signUpH := handlers.NewSignUpHandler(d.SignUp, d.Cookie)
	signInH := handlers.NewSignInHandler(d.SignIn, d.Cookie)
	signOutH := handlers.NewSignOutHandler(d.SignOut, d.Cookie)
	getMeH := handlers.NewGetMeHandler(d.GetMe)

	signUpRoute := publicMutatingChain(signUpH.Handle)
	signInRoute := publicMutatingChain(signInH.Handle)
	signOutRoute := protectedMutatingChain(signOutH.Handle)
	getMeRoute := protectedReadChain(getMeH.Handle)
	healthzRoute := infraChain(handlers.Healthz)
	openapiRoute := infraChain(handlers.OpenAPI)

	return func(ctx *fasthttp.RequestCtx) {
		method := string(ctx.Method())
		path := string(ctx.Path())

		switch {
		case method == fasthttp.MethodGet && path == "/healthz":
			healthzRoute(ctx)
		case method == fasthttp.MethodGet && path == "/openapi.json":
			openapiRoute(ctx)
		case method == fasthttp.MethodPost && path == "/api/v1/crm/public/identity/sign-up":
			signUpRoute(ctx)
		case method == fasthttp.MethodPost && path == "/api/v1/crm/public/identity/sessions":
			signInRoute(ctx)
		case method == fasthttp.MethodDelete && path == "/api/v1/crm/public/identity/sessions/current":
			signOutRoute(ctx)
		case method == fasthttp.MethodGet && path == "/api/v1/crm/public/identity/profile/me":
			getMeRoute(ctx)
		default:
			WriteJSON(ctx, fasthttp.StatusNotFound, map[string]any{
				"error": map[string]string{
					"code":    "not_found",
					"message": "Route not found",
				},
			})
		}
	}
}
