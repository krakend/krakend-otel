// Package otelio implements the instrumentation around the
// io.Reader and io.Writer.
package otelio

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	kotelconfig "github.com/krakend/krakend-otel/config"
)

// instruments holds the instruments for the transfer of
// information (either through a Reader or a Writer):
// - size (in bytes)
// - time (in seconds)
// - error (if an error happened).
type instruments struct {
	metricFixedAttrs []attribute.KeyValue
	traceFixedAttrs  []attribute.KeyValue

	sizeCount     metric.Int64Counter
	sizeHistogram metric.Int64Histogram
	timeCount     metric.Float64Counter
	timeHistogram metric.Float64Histogram
	errorsMeter   metric.Int64Counter // error rate when doing i/o

	tracer            trace.Tracer
	traceSizeAttrName string
	traceTimeAttrName string
	traceName         string

	// attribute sets to be reused, and not computed in the
	// critical path
	metricAttributeSetOpt          metric.MeasurementOption
	metricAttributeSetWithErrorOpt metric.MeasurementOption
}

// newInstruments holds the information for an i/o operation, no mater
// if is for reading or writing.
func newInstruments(prefix string,
	attrT []attribute.KeyValue, attrM []attribute.KeyValue,
	tracer trace.Tracer, meter metric.Meter,
) *instruments {
	if prefix == "" {
		prefix = "io."
	}
	strSizeCount := fmt.Sprintf("%ssize", prefix)
	strSizeHistogram := fmt.Sprintf("%ssize-hist", prefix)
	strTimeCount := fmt.Sprintf("%stime", prefix)
	strTimeHistogram := fmt.Sprintf("%stime-hist", prefix)
	strErrorsMeter := fmt.Sprintf("%serrors", prefix)

	nopMProvider := noopmetric.NewMeterProvider()
	nopM := nopMProvider.Meter(fmt.Sprintf("%snop-tracker", prefix))
	sizeCount, _ := nopM.Int64Counter(strSizeCount)
	sizeHistogram, _ := nopM.Int64Histogram(strSizeHistogram, kotelconfig.SizeBucketsOpt)
	timeCount, _ := nopM.Float64Counter(strTimeCount)
	timeHistogram, _ := nopM.Float64Histogram(strTimeHistogram, kotelconfig.TimeBucketsOpt)
	errorsMeter, _ := nopM.Int64Counter(strErrorsMeter)

	if meter != nil {
		if bsc, err := meter.Int64Counter(strSizeCount, metric.WithUnit("b")); err == nil {
			sizeCount = bsc
		}
		if bsh, err := meter.Int64Histogram(strSizeHistogram, kotelconfig.SizeBucketsOpt, metric.WithUnit("b")); err == nil {
			sizeHistogram = bsh
		}
		if brtc, err := meter.Float64Counter(strTimeCount, metric.WithUnit("s")); err == nil {
			timeCount = brtc
		}
		if brth, err := meter.Float64Histogram(strTimeHistogram, kotelconfig.TimeBucketsOpt, metric.WithUnit("s")); err == nil {
			timeHistogram = brth
		}
		if re, err := meter.Int64Counter(strErrorsMeter); err == nil {
			errorsMeter = re
		}
	}

	var metricFixedAttrs []attribute.KeyValue
	if len(attrM) > 0 {
		metricFixedAttrs = make([]attribute.KeyValue, len(attrM))
		copy(metricFixedAttrs, attrM)
	}
	var traceFixedAttrs []attribute.KeyValue
	if len(attrT) > 0 {
		traceFixedAttrs = make([]attribute.KeyValue, len(attrT))
		copy(traceFixedAttrs, attrT)
	}

	// precompute the metric options with its attributes
	l := len(metricFixedAttrs)
	metricAttrsWithErr := make([]attribute.KeyValue, l+1)
	copy(metricAttrsWithErr, metricFixedAttrs)
	metricAttrsWithErr[l] = attribute.String("error", "true")

	attrSet := attribute.NewSet(metricFixedAttrs...)
	attrSetWithErr := attribute.NewSet(metricAttrsWithErr...)

	if tracer == nil {
		tracer = nooptrace.NewTracerProvider().Tracer("read-tracer")
	}

	return &instruments{
		metricFixedAttrs:               metricFixedAttrs,
		traceFixedAttrs:                traceFixedAttrs,
		sizeCount:                      sizeCount,
		sizeHistogram:                  sizeHistogram,
		timeCount:                      timeCount,
		timeHistogram:                  timeHistogram,
		errorsMeter:                    errorsMeter,
		tracer:                         tracer,
		traceSizeAttrName:              prefix + "size",
		traceTimeAttrName:              prefix + "time",
		traceName:                      prefix + "tracker",
		metricAttributeSetOpt:          metric.WithAttributeSet(attrSet),
		metricAttributeSetWithErrorOpt: metric.WithAttributeSet(attrSetWithErr),
	}
}
