package trace

import (
	"context"

	traceotel "go.opentelemetry.io/otel/trace"
)

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

// otelTracer implements the Tracer interface based on an OpenTelemetry
// Tracer.
type otelTracer struct {
	tracerOtel traceotel.Tracer
}

func NewOtelTracer(tracerOtel traceotel.Tracer) otelTracer {
	return otelTracer{tracerOtel: tracerOtel}
}

func (t otelTracer) Start(ctx context.Context, spanName string) (context.Context, Span) {
	ctx, spanOtel := t.tracerOtel.Start(ctx, spanName)
	return ctx, newOtelSpan(spanOtel)
}
