package dto

// SignInRequest is the inbound POST /sessions body.
type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
