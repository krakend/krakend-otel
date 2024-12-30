package server

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	v127 "go.opentelemetry.io/otel/semconv/v1.27.0"

	kotelconfig "github.com/krakend/krakend-otel/config"
)

type metricsHTTP struct {
	fixedAttrs     []attribute.KeyValue
	fixedAttrsOpts metric.MeasurementOption

	latency metric.Float64Histogram // the time it takes to serve the request
	size    metric.Int64Histogram   // the response size
}

type metricsFiller func(*metricsHTTP, metric.Meter)

func newMetricsHTTP(meter metric.Meter, attrs []attribute.KeyValue, semconv string) *metricsHTTP {
	var m metricsHTTP

	supportedSemConv := map[string]metricsFiller{
		"1.27": semConv1_27MetricsFiller,
	}
	fill := noSemConvMetricsFiller
	if semConvFiller, ok := supportedSemConv[semconv]; ok {
		fill = semConvFiller
	}
	fill(&m, meter)
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
	dynAttrs := t.metricsStaticAttrs
	dynAttrs = append(dynAttrs, semconv.HTTPRoute(t.EndpointPattern()))
	dynAttrs = append(dynAttrs, semconv.HTTPRequestMethodKey.String(r.Method))
	dynAttrs = append(dynAttrs, semconv.HTTPResponseStatusCode(t.responseStatus))
	dynAttrsOpts := metric.WithAttributes(dynAttrs...)
	m.latency.Record(t.ctx, t.latencyInSecs, m.fixedAttrsOpts, dynAttrsOpts)
	m.size.Record(t.ctx, int64(t.responseSize), m.fixedAttrsOpts, dynAttrsOpts)
}

func noSemConvMetricsFiller(m *metricsHTTP, meter metric.Meter) {
	m.latency, _ = meter.Float64Histogram("http.server.duration", kotelconfig.TimeBucketsOpt)
	m.size, _ = meter.Int64Histogram("http.server.response.size", kotelconfig.SizeBucketsOpt)
}

func semConv1_27MetricsFiller(m *metricsHTTP, meter metric.Meter) {
	m.latency, _ = meter.Float64Histogram(v127.HTTPServerRequestDurationName,
		metric.WithUnit(v127.HTTPServerRequestDurationUnit),
		metric.WithDescription(v127.HTTPServerRequestDurationDescription),
		kotelconfig.TimeBucketsOpt)

	m.size, _ = meter.Int64Histogram(v127.HTTPServerResponseBodySizeName,
		metric.WithUnit(v127.HTTPServerResponseBodySizeUnit),
		metric.WithDescription(v127.HTTPServerResponseBodySizeDescription),
		kotelconfig.SizeBucketsOpt)
}
