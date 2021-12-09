package trace

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestTracer(t *testing.T) {
	testCases := []struct {
		name         string
		providerType TracerProviderType
		collectorUrl string
		assertFunc   func(t *testing.T, tracer Tracer, tp TracerProvider, output *bytes.Buffer)
	}{
		{
			name:         "ConsoleProvider",
			providerType: ConsoleProviderType,
			assertFunc: func(t *testing.T, tracer Tracer, tp TracerProvider, output *bytes.Buffer) {
				// Check provider type
				assert.IsType(t, &consoleTracerProvider{}, tp)

				// Check output
				str := output.String()
				assert.Contains(t, str, "ConsoleProvider_normal")
				assert.Contains(t, str, "ConsoleProvider_error")
				assert.Contains(t, str, "some fake error")
				assert.Contains(t, str, "testAttr")
				assert.Contains(t, str, "testValue")
			},
		},
		{
			name:         "JaegerProvider",
			providerType: JaegerProviderType,
			collectorUrl: "",
			assertFunc: func(t *testing.T, tracer Tracer, tp TracerProvider, output *bytes.Buffer) {
				// Check provider type
				assert.IsType(t, &jaegerTracerProvider{}, tp)

				// Output should not be in stdout
				assert.NotContains(t, output.String(), "TRACING OUTPUT")
				assert.NotContains(t, output.String(), "JaegerProvider_normal")

				// TODO: Check Jaeger output (mock server similar to conjur authn tests?)
			},
		},
		{
			name:         "NoopProvider",
			providerType: NoopProviderType,
			assertFunc: func(t *testing.T, tracer Tracer, tp TracerProvider, output *bytes.Buffer) {
				// Check provider type
				assert.IsType(t, &noopTracerProvider{}, tp)

				// Output should not be in stdout
				assert.NotContains(t, output.String(), "TRACING OUTPUT")
				assert.NotContains(t, output.String(), "NoopProvider_normal")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create an in memory writer to capture any console output
			output := &bytes.Buffer{}
			ctx := context.Background()

			tp, err := NewTracerProvider(tc.providerType, tc.collectorUrl, output, false)
			assert.NoError(t, err)
			tracer := tp.Tracer(tc.name)

			_, span := tracer.Start(ctx, tc.name+"_normal")
			span.SetAttributes(attribute.String("testAttr", "testValue"))
			time.Sleep(time.Millisecond * 10)
			span.End()

			_, span = tracer.Start(ctx, tc.name+"_error")
			time.Sleep(time.Millisecond * 10)
			span.RecordErrorAndSetStatus(errors.New("some fake error"))
			span.End()

			// Shutdown the tracer to flush the output
			tp.Shutdown(ctx)

			tc.assertFunc(t, tracer, tp, output)
		})
	}

	t.Run("Errors on invalid provider", func(t *testing.T) {
		_, err := NewTracerProvider(TracerProviderType(10), "", nil, false)
		assert.Error(t, err)
	})
}
