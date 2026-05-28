package sign_up

import "github.com/dev1klas/1klas-identity/internal/domain/tenant"

// Input is the use case command for sign-up.
type Input struct {
	TenantID tenant.ID
	Email    string
	Password string
}
