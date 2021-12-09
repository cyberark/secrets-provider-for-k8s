package trace

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	traceotel "go.opentelemetry.io/otel/trace"
)

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

	// SetAttributes sets kv as attributes of the Span. If a key from kv
	// already exists for an attribute of the Span it will be overwritten with
	// the value contained in kv.
	SetAttributes(kv ...attribute.KeyValue)

	// Record an error and set the status for a Span.
	RecordErrorAndSetStatus(err error)
}

// otelSpan implements the Span interface for a OpenTelemetry exporter.
type otelSpan struct {
	spanOtel traceotel.Span
}

func newOtelSpan(spanOtel traceotel.Span) otelSpan {
	return otelSpan{spanOtel: spanOtel}
}

func (s otelSpan) End() {
	s.spanOtel.End()
}

func (s otelSpan) RecordError(err error) {
	s.spanOtel.RecordError(err)
}

func (s otelSpan) SetStatus(code codes.Code, description string) {
	s.spanOtel.SetStatus(code, description)
}

func (s otelSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.spanOtel.SetAttributes(kv...)
}

func (s otelSpan) RecordErrorAndSetStatus(err error) {
	s.RecordError(err)
	s.SetStatus(codes.Error, err.Error())
}
