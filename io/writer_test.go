package otelio

import (
	"bytes"
	"context"
	"io"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWriterHappyPath(t *testing.T) {
	var err error

	spanRecorder := sdktracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	tracer := tracerProvider.Tracer("io-writer-test-tracer")
	ctx, _ := tracer.Start(context.Background(), "io-writer-test-body-tracker")

	metricReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	meter := metricProvider.Meter("io-writer-test-meter")

	// this is the buffer where we want to write something:
	var bw bytes.Buffer
	bw.Grow(64 * 1024)
	payload := "foo bar"

	initialStartedSpansLen := len(spanRecorder.Started())

	attrsT := []attribute.KeyValue{
		attribute.Int("http.status_code", 201),
		attribute.String("url.path.pattern", "/this/{matched}/path"),
		attribute.String("url.path", "/this/some_value/path"),
	}
	attrsM := []attribute.KeyValue{
		attribute.Int("http.status_code", 201),
		attribute.String("url.path", "/this/{matched}/path"),
	}
	writer := NewInstrumentedWriter(&bw, ctx, attrsT, attrsM, tracer, meter)

	startedSpans := spanRecorder.Started()
	if len(startedSpans) > initialStartedSpansLen {
		t.Errorf("span cannot start until first write: %#v", startedSpans)
		return
	}
	_, err = io.WriteString(writer, payload)
	if err != nil {
		t.Errorf("cannot write: %s", err.Error())
		return
	}
	startedSpans = spanRecorder.Started()
	if len(startedSpans) < initialStartedSpansLen+1 {
		t.Errorf("the span should have started after writing: %#v", startedSpans)
		return
	}
	endedSpans := spanRecorder.Ended()
	if len(endedSpans) > 0 {
		t.Errorf("the writing does not end a span until its closed")
		return
	}

	writer.Close()
	endedSpans = spanRecorder.Ended()
	if len(endedSpans) != 1 {
		t.Errorf("num ended spans, want: 1, got: %d", len(endedSpans))
		for idx, s := range endedSpans {
			t.Errorf("%d -> %#v", idx, s)
		}
		return
	}

	span := endedSpans[0]
	gotTraceAttrs := span.Attributes()
	// trace will have static attributes, plus the size and time it
	// took to read the data:
	if len(gotTraceAttrs) != len(attrsT)+2 {
		t.Errorf("trace attributes, want %d, got %d (%#v)",
			len(attrsT)+2, len(gotTraceAttrs), gotTraceAttrs)
		return
	}

	// check the reported metrics
	rm := metricdata.ResourceMetrics{}
	err = metricReader.Collect(context.Background(), &rm)
	if err != nil {
		t.Errorf("collecting metrics err: %s", err.Error())
		return
	}

	if len(rm.ScopeMetrics) != 1 {
		t.Errorf("wrong amount of metrics, want: 1, got: %d", len(rm.ScopeMetrics))
		for idx, sm := range rm.ScopeMetrics {
			t.Errorf("%d -> %#v", idx, sm)
		}
		return
	}

	// --> check that we have all the metrics we want to report
	sm := rm.ScopeMetrics[0]
	wantedMetrics := map[string]bool{
		"written-size":      false,
		"written-size-hist": false,
		"written-time":      false,
		"written-time-hist": false,
		// "written-errors":    false,
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

	// --> check that the metrics have the expected attributes set
	writtenSize := gotMetrics["written-size"]
	writtenSizeSum, ok := writtenSize.Data.(metricdata.Sum[int64])
	if !ok {
		t.Errorf("cannot access written size aggregation: %#v", writtenSize.Data)
		return
	}
	if len(writtenSizeSum.DataPoints) != 1 {
		t.Errorf("written sum data points, want: 1, got: %d", len(writtenSizeSum.DataPoints))
		return
	}
	dp := writtenSizeSum.DataPoints[0]

	if dp.Attributes.Len() != 2 {
		t.Errorf("missing attributes")
		return
	}
	if dp.Value != int64(len(payload)) {
		t.Errorf("metric size, want: %d, got: %d", len(payload), dp.Value)
		return
	}
}
