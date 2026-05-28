// Package cookies provides helpers for the session cookie. Two names are
// supported: __Host-session (production, requires TLS) and session
// (local-dev http only). The Secure attribute is driven from config.
package cookies

import (
	"net/http"
	"time"
)

const (
	// NameSecure is used in production. The __Host- prefix forbids the
	// Domain attribute, forbids non-Secure transport, and forbids non-"/"
	// Path.
	NameSecure = "__Host-session"
	// NameInsecure is used only in local non-TLS dev when COOKIE_SECURE=false.
	NameInsecure = "session"
)

// Config drives cookie attributes.
type Config struct {
	// Secure controls the Secure attribute AND the cookie name choice.
	Secure bool
}

// Name returns the cookie name appropriate for the config.
func (c Config) Name() string {
	if c.Secure {
		return NameSecure
	}
	return NameInsecure
}

// Set writes the session cookie with the given plaintext token and absolute
// expiry. Always HttpOnly + SameSite=Lax + Path=/.
func Set(w http.ResponseWriter, cfg Config, token string, expiresAt time.Time) {
	c := &http.Cookie{
		Name:     cfg.Name(),
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, c)
}

// Clear emits a Set-Cookie that overwrites the existing one with Max-Age=0.
func Clear(w http.ResponseWriter, cfg Config) {
	c := &http.Cookie{
		Name:     cfg.Name(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, c)
}

// ReadSessionCookie inspects r for either supported cookie name. Returns the
// raw value and true on hit.
func ReadSessionCookie(r *http.Request) (string, bool) {
	if c, err := r.Cookie(NameSecure); err == nil && c.Value != "" {
		return c.Value, true
	}
	if c, err := r.Cookie(NameInsecure); err == nil && c.Value != "" {
		return c.Value, true
	}
	return "", false
}
