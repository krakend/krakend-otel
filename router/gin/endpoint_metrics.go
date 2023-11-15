package gin

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
)

type ginMetrics struct {
	fixedAttrs     []attribute.KeyValue
	fixedAttrsOpts metric.MeasurementOption

	latency metric.Float64Histogram
	size    metric.Int64Histogram
}

func newGinMetrics(meter metric.Meter, attrs []attribute.KeyValue) *ginMetrics {
	var gm ginMetrics
	gm.latency, _ = meter.Float64Histogram("router-response-latency")
	gm.size, _ = meter.Int64Histogram("router-response-size")
	if len(attrs) > 0 {
		gm.fixedAttrs = make([]attribute.KeyValue, len(attrs))
		copy(gm.fixedAttrs, attrs)
		gm.fixedAttrsOpts = metric.WithAttributeSet(attribute.NewSet(gm.fixedAttrs...))
	}
	return &gm
}

func (m *ginMetrics) report(ht *handlerTracking) {
	if m == nil || m.latency == nil {
		return
	}
	dynAttrsOpts := metric.WithAttributes(
		semconv.HTTPResponseStatusCode(ht.responseStatus),
	)
	m.latency.Record(ht.ctx, ht.latencyInSecs, m.fixedAttrsOpts, dynAttrsOpts)
	m.size.Record(ht.ctx, int64(ht.responseSize), m.fixedAttrsOpts, dynAttrsOpts)
}
