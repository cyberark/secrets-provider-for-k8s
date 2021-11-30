package main

import (
	"context"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/trace"
)

func getTracerConfig() (trace.TracerProviderType, string) {
	var traceType trace.TracerProviderType
	jaegerURL := os.Getenv("JAEGER_COLLECTOR_URL")
	if jaegerURL != "" {
		traceType = trace.JaegerProviderType
	} else if os.Getenv("LOG_TRACES") == "true" {
		traceType = trace.ConsoleProviderType
	} else {
		traceType = trace.NoopProviderType
	}
	return traceType, jaegerURL
}

// Create a TracerProvider, Tracer, and top-level (parent) Span
func createTracer(tracerType trace.TracerProviderType,
	tracerURL string) (context.Context, trace.Tracer, func(context.Context), error) {

	var tp trace.TracerProvider
	var span trace.Span

	// Create a background context for tracing
	ctx, cancel := context.WithCancel(context.Background())

	cleanupFunc := func(ctx context.Context) {
		if span != nil {
			span.End()
		}
		if tp != nil {
			tp.Shutdown(ctx)
		}
		cancel()
	}

	// Create a TracerProvider
	tp, err := trace.NewTracerProvider(ctx, tracerType, tracerURL, trace.SetGlobalProvider)

	if err != nil {
		log.Error(err.Error())
		return ctx, nil, cleanupFunc, err
	}

	// Create a Tracer and a top-level trace Span
	tracer := tp.Tracer(tracerName)
	ctx, span = tracer.Start(ctx, "main")
	return ctx, tracer, cleanupFunc, err
}
