package server

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	kotelconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/state"
)

type trackingHandler struct {
	next http.Handler

	prop          propagation.TextMapPropagator
	metrics       *metricsHTTP
	traces        *tracesHTTP
	reportHeaders bool
}

func (h *trackingHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
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
		rw = newTrackingResponseWriter(rw, t, h.reportHeaders)
	}

	t.Start()
	r = h.traces.start(r, t)
	h.next.ServeHTTP(rw, r)
	t.Finish()
	h.traces.end(t)
	h.metrics.report(t, r)
}

func NewTrackingHandler(next http.Handler, obsCfg *kotelconfig.Config, stateFn state.GetterFn) http.Handler {
	s := stateFn()

	gCfg := &kotelconfig.GlobalOpts{}
	if obsCfg.Layers != nil && obsCfg.Layers.Global != nil {
		gCfg = obsCfg.Layers.Global
	}

	if gCfg.DisablePropagation && gCfg.DisableMetrics && gCfg.DisableTraces {
		// nothing to do if everything is disabled
		return next
	}

	var prop propagation.TextMapPropagator
	if !gCfg.DisablePropagation {
		prop = s.Propagator()
	}

	var m *metricsHTTP
	if !gCfg.DisableMetrics {
		m = newMetricsHTTP(s.Meter(), []attribute.KeyValue{})
	}

	var t *tracesHTTP
	if !gCfg.DisableTraces {
		t = newTracesHTTP(s.Tracer(), []attribute.KeyValue{
			attribute.String("krakend.stage", "global"),
		}, gCfg.ReportHeaders)
	}

	return &trackingHandler{
		next:          next,
		prop:          prop,
		metrics:       m,
		traces:        t,
		reportHeaders: gCfg.ReportHeaders,
	}
}
