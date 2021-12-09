package trace

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

// consoleTracerProvider implements the TracerProvider interface using
// a Console exporter.
type consoleTracerProvider struct {
	providerSDK   *tracesdk.TracerProvider
	tempFilePath  string
	consoleWriter io.Writer
}

// Accepts a writer as an argument to allow mocking stdout
func newConsoleTracerProvider(consoleWriter io.Writer) (TracerProvider, error) {
	// Write output to temp file and emit to stdout on shutdown so it'll be in one place,
	// not scattered throughout the console output
	tempFile, err := ioutil.TempFile(os.TempDir(), "traces.log")
	if err != nil {
		return nil, err
	}

	// Create the Console exporter
	exp, err := stdouttrace.New(
		stdouttrace.WithWriter(tempFile),
		// Use human-readable output.
		stdouttrace.WithPrettyPrint(),
	)
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
	tp := consoleTracerProvider{
		providerSDK:   providerSDK,
		tempFilePath:  tempFile.Name(),
		consoleWriter: consoleWriter,
	}
	return &tp, nil
}

func (tp *consoleTracerProvider) Tracer(name string) Tracer {
	return NewOtelTracer(tp.providerSDK.Tracer(tracerName))
}

func (tp *consoleTracerProvider) Shutdown(ctx context.Context) error {
	// Do not make the application hang when it is shutdown.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	err := tp.providerSDK.Shutdown(ctx)
	if err != nil {
		return err
	}

	return tp.dumpOutputToConsole()
}

func (tp *consoleTracerProvider) SetGlobalTracerProvider() {
	otel.SetTracerProvider(tp.providerSDK)
}

func (tp *consoleTracerProvider) dumpOutputToConsole() error {
	// Read the temp file and print to console
	tmpFileContent, err := ioutil.ReadFile(tp.tempFilePath)
	if err != nil {
		return err
	}
	tp.consoleWriter.Write([]byte("=== BEGIN TRACING OUTPUT ===\n"))
	tp.consoleWriter.Write(tmpFileContent)
	tp.consoleWriter.Write([]byte("=== END TRACING OUTPUT ===\n"))
	// Delete the temp file
	defer os.Remove(tp.tempFilePath)
	return nil
}
