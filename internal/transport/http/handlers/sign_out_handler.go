package handlers

import (
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	httperr "github.com/dev1klas/1klas-identity/internal/transport/http/errors"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
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

func (h *SignOutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := h.uc.Execute(r.Context(), sign_out.Input{
		TenantID:  middleware.TenantIDFrom(r.Context()),
		SessionID: middleware.SessionIDFrom(r.Context()),
		UserID:    middleware.UserIDFrom(r.Context()),
	}); err != nil {
		httperr.Write(w, httperr.FromSignOut(err))
		return
	}

	cookies.Clear(w, h.cookie)
	w.WriteHeader(http.StatusNoContent)
}
