package client

import (
	"context"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	otelio "github.com/krakend/krakend-otel/io"
	"github.com/krakend/krakend-otel/state"
)

// TransportOptions defines the detail we want for
// the metrics and traces.
// See [TrasnportMetricsOptions] and [TransportTracesOptions] for
// more details.
// The OTELInstance member defines a function to obtain a State to
// use with this transport. The state getter is used at configuration
// time, not at runtime.
type TransportOptions struct {
	MetricsOpts TransportMetricsOptions
	TracesOpts  TransportTracesOptions

	OTELInstance state.OTEL
}

// readerWrapper defines a function to wrap a reader
type readerWrapperFn func(r io.Reader, ctx context.Context) io.ReadCloser

// Transport is an http.RoundTripper that instruments all outgoing requests with
// OpenTelemetry metrics and tracing.
//
// The zero value is intended to be a useful default, but for
// now it's recommended that you explicitly set Propagation, since the default
// for this may change.
type Transport struct {
	// Base may be set to wrap another http.RoundTripper that does the actual
	// requests. By default http.DefaultTransport is used.
	//
	// If base HTTP roundtripper implements CancelRequest,
	// the returned round tripper will be cancelable.
	base http.RoundTripper

	// Propagation defines how traces are propagated. If unspecified, a default
	// (currently B3 format can be configured outside) will be used.
	propagator propagation.TextMapPropagator

	// StartOptions are applied to the span started by this Transport around each
	// request.
	//
	// StartOptions.SpanKind will always be set to trace.SpanKindClient
	// for spans started by this transport.
	// StartOptions trace.StartOptions

	tracesOpts  TransportTracesOptions
	metricsOpts TransportMetricsOptions
	otelState   state.OTEL

	metrics *transportMetrics
	traces  *transportTraces

	readerWrapper readerWrapperFn
}

func readWrapperBuilder(metricsOpts *TransportMetricsOptions, tracesOpts *TransportTracesOptions,
	meter metric.Meter, tracer trace.Tracer,
) readerWrapperFn {
	if !metricsOpts.ReadPayload && !tracesOpts.ReadPayload {
		// no metrics or traces for the payload reading
		return func(r io.Reader, ctx context.Context) io.ReadCloser {
			rc, ok := r.(io.ReadCloser)
			if !ok {
				rc = io.NopCloser(r)
			}
			return rc
		}
	}

	var attrM []attribute.KeyValue
	var attrT []attribute.KeyValue
	var t trace.Tracer
	var m metric.Meter

	if metricsOpts.ReadPayload {
		attrM = metricsOpts.FixedAttributes
		m = meter
	}

	if tracesOpts.ReadPayload {
		attrT = tracesOpts.FixedAttributes
		t = tracer
	}

	irf := otelio.NewInstrumentedReaderFactory("http.client.response.read.", attrT, attrM, t, m)
	return func(r io.Reader, ctx context.Context) io.ReadCloser {
		return irf(r, ctx)
	}
}

// NewRoundTripper creates an instrumented round tripper.
func NewRoundTripper(base http.RoundTripper, metricsOpts TransportMetricsOptions,
	tracesOpts TransportTracesOptions, clientName string, otelState state.OTEL,
) http.RoundTripper {
	rt := newTransport(base, metricsOpts, tracesOpts, clientName, otelState)
	if rt == nil {
		return base
	}
	return rt
}

func newTransport(base http.RoundTripper, metricsOpts TransportMetricsOptions,
	tracesOpts TransportTracesOptions, clientName string, otelState state.OTEL,
) *Transport {
	if !tracesOpts.Enabled() && !metricsOpts.Enabled() {
		return nil
	}
	if otelState == nil {
		return nil
	}

	meter := otelState.Meter()
	tracer := otelState.Tracer()

	return &Transport{
		base:          base,
		propagator:    otel.GetTextMapPropagator(),
		otelState:     otelState,
		tracesOpts:    tracesOpts,
		metricsOpts:   metricsOpts,
		metrics:       newTransportMetrics(&metricsOpts, meter, clientName),
		traces:        newTransportTraces(&tracesOpts, tracer, clientName),
		readerWrapper: readWrapperBuilder(&metricsOpts, &tracesOpts, meter, tracer),
	}
}

// RoundTrip implements http.RoundTripper, delegating to Base and recording
// metrics and traces for the request.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rtt := roundTripTracking{
		req: req,
	}
	if t.tracesOpts.DetailedConnection || t.metricsOpts.DetailedConnection {
		rtt.withClientTrace()
	}
	t.traces.start(&rtt, t.propagator)

	requestSentAt := time.Now()
	rtt.resp, rtt.err = t.base.RoundTrip(rtt.req)
	rtt.latencyInSecs = float64(time.Since(requestSentAt)) / float64(time.Second)

	t.metrics.report(&rtt, t.metricsOpts.FixedAttributes)

	if rtt.resp != nil && rtt.resp.Body != nil {
		rtt.resp.Body = t.readerWrapper(rtt.resp.Body, rtt.req.Context())
	}
	t.traces.end(&rtt)
	return rtt.resp, rtt.err
}
