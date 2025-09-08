package lura

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"

	kotelconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/state"
)

func tracesMiddleware(next proxy.Proxy, mt *middlewareTracer) func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
	return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
		ctx, span := mt.start(ctx, req)
		resp, err := next(ctx, req)
		mt.end(span, resp, err)
		return resp, err
	}
}

func metricsMiddleware(next proxy.Proxy, mm *middlewareMeter) func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
	return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
		startedAt := time.Now()
		resp, err := next(ctx, req)
		durationInSecs := float64(time.Since(startedAt)) / float64(time.Second)
		mm.report(ctx, durationInSecs, resp, err)
		return resp, err
	}
}

func metricsAndTracesMiddleware(next proxy.Proxy, mm *middlewareMeter, mt *middlewareTracer) func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
	return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
		ctx, span := mt.start(ctx, req)
		startedAt := time.Now()
		resp, err := next(ctx, req)
		durationInSecs := float64(time.Since(startedAt)) / float64(time.Second)
		mm.report(ctx, durationInSecs, resp, err)
		mt.end(span, resp, err)
		return resp, err
	}
}

// middleware creates a proxy that instruments the proxy it wraps by creating an span if enabled,
// and report the duration of this stage in metrics if enabled.
func middleware(gs state.OTEL, metricsEnabled bool, tracesEnabled bool,
	stageName string, urlPattern string, metricsAttrs, tracesAttrs []attribute.KeyValue,
	reportHeaders bool, skipHeaders []string,
) proxy.Middleware {
	var mt *middlewareTracer
	var mm *middlewareMeter
	var err error
	if metricsEnabled {
		mm, err = newMiddlewareMeter(gs, stageName, metricsAttrs)
		if err != nil {
			// TODO: log the error
			metricsEnabled = false
		}
	}
	if tracesEnabled {
		mt = newMiddlewareTracer(gs, urlPattern, stageName, reportHeaders, skipHeaders, tracesAttrs)
		if mt == nil {
			// TODO: log the error
			tracesEnabled = false
		}
	}

	return func(next ...proxy.Proxy) proxy.Proxy {
		if len(next) > 1 {
			panic(proxy.ErrTooManyProxies)
		}
		if len(next) < 1 {
			panic(proxy.ErrNotEnoughProxies)
		}
		n := next[0]

		if metricsEnabled {
			if tracesEnabled {
				return metricsAndTracesMiddleware(n, mm, mt)
			}
			return metricsMiddleware(n, mm)
		} else if tracesEnabled {
			return tracesMiddleware(n, mt)
		}
		return n
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
		attrs := []attribute.KeyValue{
			semconv.HTTPRequestMethodKey.String(cfg.Method),
			semconv.HTTPRoute(urlPattern),
		}

		// Add configured static attributes
		metricsAttrs := attrs
		tracesAttrs := attrs
		for _, kv := range pipeOpts.MetricsStaticAttributes {
			if kv.Key != "" && kv.Value != "" {
				metricsAttrs = append(metricsAttrs, attribute.String(kv.Key, kv.Value))
			}
		}

		for _, kv := range pipeOpts.TracesStaticAttributes {
			if kv.Key != "" && kv.Value != "" {
				tracesAttrs = append(tracesAttrs, attribute.String(kv.Key, kv.Value))
			}
		}

		return middleware(gs, !pipeOpts.DisableMetrics, !pipeOpts.DisableTraces,
			"proxy", urlPattern, metricsAttrs, tracesAttrs, pipeOpts.ReportHeaders,
			pipeOpts.SkipHeaders)(next), nil
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
		metricsDisabled := backendOpts != nil && backendOpts.Metrics != nil && backendOpts.Metrics.DisableStage
		tracesDisabled := backendOpts != nil && backendOpts.Traces != nil && backendOpts.Traces.DisableStage
		if metricsDisabled && tracesDisabled {
			return next
		}

		gs := otelCfg.BackendOTEL(cfg)
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.URLPattern)
		parentEndpoint := kotelconfig.NormalizeURLPattern(cfg.ParentEndpoint)
		attrs := []attribute.KeyValue{
			semconv.HTTPRequestMethodKey.String(cfg.Method),
			semconv.HTTPRoute(urlPattern), // <- for traces we can use URLFull to not have the matched path
			attribute.String("krakend.endpoint.route", parentEndpoint),
			attribute.String("krakend.endpoint.method", cfg.ParentEndpointMethod),
		}

		// Add configured static attributes
		metricsAttrs := attrs
		tracesAttrs := attrs
		if backendOpts.Metrics != nil {
			for _, kv := range backendOpts.Metrics.StaticAttributes {
				if kv.Key != "" && kv.Value != "" {
					metricsAttrs = append(metricsAttrs, attribute.String(kv.Key, kv.Value))
				}
			}
		}

		reportHeaders := false
		if backendOpts.Traces != nil {
			reportHeaders = backendOpts.Traces.ReportHeaders
			for _, kv := range backendOpts.Traces.StaticAttributes {
				if kv.Key != "" && kv.Value != "" {
					tracesAttrs = append(tracesAttrs, attribute.String(kv.Key, kv.Value))
				}
			}
		}

		return middleware(gs, !metricsDisabled, !tracesDisabled,
			"backend", urlPattern, metricsAttrs, tracesAttrs, reportHeaders,
			backendOpts.Traces.SkipHeaders)(next)
	}
}
