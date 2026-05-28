package handlers

import "github.com/valyala/fasthttp"

// Healthz is the liveness probe.
func Healthz(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBodyString(`{"status":"ok"}`)
}
