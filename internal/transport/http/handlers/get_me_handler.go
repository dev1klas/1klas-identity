package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/transport/http/dto"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
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

func (h *GetMeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out, err := h.uc.Execute(r.Context(), get_me.Input{
		TenantID: middleware.TenantIDFrom(r.Context()),
		UserID:   middleware.UserIDFrom(r.Context()),
	})
	if err != nil {
		httperr.Write(w, httperr.FromGetMe(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto.MeResponse{
		UserID:    out.UserID.String(),
		Email:     out.Email,
		Status:    out.Status,
		CreatedAt: out.CreatedAt,
	})
}
