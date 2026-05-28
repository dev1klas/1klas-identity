// Package http — transport surface for the identity service.
package http

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

// WriteJSON marshals v to JSON and writes it as the response body, setting
// Content-Type and status. Marshal errors degrade to a generic 500 — callers
// pre-validate inputs and own DTOs, so this should be unreachable in
// practice. Exported so handler packages can reuse it.
func WriteJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		ctx.Response.Header.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString(`{"error":{"code":"internal","message":"Internal server error"}}`)
		return
	}
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(status)
	ctx.SetBody(buf)
}
