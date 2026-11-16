package lura

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMiddlewareTracer_end(t *testing.T) {
	// like the lura merge error: it does not unwrap to context.Canceled
	mergedErr := errors.New("context canceled")

	for name, tc := range map[string]struct {
		reportClientDisconnects bool
		ctxCanceled             bool
		err                     error
		wantStatus              bool
		wantCanceled            bool
	}{
		"disabled, canceled": {
			reportClientDisconnects: false, err: context.Canceled,
			wantStatus: true, wantCanceled: true,
		},
		"disabled, merged error, canceled context": {
			reportClientDisconnects: false, ctxCanceled: true, err: mergedErr,
			wantStatus: true, wantCanceled: false,
		},
		"enabled, canceled": {
			reportClientDisconnects: true, err: fmt.Errorf("wrapped: %w", context.Canceled),
			wantStatus: false, wantCanceled: true,
		},
		"enabled, merged error, canceled context": {
			reportClientDisconnects: true, ctxCanceled: true, err: mergedErr,
			wantStatus: false, wantCanceled: true,
		},
		"enabled, regular error": {
			reportClientDisconnects: true, err: errors.New("boom"),
			wantStatus: true, wantCanceled: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			spanRecorder := sdktracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)).Tracer("test")
			mt := &middlewareTracer{
				name:                    "test",
				tracer:                  tracer,
				reportClientDisconnects: tc.reportClientDisconnects,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.ctxCanceled {
				cancel()
			}
			ctx, span := tracer.Start(ctx, "test")
			mt.end(ctx, span, nil, tc.err)

			spans := spanRecorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("unexpected number of spans: %d", len(spans))
			}
			attrs := attribute.NewSet(spans[0].Attributes()...)
			status, ok := attrs.Value("http.response.status_code")
			if ok != tc.wantStatus {
				t.Errorf("unexpected presence of the status code: %v", ok)
			}
			if ok && status.AsInt64() != 500 {
				t.Errorf("unexpected status code: %d", status.AsInt64())
			}
			canceled, _ := attrs.Value("canceled")
			if canceled.AsBool() != tc.wantCanceled {
				t.Errorf("unexpected canceled attribute: %v", canceled.AsBool())
			}
		})
	}
}
