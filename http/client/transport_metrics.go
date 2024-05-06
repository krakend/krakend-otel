package client

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.21.0"

	kotelconfig "github.com/krakend/krakend-otel/config"
	otelhttp "github.com/krakend/krakend-otel/http"
)

// TransportMetricsOptions contains the options to enable / disable
// for reporting metrics, and a set of fixed attributes to add
// to all metrics.
type TransportMetricsOptions struct {
	RoundTrip          bool                 // provide the round trip metrics
	ReadPayload        bool                 // provide metrics for the reading the full body
	DetailedConnection bool                 // provide detailed metrics about the connection: dns lookup, tls ...
	FixedAttributes    []attribute.KeyValue // "static" attributes set at config time.
}

// Enabled tells if metrics should be reported for the transport.
func (o *TransportMetricsOptions) Enabled() bool {
	return o.RoundTrip || o.ReadPayload
}

// transportMetrics holds the metric instruments for the round trip
type transportMetrics struct {
	// total of initiated requests (sucessful, failed and cancelled)
	requestsStarted  metric.Int64Counter
	requestsFailed   metric.Int64Counter
	requestsCanceled metric.Int64Counter
	requestsTimedOut metric.Int64Counter

	// the value of the Content-Length header for the request (not the
	// actual written bytes of the request, that might be cancelled
	// when it is already on flight.
	requestContentLength metric.Int64Counter

	responseLatency metric.Float64Histogram

	// the response content lenght comes from the server provided header
	// and might differ from the actual number of bytes read from the body
	responseContentLength   metric.Int64Histogram
	responseNoContentLength metric.Int64Counter

	// from the httptrace details
	detailsEnabled bool
	getConnLatency metric.Float64Histogram
	dnsLatency     metric.Float64Histogram
	tlsLatency     metric.Float64Histogram

	// to identify the source of the request (in KrakenD the front facing endpoint)
	clientName string
}

func newTransportMetrics(metricsOpts *TransportMetricsOptions, meter metric.Meter, clientName string) *transportMetrics {
	if meter == nil {
		return nil
	}

	var tm transportMetrics
	tm.requestsStarted, _ = meter.Int64Counter("http.client.request.started.count")   // number of reqs started
	tm.requestsFailed, _ = meter.Int64Counter("http.client.request.failed.count")     // number of reqs failed
	tm.requestsCanceled, _ = meter.Int64Counter("http.client.request.canceled.count") // number of canceled requests
	tm.requestsTimedOut, _ = meter.Int64Counter("http.client.request.timedout.count") // numer of timedout request (inclued in failed)

	tm.requestContentLength, _ = meter.Int64Counter("http.client.request.size") // the value of the Content-Length header for the request

	tm.responseLatency, _ = meter.Float64Histogram("http.client.duration", kotelconfig.TimeBucketsOpt)
	tm.responseContentLength, _ = meter.Int64Histogram("http.client.response.size", kotelconfig.SizeBucketsOpt)
	tm.responseNoContentLength, _ = meter.Int64Counter("http.client.response.no-content-length")

	tm.detailsEnabled = metricsOpts.DetailedConnection
	tm.getConnLatency, _ = meter.Float64Histogram("http.client.request.get-conn.duration", kotelconfig.TimeBucketsOpt)
	tm.dnsLatency, _ = meter.Float64Histogram("http.client.request.dns.duration", kotelconfig.TimeBucketsOpt)
	tm.tlsLatency, _ = meter.Float64Histogram("http.client.request.tls.duration", kotelconfig.TimeBucketsOpt)
	return &tm
}

func (m *transportMetrics) report(rtt *roundTripTracking, attrs []attribute.KeyValue) {
	if m == nil || m.requestsStarted == nil {
		// if metrics are nil or not initialized, we just return
		return
	}

	customAttributes := otelhttp.CustomMetricAttributes(rtt.req)
	attrM := make([]attribute.KeyValue, len(attrs), len(attrs)+4+len(customAttributes))

	copy(attrM, attrs)
	if len(m.clientName) > 0 {
		attrM = append(attrM, attribute.Key("clientname").String(m.clientName))
	}
	attrM = append(attrM, semconv.HTTPRequestMethodKey.String(rtt.req.Method))
	attrM = append(attrM, semconv.ServerAddress(rtt.req.RemoteAddr))

	attrM = append(attrM, customAttributes...)
	statusCode := 0
	if rtt.err == nil {
		// if we fail on the client side, we do not have a status code, but we
		// want it set to 0 to be displayed on the dashboard
		statusCode = int(rtt.resp.StatusCode)
	}
	attrM = append(attrM, semconv.HTTPResponseStatusCode(statusCode))
	attrOpt := metric.WithAttributeSet(attribute.NewSet(attrM...))

	ctx := rtt.req.Context()

	m.requestsStarted.Add(ctx, 1, attrOpt)
	if rtt.req.ContentLength >= 0 {
		// TOOD: should we check the http verb / method to report this ?
		m.requestContentLength.Add(ctx, rtt.req.ContentLength, attrOpt)
	}

	if rtt.err != nil {
		reqCtx := rtt.req.Context()
		var ctxErr error
		if reqCtx != nil {
			ctxErr = rtt.req.Context().Err()
		}
		if errors.Is(ctxErr, context.Canceled) {
			// ATTENTION: a Cancelled requests is not considered failed
			m.requestsCanceled.Add(ctx, 1, attrOpt)
		} else if errors.Is(ctxErr, context.DeadlineExceeded) {
			m.requestsTimedOut.Add(ctx, 1, attrOpt)
			m.requestsFailed.Add(ctx, 1, attrOpt)
		} else {
			m.requestsFailed.Add(ctx, 1, attrOpt)
		}
	}

	m.responseLatency.Record(ctx, rtt.latencyInSecs, attrOpt)
	if rtt.req.Method != "HEAD" && rtt.resp != nil {
		if rtt.resp.ContentLength >= 0 {
			// it might be the case were we receive a chunked response, and then
			// we will not record a metric for it.
			m.responseContentLength.Record(ctx, rtt.resp.ContentLength, attrOpt)
		} else {
			m.responseNoContentLength.Add(ctx, 1, attrOpt)
		}
	}

	if m.detailsEnabled {
		m.getConnLatency.Record(ctx, rtt.getConnLatency, attrOpt)
		m.dnsLatency.Record(ctx, rtt.dnsLatency, attrOpt)
		m.tlsLatency.Record(ctx, rtt.tlsLatency, attrOpt)
	}
}
