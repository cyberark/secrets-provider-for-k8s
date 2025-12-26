package entrypoint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyberark/conjur-opentelemetry-tracer/pkg/trace"
	"github.com/stretchr/testify/assert"
)

func TestGetTracerConfigFromEnv(t *testing.T) {
	testCases := []struct {
		description       string
		environment       map[string]string
		expectedType      trace.TracerProviderType
		expectedJaegerURL string
	}{
		{
			description:       "Jaeger config",
			environment:       map[string]string{"JAEGER_COLLECTOR_URL": "http://jaeger:14268/api/traces"},
			expectedType:      trace.JaegerProviderType,
			expectedJaegerURL: "http://jaeger:14268/api/traces",
		},
		{
			description:       "Console config",
			environment:       map[string]string{"LOG_TRACES": "true"},
			expectedType:      trace.ConsoleProviderType,
			expectedJaegerURL: "",
		},
		{
			description: "Jaeger priority over Console",
			environment: map[string]string{
				"JAEGER_COLLECTOR_URL": "http://jaeger:14268/api/traces",
				"LOG_TRACES":           "true",
			},
			expectedType:      trace.JaegerProviderType,
			expectedJaegerURL: "http://jaeger:14268/api/traces",
		},
		{
			description:       "Noop default",
			environment:       map[string]string{},
			expectedType:      trace.NoopProviderType,
			expectedJaegerURL: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			for k, v := range tc.environment {
				t.Setenv(k, v)
			}

			tracerType, jaegerURL := getTracerConfigFromEnv()

			assert.Equal(t, tc.expectedType, tracerType)
			assert.Equal(t, tc.expectedJaegerURL, jaegerURL)
		})
	}
}

func TestGetTracerConfigFromAnnotations(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		description       string
		annotations       map[string]string
		expectedType      trace.TracerProviderType
		expectedJaegerURL string
	}{
		{
			description:       "Jaeger config",
			annotations:       map[string]string{"conjur.org/jaeger-collector-url": "http://jaeger:14268/api/traces"},
			expectedType:      trace.JaegerProviderType,
			expectedJaegerURL: "http://jaeger:14268/api/traces",
		},
		{
			description:       "Console config",
			annotations:       map[string]string{"conjur.org/log-traces": "true"},
			expectedType:      trace.ConsoleProviderType,
			expectedJaegerURL: "",
		},
		{
			description: "Jaeger priority over Console",
			annotations: map[string]string{
				"conjur.org/jaeger-collector-url": "http://jaeger:14268/api/traces",
				"conjur.org/log-traces":           "true",
			},
			expectedType:      trace.JaegerProviderType,
			expectedJaegerURL: "http://jaeger:14268/api/traces",
		},
		{
			description:       "Noop default",
			annotations:       map[string]string{},
			expectedType:      trace.NoopProviderType,
			expectedJaegerURL: "",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			annotationsFilePath := filepath.Join(tmpDir, fmt.Sprintf("annotations_%d", i))
			var content strings.Builder
			for k, v := range tc.annotations {
				content.WriteString(k + "=\"" + v + "\"\n")
			}
			err := os.WriteFile(annotationsFilePath, []byte(content.String()), 0666)
			assert.NoError(t, err)

			tracerType, jaegerURL, err := getTracerConfigFromAnnotations(annotationsFilePath)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedType, tracerType)
			assert.Equal(t, tc.expectedJaegerURL, jaegerURL)
		})
	}

	t.Run("error on missing file", func(t *testing.T) {
		tracerType, jaegerURL, err := getTracerConfigFromAnnotations("/nonexistent/path")
		assert.Error(t, err)
		assert.Equal(t, int(trace.NoopProviderType), int(tracerType))
		assert.Equal(t, "", jaegerURL)
	})
}

func TestGetTracerConfig(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		description       string
		annotations       map[string]string
		environment       map[string]string
		expectedType      trace.TracerProviderType
		expectedJaegerURL string
	}{
		{
			description:       "annotation priority",
			annotations:       map[string]string{"conjur.org/jaeger-collector-url": "http://annotation:14268"},
			environment:       map[string]string{"JAEGER_COLLECTOR_URL": "http://env:14268"},
			expectedType:      trace.JaegerProviderType,
			expectedJaegerURL: "http://annotation:14268",
		},
		{
			description:       "env fallback",
			annotations:       map[string]string{},
			environment:       map[string]string{"JAEGER_COLLECTOR_URL": "http://env:14268"},
			expectedType:      trace.JaegerProviderType,
			expectedJaegerURL: "http://env:14268",
		},
		{
			description:       "Noop default",
			annotations:       map[string]string{},
			environment:       map[string]string{},
			expectedType:      trace.NoopProviderType,
			expectedJaegerURL: "",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			for k, v := range tc.environment {
				t.Setenv(k, v)
			}

			annotationsFilePath := filepath.Join(tmpDir, fmt.Sprintf("annotations_%d", i))
			var content strings.Builder
			for k, v := range tc.annotations {
				content.WriteString(k + "=\"" + v + "\"\n")
			}
			err := os.WriteFile(annotationsFilePath, []byte(content.String()), 0666)
			assert.NoError(t, err)

			tracerType, jaegerURL := getTracerConfig(annotationsFilePath)

			assert.Equal(t, tc.expectedType, tracerType)
			assert.Equal(t, tc.expectedJaegerURL, jaegerURL)
		})
	}
}

func TestCreateTracer(t *testing.T) {
	testCases := []struct {
		description string
		tracerType  trace.TracerProviderType
	}{
		{"Noop", trace.NoopProviderType},
		{"Console", trace.ConsoleProviderType},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ctx, tracer, cleanup, err := createTracer(tc.tracerType, "")
			defer cleanup(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, ctx)
			assert.NotNil(t, tracer)
			assert.NotNil(t, cleanup)
		})
	}
}
