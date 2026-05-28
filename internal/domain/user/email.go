package user

import (
	"regexp"
	"strings"
)

// emailRegex is a deliberately conservative RFC-leaning regex. The cap on
// length is enforced separately (max 254 per RFC 5321).
var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)

const emailMaxLen = 254

// Email is a normalised, validated email address value object.
// Always lower-cased and trimmed.
type Email struct {
	value string
}

// NewEmail validates and normalises s into an Email. Returns ErrInvalidEmail
// if s does not pass validation.
func NewEmail(s string) (Email, error) {
	v := strings.TrimSpace(strings.ToLower(s))
	if v == "" || len(v) > emailMaxLen {
		return Email{}, ErrInvalidEmail
	}
	if !emailRegex.MatchString(v) {
		return Email{}, ErrInvalidEmail
	}
	return Email{value: v}, nil
}

// String returns the canonical (normalised) email.
func (e Email) String() string { return e.value }
