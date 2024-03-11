package lura

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/luraproject/lura/v2/proxy"

	"github.com/krakend/krakend-otel/state"
)

type middlewareTracer struct {
	name          string
	tracer        trace.Tracer
	reportHeaders bool
	attrs         []attribute.KeyValue
}

func newMiddlewareTracer(s state.OTEL, name string, stageName string, reportHeaders bool, attrs []attribute.KeyValue) *middlewareTracer {
	tracer := s.Tracer()
	tAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	tAttrs = append(tAttrs, attrs...)
	tAttrs = append(tAttrs, attribute.String("krakend.stage", stageName))
	return &middlewareTracer{
		name:          name,
		tracer:        tracer,
		reportHeaders: reportHeaders,
		attrs:         tAttrs,
	}
}

func (t *middlewareTracer) start(ctx context.Context, req *proxy.Request) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, t.name)
	span.SetAttributes(t.attrs...)
	if t.reportHeaders {
		for hk, hv := range req.Headers {
			span.SetAttributes(attribute.StringSlice("http.request.header."+strings.ToLower(hk), hv))
		}
	}
	return ctx, span
}

func (t *middlewareTracer) end(span trace.Span, resp *proxy.Response, err error) {
	if err != nil {
		if errors.Is(err, context.Canceled) {
			span.SetAttributes(attribute.Bool("canceled", true))
		} else {
			span.SetAttributes(attribute.String("error", err.Error()))
		}
		span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(500))
	} else if resp != nil {
		span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(resp.Metadata.StatusCode))
		if t.reportHeaders {
			for hk, hv := range resp.Metadata.Headers {
				span.SetAttributes(attribute.StringSlice("http.response.header."+strings.ToLower(hk), hv))
			}
		}
	}
	span.SetAttributes(attribute.Bool("complete", resp != nil && resp.IsComplete))
	span.End()
}
