package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type KrakenDContextTrackingTypeKey string

const (
	// KrakenDContextOTELStrKey is a special key to be used when there
	// is no way to obtain the span context from an inner context
	// (like when gin has not the fallback option enabled in the engine).
	krakenDContextTrackingStrKey KrakenDContextTrackingTypeKey = "KrakendD-Context-OTEL"
)

type tracking struct {
	startTime time.Time
	ctx       context.Context
	span      trace.Span

	latencyInSecs      float64
	responseSize       int
	responseStatus     int
	responseHeaders    map[string][]string
	writeErrs          []error
	endpointPattern    string
	isHijacked         bool
	metricsStaticAttrs []attribute.KeyValue
	tracesStaticAttrs  []attribute.KeyValue
	hijackedErr        error
}

func (t *tracking) EndpointPattern() string {
	if t.endpointPattern != "" {
		return t.endpointPattern
	}
	if t.isHijacked {
		return "Upgraded Connection"
	}
	return strconv.Itoa(t.responseStatus) + " " + http.StatusText(t.responseStatus)
}

func (t *tracking) MetricsStaticAttributes() []attribute.KeyValue {
	return t.metricsStaticAttrs
}

func (t *tracking) TracesStaticAttributes() []attribute.KeyValue {
	return t.tracesStaticAttrs
}

func newTracking() *tracking {
	return &tracking{
		responseStatus: 200,
	}
}

func fromContext(ctx context.Context) *tracking {
	v := ctx.Value(krakenDContextTrackingStrKey)
	if v != nil {
		t, _ := v.(*tracking)
		return t
	}
	return nil
}

// SetEndpointPattern allows to set the endpoint attribute once it
// has been matched down the http handling pipeline.
func SetEndpointPattern(ctx context.Context, endpointPattern string) {
	if t := fromContext(ctx); t != nil {
		t.endpointPattern = endpointPattern
	}
}

// SetStaticAttributtes allows to set metrics and traces static attributes in
// the request context
func SetStaticAttributtes(ctx context.Context, metricAttrs, tracesAttrs []attribute.KeyValue) {
	if t := fromContext(ctx); t != nil {
		t.metricsStaticAttrs = metricAttrs
		t.tracesStaticAttrs = tracesAttrs
	}
}

func (t *tracking) Start() {
	t.startTime = time.Now()
}

func (t *tracking) Finish() {
	t.latencyInSecs = float64(time.Since(t.startTime)) / float64(time.Second)
}
