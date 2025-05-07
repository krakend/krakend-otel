package server

import (
	"context"
	"net"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/krakend/krakend-otel/state"
)

type trackingHandler struct {
	next http.Handler

	prop          propagation.TextMapPropagator
	metrics       *metricsHTTP
	traces        *tracesHTTP
	reportHeaders bool
	config        state.Config
}

func (h *trackingHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.URL != nil && h.config.SkipEndpoint(r.URL.Path) {
		h.next.ServeHTTP(rw, r)
		return
	}

	t := newTracking()
	t.ctx = r.Context()
	if h.prop != nil {
		t.ctx = h.prop.Extract(t.ctx, propagation.HeaderCarrier(r.Header))
		if t.ctx != r.Context() {
			t.span = trace.SpanFromContext(t.ctx)
		}
	}
	t.ctx = context.WithValue(t.ctx, krakenDContextTrackingStrKey, t)
	r = r.WithContext(t.ctx)

	if h.metrics != nil || h.traces != nil {
		rw = newTrackingResponseWriter(rw, t, h.reportHeaders, func(c net.Conn, _ error) (net.Conn, error) {
			t.Finish()
			h.traces.end(t)
			h.metrics.report(t, r)
			return c, nil
		})
	}

	t.Start()
	r = h.traces.start(r, t)
	h.next.ServeHTTP(rw, r)
	t.Finish()
	h.traces.end(t)
	h.metrics.report(t, r)
}

func NewTrackingHandler(next http.Handler) http.Handler {
	return NewTrackingHandlerWithTrustedProxies(next, nil)
}

func NewTrackingHandlerWithTrustedProxies(next http.Handler, trustedProxies []string) http.Handler {
	otelCfg := state.GlobalConfig()
	if otelCfg == nil {
		return next
	}

	gCfg := otelCfg.GlobalOpts()
	if gCfg.DisablePropagation && gCfg.DisableMetrics && gCfg.DisableTraces {
		return next
	}
	s := otelCfg.OTEL()
	var prop propagation.TextMapPropagator
	if !gCfg.DisablePropagation {
		prop = s.Propagator()
	}

	var m *metricsHTTP
	if !gCfg.DisableMetrics {
		var metricsAttrs []attribute.KeyValue
		for _, kv := range gCfg.MetricsStaticAttributes {
			if len(kv.Key) > 0 && len(kv.Value) > 0 {
				metricsAttrs = append(metricsAttrs, attribute.String(kv.Key, kv.Value))
			}
		}

		m = newMetricsHTTP(s.Meter(), metricsAttrs, gCfg.SemConv)
	}

	var t *tracesHTTP
	if !gCfg.DisableTraces {
		tracesAttrs := []attribute.KeyValue{attribute.String("krakend.stage", "global")}
		for _, kv := range gCfg.TracesStaticAttributes {
			if len(kv.Key) > 0 && len(kv.Value) > 0 {
				tracesAttrs = append(tracesAttrs, attribute.String(kv.Key, kv.Value))
			}
		}

		t = newTracesHTTP(s.Tracer(), tracesAttrs, gCfg.ReportHeaders, trustedProxies)
	}

	return &trackingHandler{
		next:          next,
		prop:          prop,
		metrics:       m,
		traces:        t,
		reportHeaders: gCfg.ReportHeaders,
		config:        otelCfg,
	}
}
