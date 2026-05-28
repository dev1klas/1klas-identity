package dto

// SignUpResponse is the success body of POST /sign-up.
type SignUpResponse struct {
	UserID string `json:"user_id"`
}
