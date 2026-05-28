package dto

import "time"

// MeResponse is the success body of GET /profile/me.
type MeResponse struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
