package gin

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	kconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/state"
)

// defaultRouterOpts return the default options when no options
// are provided
func defaultRouterOpts() *kconfig.RouterOpts {
	return &kconfig.RouterOpts{
		Metrics:            true,
		Traces:             true,
		DisablePropagation: false,
	}
}

// New wraps a handler factory adding some simple instrumentation to the generated handlers
func New(hf krakendgin.HandlerFactory, gsfn state.GetterFn, opts *kconfig.RouterOpts) krakendgin.HandlerFactory {
	if opts == nil {
		opts = defaultRouterOpts()
	}
	if gsfn == nil {
		gsfn = state.GlobalState
	}
	return func(cfg *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		return HandlerFunc(cfg, opts, gsfn, hf(cfg, p))
	}
}

// HandlerFunc creates and instrumented gin.Handler wrapper with traces and / or metrics enabled
// according to the [config.RouterOpts].
func HandlerFunc(cfg *config.EndpointConfig, opts *kconfig.RouterOpts, gsfn state.GetterFn, next gin.HandlerFunc) gin.HandlerFunc {
	if opts == nil || (!opts.Metrics && !opts.Traces) {
		return next
	}
	s := gsfn()

	// TODO: check that the endpoint path parameters are all standarized;
	// either {param} or :param , must all become {param}.
	urlPattern := kconfig.NormalizeURLPattern(cfg.Endpoint)

	staticAttrs := []attribute.KeyValue{
		semconv.URLPath(urlPattern),
		attribute.String("krakend.stage", "router"),
	}
	traces := newGinTraces(&ginTracesOptions{
		DisablePropagation: opts.DisablePropagation,
		FixedAttributes:    staticAttrs,
	}, s.Tracer(), urlPattern, s.Propagator())

	metrics := newGinMetrics(s.MeterProvider().Meter("io.krakend.krakend-otel"),
		staticAttrs)

	h := &handler{
		handler: next,
		traces:  traces,
		metrics: metrics,
	}
	return h.HandlerFunc
}

// handlerTracking contains the per-request information to create the span
// with its attributes, and collect the latency metrics for the router stage.
type handlerTracking struct {
	ctx  context.Context
	span trace.Span

	latencyInSecs float64
	status        int
	// err           error  // <- how do we fill this ?
	responseSize   int
	responseStatus int
}

// handler is the instrumented handler for an endpoint.
type handler struct {
	handler gin.HandlerFunc
	traces  *ginTraces
	metrics *ginMetrics
}

// HandlerFunc is the gin handling function that wraps another handler
// and instruments it.
func (h *handler) HandlerFunc(c *gin.Context) {
	ht := handlerTracking{
		ctx: c,
	}
	h.traces.start(c, &ht)

	started := time.Now()
	h.handler(c)
	ht.latencyInSecs = float64(time.Since(started)) / float64(time.Second)
	ht.responseStatus = c.Writer.Status()
	ht.responseSize = c.Writer.Size()

	h.metrics.report(&ht)
	h.traces.end(&ht)
}
