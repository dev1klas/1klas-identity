package dto

// SignInResponse is the success body of POST /sessions.
type SignInResponse struct {
	UserID string `json:"user_id"`
}
