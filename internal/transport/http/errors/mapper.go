// Package errors maps typed use-case errors to HTTP responses.
package errors

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

// Body is the wire shape of every 4xx/5xx response.
type Body struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Response is an HTTP error response in flight: status + stable code + msg.
type Response struct {
	Status  int
	Code    string
	Message string
}

// Predefined responses for ad-hoc transport-layer errors.

// InvalidBody covers JSON unmarshal failures + missing required fields.
func InvalidBody() Response {
	return Response{Status: http.StatusBadRequest, Code: "invalid_body", Message: "Invalid request body"}
}

// SessionRequired covers missing cookie on a protected route.
func SessionRequired() Response {
	return Response{Status: http.StatusUnauthorized, Code: "session_required", Message: "Authentication required"}
}

// SessionInvalid covers expired / revoked / unknown session token.
func SessionInvalid() Response {
	return Response{Status: http.StatusUnauthorized, Code: "session_invalid", Message: "Session is invalid"}
}

// Internal is the catch-all 500.
func Internal() Response {
	return Response{Status: http.StatusInternalServerError, Code: "internal", Message: "Internal server error"}
}

// Forbidden covers cross-origin / CSRF rejections on mutating routes.
func Forbidden() Response {
	return Response{Status: http.StatusForbidden, Code: "forbidden", Message: "Request forbidden"}
}

// FromSignUp maps a sign_up error to a Response.
func FromSignUp(err error) Response {
	switch {
	case errors.Is(err, sign_up.ErrInvalidEmail):
		return Response{Status: http.StatusBadRequest, Code: "invalid_email", Message: "Email is invalid"}
	case errors.Is(err, sign_up.ErrWeakPassword):
		return Response{Status: http.StatusBadRequest, Code: "weak_password", Message: "Password does not meet policy"}
	case errors.Is(err, sign_up.ErrEmailTaken):
		return Response{Status: http.StatusConflict, Code: "email_taken", Message: "Email already in use"}
	default:
		return Internal()
	}
}

// FromSignIn maps a sign_in error to a Response.
func FromSignIn(err error) Response {
	switch {
	// ErrInvalidEmail collapses into the same 401 / invalid_credentials as
	// ErrInvalidCredentials by design: surfacing a 400 here would leak whether
	// the address was even syntactically valid, an account-enumeration vector.
	case errors.Is(err, sign_in.ErrInvalidCredentials), errors.Is(err, sign_in.ErrInvalidEmail):
		return Response{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "Invalid credentials"}
	default:
		return Internal()
	}
}

// FromSignOut maps a sign_out error to a Response.
//
// sign_out has no recoverable domain errors today — every failure is an
// internal one. Sign-out is idempotent at the domain layer (already-revoked
// sessions return nil), so the caller never sees a "not found" path. If new
// branches are introduced in sign_out_errors.go, mirror FromSignUp / FromSignIn
// and add concrete errors.Is cases here.
func FromSignOut(_ error) Response {
	return Internal()
}

// FromGetMe maps a get_me error to a Response.
func FromGetMe(err error) Response {
	switch {
	case errors.Is(err, get_me.ErrNotFound):
		return Response{Status: http.StatusUnauthorized, Code: "session_invalid", Message: "Session is invalid"}
	default:
		return Internal()
	}
}

// Write serialises r to w.
func Write(w http.ResponseWriter, r Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.Status)
	var body Body
	body.Error.Code = r.Code
	body.Error.Message = r.Message
	_ = json.NewEncoder(w).Encode(body)
}
