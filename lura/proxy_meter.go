package lura

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/luraproject/lura/v2/proxy"

	kotelconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/state"
)

type middlewareMeter struct {
	duration metric.Float64Histogram
	attrs    metric.MeasurementOption
}

func newMiddlewareMeter(s state.OTEL, stageName string, attrs []attribute.KeyValue) (*middlewareMeter, error) {
	if s == nil {
		return nil, errors.New("no OTEL state provided")
	}
	mAttrs := make([]attribute.KeyValue, 0, len(attrs))
	mAttrs = append(mAttrs, attrs...)

	meter := s.Meter()
	if meter == nil {
		return nil, errors.New("OTEL state returned nil meter")
	}
	var err error
	durationName := "krakend." + stageName + ".duration"
	duration, err := meter.Float64Histogram(durationName, kotelconfig.TimeBucketsOpt)
	if err != nil {
		return nil, err
	}
	return &middlewareMeter{
		duration: duration,
		attrs:    metric.WithAttributes(mAttrs...),
	}, nil
}

// multiError is an interface that is implemented by lura at the
// proxy level to gather errors for each of the backends
type multiError interface {
	error
	Errors() []error
}

func (m *middlewareMeter) report(ctx context.Context, secs float64, resp *proxy.Response, err error) {
	isErr := false
	isCanceled := false
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			isCanceled = true
		}
		if mErr, ok := err.(multiError); ok {
			errs := mErr.Errors()
			for _, e := range errs {
				if errors.Is(e, context.Canceled) {
					isCanceled = true
				} else {
					isErr = true
				}
			}
		}
		if !isCanceled {
			isErr = true
		}
	}
	metricDynAttrs := metric.WithAttributes(
		attribute.Bool("error", isErr),
		attribute.Bool("canceled", isCanceled),
		attribute.Bool("complete", resp != nil && resp.IsComplete))
	m.duration.Record(ctx, secs, m.attrs, metricDynAttrs)
}
