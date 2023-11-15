package lura

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/luraproject/lura/v2/config"
	transport "github.com/luraproject/lura/v2/transport/http/client"

	otelconfig "github.com/krakend/krakend-otel/config"
	clienthttp "github.com/krakend/krakend-otel/http/client"
)

var (
	defaultSkipPaths = []string{
		"/healthz",
		"/_ah/health",
		"/__debug",
		"/__echo",
	}
	defaultOpts = otelconfig.BackendOpts{
		Metrics: &otelconfig.BackendMetricOpts{
			RoundTrip:          true,
			ReadPayload:        true,
			DetailedConnection: true,
			StaticAttributes:   make(map[string]string),
		},
		Traces: &otelconfig.BackendTraceOpts{
			RoundTrip:          true,
			ReadPayload:        true,
			DetailedConnection: true,
			StaticAttributes:   make(map[string]string),
		},
		SkipPaths: defaultSkipPaths,
	}
)

// HTTPRequestExecutorFromConfig creates an HTTPRequestExecutor to be used
// for the backend requests.
func HTTPRequestExecutorFromConfig(clientFactory transport.HTTPClientFactory,
	cfg *config.Backend, opts *otelconfig.BackendOpts) transport.HTTPRequestExecutor {

	if !opts.Enabled() {
		// no configuration for the backend, then .. no metrics nor tracing:
		return transport.DefaultHTTPRequestExecutor(clientFactory)
	}

	if opts == nil {
		opts = &defaultOpts
	}

	if len(opts.SkipPaths) == 0 {
		// if there are no defined skip paths, we use the default ones:
		opts.SkipPaths = defaultSkipPaths
	}

	if opts.Metrics == nil {
		opts.Metrics = defaultOpts.Metrics
	}
	if opts.Traces == nil {
		opts.Traces = defaultOpts.Traces
	}

	for _, sp := range opts.SkipPaths {
		if cfg.URLPattern == sp {
			return transport.DefaultHTTPRequestExecutor(clientFactory)
		}
	}

	urlPattern := otelconfig.NormalizeURLPattern(cfg.URLPattern)

	// we set a basic list of attributes that will be set for both traces and
	// metrics, as those are expected to have low cardinality
	// - the method: one of the `GET`, `POST`, `PUT` .. etc
	// - the "path" , that is actually the path "template" to not have different values
	//      for different params but the same endpoint.
	// - the krakend stage, that can be one of
	//      - router: includes from the very point of receiving a request until
	//          a response is returned to the client.
	//      - pipe: includes all the processing that is performed
	//          for the endpoint part of a request (like merging and grouping
	//          responses from different backends).
	//      - backend: includes all middlewares and processing that is done for
	//          a given backend.
	//      - backend-request: when reporting the request to the backends
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodOriginal(cfg.Method),
		semconv.URLPath(urlPattern), // <- for traces we can use URLFull to not have the matched path
		attribute.String("krakend.stage", "backend-request"),
	}

	if len(cfg.Host) > 0 {
		attrs = append(attrs, semconv.ServerAddress(cfg.Host[0]))
	}

	// TODO: check how we want to deal with this "clientName"
	endpoint := "" // we need to have the endpoint accessible in the backend config structure
	clientName := endpoint + urlPattern

	t := clienthttp.TransportOptions{
		MetricsOpts: clienthttp.TransportMetricsOptions{
			RoundTrip:          opts.Metrics.RoundTrip,
			ReadPayload:        opts.Metrics.ReadPayload,
			DetailedConnection: opts.Metrics.DetailedConnection,
			FixedAttributes:    attrs,
		},
		TracesOpts: clienthttp.TransportTracesOptions{
			RoundTrip:          opts.Traces.RoundTrip,
			ReadPayload:        opts.Traces.ReadPayload,
			DetailedConnection: opts.Metrics.DetailedConnection,
			FixedAttributes:    attrs,
		},
	}

	return func(ctx context.Context, req *http.Request) (*http.Response, error) {
		c, err := clienthttp.InstrumentedHTTPClient(clientFactory(ctx), &t, clientName)
		if err != nil {
			return nil, err
		}
		return c.Do(req.WithContext(ctx))
	}
}
