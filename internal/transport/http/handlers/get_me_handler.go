package handlers

import (
	"encoding/json"

	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/transport/http/dto"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
)

// GetMeHandler wires HTTP to the get_me use case.
type GetMeHandler struct {
	uc *get_me.UseCase
}

// NewGetMeHandler constructs the handler.
func NewGetMeHandler(uc *get_me.UseCase) *GetMeHandler {
	return &GetMeHandler{uc: uc}
}

// Handle is the fasthttp request handler.
func (h *GetMeHandler) Handle(ctx *fasthttp.RequestCtx) {
	out, err := h.uc.Execute(ctx, get_me.Input{
		TenantID: mustTenant(ctx),
		UserID:   userIDOrNil(ctx),
	})
	if err != nil {
		httperr.WriteFast(ctx, httperr.FromGetMe(err))
		return
	}

	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	resp, _ := json.Marshal(dto.MeResponse{
		UserID:    out.UserID.String(),
		Email:     out.Email,
		Status:    out.Status,
		CreatedAt: out.CreatedAt,
	})
	ctx.SetBody(resp)
}
