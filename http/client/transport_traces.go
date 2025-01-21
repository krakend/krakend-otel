package client

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	otelhttp "github.com/krakend/krakend-otel/http"
)

// TransportTracesOptions defines what information
// is enabled, and extra fixed attributes to add to
// the trace.
type TransportTracesOptions struct {
	RoundTrip          bool                 // use a span for the round trip
	ReadPayload        bool                 // use a span for the process of reading the full body
	DetailedConnection bool                 // add extra detail about the connection to the server: dns lookup, tls...
	FixedAttributes    []attribute.KeyValue // "static" attributes set at config time.
	ReportHeaders      bool
}

// Enabled returns if the transport should create a trace.
func (o *TransportTracesOptions) Enabled() bool {
	return o.RoundTrip || o.ReadPayload
}

type transportTraces struct {
	tracer             trace.Tracer
	spanName           string
	fixedAttrs         []attribute.KeyValue
	detailedConnection bool
	reportHeaders      bool
}

func newTransportTraces(tracesOpts *TransportTracesOptions, tracer trace.Tracer, spanName string) *transportTraces {
	if tracer == nil {
		return nil
	}

	return &transportTraces{
		tracer:             tracer,
		spanName:           spanName,
		fixedAttrs:         tracesOpts.FixedAttributes,
		detailedConnection: tracesOpts.DetailedConnection,
		reportHeaders:      tracesOpts.ReportHeaders,
	}
}

func (t *transportTraces) start(rtt *roundTripTracking,
	propagator propagation.TextMapPropagator,
) {
	if t == nil || rtt.req == nil {
		return
	}

	ctx, span := t.tracer.Start(rtt.req.Context(), t.spanName, trace.WithSpanKind(trace.SpanKindClient))
	if span == nil || !span.IsRecording() {
		// we might not be recording because of sampling
		return
	}
	rtt.span = span
	rtt.req = rtt.req.WithContext(ctx)

	reqAttrs := otelhttp.TraceRequestAttrs(rtt.req)
	// propagate the context, see `example/passthrough/main.go` in OTEL repo
	// SpanContextToRequest will modify its Request argument, which is
	// contrary to the contract for http.RoundTripper, so we need to
	// pass it a copy of the Request.
	// However, the Request struct itself was already copied by
	// the WithContext calls above and so we just need to copy the header.
	header := make(http.Header, len(rtt.req.Header)+1)
	for k, v := range rtt.req.Header {
		header[k] = v
		if t.reportHeaders {
			reqAttrs = append(reqAttrs,
				attribute.StringSlice("http.request.header."+strings.ToLower(k), v))
		}
	}
	rtt.req.Header = header
	propagator.Inject(rtt.req.Context(), propagation.HeaderCarrier(rtt.req.Header))

	rtt.span.SetAttributes(t.fixedAttrs...)
	rtt.span.SetAttributes(reqAttrs...)
}

func (t *transportTraces) end(rtt *roundTripTracking) {
	if t == nil || rtt.span == nil || !rtt.span.IsRecording() {
		return
	}

	if rtt.err != nil {
		rtt.span.RecordError(rtt.err)
		rtt.span.SetStatus(codes.Error, rtt.err.Error())
	} else {
		respAttrs := otelhttp.TraceResponseAttrs(rtt.resp)
		if t.reportHeaders {
			for k, v := range rtt.resp.Header {
				respAttrs = append(respAttrs,
					attribute.StringSlice("http.response.header."+strings.ToLower(k), v))
			}
		}
		rtt.span.SetAttributes(respAttrs...)
		rtt.span.SetAttributes(attribute.Float64("response-duration", rtt.latencyInSecs))
		if t.detailedConnection {
			rtt.span.SetAttributes(
				attribute.Float64("get-conn-duration", rtt.getConnLatency),
				attribute.Float64("dns-duration", rtt.dnsLatency),
				attribute.Float64("tls-duration", rtt.tlsLatency),
			)
			rtt.span.AddEvent("first-byte-time", trace.WithTimestamp(rtt.firstByteTime))
		}
		rtt.span.SetStatus(codes.Ok, "")
	}

	rtt.span.End()
}
