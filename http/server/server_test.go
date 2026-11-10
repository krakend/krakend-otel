package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/krakend/krakend-otel/v2/config"
	"github.com/krakend/krakend-otel/v2/state"
)

type clientDisconnectTest struct {
	reportClientDisconnects bool
	clientGone              bool
	wantStatus              bool
	wantErrorType           bool
	wantSpanStatus          codes.Code
}

func TestTrackingHandler_clientDisconnect(t *testing.T) {
	for name, tc := range map[string]clientDisconnectTest{
		"disabled, client gone": {
			reportClientDisconnects: false, clientGone: true,
			wantStatus: true, wantErrorType: false, wantSpanStatus: codes.Ok,
		},
		"enabled, client gone": {
			reportClientDisconnects: true, clientGone: true,
			wantStatus: false, wantErrorType: true, wantSpanStatus: codes.Unset,
		},
		"enabled, client still there": {
			reportClientDisconnects: true, clientGone: false,
			wantStatus: true, wantErrorType: false, wantSpanStatus: codes.Ok,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

func (tc clientDisconnectTest) run(t *testing.T) {
	t.Helper()
	spanRecorder := sdktracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)).Tracer("test")
	metricReader := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader)).Meter("test")

	h := &trackingHandler{
		next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
		metrics:                 newMetricsHTTP(meter, nil, ""),
		traces:                  newTracesHTTP(tracer, nil, false, nil, nil),
		config:                  state.NewConfig(&config.ConfigData{}),
		reportClientDisconnects: tc.reportClientDisconnects,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if tc.clientGone {
		cancel()
	}
	req := httptest.NewRequest(http.MethodGet, "/foo", http.NoBody).WithContext(ctx)
	h.ServeHTTP(httptest.NewRecorder(), req)

	var rm metricdata.ResourceMetrics
	if err := metricReader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var metricAttrs *attribute.Set
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "http.server.duration" {
				continue
			}
			dps := m.Data.(metricdata.Histogram[float64]).DataPoints
			if len(dps) != 1 {
				t.Fatalf("unexpected number of data points: %d", len(dps))
			}
			metricAttrs = &dps[0].Attributes
		}
	}
	if metricAttrs == nil {
		t.Fatal("http.server.duration not reported")
	}
	tc.checkAttrs(t, "metric", metricAttrs.Value)

	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("unexpected number of spans: %d", len(spans))
	}
	spanAttrs := attribute.NewSet(spans[0].Attributes()...)
	tc.checkAttrs(t, "span", spanAttrs.Value)
	if code := spans[0].Status().Code; code != tc.wantSpanStatus {
		t.Errorf("unexpected span status: %s", code)
	}
}

func (tc clientDisconnectTest) checkAttrs(t *testing.T, kind string, get func(attribute.Key) (attribute.Value, bool)) {
	t.Helper()
	status, hasStatus := get("http.response.status_code")
	if hasStatus != tc.wantStatus {
		t.Errorf("%s: unexpected presence of the status code: %v", kind, hasStatus)
	}
	if hasStatus && status.AsInt64() != http.StatusInternalServerError {
		t.Errorf("%s: unexpected status code: %d", kind, status.AsInt64())
	}
	errType, hasErrType := get("error.type")
	if hasErrType != tc.wantErrorType {
		t.Errorf("%s: unexpected presence of the error type: %v", kind, hasErrType)
	}
	if hasErrType && errType.AsString() != "client_disconnected" {
		t.Errorf("%s: unexpected error type: %s", kind, errType.AsString())
	}
}
