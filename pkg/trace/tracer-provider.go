package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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

// Tracer is responsible for creating trace Spans.
type Tracer interface {
	// Start creates a span and a context.Context containing the newly-created
	// span.
	//
	// If the context.Context provided in `ctx` contains a Span then the
	// newly-created Span will be a child of that span, otherwise it will be a
	// root span.
	Start(ctx context.Context, spanName string) (context.Context, Span)
}

// Span represents a single operation within a trace.
type Span interface {
	// End completes the Span. The Span is considered complete and ready to be
	// delivered through the rest of the telemetry pipeline after this method
	// is called. Therefore, updates to the Span are not allowed after this
	// method has been called.
	End()

	// RecordError will record err as an exception span event for this span. An
	// additional call to SetStatus is required if the Status of the Span should
	// be set to Error, as this method does not change the Span status. If this
	// span is not being recorded or err is nil then this method does nothing.
	RecordError(err error)

	// SetStatus sets the status of the Span in the form of a code and a
	// description, overriding previous values set. The description is only
	// included in a status when the code is for an error.
	SetStatus(code codes.Code, description string)
}

// NewTracerProvider creates a TracerProvider of a given type, and
// optionally sets the new TracerProvider as the global TracerProvider.
func NewTracerProvider(
	ctx context.Context,
	providerType TracerProviderType,
	collectorURL string,
	setGlobalProvider bool) (TracerProvider, error) {

	var tp TracerProvider
	var err error

	switch providerType {
	case NoopProviderType:
		// TODO: Add Noop TracerProvider code
	case ConsoleProviderType:
		// TODO: Add Console TracerProvider code
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
	return newJaegerTracer(tracer)
}
