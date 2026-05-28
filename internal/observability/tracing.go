// Package observability wires OpenTelemetry. At walking skeleton the
// exporter is a no-op; the SDK is instrumented so spans propagate but
// nothing leaves the process.
package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// InitTracing installs a no-op TracerProvider.
func InitTracing() trace.TracerProvider {
	tp := noop.NewTracerProvider()
	otel.SetTracerProvider(tp)
	return tp
}
