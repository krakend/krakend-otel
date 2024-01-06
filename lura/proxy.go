package lura

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"

	kotelconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/state"
)

// Middleare creates a proxy that instruments the proxy it wraps by creating an span if enabled,
// and report the duration of this stage in metrics if enabled.
func Middleware(name string, stage string, gsf state.GetterFn, metricsEnabled bool, tracesEnabled bool) proxy.Middleware {
	if gsf == nil {
		gsf = state.GlobalState
	}

	return func(next ...proxy.Proxy) proxy.Proxy {
		if len(next) > 1 {
			panic(proxy.ErrTooManyProxies)
		}
		if len(next) < 1 {
			panic(proxy.ErrNotEnoughProxies)
		}
		n := next[0]
		gs := gsf()
		if gs == nil {
			return n // no instrumentation available
		}

		// measure the time it takes to process all
		reportMetrics := metricsEnabled
		meterProvider := gs.MeterProvider()
		meter := meterProvider.Meter("io.krakend.krakend-otel.lura") // TODO: check these namings
		duration, err := meter.Float64Histogram("stage-duration")
		if err != nil {
			reportMetrics = false
		}
		metricAttrs := metric.WithAttributes(
			attribute.String("krakend.name", name),
			attribute.String("krakend.stage", stage),
		)

		reportTrace := tracesEnabled
		tracer := gs.Tracer()
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
			var span trace.Span
			var ginSpan trace.Span

			// start trace span
			if reportTrace {
				if ginSpan, _ = ctx.Value(state.KrakenDContextOTELStrKey).(trace.Span); ginSpan != nil {
					// wrap the context (that might contain at the lower level a gin.Context)
					// with the library context key.
					// (This key is set at the gin.Context level because gin does not use
					// standard context.Context).
					ctx = trace.ContextWithSpan(ctx, ginSpan)
				}
				// start the new Context, for the stage:
				ctx, span = tracer.Start(ctx, name)
				if ginSpan != nil {
					// we need to update the key with the new span, otherwise, deeper middlewares
					// (like when from pipe -> proxy), would get the span from the parent, instead
					// of the one in the context
					ctx = context.WithValue(ctx, state.KrakenDContextOTELStrKey, span)
				}
				// TODO: CHECK that we have this attribute set !!
				span.SetAttributes(attribute.String("krakend.stage", stage))
			}

			startedAt := time.Now()
			resp, err := n(ctx, req)
			durationInSecs := float64(time.Since(startedAt)) / float64(time.Second)
			if reportMetrics {
				duration.Record(ctx, durationInSecs, metricAttrs)
			}

			// finish trace span:
			if reportTrace {
				if err != nil {
					// TODO: check if the error
					if errors.Is(err, context.Canceled) {
						span.SetAttributes(attribute.Bool("canceled", true))
					} else {
						span.SetAttributes(attribute.String("error", err.Error()))
					}
				}
				span.SetAttributes(attribute.Bool("complete", resp != nil && resp.IsComplete))
				span.End()
			}
			return resp, err
		}
	}
}

// ProxyFactory returns a pipe stage factory that wraps the provided proxy factory with the
// instrumentation [Middleware] based on the configuration options.
func ProxyFactory(pf proxy.Factory, gsfn state.GetterFn, opts *kotelconfig.PipeOpts) proxy.FactoryFunc {
	if opts == nil {
		return pf.New
	}
	if gsfn == nil {
		gsfn = state.GlobalState
	}

	metricsEnabled := !opts.DisableMetrics
	tracesEnabled := !opts.DisableTraces

	if !metricsEnabled && !tracesEnabled {
		return pf.New
	}

	return func(cfg *config.EndpointConfig) (proxy.Proxy, error) {
		next, err := pf.New(cfg)
		if err != nil {
			return next, err
		}

		// TODO: check the followint warning:
		// WARNING: this changes how it was working before:
		// in original opencensus, we prefixed the endpoint with a `pipe` prefix
		// return Middleware("pipe-" + cfg.Endpoint)(next), nil
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.Endpoint)
		return Middleware(urlPattern, "pipe", gsfn, metricsEnabled, tracesEnabled)(next), nil
	}
}

// BackendFactory returns a backend factory that wraps the provided backend factory with the
// instrumentation [Middleware] based on the configuration options.
func BackendFactory(bf proxy.BackendFactory, gsfn state.GetterFn, opts *kotelconfig.BackendOpts) proxy.BackendFactory {
	if opts == nil || (opts.Metrics.DisableStage && opts.Traces.DisableStage) {
		return bf
	}
	metricsEnabled := !opts.Metrics.DisableStage
	tracesEnabled := !opts.Traces.DisableStage

	return func(cfg *config.Backend) proxy.Proxy {
		next := bf(cfg)
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.URLPattern)
		return Middleware(urlPattern, "backend", gsfn, metricsEnabled, tracesEnabled)(next)
	}
}
