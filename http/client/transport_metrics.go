package client

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	v127 "go.opentelemetry.io/otel/semconv/v1.27.0"

	kotelconfig "github.com/krakend/krakend-otel/config"
)

// TransportMetricsOptions contains the options to enable / disable
// for reporting metrics, and a set of fixed attributes to add
// to all metrics.
type TransportMetricsOptions struct {
	RoundTrip          bool                 // provide the round trip metrics
	ReadPayload        bool                 // provide metrics for the reading the full body
	DetailedConnection bool                 // provide detailed metrics about the connection: dns lookup, tls ...
	FixedAttributes    []attribute.KeyValue // "static" attributes set at config time.
	SemConv            string               // to use the latest metric names conventions
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
	requestContentLength     metric.Int64Counter
	requestContentLengthHist metric.Int64Histogram

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

type metricFillerFn func(*TransportMetricsOptions, metric.Meter, *transportMetrics)

func noSemConvMetricsFiller(metricsOpts *TransportMetricsOptions, meter metric.Meter, tm *transportMetrics) {
	nopMeter := noop.Meter{}

	tm.requestsStarted, _ = meter.Int64Counter("http.client.request.started.count")   // number of reqs started
	tm.requestsFailed, _ = meter.Int64Counter("http.client.request.failed.count")     // number of reqs failed
	tm.requestsCanceled, _ = meter.Int64Counter("http.client.request.canceled.count") // number of canceled requests
	tm.requestsTimedOut, _ = meter.Int64Counter("http.client.request.timedout.count") // numer of timedout request (inclued in failed)

	tm.requestContentLength, _ = meter.Int64Counter("http.client.request.size") // the value of the Content-Length header for the request
	// For non implemented metrics that can be used with other config, we use a noop meter:
	tm.requestContentLengthHist, _ = nopMeter.Int64Histogram(v127.HTTPClientRequestBodySizeName)

	tm.responseLatency, _ = meter.Float64Histogram("http.client.duration", kotelconfig.TimeBucketsOpt)
	tm.responseContentLength, _ = meter.Int64Histogram("http.client.response.size", kotelconfig.SizeBucketsOpt)
	tm.responseNoContentLength, _ = meter.Int64Counter("http.client.response.no-content-length")

	tm.detailsEnabled = metricsOpts.DetailedConnection
	tm.getConnLatency, _ = meter.Float64Histogram("http.client.request.get-conn.duration", kotelconfig.TimeBucketsOpt)
	tm.dnsLatency, _ = meter.Float64Histogram("http.client.request.dns.duration", kotelconfig.TimeBucketsOpt)
	tm.tlsLatency, _ = meter.Float64Histogram("http.client.request.tls.duration", kotelconfig.TimeBucketsOpt)
}

func semConv1_27MetricsFiller(metricsOpts *TransportMetricsOptions, meter metric.Meter, tm *transportMetrics) {
	nopMeter := noop.Meter{}

	tm.detailsEnabled = metricsOpts.DetailedConnection

	// For non implemented metrics that can be used with other config, we use a noop meter:
	tm.requestContentLength, _ = nopMeter.Int64Counter("http.client.request.size")

	// WARNING: Stability => Experimental (subject to change in the future)
	tm.requestContentLengthHist, _ = meter.Int64Histogram(v127.HTTPClientRequestBodySizeName,
		metric.WithUnit(v127.HTTPClientRequestBodySizeUnit),
		metric.WithDescription(v127.HTTPClientRequestBodySizeDescription),
		kotelconfig.SizeBucketsOpt) // the value of the Content-Length header for the request

	// this is `http.client.request.duration`, and is a required metric:
	// https://opentelemetry.io/docs/specs/semconv/http/http-metrics/#metric-httpclientrequestduration
	tm.responseLatency, _ = meter.Float64Histogram(v127.HTTPClientRequestDurationName,
		metric.WithUnit(v127.HTTPClientRequestDurationUnit),
		metric.WithDescription(v127.HTTPClientRequestDurationDescription),
		kotelconfig.TimeBucketsOpt)

	// WARNING: Stability => Experimental (subject to change in the future)
	tm.responseContentLength, _ = meter.Int64Histogram(v127.HTTPClientResponseBodySizeName,
		metric.WithUnit(v127.HTTPClientResponseBodySizeUnit),
		metric.WithDescription(v127.HTTPClientResponseBodySizeDescription),
		kotelconfig.SizeBucketsOpt) // the value of the Content-Length header for the response

	// TODO: if we want the exact semantic convention, what should we do with our own extra data, that
	// is not standarized by OTEL ? for now we only set it if the `metricsOpts.DetailedConnection` is
	// set too:
	if metricsOpts.DetailedConnection {
		tm.requestsStarted, _ = meter.Int64Counter("http.client.request.started.count")   // number of reqs started
		tm.requestsFailed, _ = meter.Int64Counter("http.client.request.failed.count")     // number of reqs failed
		tm.requestsCanceled, _ = meter.Int64Counter("http.client.request.canceled.count") // number of canceled requests
		tm.requestsTimedOut, _ = meter.Int64Counter("http.client.request.timedout.count") // numer of timedout request (inclued in failed)

		tm.responseNoContentLength, _ = meter.Int64Counter("http.client.response.no-content-length",
			metric.WithUnit(v127.HTTPClientResponseBodySizeUnit),
			metric.WithDescription("Client received responses that do not have 'Content-Length' value set"))
		tm.getConnLatency, _ = meter.Float64Histogram("http.client.request.get-conn.duration",
			metric.WithUnit(v127.HTTPClientRequestDurationUnit),
			metric.WithDescription("Time spent acquiring a client connection"),
			kotelconfig.TimeBucketsOpt)
		tm.dnsLatency, _ = meter.Float64Histogram("http.client.request.dns.duration",
			metric.WithUnit(v127.HTTPClientRequestDurationUnit),
			metric.WithDescription("Time spent resolving the DNS name"),
			kotelconfig.TimeBucketsOpt)
		tm.tlsLatency, _ = meter.Float64Histogram("http.client.request.tls.duration",
			metric.WithUnit(v127.HTTPClientRequestDurationUnit),
			metric.WithDescription("Time spent on TLS negotiation and connection"),
			kotelconfig.TimeBucketsOpt)
		return
	}
	tm.requestsStarted, _ = nopMeter.Int64Counter("http.client.request.started.count")   // number of reqs started
	tm.requestsFailed, _ = nopMeter.Int64Counter("http.client.request.failed.count")     // number of reqs failed
	tm.requestsCanceled, _ = nopMeter.Int64Counter("http.client.request.canceled.count") // number of canceled requests
	tm.requestsTimedOut, _ = nopMeter.Int64Counter("http.client.request.timedout.count") // numer of timedout request (inclued in failed)

	tm.responseNoContentLength, _ = nopMeter.Int64Counter("http.client.response.no-content-length")
	tm.getConnLatency, _ = nopMeter.Float64Histogram("http.client.request.get-conn.duration")
	tm.dnsLatency, _ = nopMeter.Float64Histogram("http.client.request.dns.duration")
	tm.tlsLatency, _ = nopMeter.Float64Histogram("http.client.request.tls.duration")
}

func newTransportMetrics(metricsOpts *TransportMetricsOptions, meter metric.Meter, clientName string) *transportMetrics {
	if meter == nil {
		return nil
	}

	supportedSemConv := map[string]metricFillerFn{
		"":     noSemConvMetricsFiller,
		"1.27": semConv1_27MetricsFiller,
	}
	tm := transportMetrics{
		clientName: clientName,
	}
	filler := noSemConvMetricsFiller
	if versionFiller, ok := supportedSemConv[metricsOpts.SemConv]; ok {
		filler = versionFiller
	}
	filler(metricsOpts, meter, &tm)
	return &tm
}

func (m *transportMetrics) report(rtt *roundTripTracking, attrs []attribute.KeyValue) {
	if m == nil || m.requestsStarted == nil {
		return
	}
	attrOpt := m.attributesOption(rtt, attrs)
	ctx := rtt.req.Context()

	m.requestsStarted.Add(ctx, 1, attrOpt)
	if rtt.req.ContentLength >= 0 {
		m.requestContentLength.Add(ctx, rtt.req.ContentLength, attrOpt)
		// the content-length is optional and experimental for 1.27 and also 1.29 sem conv:
		// https://opentelemetry.io/docs/specs/semconv/http/http-metrics/#metric-httpclientrequestbodysize
		m.requestContentLengthHist.Record(ctx, rtt.req.ContentLength, attrOpt)
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

	// the `http.client.request.duration` is required for semconv 1.27 and 1.29
	m.responseLatency.Record(ctx, rtt.latencyInSecs, attrOpt)

	if rtt.req.Method != "HEAD" && rtt.resp != nil {
		if rtt.resp.ContentLength >= 0 {
			// it might be the case were we receive a chunked response, and then
			// we will not record a metric for it.
			// the `http.client.response.body.size` is optional and experimental
			// for semconv 1.27 and 1.29
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

func (m *transportMetrics) attributesOption(rtt *roundTripTracking, attrs []attribute.KeyValue) metric.MeasurementOption {
	numAttrs := len(attrs) + 4 + 1 // static attrs + required attributes + clientname
	attrM := make([]attribute.KeyValue, len(attrs), numAttrs)
	copy(attrM, attrs)
	if len(m.clientName) > 0 {
		attrM = append(attrM, attribute.Key("clientname").String(m.clientName))
	}

	serverAddress, serverPort := requestServerAndPort(rtt.req)

	statusCode := 0
	if rtt.err == nil {
		// if we fail on the client side, we do not have a status code, but we
		// want it set to 0 to be displayed on the dashboard
		statusCode = int(rtt.resp.StatusCode)
	}

	attrM = append(attrM,
		semconv.HTTPRequestMethodKey.String(rtt.req.Method), // required
		// this is wrong, as the RemoteAddr is ignored when the request is used
		// for a client (is filled by the server side):
		// semconv.ServerAddress(rtt.req.RemoteAddr),
		semconv.ServerAddress(serverAddress),       // required by sem conv 1.29
		semconv.ServerPort(serverPort),             // required by sem conv 1.29
		semconv.HTTPResponseStatusCode(statusCode), // required if received
	)
	return metric.WithAttributeSet(attribute.NewSet(attrM...))
}

func requestServerAndPort(r *http.Request) (string, int) {
	serverAddress := r.Host
	serverPort := int(80)
	if r.URL != nil {
		serverAddress = r.URL.Hostname()
		strPort := r.URL.Port()
		if p, err := strconv.ParseInt(strPort, 10, 32); err != nil {
			serverPort = int(p)
		}
	}
	return serverAddress, serverPort
}
