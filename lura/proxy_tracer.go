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
	skipHeaders   map[string]bool
	attrs         []attribute.KeyValue
}

func newMiddlewareTracer(s state.OTEL, name string, stageName string, reportHeaders bool,
	skipHeaders []string, attrs []attribute.KeyValue) *middlewareTracer {
	tracer := s.Tracer()
	if tracer == nil {
		return nil
	}
	tAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	tAttrs = append(tAttrs, attrs...)
	tAttrs = append(tAttrs, attribute.String("krakend.stage", stageName))
	var sh map[string]bool
	if len(skipHeaders) > 0 {
		sh = make(map[string]bool, len(skipHeaders))
		for _, k := range skipHeaders {
			sh[k] = true
		}
	}
	return &middlewareTracer{
		name:          name,
		tracer:        tracer,
		reportHeaders: reportHeaders,
		skipHeaders:   sh,
		attrs:         tAttrs,
	}
}

func (t *middlewareTracer) start(ctx context.Context, req *proxy.Request) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, t.name)
	span.SetAttributes(t.attrs...)
	if t.reportHeaders {
		for hk, hv := range req.Headers {
			if t.skipHeaders == nil || !t.skipHeaders[hk] {
				span.SetAttributes(attribute.StringSlice("http.request.header."+strings.ToLower(hk), hv))
			}
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
				if t.skipHeaders == nil || !t.skipHeaders[hk] {
					span.SetAttributes(attribute.StringSlice("http.response.header."+strings.ToLower(hk), hv))
				}
			}
		}
	}
	span.SetAttributes(attribute.Bool("complete", resp != nil && resp.IsComplete))
	span.End()
}
