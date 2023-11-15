package lura

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"github.com/luraproject/lura/v2/config"
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
	cfg *config.Backend, opts *otelconfig.BackendOpts, skipPaths []string,
	getState otelstate.GetterFn,
) transport.HTTPRequestExecutor {
	cf := InstrumentedHTTPClientFactory(clientFactory, cfg, opts, skipPaths, getState)
	return transport.DefaultHTTPRequestExecutor(cf)
}

func InstrumentedHTTPClientFactory(clientFactory transport.HTTPClientFactory,
	cfg *config.Backend, opts *otelconfig.BackendOpts, skipPaths []string,
	getState otelstate.GetterFn,
) transport.HTTPClientFactory {
	for _, sp := range skipPaths {
		if cfg.ParentEndpoint == sp {
			return clientFactory
		}
	}

	if !opts.Enabled() {
		// no configuration for the backend, then .. no metrics nor tracing:
		return clientFactory
	}

	if opts == nil {
		opts = &defaultOpts
	}

	if opts.Metrics == nil {
		opts.Metrics = defaultOpts.Metrics
	}
	if opts.Traces == nil {
		opts.Traces = defaultOpts.Traces
	}

	urlPattern := otelconfig.NormalizeURLPattern(cfg.URLPattern)
	attrs := backendConfigAttributes(cfg)

	metricAttrs := attrs
	if len(opts.Metrics.StaticAttributes) > 0 {
		for _, kv := range opts.Metrics.StaticAttributes {
			if len(kv.Key) > 0 && len(kv.Value) > 0 {
				metricAttrs = append(metricAttrs, attribute.String(kv.Key, kv.Value))
			}
		}
	}

	traceAttrs := make([]attribute.KeyValue, len(attrs),
		len(attrs)+1+len(opts.Traces.StaticAttributes))
	copy(traceAttrs, attrs)
	traceAttrs = append(traceAttrs, attribute.String("krakend.stage", "backend-request"))
	if len(opts.Traces.StaticAttributes) > 0 {
		for _, kv := range opts.Traces.StaticAttributes {
			if len(kv.Key) > 0 && len(kv.Value) > 0 {
				traceAttrs = append(traceAttrs, attribute.String(kv.Key, kv.Value))
			}
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
		OTELInstance: getState,
	}

	return func(ctx context.Context) *http.Client {
		return clienthttp.InstrumentedHTTPClient(clientFactory(ctx), &t, urlPattern)
	}
}
