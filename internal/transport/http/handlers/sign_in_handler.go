package handlers

import (
	"encoding/json"

	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/dto"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
)

// SignInHandler wires HTTP to the sign_in use case.
type SignInHandler struct {
	uc     *sign_in.UseCase
	cookie cookies.Config
}

// NewSignInHandler constructs the handler.
func NewSignInHandler(uc *sign_in.UseCase, cookie cookies.Config) *SignInHandler {
	return &SignInHandler{uc: uc, cookie: cookie}
}

// Handle is the fasthttp request handler.
func (h *SignInHandler) Handle(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) > maxBodyBytes {
		httperr.WriteFast(ctx, httperr.InvalidBody())
		return
	}

	var req dto.SignInRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httperr.WriteFast(ctx, httperr.InvalidBody())
		return
	}

	out, err := h.uc.Execute(ctx, sign_in.Input{
		TenantID: mustTenant(ctx),
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httperr.WriteFast(ctx, httperr.FromSignIn(err))
		return
	}

	cookies.SetFast(ctx, h.cookie, out.SessionToken, out.SessionExpiresAt)
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	resp, _ := json.Marshal(dto.SignInResponse{UserID: out.UserID.String()})
	ctx.SetBody(resp)
}
