package server

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	otelhttp "github.com/krakend/krakend-otel/http"
)

type tracesHTTP struct {
	tracer        trace.Tracer
	fixedAttrs    []attribute.KeyValue
	reportHeaders bool
}

func newTracesHTTP(tracer trace.Tracer, attrs []attribute.KeyValue, reportHeaders bool) *tracesHTTP {
	var fa []attribute.KeyValue
	if len(attrs) > 0 {
		fa = make([]attribute.KeyValue, len(attrs))
		copy(fa, attrs)
	}
	return &tracesHTTP{
		tracer:        tracer,
		fixedAttrs:    fa,
		reportHeaders: reportHeaders,
	}
}

// start starts a trace from using the handlerTracking information provided.
//
// When traces are disabled, the tracer will be nil.
func (t *tracesHTTP) start(r *http.Request, tr *tracking) *http.Request {
	if t == nil || t.tracer == nil || r.URL == nil {
		return r
	}
	tr.ctx, tr.span = t.tracer.Start(r.Context(), r.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
	r = r.WithContext(tr.ctx)
	attrs := otelhttp.TraceRequestAttrs(r)
	tr.span.SetAttributes(attrs...)
	if len(t.fixedAttrs) > 0 {
		tr.span.SetAttributes(t.fixedAttrs...)
	}
	if t.reportHeaders {
		// report all incoming headers
		for hk, hv := range r.Header {
			tr.span.SetAttributes(attribute.StringSlice("http.request.header."+strings.ToLower(hk), hv))
		}
	}
	return r
}

// end finishes the span started and tracked using the [handlerTracking] info.
func (t *tracesHTTP) end(tr *tracking) {
	if t == nil || tr.span == nil || !tr.span.IsRecording() {
		return
	}

	if tr.isHijacked {
		tr.span.SetAttributes(attribute.Bool("http.connection.hijacked", true))
		if tr.hijackedErr != nil {
			tr.span.SetAttributes(attribute.String("http.connection.error", tr.hijackedErr.Error()))
		}
	}

	tr.span.SetAttributes(
		semconv.HTTPRoute(tr.EndpointPattern()),
		semconv.HTTPResponseStatusCode(tr.responseStatus),
		semconv.HTTPResponseBodySize(tr.responseSize))
	tr.span.SetAttributes(tr.tracesStaticAttrs...)

	if tr.responseHeaders != nil {
		// report all incoming headers
		for hk, hv := range tr.responseHeaders {
			tr.span.SetAttributes(attribute.StringSlice("http.response.header."+strings.ToLower(hk), hv))
		}
	}
	if len(tr.writeErrs) > 0 {
		e := tr.writeErrs[0]
		tr.span.RecordError(e)
		tr.span.SetStatus(codes.Error, e.Error())
	} else {
		tr.span.SetStatus(codes.Ok, "")
	}
	tr.span.End()
}
