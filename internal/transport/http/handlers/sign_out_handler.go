package handlers

import (
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"
)

// SignOutHandler wires HTTP to the sign_out use case.
type SignOutHandler struct {
	uc     *sign_out.UseCase
	cookie cookies.Config
}

// NewSignOutHandler constructs the handler.
func NewSignOutHandler(uc *sign_out.UseCase, cookie cookies.Config) *SignOutHandler {
	return &SignOutHandler{uc: uc, cookie: cookie}
}

// Handle is the fasthttp request handler.
func (h *SignOutHandler) Handle(ctx *fasthttp.RequestCtx) {
	if _, err := h.uc.Execute(ctx, sign_out.Input{
		TenantID:     mustTenant(ctx),
		SessionID:    sessionIDOrNil(ctx),
		UserID:       userIDOrNil(ctx),
		TokenHashHex: tokenHashHex(ctx),
	}); err != nil {
		httperr.WriteFast(ctx, httperr.FromSignOut(err))
		return
	}

	cookies.ClearFast(ctx, h.cookie)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
