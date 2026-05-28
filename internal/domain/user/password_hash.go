package user

import "context"

const (
	// PasswordMinLen is the policy floor for raw passwords.
	PasswordMinLen = 12
	// PasswordMaxLen guards against argon2id DoS by huge inputs.
	PasswordMaxLen = 256
)

// PasswordHash is an opaque, encoded password hash in PHC string format
// (e.g. "$argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>"). It is treated as
// a black-box string by the domain — verification logic lives in
// infrastructure.
type PasswordHash struct {
	encoded string
}

// NewPasswordHash wraps an already-encoded PHC string.
func NewPasswordHash(encoded string) PasswordHash {
	return PasswordHash{encoded: encoded}
}

// String returns the encoded PHC form. Safe to write to the database.
func (p PasswordHash) String() string { return p.encoded }

// IsZero reports whether the hash is unset.
func (p PasswordHash) IsZero() bool { return p.encoded == "" }

// PasswordHasher turns a raw password into a PasswordHash and verifies a raw
// password against a stored hash. The implementation lives in
// internal/infrastructure/argon2id.
type PasswordHasher interface {
	// Hash returns the encoded PHC string for the given raw password.
	Hash(ctx context.Context, raw string) (PasswordHash, error)
	// Verify returns true iff raw matches hash. Implementations MUST run
	// in constant time per input length and MUST always execute the
	// expensive KDF (even on a dummy hash) to avoid timing leaks.
	Verify(ctx context.Context, hash PasswordHash, raw string) (bool, error)
}

// ValidatePasswordPolicy enforces the raw-password length policy. Returns
// ErrWeakPassword if outside [PasswordMinLen, PasswordMaxLen].
func ValidatePasswordPolicy(raw string) error {
	if len(raw) < PasswordMinLen || len(raw) > PasswordMaxLen {
		return ErrWeakPassword
	}
	return nil
}
