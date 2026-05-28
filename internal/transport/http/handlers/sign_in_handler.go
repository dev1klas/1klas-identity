package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/dto"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
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

func (h *SignInHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req dto.SignInRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&req); err != nil {
		httperr.Write(w, httperr.InvalidBody())
		return
	}

	out, err := h.uc.Execute(r.Context(), sign_in.Input{
		TenantID: middleware.TenantIDFrom(r.Context()),
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httperr.Write(w, httperr.FromSignIn(err))
		return
	}

	cookies.Set(w, h.cookie, out.SessionToken, out.SessionExpiresAt)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.SignInResponse{UserID: out.UserID.String()})
}
