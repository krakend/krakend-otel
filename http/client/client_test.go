package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"

	// "go.opentelemetry.io/otel/codes"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type fakeService struct{}

func (_ *fakeService) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("foo bar"))
}

type testOTEL struct {
	tracer         trace.Tracer
	tracerProvider trace.TracerProvider
	meter          metric.Meter
	metricProvider metric.MeterProvider

	metricReader *sdkmetric.ManualReader
	spanRecorder *sdktracetest.SpanRecorder
}

func newTestOTEL() *testOTEL {
	spanRecorder := sdktracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	tracer := tracerProvider.Tracer("testotel-tracer")

	metricReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))

	return &testOTEL{
		tracer:         tracer,
		tracerProvider: tracerProvider,
		meter:          metricProvider.Meter("io.krakend.krakend-otel"),
		metricProvider: metricProvider,
		metricReader:   metricReader,
		spanRecorder:   spanRecorder,
	}
}

func (o *testOTEL) Tracer() trace.Tracer {
	return o.tracer
}

func (o *testOTEL) TracerProvider() trace.TracerProvider {
	return o.tracerProvider
}

func (o *testOTEL) MeterProvider() metric.MeterProvider {
	return o.metricProvider
}

func (o *testOTEL) Meter() metric.Meter {
	return o.meter
}

func (_ *testOTEL) Propagator() propagation.TextMapPropagator {
	return nil
}

func (_ *testOTEL) Shutdown(_ context.Context) {
}

func TestInstrumentedHTTPClient(t *testing.T) {
	svc := &fakeService{}
	server := httptest.NewServer(svc)

	innerClient := &http.Client{}
	otelInstance := newTestOTEL()
	transportOptions := &TransportOptions{
		OTELInstance: otelInstance,
		TracesOpts: TransportTracesOptions{
			RoundTrip:   true,
			ReadPayload: true,
			FixedAttributes: []attribute.KeyValue{
				attribute.String("test-trace-attr", "a_trace"),
			},
		},
		MetricsOpts: TransportMetricsOptions{
			RoundTrip:   true,
			ReadPayload: true,
			FixedAttributes: []attribute.KeyValue{
				attribute.String("test-metric-attr", "a_metric"),
			},
		},
	}

	c := InstrumentedHTTPClient(innerClient, transportOptions, "test-http-client")
	if c == nil {
		t.Error("unable to create client")
		return
	}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Errorf("unexpected error creating request: %s", err.Error())
		return
	}

	resp, err := c.Do(req)
	if err != nil {
		t.Errorf("unexpected client error: %s", err.Error())
		return
	}

	if resp == nil {
		t.Errorf("nil response")
		return
	}

	b, _ := io.ReadAll(resp.Body)
	if len(b) == 0 {
		t.Errorf("no bytes read")
		return
	}

	// check that we have two traces, and one is the parent of the other
	endedSpans := otelInstance.spanRecorder.Ended()
	if len(endedSpans) != 2 {
		t.Errorf("traces, want: 2 got: %d", len(endedSpans))
		return
	}
	parentSpan := endedSpans[0]
	childSpan := endedSpans[1]
	fromChildTraceID := childSpan.Parent().TraceID()
	fromParentTraceID := parentSpan.SpanContext().TraceID()
	for idx, c := range fromChildTraceID {
		p := fromParentTraceID[idx]
		if p != c {
			t.Errorf("trace id mismatch at idx: %d (%#v != %#v)", idx,
				fromChildTraceID, fromParentTraceID)
			return
		}
	}

	// check the metrics
	mdata := metricdata.ResourceMetrics{}
	err = otelInstance.metricReader.Collect(context.Background(), &mdata)
	if err != nil {
		t.Errorf("cannot collect the recorded metrics")
		return
	}

	if len(mdata.ScopeMetrics) != 1 {
		t.Errorf("wrong amount of metrics, want: 1, got: %d", len(mdata.ScopeMetrics))
		for idx, sm := range mdata.ScopeMetrics {
			t.Errorf("%d -> %#v", idx, sm)
		}
		return
	}

	// --> check that we have all the metrics we want to report
	sm := mdata.ScopeMetrics[0]
	wantedMetrics := map[string]bool{
		// "requests-failed-count": false // <- we do not have requests that failed
		// "requests-canceled-count": false // <- we do not have requests cancelled
		// "requests-timedout-count": false // <- we do not have requests timed out
		"http.client.request.started.count":   false,
		"http.client.request.size":            false,
		"http.client.duration":                false,
		"http.client.response.size":           false,
		"http.client.response.read.size":      false,
		"http.client.response.read.size-hist": false,
		"http.client.response.read.time":      false,
		"http.client.response.read.time-hist": false,
		// "reader-errors":    false,
	}
	numWantedMetrics := len(wantedMetrics)
	gotMetrics := make(map[string]metricdata.Metrics, numWantedMetrics)
	for _, m := range sm.Metrics {
		gotMetrics[m.Name] = m
	}
	for k := range wantedMetrics {
		if _, ok := gotMetrics[k]; !ok {
			t.Errorf("missing metric %s", k)
			return
		}
	}
	// check that we do not have not expected metrics:
	for k := range gotMetrics {
		if _, ok := wantedMetrics[k]; !ok {
			t.Errorf("got unexpected metric %s", k)
			return
		}
	}

	// --> check that the metrics have the expected attributes set
	readSize := gotMetrics["http.client.response.read.size"]
	readSizeSum, ok := readSize.Data.(metricdata.Sum[int64])
	if !ok {
		t.Errorf("cannot access read size aggregation: %#v", readSize.Data)
		return
	}
	if len(readSizeSum.DataPoints) != 1 {
		t.Errorf("read sum data points, want: 1, got: %d", len(readSizeSum.DataPoints))
		return
	}
	dp := readSizeSum.DataPoints[0]

	if dp.Attributes.Len() != 1 {
		t.Errorf("missing attributes, want 1, got: %d\n%#v", dp.Attributes.Len(), dp.Attributes)
		return
	}
	payload := "foo bar"
	if dp.Value != int64(len(payload)) {
		t.Errorf("metric size, want: %d, got: %d", len(payload), dp.Value)
		return
	}
}
