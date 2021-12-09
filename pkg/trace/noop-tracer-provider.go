package trace

import (
	"context"

	"go.opentelemetry.io/otel"
	traceotel "go.opentelemetry.io/otel/trace"
)

// noopTracerProvider implements the TracerProvider interface using
// a OpenTelemetry TracerProvider that does nothing for all trace operations.
type noopTracerProvider struct {
	providerOtel traceotel.TracerProvider
}

func newNoopTracerProvider() TracerProvider {
	// Create a Noop exporter
	providerOtel := traceotel.NewNoopTracerProvider()
	tp := noopTracerProvider{providerOtel: providerOtel}
	return &tp
}

func (tp *noopTracerProvider) Tracer(name string) Tracer {
	return NewOtelTracer(tp.providerOtel.Tracer(name))
}

func (tp *noopTracerProvider) Shutdown(ctx context.Context) error {
	return nil
}

func (tp *noopTracerProvider) SetGlobalTracerProvider() {
	otel.SetTracerProvider(tp.providerOtel)
}
