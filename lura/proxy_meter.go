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
	mAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, sa := range attrs {
		mAttrs = append(mAttrs, sa)
	}

	meter := s.Meter()
	var err error
	durationName := "krakend." + stageName + ".duration"
	duration, err := meter.Float64Histogram(durationName, kotelconfig.TimeBucketsOpt)
	if err != nil {
		return nil, err
	}
	return &middlewareMeter{
		duration: duration,
		attrs:    metric.WithAttributes(attrs...),
	}, nil
}

func (m *middlewareMeter) report(ctx context.Context, secs float64, resp *proxy.Response, err error) {
	isErr := false
	isCanceled := false
	if err != nil {
		if errors.Is(err, context.Canceled) {
			isCanceled = true
		} else {
			isErr = true
		}
	}
	metricDynAttrs := metric.WithAttributes(
		attribute.Bool("error", isErr),
		attribute.Bool("canceled", isCanceled),
		attribute.Bool("complete", resp != nil && resp.IsComplete))
	m.duration.Record(ctx, secs, m.attrs, metricDynAttrs)
}
