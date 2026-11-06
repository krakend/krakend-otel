package otelio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"testing/iotest"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestCancelAwareInstrumentedReaderFactory(t *testing.T) {
	for name, tc := range map[string]struct {
		cancelAware  bool
		readErr      error
		ctxCanceled  bool
		wantStatus   codes.Code
		wantCanceled bool
	}{
		"plain factory, canceled": {
			cancelAware: false, readErr: context.Canceled,
			wantStatus: codes.Error, wantCanceled: false,
		},
		"cancel aware, canceled error": {
			cancelAware: true, readErr: fmt.Errorf("read: %w", context.Canceled),
			wantStatus: codes.Unset, wantCanceled: true,
		},
		"cancel aware, canceled context": {
			cancelAware: true, readErr: errors.New("body closed"), ctxCanceled: true,
			wantStatus: codes.Unset, wantCanceled: true,
		},
		"cancel aware, other error": {
			cancelAware: true, readErr: errors.New("boom"),
			wantStatus: codes.Error, wantCanceled: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			spanRecorder := sdktracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)).Tracer("test")

			factory := NewInstrumentedReaderFactory
			if tc.cancelAware {
				factory = NewCancelAwareInstrumentedReaderFactory
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.ctxCanceled {
				cancel()
			}
			reader := factory("", nil, nil, tracer, nil)(iotest.ErrReader(tc.readErr), ctx)
			if _, err := io.ReadAll(reader); err == nil {
				t.Fatal("expected a read error")
			}

			spans := spanRecorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("unexpected number of spans: %d", len(spans))
			}
			if code := spans[0].Status().Code; code != tc.wantStatus {
				t.Errorf("unexpected span status: %s", code)
			}
			attrs := attribute.NewSet(spans[0].Attributes()...)
			canceled, _ := attrs.Value("canceled")
			if canceled.AsBool() != tc.wantCanceled {
				t.Errorf("unexpected canceled attribute: %v", canceled.AsBool())
			}
		})
	}
}
