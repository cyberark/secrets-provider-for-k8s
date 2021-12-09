package main

import (
	"context"
	"os"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/annotations"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/trace"
)

func getTracerConfig() (trace.TracerProviderType, string) {
	// First try to get the tracer config from annotations
	log.Debug("Getting tracer config from annotations")
	traceType, jaegerUrl, err := getTracerConfigFromAnnotations()

	// If no tracer is specified in annotations, get it from environment variables
	if err != nil || traceType == trace.NoopProviderType {
		log.Debug("Getting tracer config from environment variables")
		traceType, jaegerUrl = getTracerConfigFromEnv()
	}

	log.Debug("Tracer config: ", traceType, jaegerUrl)
	return traceType, jaegerUrl
}

func getTracerConfigFromEnv() (trace.TracerProviderType, string) {
	jaegerURL := os.Getenv("JAEGER_COLLECTOR_URL")
	if jaegerURL != "" {
		return trace.JaegerProviderType, jaegerURL
	}
	if os.Getenv("LOG_TRACES") == "true" {
		return trace.ConsoleProviderType, ""
	}
	return trace.NoopProviderType, ""
}

func getTracerConfigFromAnnotations() (trace.TracerProviderType, string, error) {
	annotationsMap, err := annotations.NewAnnotationsFromFile(annotationsFilePath)
	if err != nil {
		return trace.NoopProviderType, "", err
	}
	var jaegerURL string = annotationsMap[envAnnotationsConversion["JAEGER_COLLECTOR_URL"]]
	if jaegerURL != "" {
		return trace.JaegerProviderType, jaegerURL, nil
	}

	if annotationsMap[envAnnotationsConversion["LOG_TRACES"]] == "true" {
		return trace.ConsoleProviderType, "", nil
	}
	return trace.NoopProviderType, "", nil
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
	tp, err := trace.NewTracerProvider(tracerType, tracerURL, os.Stdout, trace.SetGlobalProvider)

	if err != nil {
		log.Error(err.Error())
		return ctx, nil, cleanupFunc, err
	}

	// Create a Tracer and a top-level trace Span
	tracer := tp.Tracer(tracerName)
	ctx, span = tracer.Start(ctx, "main")
	return ctx, tracer, cleanupFunc, err
}
