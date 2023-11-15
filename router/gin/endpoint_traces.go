package gin

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	otelhttp "github.com/krakend/krakend-otel/http"
	"github.com/krakend/krakend-otel/state"
)

type ginTracesOptions struct {
	DisablePropagation bool
	FixedAttributes    []attribute.KeyValue // "static" attributes set at config time.
}

type ginTraces struct {
	tracer     trace.Tracer
	endpoint   string
	prop       propagation.TextMapPropagator
	fixedAttrs []attribute.KeyValue
}

func newGinTraces(tracesOpts *ginTracesOptions, tracer trace.Tracer,
	endpoint string, prop propagation.TextMapPropagator) *ginTraces {

	if tracesOpts.DisablePropagation {
		prop = nil
	}
	var fa []attribute.KeyValue
	if len(tracesOpts.FixedAttributes) > 0 {
		fa := make([]attribute.KeyValue, len(tracesOpts.FixedAttributes))
		copy(fa, tracesOpts.FixedAttributes)
	}
	return &ginTraces{
		tracer:     tracer,
		endpoint:   endpoint,
		prop:       prop,
		fixedAttrs: fa,
	}
}

// start starts a trace from using the handlerTracking information provided.
//
// When traces are disabled, the tracer will be nil.
func (t *ginTraces) start(c *gin.Context, ht *handlerTracking) {
	if t == nil || t.tracer == nil {
		return
	}

	reqCtx := c.Request.Context()
	if t.prop != nil {
		// trace propagation is not disabled
		reqCtx = t.prop.Extract(reqCtx, propagation.HeaderCarrier(c.Request.Header))
	}
	// Setting the context for the underlying request works for Lura
	// because the Fallback option in gin Engine is set, but just in case
	// it was disabled, we have the option to use `state.KrakenDContextOTELStrKey`
	// for the gin context:
	ht.ctx, ht.span = t.tracer.Start(reqCtx, t.endpoint)
	c.Request = c.Request.WithContext(ht.ctx)
	c.Set(state.KrakenDContextOTELStrKey, ht.span)

	attrs := otelhttp.TraceRequestAttrs(c.Request)
	ht.span.SetAttributes(attrs...)

	if len(t.fixedAttrs) > 0 {
		ht.span.SetAttributes(t.fixedAttrs...)
	}
}

// end finishes the span started and tracked using the [handlerTracking] info.
func (t *ginTraces) end(ht *handlerTracking) {
	if t == nil || ht.span == nil || !ht.span.IsRecording() {
		return
	}
	/*
		if ht.err != nil {
			// TODO: check what options we might add
			ht.span.RecordError(ht.err)
			ht.span.SetStatus(codes.Error, ht.err.Error())
		}
	*/
	ht.span.SetAttributes(
		semconv.HTTPResponseStatusCode(ht.responseStatus),
		semconv.HTTPResponseBodySize(ht.responseSize))
	ht.span.End()
}
