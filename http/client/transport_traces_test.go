package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTransportTraces_endCanceled(t *testing.T) {
	canceledErr := &url.Error{Op: "Get", URL: "http://backend/foo", Err: context.Canceled}

	for name, tc := range map[string]struct {
		reportCanceled bool
		err            error
		ctxCanceled    bool
		wantStatus     codes.Code
		wantCanceled   bool
	}{
		"disabled, canceled": {
			reportCanceled: false, err: canceledErr,
			wantStatus: codes.Error, wantCanceled: false,
		},
		"enabled, canceled error": {
			reportCanceled: true, err: canceledErr,
			wantStatus: codes.Unset, wantCanceled: true,
		},
		"enabled, canceled context": {
			reportCanceled: true, err: errors.New("connection reset"), ctxCanceled: true,
			wantStatus: codes.Unset, wantCanceled: true,
		},
		"enabled, other error": {
			reportCanceled: true, err: errors.New("connection refused"),
			wantStatus: codes.Error, wantCanceled: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			spanRecorder := sdktracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)).Tracer("test")
			tt := newTransportTraces(&TransportTracesOptions{RoundTrip: true, ReportCanceled: tc.reportCanceled}, tracer, "test")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.ctxCanceled {
				cancel()
			}
			rtt := &roundTripTracking{
				req: httptest.NewRequest(http.MethodGet, "http://backend/foo", http.NoBody).WithContext(ctx),
				err: tc.err,
			}
			tt.start(rtt, propagation.TraceContext{})
			tt.end(rtt)

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
