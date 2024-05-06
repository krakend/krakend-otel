package http

import (
	"context"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
)

// TraceRequestAttrs returns a list of attributes to be set
// for a given http.Request (only useful for traces, as
// it reports the url string with any variable parameter
// that it might contain).
func TraceRequestAttrs(r *http.Request) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 5)
	attrs = append(attrs,
		semconv.URLFull(r.URL.String()),
		semconv.ServerAddress(r.Host),
		semconv.ClientAddress(r.RemoteAddr),
		semconv.HTTPRequestMethodKey.String(r.Method),
	)

	if r.ContentLength >= 0 {
		attrs = append(attrs, semconv.HTTPRequestBodySize(int(r.ContentLength)))
	}

	userAgent := r.UserAgent()
	if userAgent != "" {
		attrs = append(attrs, semconv.UserAgentOriginal(userAgent))
	}

	return attrs
}

// TraceResponseAttrs returns a list of attributes to be set
// for a given http.Response.
func TraceResponseAttrs(resp *http.Response) []attribute.KeyValue {
	if resp == nil {
		return []attribute.KeyValue{}
	}
	return []attribute.KeyValue{
		semconv.HTTPResponseStatusCode(int(resp.StatusCode)),
		semconv.HTTPResponseBodySize(int(resp.ContentLength)),
	}
}

type customLabelerType int

const customLabelerKey customLabelerType = 0

// Labeler For a server plugin wanting to add custom metric attributes
func Labeler(ctx context.Context) (*otelhttp.Labeler, bool) {
	l, ok := ctx.Value(customLabelerKey).(*otelhttp.Labeler)
	if !ok {
		l = &otelhttp.Labeler{}
	}
	return l, ok
}

// InjectLabeler Set up the labeler in the context
func InjectLabeler(ctx context.Context) context.Context {
	labeler, found := Labeler(ctx)

	if !found {
		ctx = context.WithValue(ctx, customLabelerKey, labeler)
	}

	return ctx
}

// CustomMetricAttributes Get custom attributes set up for this request
func CustomMetricAttributes(r *http.Request) []attribute.KeyValue {
	labeler, _ := Labeler(r.Context())
	return labeler.Get()
}
