package sign_in

import "github.com/dev1klas/1klas-identity/internal/domain/tenant"

// Input is the use case command for sign-in.
type Input struct {
	TenantID tenant.ID
	Email    string
	Password string
}
