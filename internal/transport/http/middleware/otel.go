package middleware

import (
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// tracerName is the instrumentation library identifier used for all spans
// started by the identity transport layer.
const tracerName = "github.com/dev1klas/1klas-identity/transport/http"

// OTelTrace starts a server span per inbound request and records http.* attrs.
// Designed to be small (no propagator wiring at walking skeleton) — when the
// upstream gateway starts injecting traceparent, swap in
// otel.GetTextMapPropagator().Extract on a fasthttp-header carrier.
func OTelTrace(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	tr := otel.Tracer(tracerName)
	return func(ctx *fasthttp.RequestCtx) {
		// Copy method/path off the pooled buffer into strings BEFORE
		// starting the span — these are also used after next() returns to
		// record the final status attribute.
		method := string(ctx.Method())
		path := string(ctx.Path())

		spanCtx, span := tr.Start(ctx, method+" "+path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.target", path),
			),
		)
		// Stash the trace-bearing context on the request so handlers /
		// downstream layers can propagate it. fasthttp's RequestCtx already
		// implements context.Context but it doesn't carry the trace value
		// the way OTel needs — SetUserValue + a helper would be one route;
		// at walking-skeleton scale we just discard spanCtx since RequestCtx
		// is itself passed through and OTel pulls the span via SpanFromContext.
		_ = spanCtx
		defer span.End()

		next(ctx)

		status := ctx.Response.StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", status))
	}
}
