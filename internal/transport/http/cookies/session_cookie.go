// Package cookies provides helpers for the session cookie. Two names are
// supported: __Host-session (production, requires TLS) and session
// (local-dev http only). The Secure attribute is driven from config.
package cookies

import (
	"time"

	"github.com/valyala/fasthttp"
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

// SetFast writes the session cookie with the given plaintext token and
// absolute expiry. Always HttpOnly + SameSite=Lax + Path=/.
func SetFast(ctx *fasthttp.RequestCtx, cfg Config, token string, expiresAt time.Time) {
	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKey(cfg.Name())
	c.SetValue(token)
	c.SetPath("/")
	c.SetExpire(expiresAt)
	c.SetMaxAge(int(time.Until(expiresAt).Seconds()))
	c.SetHTTPOnly(true)
	c.SetSecure(cfg.Secure)
	c.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	ctx.Response.Header.SetCookie(c)
}

// ClearFast emits a Set-Cookie that overwrites the existing one with
// Max-Age=0 / Expires in the past.
func ClearFast(ctx *fasthttp.RequestCtx, cfg Config) {
	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKey(cfg.Name())
	c.SetValue("")
	c.SetPath("/")
	// fasthttp.CookieExpireDelete is the canonical "delete me" sentinel.
	c.SetExpire(fasthttp.CookieExpireDelete)
	c.SetMaxAge(-1)
	c.SetHTTPOnly(true)
	c.SetSecure(cfg.Secure)
	c.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	ctx.Response.Header.SetCookie(c)
}

// ReadSessionCookieFast inspects ctx for either supported cookie name.
// Returns the raw value (as a copy — safe to keep past handler return) and
// true on hit.
func ReadSessionCookieFast(ctx *fasthttp.RequestCtx) (string, bool) {
	if v := ctx.Request.Header.Cookie(NameSecure); len(v) > 0 {
		return string(v), true
	}
	if v := ctx.Request.Header.Cookie(NameInsecure); len(v) > 0 {
		return string(v), true
	}
	return "", false
}
