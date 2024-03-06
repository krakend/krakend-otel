package lura

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"

	kotelconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/state"
)

func middlewareProxy(next proxy.Proxy, tracer trace.Tracer, urlPattern string, duration metric.Float64Histogram,
	tAttrs []attribute.KeyValue, mAttrs metric.MeasurementOption, reportHeaders bool,
) func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
	return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
		var span trace.Span
		if tracer != nil {
			ctx, span = tracer.Start(ctx, urlPattern)
			span.SetAttributes(tAttrs...)
			span.SetAttributes(semconv.HTTPRequestMethodKey.String(req.Method))
			if reportHeaders {
				for hk, hv := range req.Headers {
					span.SetAttributes(attribute.StringSlice("http.request.header."+strings.ToLower(hk), hv))
				}
			}
		}

		startedAt := time.Now()
		resp, err := next(ctx, req)

		durationInSecs := float64(time.Since(startedAt)) / float64(time.Second)
		if duration != nil {
			isErr := false
			isCanceled := false
			if err != nil {
				if errors.Is(err, context.Canceled) {
					isCanceled = true
				} else {
					isErr = true
				}
			}
			metricDynAttrs := metric.WithAttributes(
				semconv.HTTPRequestMethodKey.String(req.Method),
				attribute.Bool("error", isErr),
				attribute.Bool("canceled", isCanceled),
				attribute.Bool("complete", resp != nil && resp.IsComplete))
			duration.Record(ctx, durationInSecs, mAttrs, metricDynAttrs)
		}

		if tracer != nil {
			if err != nil {
				if errors.Is(err, context.Canceled) {
					span.SetAttributes(attribute.Bool("canceled", true))
				} else {
					span.SetAttributes(attribute.String("error", err.Error()))
				}
				span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(500))
			} else if resp != nil {
				span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(resp.Metadata.StatusCode))
				if reportHeaders {
					for hk, hv := range resp.Metadata.Headers {
						span.SetAttributes(attribute.StringSlice("http.response.header."+strings.ToLower(hk), hv))
					}
				}
			}
			span.SetAttributes(attribute.Bool("complete", resp != nil && resp.IsComplete))
			span.End()
		}
		return resp, err
	}
}

// Middleware creates a proxy that instruments the proxy it wraps by creating an span if enabled,
// and report the duration of this stage in metrics if enabled.
func Middleware(gs state.OTEL, metricsEnabled bool, tracesEnabled bool,
	stageName string, urlPattern string, staticAttrs []attribute.KeyValue,
	reportHeaders bool,
) proxy.Middleware {
	mAttrs := make([]attribute.KeyValue, 0, len(staticAttrs)+1)
	tAttrs := make([]attribute.KeyValue, 0, len(staticAttrs)+1)
	for _, sa := range staticAttrs {
		mAttrs = append(mAttrs, sa)
		tAttrs = append(tAttrs, sa)
	}

	tAttrs = append(tAttrs, attribute.String("krakend.stage", stageName))
	metricAttrs := metric.WithAttributes(mAttrs...)

	return func(next ...proxy.Proxy) proxy.Proxy {
		if len(next) > 1 {
			panic(proxy.ErrTooManyProxies)
		}
		if len(next) < 1 {
			panic(proxy.ErrNotEnoughProxies)
		}

		var duration metric.Float64Histogram
		if metricsEnabled {
			meter := gs.Meter()
			var err error
			duration, err = meter.Float64Histogram("krakend."+stageName+".duration",
				kotelconfig.TimeBucketsOpt)
			if err != nil {
				duration = nil
			}
		}

		var tracer trace.Tracer
		if tracesEnabled {
			tracer = gs.Tracer()
		}
		return middlewareProxy(next[0], tracer, urlPattern, duration,
			tAttrs, metricAttrs, reportHeaders)
	}
}

// ProxyFactory returns a proxy stage factory that wraps the provided proxy factory with the
// instrumentation [Middleware] based on the configuration options.
func ProxyFactory(pf proxy.Factory) proxy.FactoryFunc {
	otelCfg := state.GlobalConfig()
	if otelCfg == nil {
		return pf.New
	}

	return func(cfg *config.EndpointConfig) (proxy.Proxy, error) {
		next, err := pf.New(cfg)
		if err != nil {
			return next, err
		}

		if otelCfg.SkipEndpoint(cfg.Endpoint) {
			return next, nil
		}

		pipeOpts := otelCfg.EndpointPipeOpts(cfg)
		if pipeOpts.DisableMetrics && pipeOpts.DisableTraces {
			return next, nil
		}

		gs := otelCfg.EndpointOTEL(cfg)
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.Endpoint)
		attrs := []attribute.KeyValue{semconv.HTTPRoute(urlPattern)}
		return Middleware(gs, !pipeOpts.DisableMetrics, !pipeOpts.DisableTraces,
			"proxy", urlPattern, attrs, pipeOpts.ReportHeaders)(next), nil
	}
}

// BackendFactory returns a backend factory that wraps the provided backend factory with the
// instrumentation [Middleware] based on the configuration options.
func BackendFactory(bf proxy.BackendFactory) proxy.BackendFactory {
	otelCfg := state.GlobalConfig()
	if otelCfg == nil {
		return bf
	}

	return func(cfg *config.Backend) proxy.Proxy {
		next := bf(cfg)
		if otelCfg.SkipEndpoint(cfg.ParentEndpoint) {
			return next
		}

		backendOpts := otelCfg.BackendOpts(cfg)
		gs := otelCfg.BackendOTEL(cfg)
		staticAttrs := backendConfigAttributes(cfg)
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.URLPattern)
		return Middleware(gs, !backendOpts.Metrics.DisableStage, !backendOpts.Traces.DisableStage,
			"backend", urlPattern, staticAttrs, backendOpts.Traces.ReportHeaders)(next)
	}
}
