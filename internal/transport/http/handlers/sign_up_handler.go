package handlers

import (
	"encoding/json"

	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/dto"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

// maxBodyBytes mirrors the previous MaxBytesReader cap.
const maxBodyBytes = 1 << 14

// SignUpHandler wires HTTP to the sign_up use case.
type SignUpHandler struct {
	uc     *sign_up.UseCase
	cookie cookies.Config
}

// NewSignUpHandler constructs the handler.
func NewSignUpHandler(uc *sign_up.UseCase, cookie cookies.Config) *SignUpHandler {
	return &SignUpHandler{uc: uc, cookie: cookie}
}

// Handle is the fasthttp request handler.
func (h *SignUpHandler) Handle(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) > maxBodyBytes {
		httperr.WriteFast(ctx, httperr.InvalidBody())
		return
	}

	var req dto.SignUpRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httperr.WriteFast(ctx, httperr.InvalidBody())
		return
	}

	out, err := h.uc.Execute(ctx, sign_up.Input{
		TenantID: mustTenant(ctx),
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httperr.WriteFast(ctx, httperr.FromSignUp(err))
		return
	}

	cookies.SetFast(ctx, h.cookie, out.SessionToken, out.SessionExpiresAt)
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusCreated)
	resp, _ := json.Marshal(dto.SignUpResponse{UserID: out.UserID.String()})
	ctx.SetBody(resp)
}
