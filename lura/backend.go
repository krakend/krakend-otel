package lura

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"

	luraconfig "github.com/luraproject/lura/v2/config"
	transport "github.com/luraproject/lura/v2/transport/http/client"

	otelconfig "github.com/krakend/krakend-otel/config"
	clienthttp "github.com/krakend/krakend-otel/http/client"
	otelstate "github.com/krakend/krakend-otel/state"
)

var defaultOpts = otelconfig.BackendOpts{
	Metrics: &otelconfig.BackendMetricOpts{
		DisableStage:       false,
		RoundTrip:          true,
		ReadPayload:        true,
		DetailedConnection: true,
		StaticAttributes:   make(otelconfig.Attributes, 0),
	},
	Traces: &otelconfig.BackendTraceOpts{
		DisableStage:       false,
		RoundTrip:          true,
		ReadPayload:        true,
		DetailedConnection: true,
		StaticAttributes:   make(otelconfig.Attributes, 0),
	},
}

// HTTPRequestExecutorFromConfig creates an HTTPRequestExecutor to be used
// for the backend requests.
func HTTPRequestExecutorFromConfig(clientFactory transport.HTTPClientFactory,
	cfg *luraconfig.Backend,
) transport.HTTPRequestExecutor {
	cf := InstrumentedHTTPClientFactory(clientFactory, cfg)
	return transport.DefaultHTTPRequestExecutor(cf)
}

func InstrumentedHTTPClientFactory(clientFactory transport.HTTPClientFactory,
	cfg *luraconfig.Backend,
) transport.HTTPClientFactory {
	otelCfg := otelstate.GlobalConfig()
	if otelCfg == nil {
		return clientFactory
	}
	if otelCfg.SkipEndpoint(cfg.ParentEndpoint) {
		return clientFactory
	}

	opts := otelCfg.BackendOpts(cfg)
	if !opts.Enabled() {
		return clientFactory
	}
	otelState := otelCfg.BackendOTEL(cfg)

	// this might not be necessary:
	if opts.Metrics == nil {
		opts.Metrics = defaultOpts.Metrics
	}
	if opts.Traces == nil {
		opts.Traces = defaultOpts.Traces
	}

	urlPattern := otelconfig.NormalizeURLPattern(cfg.URLPattern)
	parentEndpoint := otelconfig.NormalizeURLPattern(cfg.ParentEndpoint)
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(cfg.Method),
		semconv.HTTPRoute(urlPattern), // <- for traces we can use URLFull to not have the matched path
		attribute.String("krakend.endpoint.route", parentEndpoint),
		attribute.String("krakend.endpoint.method", cfg.ParentEndpointMethod),
	}

	metricAttrs := attrs
	for _, kv := range opts.Metrics.StaticAttributes {
		if len(kv.Key) > 0 && len(kv.Value) > 0 {
			metricAttrs = append(metricAttrs, attribute.String(kv.Key, kv.Value))
		}
	}

	traceAttrs := make([]attribute.KeyValue, len(attrs),
		len(attrs)+1+len(opts.Traces.StaticAttributes))
	copy(traceAttrs, attrs)
	traceAttrs = append(traceAttrs,
		attribute.String("krakend.stage", "backend-request"))
	for _, kv := range opts.Traces.StaticAttributes {
		if len(kv.Key) > 0 && len(kv.Value) > 0 {
			traceAttrs = append(traceAttrs, attribute.String(kv.Key, kv.Value))
		}
	}

	t := clienthttp.TransportOptions{
		MetricsOpts: clienthttp.TransportMetricsOptions{
			RoundTrip:          opts.Metrics.RoundTrip,
			ReadPayload:        opts.Metrics.ReadPayload,
			DetailedConnection: opts.Metrics.DetailedConnection,
			FixedAttributes:    metricAttrs,
		},
		TracesOpts: clienthttp.TransportTracesOptions{
			RoundTrip:          opts.Traces.RoundTrip,
			ReadPayload:        opts.Traces.ReadPayload,
			DetailedConnection: opts.Traces.DetailedConnection,
			FixedAttributes:    traceAttrs,
			ReportHeaders:      opts.Traces.ReportHeaders,
		},
		OTELInstance: otelState,
	}

	return func(ctx context.Context) *http.Client {
		return clienthttp.InstrumentedHTTPClient(clientFactory(ctx), &t, urlPattern)
	}
}
