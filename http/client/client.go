// Package client provides the instrumentation for an http client.
//
// The detail that metrics and traces provides can be configured
// with the [TransportOptions] struct.
//
// Metrics:
//
//	Tags for all metrics:
//	- Host
//	- URLPattern
//	- Name (the name might be the combination of the endpoint + url pattern)
//
//	Requests:
//
//	- num cancelled requests:
//	    - per context timeout
//	    - per caller cancellation (for example when sending to multiple hosts)
//	- content-lenght: the size of the payload to be sent
//
//	Responses:
//	    Tags for all responses:
//	        - Status Code
//
//	- content length: the server side provided content length
//	- latency: the time since we send the request, until we have the
//	    response available (might or might not included the time for
//	    buffereing part or all the payload).
//
//	- response bytes read: using the body tracker, account for the actual
//	    number of bytes read by the client.
//	- response reading time: measures the time spent
//
// Traces:
package client

import (
	"net/http"

	"github.com/krakend/krakend-otel/state"
)

// InstrumentedHTTPClient creates a new instrumented http client with the options provided.
// If the provided options are nil, the default options (with everything enabled) will be used.
// Check the [TransportOptions] struct for details..
func InstrumentedHTTPClient(c *http.Client, t *TransportOptions, clientName string) (*http.Client, error) {
	// the default options is to have everything activated
	// TODO: we might want to return the provided client if no transport options are provided ?
	// TODO: we might want to return an error if no transport options are provided ?
	opts := TransportOptions{
		MetricsOpts: TransportMetricsOptions{
			RoundTrip:   true,
			ReadPayload: true,
		},
		TracesOpts: TransportTracesOptions{
			RoundTrip:   true,
			ReadPayload: true,
		},
		OTELInstance: state.GlobalState,
	}
	if t != nil {
		opts = *t
	}

	transport := http.DefaultTransport
	if c.Transport != nil {
		transport = c.Transport
	}

	wc := &http.Client{
		Transport: NewRoundTripper(transport, opts.MetricsOpts, opts.TracesOpts,
			clientName, t.OTELInstance),
		CheckRedirect: c.CheckRedirect,
		Jar:           c.Jar,
		Timeout:       c.Timeout,
	}
	return wc, nil
}
