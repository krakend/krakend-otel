package server

import (
	"net/http"
	"strings"

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

// validMethod allows us to limit the different valid methods to avoid
// a client with bad intentions to keep using weird methods and blow up
// the cardinality of the metrics
func validMethod(r *http.Request) string {
	valid := map[string]bool{
		"_OTHER":  true,
		"CONNECT": true,
		"DELETE":  true,
		"GET":     true,
		"HEAD":    true,
		"OPTIONS": true,
		"PATCH":   true,
		"POST":    true,
		"PUT":     true,
		"TRACE":   true,
	}
	v := strings.ToUpper(r.Method)
	if !valid[v] {
		v = "_OTHER"
	}
	return v
}

func (m *metricsHTTP) report(t *tracking, r *http.Request) {
	if m == nil || m.latency == nil {
		return
	}
	urlScheme := "http"
	if r.URL != nil && r.URL.Scheme != "" {
		urlScheme = r.URL.Scheme
	}

	// https://opentelemetry.io/docs/specs/semconv/http/http-metrics/#http-server
	dynAttrs := t.metricsStaticAttrs
	dynAttrs = append(dynAttrs,
		semconv.HTTPRequestMethodKey.String(validMethod(r)), // required attribute
		semconv.URLScheme(urlScheme),                        // required attribute
		semconv.HTTPRoute(t.EndpointPattern()),              // required if available
		semconv.HTTPResponseStatusCode(t.responseStatus))    // required if was sent
	dynAttrsOpts := metric.WithAttributes(dynAttrs...)

	m.latency.Record(t.ctx, t.latencyInSecs, m.fixedAttrsOpts, dynAttrsOpts)
	m.size.Record(t.ctx, int64(t.responseSize), m.fixedAttrsOpts, dynAttrsOpts)
}

func noSemConvMetricsFiller(m *metricsHTTP, meter metric.Meter) {
	m.latency, _ = meter.Float64Histogram("http.server.duration", kotelconfig.TimeBucketsOpt)
	m.size, _ = meter.Int64Histogram("http.server.response.size", kotelconfig.SizeBucketsOpt)
}

func semConv1_27MetricsFiller(m *metricsHTTP, meter metric.Meter) {
	// latency -> http.server.request.duration (required and stable)
	m.latency, _ = meter.Float64Histogram(v127.HTTPServerRequestDurationName,
		metric.WithUnit(v127.HTTPServerRequestDurationUnit),
		metric.WithDescription(v127.HTTPServerRequestDurationDescription),
		kotelconfig.TimeBucketsOpt)

	// as 1.27 AND still on 1.29, this is an "experimental" metric, tha must be a histogram:
	// http.servers.response.body.size
	// that must be Content-Length or compressed size.
	// https://opentelemetry.io/docs/specs/semconv/http/http-metrics/#metric-httpserverresponsebodysize
	m.size, _ = meter.Int64Histogram(v127.HTTPServerResponseBodySizeName,
		metric.WithUnit(v127.HTTPServerResponseBodySizeUnit),
		metric.WithDescription(v127.HTTPServerResponseBodySizeDescription),
		kotelconfig.SizeBucketsOpt)

	// tracking request body size would mean rely on input 'Content-Length' header, or
	// wrapping the reader to count the body size, that might involve checking if is a
	// hijacked connection, etc ... since is optional and experimental too
	// is left as further improvement.
}
