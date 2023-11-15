package client

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// roundTripTracking holds all the data to report
// for a round trip: some data will be used only
// for metrics, some for traces, and some for both.
type roundTripTracking struct {
	req  *http.Request
	resp *http.Response

	span trace.Span

	latencyInSecs float64
	err           error

	getConnStart   time.Time
	getConnLatency float64

	firstByteTime time.Time // reported as an span event

	dnsStart   time.Time
	dnsLatency float64

	tlsStart   time.Time
	tlsLatency float64
}

// withClientTrace may be set to a function allowing the current *trace.Span
// to be annotated with HTTP request event information emitted by the
// httptrace package.
func (t *roundTripTracking) withClientTrace() {
	// Commented out are the callback calls that we are not interested in:
	httpTrace := &httptrace.ClientTrace{
		GetConn: t.GetConn,
		GotConn: t.GotConn,
		// PutIdleConn:          t.PutIdleConn,
		GotFirstResponseByte: t.GotFirstResponseByte,
		// Got100Continue:       t.Got100Continue,
		// Got1xxResponse:       t.Got1xxResponse,
		DNSStart: t.DNSStart,
		DNSDone:  t.DNSDone,
		// ConnectStart:      t.ConnectStart,
		// ConnectDone:       t.ConnectDone,
		TLSHandshakeStart: t.TLSHandshakeStart,
		TLSHandshakeDone:  t.TLSHandshakeDone,
		// WroteHeaderField:  t.WroteHeaderField,
		// WroteHeaders: t.WroteHeaders,
		// Wait100Continue:   t.Wait100Continue,
		// WroteRequest: t.WroteRequest,
	}
	t.req = t.req.WithContext(httptrace.WithClientTrace(t.req.Context(), httpTrace))
}

// GetConn is called before a connection is created or
// retrieved from an idle pool. The hostPort is the
// "host:port" of the target or proxy. GetConn is called even
// if there's already an idle cached connection available.
func (t *roundTripTracking) GetConn(hostPort string) {
	t.getConnStart = time.Now()
}

// GotConn is called after a successful connection is
// obtained. There is no hook for failure to obtain a
// connection; instead, use the error from
// Transport.RoundTrip.
func (t *roundTripTracking) GotConn(info httptrace.GotConnInfo) {
	t.getConnLatency = float64(time.Since(t.getConnStart)) / float64(time.Second)
}

// PutIdleConn is called when the connection is returned to
// the idle pool. If err is nil, the connection was
// successfully returned to the idle pool. If err is non-nil,
// it describes why not. PutIdleConn is not called if
// connection reuse is disabled via Transport.DisableKeepAlives.
// PutIdleConn is called before the caller's Response.Body.Close
// call returns.
// For HTTP/2, this hook is not currently used.
func (t *roundTripTracking) PutIdleConn(err error) {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// GotFirstResponseByte is called when the first byte of the response
// headers is available.
func (t *roundTripTracking) GotFirstResponseByte() {
	t.firstByteTime = time.Now()
}

// Got100Continue is called if the server replies with a "100
// Continue" response.
func (t *roundTripTracking) Got100Continue() {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// Got1xxResponse is called for each 1xx informational response header
// returned before the final non-1xx response. Got1xxResponse is called
// for "100 Continue" responses, even if Got100Continue is also defined.
// If it returns an error, the client request is aborted with that error value.
func (t *roundTripTracking) Got1xxResponse(code int, header textproto.MIMEHeader) error {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
	return nil
}

// DNSStart is called when a DNS lookup begins.
func (t *roundTripTracking) DNSStart(httptrace.DNSStartInfo) {
	t.dnsStart = time.Now()
}

// DNSDone is called when a DNS lookup ends.
func (t *roundTripTracking) DNSDone(httptrace.DNSDoneInfo) {
	t.dnsLatency = float64(time.Since(t.dnsStart)) / float64(time.Second)
}

// ConnectStart is called when a new connection's Dial begins.
// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (t *roundTripTracking) ConnectStart(network, addr string) {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// ConnectDone is called when a new connection's Dial
// completes. The provided err indicates whether the
// connection completed successfully.
// If net.Dialer.DualStack ("Happy Eyeballs") support is
// enabled, this may be called multiple times.
func (t *roundTripTracking) ConnectDone(network, addr string, err error) {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// TLSHandshakeStart is called when the TLS handshake is started. When
// connecting to an HTTPS site via an HTTP proxy, the handshake happens
// after the CONNECT request is processed by the proxy.
func (t *roundTripTracking) TLSHandshakeStart() {
	t.tlsStart = time.Now()
}

// TLSHandshakeDone is called after the TLS handshake with either the
// successful handshake's connection state, or a non-nil error on handshake
// failure.
func (t *roundTripTracking) TLSHandshakeDone(tls.ConnectionState, error) {
	t.tlsLatency = float64(time.Since(t.tlsStart)) / float64(time.Second)
}

// WroteHeaderField is called after the Transport has written
// each request header. At the time of this call the values
// might be buffered and not yet written to the network.
func (t *roundTripTracking) WroteHeaderField(key string, value []string) {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// WroteHeaders is called after the Transport has written
// all request headers.
func (t *roundTripTracking) WroteHeaders() {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// Wait100Continue is called if the Request specified
// "Expect: 100-continue" and the Transport has written the
// request headers but is waiting for "100 Continue" from the
// server before writing the request body.
func (t *roundTripTracking) Wait100Continue() {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}

// WroteRequest is called with the result of writing the
// request and any body. It may be called multiple times
// in the case of retried requests.
func (t *roundTripTracking) WroteRequest(httptrace.WroteRequestInfo) {
	// we are not interested in this metric, but we leave the function
	// definition to make clear that leaving this out has been
	// a decision
}
