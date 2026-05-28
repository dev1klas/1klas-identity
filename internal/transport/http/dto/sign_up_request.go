package dto

// SignUpRequest is the inbound POST /sign-up body.
type SignUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
