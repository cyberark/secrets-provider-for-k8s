package trace

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	traceotel "go.opentelemetry.io/otel/trace"
)

// jaegerTracerProvider implements the TracerProvider interface using
// a Jaeger exporter.
type jaegerTracerProvider struct {
	providerSDK *tracesdk.TracerProvider
}

func newJaegerTracerProvider(url string) (TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	providerSDK := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(tracerService),
			attribute.String("environment", tracerEnvironment),
			attribute.Int64("ID", tracerID),
		)),
	)
	tp := jaegerTracerProvider{providerSDK: providerSDK}
	return &tp, nil
}

func (tp *jaegerTracerProvider) Tracer(name string) Tracer {
	return newJaegerTracer(tp.providerSDK.Tracer(tracerName))
}

func (tp *jaegerTracerProvider) Shutdown(ctx context.Context) error {
	// Do not make the application hang when it is shutdown.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	return tp.providerSDK.Shutdown(ctx)
}

func (tp *jaegerTracerProvider) SetGlobalTracerProvider() {
	otel.SetTracerProvider(tp.providerSDK)
}

// jaegerTracer implements the Tracer interface for a Jaeger exporter.
type jaegerTracer struct {
	tracerOtel traceotel.Tracer
}

func newJaegerTracer(tracerOtel traceotel.Tracer) jaegerTracer {
	return jaegerTracer{tracerOtel: tracerOtel}
}

func (t jaegerTracer) Start(ctx context.Context, spanName string) (context.Context, Span) {
	ctx, spanOtel := t.tracerOtel.Start(ctx, spanName)
	return ctx, newJaegerSpan(spanOtel)
}

// jaegerSpan implements the Span interface for a Jaeger exporter.
type jaegerSpan struct {
	spanOtel traceotel.Span
}

func newJaegerSpan(spanOtel traceotel.Span) jaegerSpan {
	return jaegerSpan{spanOtel: spanOtel}
}

func (s jaegerSpan) End() {
	s.spanOtel.End()
}

func (s jaegerSpan) RecordError(err error) {
	s.spanOtel.RecordError(err)
}

func (s jaegerSpan) SetStatus(code codes.Code, description string) {
	s.spanOtel.SetStatus(code, description)
}
