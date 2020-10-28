package tele

import (
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
)

// Tracer returns the default opentelemetry tracer
func Tracer() trace.Tracer {
	return global.Tracer("capz")
}