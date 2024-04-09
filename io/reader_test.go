package otelio

import (
	"bytes"
	"context"
	"io"
	"testing"
	"testing/iotest"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	sdktracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestReaderHappyPath(t *testing.T) {
	var err error

	spanRecorder := sdktracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	tracer := tracerProvider.Tracer("test-tracer")
	ctx, _ := tracer.Start(context.Background(), "test-body-tracker")

	metricReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	meter := metricProvider.Meter("test-meter")

	// this is the buffer where we want to write something:
	var bw bytes.Buffer
	bw.Grow(64 * 1024)

	payload := "foo bar"
	bw.WriteString(payload)

	attrsT := []attribute.KeyValue{
		attribute.Int("http.status_code", 201),
		attribute.String("url.path.pattern", "/this/{matched}/path"),
		attribute.String("url.path", "/this/some_value/path"),
	}
	attrsM := []attribute.KeyValue{
		attribute.Int("http.status_code", 201),
		attribute.String("url.path", "/this/{matched}/path"),
	}
	reader := NewInstrumentedReader("", &bw, ctx, attrsT, attrsM, tracer, meter)

	_, err = io.ReadAll(reader)
	if err != nil {
		t.Errorf("cannot read: %s", err.Error())
		return
	}

	endedSpans := spanRecorder.Ended()
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
		t.Errorf("trace attributes, want %d, got %d", len(attrsT), len(gotTraceAttrs))
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
		"read.size":      false,
		"read.size-hist": false,
		"read.time":      false,
		"read.time-hist": false,
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

	// --> check that the metrics have the expected attributes set
	readSize := gotMetrics["read.size"]
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

	if dp.Attributes.Len() != 2 {
		t.Errorf("missing attributes")
		return
	}
	if dp.Value != int64(len(payload)) {
		t.Errorf("metric size, want: %d, got: %d", len(payload), dp.Value)
		return
	}
}

func TestReaderTimeout(t *testing.T) {
	var err error
	ctx := context.Background()

	spanRecorder := sdktracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	tracer := tracerProvider.Tracer("test-tracer")
	ctx, _ = tracer.Start(ctx, "test-body-tracker")

	metricReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	meter := metricProvider.Meter("test-meter")

	// this is the buffer where we want to write something:
	payload := "foo bar"
	var bw bytes.Buffer
	bw.WriteString(payload)
	innerReader := iotest.TimeoutReader(&bw)

	attrsT := []attribute.KeyValue{
		attribute.Int("http.status_code", 201),
		attribute.String("url.path.pattern", "/this/{matched}/path"),
		attribute.String("url.path", "/this/some_value/path"),
	}
	attrsM := []attribute.KeyValue{
		attribute.Int("http.status_code", 201),
		attribute.String("url.path", "/this/{matched}/path"),
	}
	reader := NewInstrumentedReader("", innerReader, ctx, attrsT, attrsM, tracer, meter)

	fourBytes := make([]byte, 4)
	_, err = reader.Read(fourBytes)
	if err != nil {
		t.Errorf("cannot read: %s", err.Error())
		return
	}

	_, err = io.ReadAll(reader)
	if err == nil {
		t.Errorf("expected timeout error")
		return
	}

	endedSpans := spanRecorder.Ended()
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
	// took to read the data.
	if len(gotTraceAttrs) != len(attrsT)+2 {
		t.Errorf("trace attributes, want %d, got %d", len(attrsT), len(gotTraceAttrs))
		return
	}
	spanStatus := span.Status()
	if spanStatus.Code != codes.Error {
		t.Errorf("expected error status")
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
		"read.size":      false,
		"read.size-hist": false,
		"read.time":      false,
		"read.time-hist": false,
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

	// --> check that the metrics have the expected attributes set
	readSize := gotMetrics["read.size"]
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

	// attributes are the 2 from the fixed attributes + the error=true
	if dp.Attributes.Len() != 3 {
		t.Errorf("missing attributes")
		return
	}
	if dp.Value != 4 {
		t.Errorf("metric size, want: 4, got: %d", dp.Value)
		return
	}
}
