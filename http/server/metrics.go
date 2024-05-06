package server

import (
	otelhttp "github.com/krakend/krakend-otel/http"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	"net/http"

	kotelconfig "github.com/krakend/krakend-otel/config"
)

type metricsHTTP struct {
	fixedAttrs     []attribute.KeyValue
	fixedAttrsOpts metric.MeasurementOption

	latency metric.Float64Histogram
	size    metric.Int64Histogram
}

func newMetricsHTTP(meter metric.Meter, attrs []attribute.KeyValue) *metricsHTTP {
	var m metricsHTTP
	m.latency, _ = meter.Float64Histogram("http.server.duration", kotelconfig.TimeBucketsOpt)
	m.size, _ = meter.Int64Histogram("http.server.response.size", kotelconfig.SizeBucketsOpt)
	if len(attrs) > 0 {
		m.fixedAttrs = make([]attribute.KeyValue, len(attrs))
		copy(m.fixedAttrs, attrs)
		m.fixedAttrsOpts = metric.WithAttributeSet(attribute.NewSet(m.fixedAttrs...))
	} else {
		m.fixedAttrsOpts = metric.WithAttributeSet(attribute.NewSet())
	}
	return &m
}

func (m *metricsHTTP) report(t *tracking, r *http.Request) {
	if m == nil || m.latency == nil {
		return
	}

	dynAttrsOpts := metric.WithAttributes(
		append(
			otelhttp.CustomMetricAttributes(r),
			semconv.HTTPRoute(t.EndpointPattern()),
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.HTTPResponseStatusCode(t.responseStatus),
		)...,
	)

	m.latency.Record(t.ctx, t.latencyInSecs, m.fixedAttrsOpts, dynAttrsOpts)
	m.size.Record(t.ctx, int64(t.responseSize), m.fixedAttrsOpts, dynAttrsOpts)
}
