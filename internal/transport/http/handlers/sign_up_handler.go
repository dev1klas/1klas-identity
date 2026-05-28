package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/dto"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

// SignUpHandler wires HTTP to the sign_up use case.
type SignUpHandler struct {
	uc     *sign_up.UseCase
	cookie cookies.Config
}

// NewSignUpHandler constructs the handler.
func NewSignUpHandler(uc *sign_up.UseCase, cookie cookies.Config) *SignUpHandler {
	return &SignUpHandler{uc: uc, cookie: cookie}
}

func (h *SignUpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req dto.SignUpRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&req); err != nil {
		httperr.Write(w, httperr.InvalidBody())
		return
	}

	out, err := h.uc.Execute(r.Context(), sign_up.Input{
		TenantID: middleware.TenantIDFrom(r.Context()),
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httperr.Write(w, httperr.FromSignUp(err))
		return
	}

	cookies.Set(w, h.cookie, out.SessionToken, out.SessionExpiresAt)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(dto.SignUpResponse{UserID: out.UserID.String()})
}
