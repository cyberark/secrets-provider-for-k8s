package trace

import (
	"context"
	"fmt"
	"io"

	"go.opentelemetry.io/otel"
)

// TracerProviderType represents a type of TracerProvider
type TracerProviderType int64

// Valid values for TracerProviderType
const (
	NoopProviderType = iota
	ConsoleProviderType
	JaegerProviderType
)

// Boolean flags for indicating whether a TracerProvider should be set as
// the global TracerProvider
const (
	SetGlobalProvider     = true
	DontSetGlobalProvider = false
)

const (
	tracerName        = "secrets-provider"
	tracerService     = "secrets-provider"
	tracerEnvironment = "production"
	tracerID          = 1
)

// TracerProvider provides access to Tracers, which in turn allow for creation
// of trace Spans.
type TracerProvider interface {
	// Tracer creates an implementation of the Tracer interface.
	Tracer(tracerName string) Tracer
	// Shutdown and flush telemetry.
	Shutdown(ctx context.Context) error
	SetGlobalTracerProvider()
}

// NewTracerProvider creates a TracerProvider of a given type, and
// optionally sets the new TracerProvider as the global TracerProvider.
func NewTracerProvider(
	providerType TracerProviderType,
	collectorURL string,
	consoleWriter io.Writer,
	setGlobalProvider bool) (TracerProvider, error) {

	var tp TracerProvider
	var err error

	switch providerType {
	case NoopProviderType:
		tp = newNoopTracerProvider()
	case ConsoleProviderType:
		tp, err = newConsoleTracerProvider(consoleWriter)
	case JaegerProviderType:
		tp, err = newJaegerTracerProvider(collectorURL)
	default:
		err = fmt.Errorf("invalid TracerProviderType '%d' in call to NewTracerProvider",
			providerType)
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	if setGlobalProvider {
		tp.SetGlobalTracerProvider()
	}

	return tp, nil
}

// GlobalTracer returns a Tracer using the registered global trace provider.
func GlobalTracer(tracerName string) Tracer {
	tracer := otel.Tracer(tracerName)
	return NewOtelTracer(tracer)
}
